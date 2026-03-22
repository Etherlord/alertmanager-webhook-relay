package pachca

import (
	"strings"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/notify"

	"github.com/stretchr/testify/assert"
)

func makeTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func TestFormatNotification_FiringGroup(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "ServiceDown",
			"severity":  "critical",
		},
		Alerts: []alerts.Alert{
			{
				Status:       alerts.StatusFiring,
				Labels:       map[string]string{"alertname": "ServiceDown", "severity": "critical", "namespace": "monitoring", "pod": "pod-1"},
				Annotations:  map[string]string{"summary": "Service is down on pod-1", "description": "pod-1 has been down for 2 minutes."},
				StartsAt:     makeTime("2026-03-16T07:15:20Z"),
				GeneratorURL: "http://vmalert:8080/alert?id=1",
			},
			{
				Status:       alerts.StatusFiring,
				Labels:       map[string]string{"alertname": "ServiceDown", "severity": "critical", "namespace": "monitoring", "pod": "pod-2"},
				Annotations:  map[string]string{"summary": "Service is down on pod-2", "description": "pod-2 has been down for 2 minutes."},
				StartsAt:     makeTime("2026-03-16T07:16:10Z"),
				GeneratorURL: "http://vmalert:8080/alert?id=2",
			},
		},
		AlertsCount: 2,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	// Header
	assert.Contains(t, result, "🔥")
	assert.Contains(t, result, "[FIRING:2]")
	assert.Contains(t, result, "ServiceDown")
	assert.Contains(t, result, "(critical)")

	// Alertmanager URL
	assert.Contains(t, result, "http://vmalertmanager:9093")

	// Alert 1
	assert.Contains(t, result, "Alert 1")
	assert.Contains(t, result, "FIRING")
	assert.Contains(t, result, "Service is down on pod-1")
	assert.Contains(t, result, "pod-1 has been down for 2 minutes.")
	assert.Contains(t, result, "2026-03-16 07:15:20 UTC")
	assert.Contains(t, result, "http://vmalert:8080/alert?id=1")

	// Alert 2
	assert.Contains(t, result, "Alert 2")
	assert.Contains(t, result, "Service is down on pod-2")
}

func TestFormatNotification_ResolvedGroup(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusResolved,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "ServiceDown",
			"severity":  "warning",
		},
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusResolved,
				Labels:      map[string]string{"alertname": "ServiceDown", "severity": "warning"},
				Annotations: map[string]string{"summary": "Service recovered"},
				StartsAt:    makeTime("2026-03-16T07:15:20Z"),
				EndsAt:      makeTime("2026-03-16T16:21:50Z"),
			},
		},
		AlertsCount: 1,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	assert.Contains(t, result, "✅")
	assert.Contains(t, result, "[RESOLVED:1]")
	assert.Contains(t, result, "ServiceDown")
	assert.Contains(t, result, "(warning)")
	assert.Contains(t, result, "RESOLVED ✅")
	assert.Contains(t, result, "2026-03-16 16:21:50 UTC")
}

func TestFormatNotification_EmptyAlerts(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "info",
		},
		Alerts:      []alerts.Alert{},
		AlertsCount: 0,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	// Should still have header
	assert.Contains(t, result, "TestAlert")
	assert.Contains(t, result, "(info)")
	// No alert details
	assert.NotContains(t, result, "Alert 1")
}

func TestFormatNotification_EmptyAnnotations(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations: map[string]string{},
				StartsAt:    makeTime("2026-03-16T07:15:20Z"),
			},
		},
		AlertsCount: 1,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	// Should not contain Summary/Description lines
	assert.NotContains(t, result, "Summary:")
	assert.NotContains(t, result, "Description:")
	// Should contain start time
	assert.Contains(t, result, "2026-03-16 07:15:20 UTC")
}

func TestFormatNotification_NoLabelsAfterFilter(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		Alerts: []alerts.Alert{
			{
				Status: alerts.StatusFiring,
				Labels: map[string]string{
					"alertname": "TestAlert",
					"severity":  "critical",
				},
				Annotations: map[string]string{"summary": "Something broke"},
				StartsAt:    makeTime("2026-03-16T07:15:20Z"),
			},
		},
		AlertsCount: 1,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	// alertname and severity filtered — no Labels line
	assert.NotContains(t, result, "Labels:")
}

func TestFormatNotification_LabelsFilteredAndSorted(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		Alerts: []alerts.Alert{
			{
				Status: alerts.StatusFiring,
				Labels: map[string]string{
					"alertname": "TestAlert",
					"severity":  "critical",
					"namespace": "monitoring",
					"pod":       "pod-1",
					"container": "alertmanager",
				},
				Annotations: map[string]string{"summary": "Down"},
				StartsAt:    makeTime("2026-03-16T07:15:20Z"),
			},
		},
		AlertsCount: 1,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	// alertname and severity should be filtered
	lines := strings.Split(result, "\n")
	var labelsLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "Labels:") {
			labelsLine = l
			break
		}
	}

	assert.NotEmpty(t, labelsLine)
	assert.NotContains(t, labelsLine, "alertname=")
	assert.NotContains(t, labelsLine, "severity=")
	// Should be sorted: container, namespace, pod
	containerIdx := strings.Index(labelsLine, "container=")
	namespaceIdx := strings.Index(labelsLine, "namespace=")
	podIdx := strings.Index(labelsLine, "pod=")
	assert.Greater(t, namespaceIdx, containerIdx)
	assert.Greater(t, podIdx, namespaceIdx)
}

func TestFormatNotification_TruncatedAlerts(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations: map[string]string{"summary": "Alert fired"},
				StartsAt:    makeTime("2026-03-16T07:15:20Z"),
			},
		},
		AlertsCount:     4,
		TruncatedAlerts: 3,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	assert.Contains(t, result, "⚠️")
	assert.Contains(t, result, "+3")
	assert.Contains(t, result, "truncated")
}

func TestFormatNotification_MixedFiringResolved(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "ServiceDown",
			"severity":  "critical",
		},
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"alertname": "ServiceDown", "severity": "critical", "pod": "pod-1"},
				Annotations: map[string]string{"summary": "Down on pod-1"},
				StartsAt:    makeTime("2026-03-16T07:15:20Z"),
			},
			{
				Status:      alerts.StatusResolved,
				Labels:      map[string]string{"alertname": "ServiceDown", "severity": "critical", "pod": "pod-2"},
				Annotations: map[string]string{"summary": "Down on pod-2"},
				StartsAt:    makeTime("2026-03-16T07:16:10Z"),
				EndsAt:      makeTime("2026-03-16T16:21:50Z"),
			},
		},
		AlertsCount: 2,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	// Header should show firing count
	assert.Contains(t, result, "[FIRING:1]")

	// Firing alert — no Ended line
	// Get the section for Alert 1
	assert.Contains(t, result, "FIRING")

	// Resolved alert — has Ended line and checkmark
	assert.Contains(t, result, "RESOLVED ✅")
	assert.Contains(t, result, "Ended:")
	assert.Contains(t, result, "2026-03-16 16:21:50 UTC")
}

func TestFormatNotification_NoGeneratorURL(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "warning",
		},
		Alerts: []alerts.Alert{
			{
				Status:       alerts.StatusFiring,
				Labels:       map[string]string{"alertname": "TestAlert", "severity": "warning"},
				Annotations:  map[string]string{"summary": "Test"},
				StartsAt:     makeTime("2026-03-16T07:15:20Z"),
				GeneratorURL: "",
			},
		},
		AlertsCount: 1,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	assert.NotContains(t, result, "Source:")
}

func TestFormatNotification_NoSeverity(t *testing.T) {
	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://vmalertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
		},
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"alertname": "TestAlert"},
				Annotations: map[string]string{"summary": "Test"},
				StartsAt:    makeTime("2026-03-16T07:15:20Z"),
			},
		},
		AlertsCount: 1,
	}

	result := FormatNotification(n)
	t.Logf("formatted:\n%s", result)

	// Header without severity
	assert.Contains(t, result, "TestAlert")
	assert.NotContains(t, result, "()")
}
