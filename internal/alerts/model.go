package alerts

import "time"

// AlertStatus represents the status of an alert or alert group.
type AlertStatus string

const (
	// StatusFiring indicates the alert is currently active.
	StatusFiring AlertStatus = "firing"
	// StatusResolved indicates the alert has been resolved.
	StatusResolved AlertStatus = "resolved"
)

// AlertGroup represents the top-level Alertmanager webhook v4 payload.
// A single webhook POST contains one AlertGroup with one or more Alerts.
type AlertGroup struct {
	Receiver          string            `json:"receiver"`
	Status            AlertStatus       `json:"status"`
	Alerts            []Alert           `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
}

// Alert represents an individual alert within an AlertGroup.
type Alert struct {
	Status       AlertStatus       `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}
