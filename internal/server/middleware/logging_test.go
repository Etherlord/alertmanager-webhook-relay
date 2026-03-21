package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/server/middleware"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogging_GET200(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	mw := middleware.Logging(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", http.NoBody)
	req.Header.Set("User-Agent", "kube-probe/1.28")
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))

	assert.Equal(t, "INFO", logEntry["level"])
	assert.Equal(t, "request completed", logEntry["msg"])
	assert.Equal(t, "GET", logEntry["method"])
	assert.Equal(t, "/healthz", logEntry["path"])
	assert.Equal(t, float64(200), logEntry["status"])
	assert.Equal(t, float64(5), logEntry["bytes"])
	assert.Equal(t, "kube-probe/1.28", logEntry["user_agent"])
	assert.Contains(t, logEntry, "duration_ms")
	assert.Contains(t, logEntry, "remote_addr")
}

func TestLogging_POST404(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	})

	mw := middleware.Logging(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/missing", http.NoBody)
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))

	assert.Equal(t, "POST", logEntry["method"])
	assert.Equal(t, "/missing", logEntry["path"])
	assert.Equal(t, float64(404), logEntry["status"])
	assert.Equal(t, float64(9), logEntry["bytes"])
}

func TestLogging_DurationPositive(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.Logging(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	wrapped.ServeHTTP(rec, req)

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))

	durationMs, ok := logEntry["duration_ms"].(float64)
	require.True(t, ok, "duration_ms should be a number")
	assert.GreaterOrEqual(t, durationMs, float64(0))
}

func TestLogging_DefaultStatus200(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	// Handler, который не вызывает WriteHeader — Go по умолчанию вернёт 200.
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("implicit 200"))
	})

	mw := middleware.Logging(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	wrapped.ServeHTTP(rec, req)

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))

	assert.Equal(t, float64(200), logEntry["status"])
}
