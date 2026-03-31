package alerts

import (
	"encoding/json"
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
	data := []byte(`{
  "receiver": "telegram",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertgroup": "vmoperator",
        "alertname": "LogErrors",
        "severity": "warning"
      },
      "annotations": {
        "description": "Operator has too many errors at logs: 0.003, check operator logs",
        "summary": "Too many errors at logs of operator: 0.003"
      },
      "startsAt": "2026-03-16T08:38:50Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://vmalert-vm-alert:8080/vmalert/alert?group_id=123&alert_id=456",
      "fingerprint": "b76d9da35df35672"
    }
  ],
  "groupLabels": {
    "alertname": "LogErrors"
  },
  "commonLabels": {
    "alertgroup": "vmoperator",
    "alertname": "LogErrors",
    "severity": "warning"
  },
  "commonAnnotations": {
    "description": "Operator has too many errors at logs: 0.003, check operator logs",
    "summary": "Too many errors at logs of operator: 0.003"
  },
  "externalURL": "http://vmalertmanager-alertmanager-1:9093",
  "version": "4",
  "groupKey": "{}/{severity=~\"critical|warning|major\"}:{alertname=\"LogErrors\"}",
  "truncatedAlerts": 0
}`)

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
	data := []byte(`{
  "receiver": "telegram",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertgroup": "vm-health",
        "alertname": "ServiceDown",
        "instance": "10.244.19.201:9093",
        "severity": "critical"
      },
      "annotations": {
        "description": "10.244.19.201:9093 of job vmalertmanager has been down for more than 2 minutes.",
        "summary": "Service vmalertmanager is down on 10.244.19.201:9093"
      },
      "startsAt": "2026-03-16T07:15:20Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://vmalert:8080/vmalert/alert?group_id=1&alert_id=1",
      "fingerprint": "a5d5e52f18dc11ff"
    },
    {
      "status": "resolved",
      "labels": {
        "alertgroup": "vm-health",
        "alertname": "ServiceDown",
        "instance": "10.244.20.199:9093",
        "severity": "critical"
      },
      "annotations": {
        "description": "10.244.20.199:9093 of job vmalertmanager has been down for more than 2 minutes.",
        "summary": "Service vmalertmanager is down on 10.244.20.199:9093"
      },
      "startsAt": "2026-03-16T07:16:10Z",
      "endsAt": "2026-03-16T16:21:50Z",
      "generatorURL": "http://vmalert:8080/vmalert/alert?group_id=1&alert_id=2",
      "fingerprint": "9d747f5fa8000660"
    },
    {
      "status": "firing",
      "labels": {
        "alertgroup": "vm-health",
        "alertname": "ServiceDown",
        "instance": "10.244.21.234:9093",
        "severity": "critical"
      },
      "annotations": {
        "description": "10.244.21.234:9093 of job vmalertmanager has been down for more than 2 minutes.",
        "summary": "Service vmalertmanager is down on 10.244.21.234:9093"
      },
      "startsAt": "2026-03-16T07:16:10Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://vmalert:8080/vmalert/alert?group_id=1&alert_id=3",
      "fingerprint": "2757cb12d09d1af5"
    }
  ],
  "groupLabels": {
    "alertname": "ServiceDown"
  },
  "commonLabels": {
    "alertgroup": "vm-health",
    "alertname": "ServiceDown",
    "severity": "critical"
  },
  "commonAnnotations": {},
  "externalURL": "http://vmalertmanager-alertmanager-1:9093",
  "version": "4",
  "groupKey": "{}/{severity=~\"critical|warning|major\"}:{alertname=\"ServiceDown\"}",
  "truncatedAlerts": 0
}`)

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

func TestAlertGroup_UnmarshalJSON_Invariants(t *testing.T) {
	samples := map[string]string{
		"single_firing": `{
  "receiver": "webhook",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {"alertname": "HighMemory", "severity": "warning"},
      "annotations": {"summary": "Memory usage is high"},
      "startsAt": "2026-03-16T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://vmalert:8080/vmalert/alert?id=1",
      "fingerprint": "aaa111"
    }
  ],
  "groupLabels": {"alertname": "HighMemory"},
  "commonLabels": {"alertname": "HighMemory", "severity": "warning"},
  "commonAnnotations": {"summary": "Memory usage is high"},
  "externalURL": "http://alertmanager:9093",
  "version": "4",
  "groupKey": "{}/{severity=~\"critical|warning\"}:{alertname=\"HighMemory\"}",
  "truncatedAlerts": 0
}`,
		"resolved": `{
  "receiver": "email",
  "status": "resolved",
  "alerts": [
    {
      "status": "resolved",
      "labels": {"alertname": "DiskFull", "severity": "critical"},
      "annotations": {"summary": "Disk full"},
      "startsAt": "2026-03-16T08:00:00Z",
      "endsAt": "2026-03-16T09:00:00Z",
      "generatorURL": "http://vmalert:8080/vmalert/alert?id=2",
      "fingerprint": "bbb222"
    }
  ],
  "groupLabels": {"alertname": "DiskFull"},
  "commonLabels": {"alertname": "DiskFull", "severity": "critical"},
  "commonAnnotations": {"summary": "Disk full"},
  "externalURL": "http://alertmanager:9093",
  "version": "4",
  "groupKey": "{}/{severity=~\"critical\"}:{alertname=\"DiskFull\"}",
  "truncatedAlerts": 0
}`,
		"multiple_mixed": `{
  "receiver": "slack",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {"alertname": "PodCrash", "severity": "critical"},
      "annotations": {"summary": "Pod crashing"},
      "startsAt": "2026-03-16T07:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://vmalert:8080/vmalert/alert?id=3",
      "fingerprint": "ccc333"
    },
    {
      "status": "resolved",
      "labels": {"alertname": "PodCrash", "severity": "critical"},
      "annotations": {"summary": "Pod crashing"},
      "startsAt": "2026-03-16T06:00:00Z",
      "endsAt": "2026-03-16T06:30:00Z",
      "generatorURL": "http://vmalert:8080/vmalert/alert?id=4",
      "fingerprint": "ddd444"
    }
  ],
  "groupLabels": {"alertname": "PodCrash"},
  "commonLabels": {"alertname": "PodCrash", "severity": "critical"},
  "commonAnnotations": {"summary": "Pod crashing"},
  "externalURL": "http://alertmanager:9093",
  "version": "4",
  "groupKey": "{}/{severity=~\"critical\"}:{alertname=\"PodCrash\"}",
  "truncatedAlerts": 0
}`,
	}

	for name, raw := range samples {
		t.Run(name, func(t *testing.T) {
			var group AlertGroup
			err := json.Unmarshal([]byte(raw), &group)
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
				name, group.Receiver, group.Status, len(group.Alerts))
		})
	}
}

func TestAlertGroup_MarshalJSON_RoundTrip(t *testing.T) {
	original := AlertGroup{
		Receiver: "webhook",
		Status:   StatusFiring,
		Alerts: []Alert{
			{
				Status:       StatusFiring,
				Labels:       map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations:  map[string]string{"summary": "test"},
				StartsAt:     time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC),
				EndsAt:       time.Time{},
				GeneratorURL: "http://example.com",
				Fingerprint:  "abc123",
			},
		},
		GroupLabels:       map[string]string{"alertname": "TestAlert"},
		CommonLabels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
		CommonAnnotations: map[string]string{"summary": "test"},
		ExternalURL:       "http://alertmanager:9093",
		Version:           "4",
		GroupKey:          "test-group-key",
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
