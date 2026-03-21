package alerts

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validGroup returns a minimal valid AlertGroup for test purposes.
func validGroup() AlertGroup {
	return AlertGroup{
		Receiver: "webhook",
		Status:   StatusFiring,
		Alerts: []Alert{
			{
				Status:      StatusFiring,
				Labels:      map[string]string{"alertname": "TestAlert"},
				Annotations: map[string]string{},
				StartsAt:    time.Date(2026, 3, 16, 8, 0, 0, 0, time.UTC),
				Fingerprint: "abc123",
			},
		},
		GroupLabels:       map[string]string{},
		CommonLabels:      map[string]string{},
		CommonAnnotations: map[string]string{},
		ExternalURL:       "http://alertmanager:9093",
		Version:           "4",
		GroupKey:           "test-key",
	}
}

func TestValidatePayload_Valid(t *testing.T) {
	group := validGroup()
	err := ValidatePayload(group, 100)
	require.NoError(t, err)
	t.Log("valid payload passed validation")
}

func TestValidatePayload_InvalidVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"empty version", ""},
		{"version 3", "3"},
		{"version 5", "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := validGroup()
			group.Version = tt.version

			err := ValidatePayload(group, 100)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidPayload)
			assert.Contains(t, err.Error(), "version")

			t.Logf("version=%q → error: %v", tt.version, err)
		})
	}
}

func TestValidatePayload_EmptyAlerts(t *testing.T) {
	group := validGroup()
	group.Alerts = nil

	err := ValidatePayload(group, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayload)
	assert.Contains(t, err.Error(), "alerts")

	t.Logf("empty alerts → error: %v", err)
}

func TestValidatePayload_TooManyAlerts(t *testing.T) {
	group := validGroup()
	group.Alerts = make([]Alert, 101)
	for i := range group.Alerts {
		group.Alerts[i] = Alert{
			Status:      StatusFiring,
			Labels:      map[string]string{"alertname": "Test"},
			Fingerprint: "fp",
		}
	}

	err := ValidatePayload(group, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPayloadTooLarge)
	assert.Contains(t, err.Error(), "101")

	t.Logf("101 alerts with maxAlerts=100 → error: %v", err)
}

func TestValidatePayload_MaxAlertsEdge(t *testing.T) {
	group := validGroup()
	group.Alerts = make([]Alert, 100)
	for i := range group.Alerts {
		group.Alerts[i] = Alert{
			Status:      StatusFiring,
			Labels:      map[string]string{"alertname": "Test"},
			Fingerprint: "fp",
		}
	}

	err := ValidatePayload(group, 100)
	require.NoError(t, err)
	t.Log("exactly maxAlerts=100 alerts passed validation")
}

func TestValidatePayload_MissingFingerprint(t *testing.T) {
	group := validGroup()
	group.Alerts[0].Fingerprint = ""

	err := ValidatePayload(group, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayload)
	assert.Contains(t, err.Error(), "fingerprint")

	t.Logf("empty fingerprint → error: %v", err)
}

func TestValidatePayload_MissingStatus(t *testing.T) {
	group := validGroup()
	group.Alerts[0].Status = ""

	err := ValidatePayload(group, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayload)
	assert.Contains(t, err.Error(), "status")

	t.Logf("empty alert status → error: %v", err)
}

func TestValidatePayload_MissingAlertname(t *testing.T) {
	group := validGroup()
	group.Alerts[0].Labels = map[string]string{"severity": "warning"}

	err := ValidatePayload(group, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayload)
	assert.Contains(t, err.Error(), "alertname")

	t.Logf("missing alertname → error: %v", err)
}

func TestValidatePayload_InvalidAlertStatus(t *testing.T) {
	group := validGroup()
	group.Alerts[0].Status = "unknown"

	err := ValidatePayload(group, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayload)
	assert.Contains(t, err.Error(), "status")

	t.Logf("invalid alert status → error: %v", err)
}

func TestValidatePayload_StringLengthLimits(t *testing.T) {
	longString := strings.Repeat("x", maxStringLength+1)

	tests := []struct {
		name  string
		setup func(g *AlertGroup)
	}{
		{
			name: "long label key",
			setup: func(g *AlertGroup) {
				g.Alerts[0].Labels[longString] = "value"
			},
		},
		{
			name: "long label value",
			setup: func(g *AlertGroup) {
				g.Alerts[0].Labels["key"] = longString
			},
		},
		{
			name: "long annotation key",
			setup: func(g *AlertGroup) {
				g.Alerts[0].Annotations[longString] = "value"
			},
		},
		{
			name: "long annotation value",
			setup: func(g *AlertGroup) {
				g.Alerts[0].Annotations["key"] = longString
			},
		},
		{
			name: "long fingerprint",
			setup: func(g *AlertGroup) {
				g.Alerts[0].Fingerprint = longString
			},
		},
		{
			name: "long generatorURL",
			setup: func(g *AlertGroup) {
				g.Alerts[0].GeneratorURL = longString
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := validGroup()
			tt.setup(&group)

			err := ValidatePayload(group, 100)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrPayloadTooLarge)

			t.Logf("%s → error: %v", tt.name, err)
		})
	}
}

func TestValidatePayload_MultipleAlerts_SecondInvalid(t *testing.T) {
	group := validGroup()
	group.Alerts = append(group.Alerts, Alert{
		Status:      StatusFiring,
		Labels:      map[string]string{},
		Fingerprint: "fp2",
	})

	err := ValidatePayload(group, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPayload)
	assert.Contains(t, err.Error(), "alerts[1]")

	t.Logf("second alert invalid → error: %v", err)
}
