//go:build integration

package email

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/notify"
)

const (
	mailpitSMTPHost = "mailpit"
	mailpitSMTPPort = 1025
	mailpitAPIURL   = "http://mailpit:8025"
)

func mailpitHost() string {
	if h := os.Getenv("MAILPIT_SMTP_HOST"); h != "" {
		return h
	}
	return mailpitSMTPHost
}

func mailpitAPI() string {
	if u := os.Getenv("MAILPIT_API_URL"); u != "" {
		return u
	}
	return mailpitAPIURL
}

// mailpitMessagesResponse represents the Mailpit /api/v1/messages response.
type mailpitMessagesResponse struct {
	Total    int              `json:"total"`
	Messages []mailpitMessage `json:"messages"`
}

type mailpitMessage struct {
	ID      string           `json:"ID"`
	Subject string           `json:"Subject"`
	From    mailpitAddress   `json:"From"`
	To      []mailpitAddress `json:"To"`
	Snippet string           `json:"Snippet"`
}

type mailpitAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

// mailpitMessageDetail represents the Mailpit /api/v1/message/{id} response.
type mailpitMessageDetail struct {
	ID      string           `json:"ID"`
	Subject string           `json:"Subject"`
	HTML    string           `json:"HTML"`
	From    mailpitAddress   `json:"From"`
	To      []mailpitAddress `json:"To"`
}

func cleanupMailpit(t *testing.T) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, mailpitAPI()+"/api/v1/messages", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

func getMailpitMessages(t *testing.T) mailpitMessagesResponse {
	t.Helper()
	resp, err := http.Get(mailpitAPI() + "/api/v1/messages")
	require.NoError(t, err)
	defer resp.Body.Close()
	var result mailpitMessagesResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func getMailpitMessage(t *testing.T, id string) mailpitMessageDetail {
	t.Helper()
	resp, err := http.Get(mailpitAPI() + "/api/v1/message/" + id)
	require.NoError(t, err)
	defer resp.Body.Close()
	var result mailpitMessageDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

func TestIntegration_EmailChannel_SendFiring(t *testing.T) {
	cleanupMailpit(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	sender := NewSender(mailpitHost(), mailpitSMTPPort, "alerts@example.com", "", "", "none", logger)
	ch := NewChannel(sender, []string{"oncall@example.com"}, "[Alert]", logger)

	n := &notify.Notification{
		GroupKey: "integration-test",
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

	err := ch.Send(t.Context(), n)
	require.NoError(t, err)

	// Small delay for Mailpit to process.
	time.Sleep(500 * time.Millisecond)

	// Verify message was received.
	msgs := getMailpitMessages(t)
	require.Equal(t, 1, msgs.Total, "expected exactly 1 message")

	msg := msgs.Messages[0]
	assert.Contains(t, msg.Subject, "[Alert] [FIRING:1] HighCPU (webhook)")
	assert.Equal(t, "alerts@example.com", msg.From.Address)
	require.Len(t, msg.To, 1)
	assert.Equal(t, "oncall@example.com", msg.To[0].Address)

	// Check HTML body.
	detail := getMailpitMessage(t, msg.ID)
	assert.Contains(t, detail.HTML, "FIRING")
	assert.Contains(t, detail.HTML, "HighCPU")
	assert.Contains(t, detail.HTML, "CPU usage is high")
	assert.Contains(t, detail.HTML, "server-1")

	t.Logf("integration test passed: subject=%s, from=%s, to=%v",
		msg.Subject, msg.From.Address, msg.To)
}

func TestIntegration_EmailChannel_SendResolved(t *testing.T) {
	cleanupMailpit(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	sender := NewSender(mailpitHost(), mailpitSMTPPort, "alerts@example.com", "", "", "none", logger)
	ch := NewChannel(sender, []string{"oncall@example.com", "dev@example.com"}, "[PROD]", logger)

	n := &notify.Notification{
		GroupKey: "integration-test-resolved",
		Status:   alerts.StatusResolved,
		Receiver: "webhook",
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusResolved,
				Labels:      map[string]string{"alertname": "DiskFull", "severity": "warning"},
				Annotations: map[string]string{"summary": "Disk space recovered"},
				StartsAt:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
				EndsAt:      time.Date(2026, 3, 22, 10, 30, 0, 0, time.UTC),
				Fingerprint: "def456",
			},
		},
		GroupLabels:       map[string]string{"alertname": "DiskFull"},
		CommonLabels:      map[string]string{"alertname": "DiskFull", "severity": "warning"},
		CommonAnnotations: map[string]string{},
		ExternalURL:       "http://alertmanager:9093",
		AlertsCount:       1,
	}

	err := ch.Send(t.Context(), n)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	msgs := getMailpitMessages(t)
	require.Equal(t, 1, msgs.Total)

	msg := msgs.Messages[0]
	assert.Contains(t, msg.Subject, "[PROD] [RESOLVED:1] DiskFull (webhook)")
	require.Len(t, msg.To, 2)

	detail := getMailpitMessage(t, msg.ID)
	assert.Contains(t, detail.HTML, "RESOLVED")
	assert.Contains(t, detail.HTML, "Disk space recovered")
}
