package email

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/notify"
)

func firingNotification() *notify.Notification {
	return &notify.Notification{
		GroupKey:  "test-group-key",
		Status:   alerts.StatusFiring,
		Receiver: "webhook",
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"alertname": "HighCPU", "severity": "critical", "instance": "server-1"},
				Annotations: map[string]string{"summary": "CPU usage is high", "description": "CPU > 90% for 5 minutes"},
				StartsAt:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
				Fingerprint: "abc123",
			},
		},
		GroupLabels:       map[string]string{"alertname": "HighCPU"},
		CommonLabels:      map[string]string{"alertname": "HighCPU", "severity": "critical"},
		CommonAnnotations: map[string]string{},
		ExternalURL:       "http://alertmanager:9093",
		AlertsCount:       1,
	}
}

func resolvedNotification() *notify.Notification {
	return &notify.Notification{
		GroupKey:  "test-group-key",
		Status:   alerts.StatusResolved,
		Receiver: "webhook",
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusResolved,
				Labels:      map[string]string{"alertname": "HighCPU", "severity": "critical", "instance": "server-1"},
				Annotations: map[string]string{"summary": "CPU usage is high"},
				StartsAt:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
				EndsAt:      time.Date(2026, 3, 22, 10, 15, 0, 0, time.UTC),
				Fingerprint: "abc123",
			},
		},
		GroupLabels:       map[string]string{"alertname": "HighCPU"},
		CommonLabels:      map[string]string{"alertname": "HighCPU", "severity": "critical"},
		CommonAnnotations: map[string]string{},
		ExternalURL:       "http://alertmanager:9093",
		AlertsCount:       1,
	}
}

func TestFormatSubject_Firing(t *testing.T) {
	n := firingNotification()
	subject := FormatSubject(n, "[Alert]")

	assert.Equal(t, "[Alert] [FIRING:1] HighCPU (webhook)", subject)
	t.Logf("subject: %s", subject)
}

func TestFormatSubject_Resolved(t *testing.T) {
	n := resolvedNotification()
	subject := FormatSubject(n, "[Alert]")

	assert.Equal(t, "[Alert] [RESOLVED:1] HighCPU (webhook)", subject)
	t.Logf("subject: %s", subject)
}

func TestFormatSubject_CustomPrefix(t *testing.T) {
	n := firingNotification()
	subject := FormatSubject(n, "[PROD]")

	assert.Equal(t, "[PROD] [FIRING:1] HighCPU (webhook)", subject)
}

func TestFormatSubject_NoReceiver(t *testing.T) {
	n := firingNotification()
	n.Receiver = ""
	subject := FormatSubject(n, "[Alert]")

	assert.Equal(t, "[Alert] [FIRING:1] HighCPU", subject)
}

func TestFormatSubject_MultipleAlerts(t *testing.T) {
	n := firingNotification()
	n.Alerts = append(n.Alerts, alerts.Alert{
		Status: alerts.StatusFiring,
		Labels: map[string]string{"alertname": "HighCPU", "instance": "server-2"},
	})
	n.AlertsCount = 2

	subject := FormatSubject(n, "[Alert]")
	assert.Equal(t, "[Alert] [FIRING:2] HighCPU (webhook)", subject)
}

func TestFormatSubject_MixedStatus(t *testing.T) {
	n := firingNotification()
	n.Alerts = append(n.Alerts, alerts.Alert{
		Status: alerts.StatusResolved,
		Labels: map[string]string{"alertname": "HighCPU", "instance": "server-2"},
	})
	// Status is "firing" overall, so count only firing alerts.
	subject := FormatSubject(n, "[Alert]")
	assert.Equal(t, "[Alert] [FIRING:1] HighCPU (webhook)", subject)
}

func TestFormatBodyDefault_FiringHTML(t *testing.T) {
	n := firingNotification()
	body := FormatBodyDefault(n)

	// HTML structure.
	assert.Contains(t, body, "<!DOCTYPE html>")
	assert.Contains(t, body, "</html>")

	// Status banner — red for firing.
	assert.Contains(t, body, "#e74c3c")
	assert.Contains(t, body, "FIRING")
	assert.Contains(t, body, "🔥")
	assert.Contains(t, body, "HighCPU")
	assert.Contains(t, body, "critical")

	// Alert details.
	assert.Contains(t, body, "CPU usage is high")
	assert.Contains(t, body, "CPU &gt; 90% for 5 minutes") // HTML escaped
	assert.Contains(t, body, "2026-03-22 10:00:00 UTC")

	// Labels (filtered — no alertname, no severity).
	assert.Contains(t, body, "instance=server-1")
	assert.NotContains(t, body, "<code>alertname=")
	assert.NotContains(t, body, "<code>severity=")

	// Alertmanager link.
	assert.Contains(t, body, "http://alertmanager:9093")

	t.Logf("body length: %d", len(body))
}

func TestFormatBodyDefault_ResolvedHTML(t *testing.T) {
	n := resolvedNotification()
	body := FormatBodyDefault(n)

	// Status — green for resolved.
	assert.Contains(t, body, "#27ae60")
	assert.Contains(t, body, "RESOLVED")
	assert.Contains(t, body, "✅")

	// End time shown for resolved.
	assert.Contains(t, body, "2026-03-22 10:15:00 UTC")
}

func TestFormatBodyDefault_HTMLEscaping(t *testing.T) {
	n := firingNotification()
	n.Alerts[0].Annotations["summary"] = `<script>alert("xss")</script>`
	n.CommonLabels["alertname"] = `Alert<br>Name`

	body := FormatBodyDefault(n)

	// XSS vectors must be escaped.
	assert.NotContains(t, body, `<script>`)
	assert.Contains(t, body, `&lt;script&gt;`)
	assert.Contains(t, body, `Alert&lt;br&gt;Name`)
}

func TestFormatBodyDefault_NoExternalURL(t *testing.T) {
	n := firingNotification()
	n.ExternalURL = ""

	body := FormatBodyDefault(n)
	assert.NotContains(t, body, "Alertmanager:")
}

func TestFormatBodyDefault_TruncatedAlerts(t *testing.T) {
	n := firingNotification()
	n.TruncatedAlerts = 5

	body := FormatBodyDefault(n)
	assert.Contains(t, body, "+5 alerts truncated")
}

func TestFormatBodyDefault_NoAnnotations(t *testing.T) {
	n := firingNotification()
	n.Alerts[0].Annotations = map[string]string{}

	body := FormatBodyDefault(n)
	assert.NotContains(t, body, "Summary:")
	assert.NotContains(t, body, "Description:")
}

func TestFormatBodyDefault_GeneratorURL(t *testing.T) {
	n := firingNotification()
	n.Alerts[0].GeneratorURL = "http://prometheus:9090/graph?g0.expr=up"

	body := FormatBodyDefault(n)
	assert.Contains(t, body, "Source")
	assert.Contains(t, body, "http://prometheus:9090/graph?g0.expr=up")
}

func TestFormatBodyDefault_NoLabelsAfterFilter(t *testing.T) {
	n := firingNotification()
	n.Alerts[0].Labels = map[string]string{"alertname": "Test", "severity": "warning"}

	body := FormatBodyDefault(n)
	// All labels are filtered out — no "Labels:" line.
	assert.NotContains(t, body, "Labels:")
}
