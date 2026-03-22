package notify

import (
	"time"

	"alertmanager-webhook-relay/internal/alerts"
)

// Notification represents a notification to be sent to channels.
// Created from an AlertGroup by the dispatcher.
type Notification struct {
	GroupKey          string
	Status            alerts.AlertStatus
	Receiver          string
	Alerts            []alerts.Alert
	GroupLabels       map[string]string
	CommonLabels      map[string]string
	CommonAnnotations map[string]string
	ExternalURL       string
	AlertsCount       int
	TruncatedAlerts   int
	CreatedAt         time.Time
}

// NewNotification creates a Notification from an AlertGroup.
func NewNotification(group *alerts.AlertGroup) *Notification {
	return &Notification{
		GroupKey:          group.GroupKey,
		Status:            group.Status,
		Receiver:          group.Receiver,
		Alerts:            group.Alerts,
		GroupLabels:       group.GroupLabels,
		CommonLabels:      group.CommonLabels,
		CommonAnnotations: group.CommonAnnotations,
		ExternalURL:       group.ExternalURL,
		AlertsCount:       len(group.Alerts),
		TruncatedAlerts:   group.TruncatedAlerts,
		CreatedAt:         time.Now(),
	}
}
