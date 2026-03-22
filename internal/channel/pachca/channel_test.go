package pachca

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/alerts"
	"alertmanager-webhook-relay/internal/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannel_Name(t *testing.T) {
	ch := NewChannel("https://api.pachca.com", "token", 1, slog.Default())
	assert.Equal(t, "pachca", ch.Name())
}

func TestChannel_Send_Success(t *testing.T) {
	var received messageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	ch := NewChannel(srv.URL, "test-token", 42, slog.Default(), WithHTTPClient(srv.Client()))

	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://alertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		Alerts: []alerts.Alert{
			{
				Status:      alerts.StatusFiring,
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations: map[string]string{"summary": "Test alert fired"},
				StartsAt:    time.Date(2026, 3, 16, 7, 15, 20, 0, time.UTC),
			},
		},
		AlertsCount: 1,
	}

	err := ch.Send(context.Background(), n)
	require.NoError(t, err)

	assert.Equal(t, "discussion", received.Message.EntityType)
	assert.Equal(t, 42, received.Message.EntityID)
	assert.Contains(t, received.Message.Content, "TestAlert")
	assert.Contains(t, received.Message.Content, "FIRING")

	t.Logf("sent content:\n%s", received.Message.Content)
}

func TestChannel_Send_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal"}`))
	}))
	defer srv.Close()

	ch := NewChannel(srv.URL, "test-token", 42, slog.Default(), WithHTTPClient(srv.Client()))

	n := &notify.Notification{
		Status:      alerts.StatusFiring,
		ExternalURL: "http://alertmanager:9093",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		Alerts:      []alerts.Alert{},
		AlertsCount: 0,
	}

	err := ch.Send(context.Background(), n)
	assert.Error(t, err)

	t.Logf("API error → %v", err)
}
