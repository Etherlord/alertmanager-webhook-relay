package pachca

import (
	"fmt"
	"sort"
	"strings"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/notify"
)

const timeFormat = "2006-01-02 15:04:05 UTC"

// filteredLabels are excluded from per-alert labels output (already in header).
var filteredLabels = map[string]struct{}{
	"alertname": {},
	"severity":  {},
}

// priorityLabels defines the display order for important labels (shown first, bold).
var priorityLabels = []string{
	"namespace",
	"exported_namespace",
	"pod",
	"exported_pod",
	"container",
	"exported_container",
	"deployment",
	"statefulset",
	"daemonset",
	"replicaset",
}

// FormatNotification formats a Notification into a Pachca Markdown message.
func FormatNotification(n *notify.Notification) string {
	var b strings.Builder

	writeHeader(&b, n)
	writeAlertmanagerURL(&b, n.ExternalURL)

	if len(n.Alerts) > 0 {
		b.WriteString("\n---\n")
		for i, a := range n.Alerts {
			if i > 0 {
				b.WriteByte('\n')
			}
			writeAlert(&b, i+1, &a)
		}
		b.WriteString("---\n")
	}

	if n.TruncatedAlerts > 0 {
		fmt.Fprintf(&b, "⚠️ +%d alerts truncated\n", n.TruncatedAlerts)
	}

	return b.String()
}

// writeHeader writes the status emoji, alert count, alertname, and severity.
func writeHeader(b *strings.Builder, n *notify.Notification) {
	alertname := n.CommonLabels["alertname"]
	severity := n.CommonLabels["severity"]

	var emoji string
	var statusLabel string
	var count int

	if n.Status == alerts.StatusResolved {
		emoji = "✅"
		statusLabel = "RESOLVED"
		count = countByStatus(n.Alerts, alerts.StatusResolved)
	} else {
		emoji = "🔥"
		statusLabel = "FIRING"
		count = countByStatus(n.Alerts, alerts.StatusFiring)
	}

	fmt.Fprintf(b, "**%s [%s:%d] %s", emoji, statusLabel, count, alertname)
	if severity != "" {
		fmt.Fprintf(b, " (%s)", severity)
	}
	b.WriteString("**\n")
}

// writeAlertmanagerURL writes the Alertmanager external URL line.
func writeAlertmanagerURL(b *strings.Builder, url string) {
	if url != "" {
		fmt.Fprintf(b, "\nAlertmanager: %s\n", url)
	}
}

// writeAlert writes the details of a single alert.
func writeAlert(b *strings.Builder, num int, a *alerts.Alert) {
	// Status line
	statusStr := strings.ToUpper(string(a.Status))
	if a.Status == alerts.StatusResolved {
		fmt.Fprintf(b, "🔹 Alert %d — %s ✅\n", num, statusStr)
	} else {
		fmt.Fprintf(b, "🔹 Alert %d — %s\n", num, statusStr)
	}

	// Annotations
	if v := a.Annotations["summary"]; v != "" {
		fmt.Fprintf(b, "Summary: %s\n", v)
	}
	if v := a.Annotations["description"]; v != "" {
		fmt.Fprintf(b, "Description: %s\n", v)
	}

	// Timestamps
	fmt.Fprintf(b, "Started: %s\n", a.StartsAt.UTC().Format(timeFormat))
	if a.Status == alerts.StatusResolved && !a.EndsAt.IsZero() {
		fmt.Fprintf(b, "Ended: %s\n", a.EndsAt.UTC().Format(timeFormat))
	}

	// Labels (filtered and sorted)
	writeLabels(b, a.Labels)

	// Source
	if a.GeneratorURL != "" {
		fmt.Fprintf(b, "Source: %s\n", a.GeneratorURL)
	}
}

// writeLabels writes labels vertically: priority labels first (bold), then the rest alphabetically.
func writeLabels(b *strings.Builder, labels map[string]string) {
	// Collect non-filtered keys.
	keys := make(map[string]struct{}, len(labels))
	for k := range labels {
		if _, skip := filteredLabels[k]; !skip {
			keys[k] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return
	}

	b.WriteString("\nLabels:\n")

	// Priority labels first, in defined order.
	for _, k := range priorityLabels {
		if _, ok := keys[k]; ok {
			fmt.Fprintf(b, "**%s:** %s\n", k, labels[k])
			delete(keys, k)
		}
	}

	// Remaining labels sorted alphabetically.
	remaining := make([]string, 0, len(keys))
	for k := range keys {
		remaining = append(remaining, k)
	}
	sort.Strings(remaining)

	for _, k := range remaining {
		fmt.Fprintf(b, "%s: %s\n", k, labels[k])
	}
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
