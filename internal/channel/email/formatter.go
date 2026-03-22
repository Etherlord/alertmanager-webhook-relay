package email

import (
	"fmt"
	"html"
	htmltemplate "html/template"
	"log/slog"
	"sort"
	"strings"
	"time"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/notify"
	"alertmanager-webhook-relay/internal/template"
)

const timeFormat = "2006-01-02 15:04:05 UTC"

// filteredLabels are excluded from per-alert labels output (already in header).
var filteredLabels = map[string]struct{}{
	"alertname": {},
	"severity":  {},
}

// FormatSubject formats the email subject line.
// Format: "[prefix] [STATUS:N] alertname (receiver)"
func FormatSubject(n *notify.Notification, prefix string) string {
	alertname := n.CommonLabels["alertname"]
	var statusLabel string
	var count int

	if n.Status == alerts.StatusResolved {
		statusLabel = "RESOLVED"
		count = countByStatus(n.Alerts, alerts.StatusResolved)
	} else {
		statusLabel = "FIRING"
		count = countByStatus(n.Alerts, alerts.StatusFiring)
	}

	subject := fmt.Sprintf("%s [%s:%d] %s", prefix, statusLabel, count, alertname)
	if n.Receiver != "" {
		subject += fmt.Sprintf(" (%s)", n.Receiver)
	}
	return subject
}

// FormatBodyDefault formats the notification as HTML with inline CSS.
// This is the fallback formatter used when template rendering fails.
func FormatBodyDefault(n *notify.Notification) string {
	var b strings.Builder

	// Status color.
	statusColor := "#e74c3c" // red for firing
	statusEmoji := "🔥"
	statusLabel := "FIRING"
	if n.Status == alerts.StatusResolved {
		statusColor = "#27ae60" // green for resolved
		statusEmoji = "✅"
		statusLabel = "RESOLVED"
	}

	alertname := html.EscapeString(n.CommonLabels["alertname"])
	severity := html.EscapeString(n.CommonLabels["severity"])

	// Header.
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body style="font-family:Arial,sans-serif;margin:0;padding:20px;background:#f5f5f5;">`)
	b.WriteString(`<div style="max-width:600px;margin:0 auto;background:#fff;border-radius:8px;overflow:hidden;">`)

	// Status banner.
	fmt.Fprintf(&b, `<div style="background:%s;color:#fff;padding:16px 20px;font-size:18px;font-weight:bold;">`, statusColor)
	fmt.Fprintf(&b, `%s [%s:%d] %s`, statusEmoji, statusLabel, n.AlertsCount, alertname)
	if severity != "" {
		fmt.Fprintf(&b, ` <span style="font-weight:normal;opacity:0.8;">(%s)</span>`, severity)
	}
	b.WriteString(`</div>`)

	// Alertmanager link.
	if n.ExternalURL != "" {
		fmt.Fprintf(&b, `<div style="padding:8px 20px;background:#f8f9fa;border-bottom:1px solid #e9ecef;font-size:13px;">Alertmanager: <a href="%s">%s</a></div>`,
			html.EscapeString(n.ExternalURL), html.EscapeString(n.ExternalURL))
	}

	// Alerts.
	b.WriteString(`<div style="padding:20px;">`)
	for i, a := range n.Alerts {
		if i > 0 {
			b.WriteString(`<hr style="border:none;border-top:1px solid #e9ecef;margin:16px 0;">`)
		}
		writeAlertHTML(&b, i+1, &a)
	}
	b.WriteString(`</div>`)

	// Truncated alerts warning.
	if n.TruncatedAlerts > 0 {
		fmt.Fprintf(&b, `<div style="padding:12px 20px;background:#fff3cd;color:#856404;font-size:13px;">⚠️ +%d alerts truncated</div>`, n.TruncatedAlerts)
	}

	b.WriteString(`</div></body></html>`)
	return b.String()
}

// writeAlertHTML writes a single alert as HTML.
func writeAlertHTML(b *strings.Builder, num int, a *alerts.Alert) {
	alertStatus := strings.ToUpper(string(a.Status))
	statusIcon := "🔴"
	if a.Status == alerts.StatusResolved {
		statusIcon = "🟢"
	}

	fmt.Fprintf(b, `<div style="margin-bottom:8px;font-weight:bold;font-size:15px;">%s Alert %d — %s</div>`, statusIcon, num, alertStatus)

	// Annotations.
	if v := a.Annotations["summary"]; v != "" {
		fmt.Fprintf(b, `<div style="margin-bottom:4px;"><strong>Summary:</strong> %s</div>`, html.EscapeString(v))
	}
	if v := a.Annotations["description"]; v != "" {
		fmt.Fprintf(b, `<div style="margin-bottom:4px;"><strong>Description:</strong> %s</div>`, html.EscapeString(v))
	}

	// Timestamps.
	fmt.Fprintf(b, `<div style="margin-bottom:4px;color:#6c757d;font-size:13px;">Started: %s</div>`, a.StartsAt.UTC().Format(timeFormat))
	if a.Status == alerts.StatusResolved && !a.EndsAt.IsZero() {
		fmt.Fprintf(b, `<div style="margin-bottom:4px;color:#6c757d;font-size:13px;">Ended: %s</div>`, a.EndsAt.UTC().Format(timeFormat))
	}

	// Labels.
	writeLabelsHTML(b, a.Labels)

	// Source link.
	if a.GeneratorURL != "" {
		fmt.Fprintf(b, `<div style="margin-top:4px;font-size:13px;"><a href="%s">Source</a></div>`,
			html.EscapeString(a.GeneratorURL))
	}
}

// writeLabelsHTML writes filtered and sorted labels as an HTML line.
func writeLabelsHTML(b *strings.Builder, labels map[string]string) {
	pairs := FilterLabels(labels)
	if len(pairs) == 0 {
		return
	}

	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, fmt.Sprintf("<code>%s=%s</code>", html.EscapeString(p.Key), html.EscapeString(p.Value)))
	}
	fmt.Fprintf(b, `<div style="margin-bottom:4px;font-size:13px;">Labels: %s</div>`, strings.Join(parts, " "))
}

// countByStatus counts alerts with the given status.
func countByStatus(alertList []alerts.Alert, status alerts.AlertStatus) int {
	count := 0
	for i := range alertList {
		if alertList[i].Status == status {
			count++
		}
	}
	return count
}

// DefaultFuncMap returns the template.FuncMap used by email templates.
func DefaultFuncMap() htmltemplate.FuncMap {
	return htmltemplate.FuncMap{
		"formatTime":   func(t time.Time) string { return t.UTC().Format(timeFormat) },
		"statusColor":  statusColorFunc,
		"upper":        func(s any) string { return strings.ToUpper(fmt.Sprint(s)) },
		"join":         strings.Join,
		"filterLabels": FilterLabels,
		"add":          func(a, b int) int { return a + b },
	}
}

// statusColorFunc returns a CSS color for the given alert status.
func statusColorFunc(status any) string {
	s := fmt.Sprint(status)
	if strings.EqualFold(s, string(alerts.StatusResolved)) {
		return "#27ae60"
	}
	return "#e74c3c"
}

// LabelPair is a key-value pair for deterministic label output in templates.
type LabelPair struct {
	Key   string
	Value string
}

// FilterLabels returns sorted label pairs excluding alertname and severity.
func FilterLabels(labels map[string]string) []LabelPair {
	pairs := make([]LabelPair, 0, len(labels))
	for k, v := range labels {
		if _, skip := filteredLabels[k]; !skip {
			pairs = append(pairs, LabelPair{Key: k, Value: v})
		}
	}
	if len(pairs) == 0 {
		return nil
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
	return pairs
}

// TemplateFormatter renders notifications using the template engine,
// falling back to FormatBodyDefault on error.
type TemplateFormatter struct {
	engine       *template.Engine
	templateName string
	logger       *slog.Logger
}

// NewTemplateFormatter creates a new TemplateFormatter.
func NewTemplateFormatter(engine *template.Engine, templateName string, logger *slog.Logger) *TemplateFormatter {
	return &TemplateFormatter{
		engine:       engine,
		templateName: templateName,
		logger:       logger,
	}
}

// FormatBody renders the notification using the template engine.
// Falls back to FormatBodyDefault if rendering fails.
func (f *TemplateFormatter) FormatBody(n *notify.Notification) string {
	result, err := f.engine.Render(f.templateName, n)
	if err != nil {
		f.logger.Warn("template render failed, using fallback",
			"template", f.templateName,
			"error", err,
		)
		return FormatBodyDefault(n)
	}
	return result
}
