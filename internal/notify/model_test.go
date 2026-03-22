package notify

import (
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/alerts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotification(t *testing.T) {
	group := alerts.AlertGroup{
		Receiver:    "test-receiver",
		Status:      alerts.StatusFiring,
		GroupKey:    "test-group-key",
		GroupLabels: map[string]string{"alertname": "TestAlert"},
		CommonLabels: map[string]string{
			"severity": "critical",
		},
		CommonAnnotations: map[string]string{
			"summary": "Test alert fired",
		},
		ExternalURL: "http://alertmanager:9093",
		Version:     "4",
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"instance": "localhost:9090"},
				Annotations: map[string]string{"description": "Test alert"},
				StartsAt:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
				Fingerprint: "abc123",
			},
		},
		TruncatedAlerts: 0,
	}

	before := time.Now()
	n := NewNotification(&group)
	after := time.Now()

	require.NotNil(t, n)
	assert.Equal(t, group.GroupKey, n.GroupKey)
	assert.Equal(t, group.Status, n.Status)
	assert.Equal(t, group.Receiver, n.Receiver)
	assert.Equal(t, group.GroupLabels, n.GroupLabels)
	assert.Equal(t, group.CommonLabels, n.CommonLabels)
	assert.Equal(t, group.CommonAnnotations, n.CommonAnnotations)
	assert.Equal(t, group.ExternalURL, n.ExternalURL)
	assert.Equal(t, len(group.Alerts), n.AlertsCount)
	assert.Equal(t, group.Alerts, n.Alerts)
	assert.Equal(t, group.TruncatedAlerts, n.TruncatedAlerts)

	require.False(t, n.CreatedAt.IsZero(), "CreatedAt must be set")
	assert.True(t, !n.CreatedAt.Before(before) && !n.CreatedAt.After(after),
		"CreatedAt should be between before and after test execution")
}

func TestNewNotification_MultipleAlerts(t *testing.T) {
	group := alerts.AlertGroup{
		GroupKey: "multi-alert-group",
		Status:   alerts.StatusFiring,
		Receiver: "webhook",
		Alerts: []alerts.Alert{
			{Status: alerts.StatusFiring, Fingerprint: "aaa"},
			{Status: alerts.StatusFiring, Fingerprint: "bbb"},
			{Status: alerts.StatusResolved, Fingerprint: "ccc"},
		},
	}

	n := NewNotification(&group)

	assert.Equal(t, 3, n.AlertsCount)
	assert.Len(t, n.Alerts, 3)
}

func TestNewNotification_ResolvedStatus(t *testing.T) {
	group := alerts.AlertGroup{
		GroupKey: "resolved-group",
		Status:   alerts.StatusResolved,
		Receiver: "webhook",
		Alerts: []alerts.Alert{
			{Status: alerts.StatusResolved, Fingerprint: "ddd"},
		},
	}

	n := NewNotification(&group)

	assert.Equal(t, alerts.StatusResolved, n.Status)
}

func TestNewNotification_EmptyAlerts(t *testing.T) {
	group := alerts.AlertGroup{
		GroupKey: "empty-group",
		Status:   alerts.StatusFiring,
		Receiver: "webhook",
		Alerts:   []alerts.Alert{},
	}

	n := NewNotification(&group)

	assert.Equal(t, 0, n.AlertsCount)
	assert.Empty(t, n.Alerts)
}
