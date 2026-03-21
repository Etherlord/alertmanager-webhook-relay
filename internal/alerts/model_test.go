package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertStatus_Constants(t *testing.T) {
	assert.Equal(t, AlertStatus("firing"), StatusFiring)
	assert.Equal(t, AlertStatus("resolved"), StatusResolved)
}

func TestAlertGroup_UnmarshalJSON_SingleAlert(t *testing.T) {
	// .scripts/alert-samples/01 — one firing alert
	data := loadSample(t, "01_2026-03-16T20-22-58_plus_04-00_LogErrors.json")

	var group AlertGroup
	err := json.Unmarshal(data, &group)
	require.NoError(t, err)

	assert.Equal(t, "telegram", group.Receiver)
	assert.Equal(t, StatusFiring, group.Status)
	assert.Equal(t, "4", group.Version)
	assert.Equal(t, `{}/{severity=~"critical|warning|major"}:{alertname="LogErrors"}`, group.GroupKey)
	assert.Equal(t, 0, group.TruncatedAlerts)
	assert.Equal(t, "http://vmalertmanager-alertmanager-1:9093", group.ExternalURL)

	// GroupLabels
	assert.Equal(t, "LogErrors", group.GroupLabels["alertname"])

	// CommonLabels
	assert.Equal(t, "warning", group.CommonLabels["severity"])
	assert.Equal(t, "LogErrors", group.CommonLabels["alertname"])

	// CommonAnnotations
	assert.Contains(t, group.CommonAnnotations["description"], "too many errors")

	// Single alert
	require.Len(t, group.Alerts, 1)
	alert := group.Alerts[0]
	assert.Equal(t, StatusFiring, alert.Status)
	assert.Equal(t, "b76d9da35df35672", alert.Fingerprint)
	assert.Equal(t, "LogErrors", alert.Labels["alertname"])
	assert.Equal(t, "warning", alert.Labels["severity"])
	assert.Contains(t, alert.Annotations["summary"], "Too many errors")
	assert.False(t, alert.StartsAt.IsZero())
	assert.Contains(t, alert.GeneratorURL, "vmalert")

	t.Logf("parsed single-alert group: receiver=%s, status=%s, alerts=%d",
		group.Receiver, group.Status, len(group.Alerts))
}

func TestAlertGroup_UnmarshalJSON_MultipleAlerts(t *testing.T) {
	// .scripts/alert-samples/06 — three alerts (2 firing + 1 resolved)
	data := loadSample(t, "06_2026-03-16T20-22-58_plus_04-00_ServiceDown.json")

	var group AlertGroup
	err := json.Unmarshal(data, &group)
	require.NoError(t, err)

	assert.Equal(t, StatusFiring, group.Status)
	require.Len(t, group.Alerts, 3)

	// Check mixed statuses
	statuses := make(map[AlertStatus]int)
	for _, a := range group.Alerts {
		statuses[a.Status]++
	}
	assert.Equal(t, 2, statuses[StatusFiring])
	assert.Equal(t, 1, statuses[StatusResolved])

	// Resolved alert should have non-zero EndsAt
	for _, a := range group.Alerts {
		if a.Status == StatusResolved {
			assert.False(t, a.EndsAt.IsZero(), "resolved alert must have non-zero EndsAt")
		}
	}

	// Empty commonAnnotations
	assert.Empty(t, group.CommonAnnotations)

	t.Logf("parsed multi-alert group: alerts=%d, firing=%d, resolved=%d",
		len(group.Alerts), statuses[StatusFiring], statuses[StatusResolved])
}

func TestAlertGroup_UnmarshalJSON_AllSamples(t *testing.T) {
	samplesDir := filepath.Join("..", "..", ".scripts", "alert-samples")
	entries, err := os.ReadDir(samplesDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "no sample files found")

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(samplesDir, entry.Name()))
			require.NoError(t, err)

			var group AlertGroup
			err = json.Unmarshal(data, &group)
			require.NoError(t, err)

			// Common invariants
			assert.NotEmpty(t, group.Receiver, "receiver must be set")
			assert.NotEmpty(t, group.Status, "status must be set")
			assert.Equal(t, "4", group.Version, "version must be 4")
			assert.NotEmpty(t, group.GroupKey, "groupKey must be set")
			assert.NotEmpty(t, group.Alerts, "alerts must be non-empty")

			for i, a := range group.Alerts {
				assert.NotEmpty(t, a.Status, "alert[%d].status must be set", i)
				assert.NotEmpty(t, a.Fingerprint, "alert[%d].fingerprint must be set", i)
				assert.NotEmpty(t, a.Labels["alertname"], "alert[%d].labels.alertname must be set", i)
				assert.False(t, a.StartsAt.IsZero(), "alert[%d].startsAt must be set", i)
			}

			t.Logf("sample %s: receiver=%s, status=%s, alerts=%d",
				entry.Name(), group.Receiver, group.Status, len(group.Alerts))
		})
	}
}

func TestAlertGroup_MarshalJSON_RoundTrip(t *testing.T) {
	original := AlertGroup{
		Receiver: "webhook",
		Status:   StatusFiring,
		Alerts: []Alert{
			{
				Status:      StatusFiring,
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations: map[string]string{"summary": "test"},
				StartsAt:    time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC),
				EndsAt:      time.Time{},
				GeneratorURL: "http://example.com",
				Fingerprint: "abc123",
			},
		},
		GroupLabels:       map[string]string{"alertname": "TestAlert"},
		CommonLabels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
		CommonAnnotations: map[string]string{"summary": "test"},
		ExternalURL:       "http://alertmanager:9093",
		Version:           "4",
		GroupKey:           "test-group-key",
		TruncatedAlerts:   0,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded AlertGroup
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Receiver, decoded.Receiver)
	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, original.Version, decoded.Version)
	assert.Equal(t, original.GroupKey, decoded.GroupKey)
	assert.Equal(t, original.ExternalURL, decoded.ExternalURL)
	require.Len(t, decoded.Alerts, 1)
	assert.Equal(t, original.Alerts[0].Fingerprint, decoded.Alerts[0].Fingerprint)
	assert.Equal(t, original.Alerts[0].Labels, decoded.Alerts[0].Labels)

	t.Logf("round-trip: marshal=%d bytes, receiver=%s, alerts=%d",
		len(data), decoded.Receiver, len(decoded.Alerts))
}

func TestAlert_ZeroEndsAt(t *testing.T) {
	raw := `{
		"status": "firing",
		"labels": {"alertname": "Test"},
		"annotations": {},
		"startsAt": "2026-03-16T08:00:00Z",
		"endsAt": "0001-01-01T00:00:00Z",
		"generatorURL": "http://example.com",
		"fingerprint": "abc123"
	}`

	var alert Alert
	err := json.Unmarshal([]byte(raw), &alert)
	require.NoError(t, err)

	assert.True(t, alert.EndsAt.IsZero(), "endsAt 0001-01-01 should parse as zero time")
	t.Logf("zero endsAt parsed: isZero=%v", alert.EndsAt.IsZero())
}

// loadSample reads a JSON file from .scripts/alert-samples/.
func loadSample(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", ".scripts", "alert-samples", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
