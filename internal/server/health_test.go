package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubChecker реализует server.Checker для тестов.
type stubChecker struct {
	name string
	err  error
}

func (c *stubChecker) Name() string                  { return c.name }
func (c *stubChecker) Check(_ context.Context) error { return c.err }

func newLogger(buf *bytes.Buffer) *slog.Logger {
	return logging.NewWithWriter(slog.LevelDebug, buf)
}

func TestHealthz_Returns200(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	handler := server.HandleHealthz(logger)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", http.NoBody)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestReadyz_NoCheckers(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	handler := server.HandleReadyz(logger)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestReadyz_AllCheckersOK(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	checkers := []server.Checker{
		&stubChecker{name: "db", err: nil},
		&stubChecker{name: "cache", err: nil},
	}

	handler := server.HandleReadyz(logger, checkers...)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestReadyz_FailedChecker(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	checkers := []server.Checker{
		&stubChecker{name: "db", err: errors.New("connection refused")},
	}

	handler := server.HandleReadyz(logger, checkers...)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "error", body["status"])

	checks, ok := body["checks"].(map[string]any)
	require.True(t, ok, "checks should be a map")
	// Ответ содержит generic статус, а не внутреннюю ошибку.
	assert.Equal(t, "unhealthy", checks["db"])
	assert.NotContains(t, rec.Body.String(), "connection refused",
		"internal error details must not leak to the client")

	// Проверяем WARN лог (последняя строка в буфере).
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.NotEmpty(t, lines)
	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(lines[len(lines)-1], &logEntry))
	assert.Equal(t, "WARN", logEntry["level"])
	assert.Contains(t, logEntry["msg"], "readiness check failed")
}

func TestReadyz_MixedCheckers(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	checkers := []server.Checker{
		&stubChecker{name: "db", err: nil},
		&stubChecker{name: "cache", err: errors.New("timeout")},
		&stubChecker{name: "queue", err: nil},
	}

	handler := server.HandleReadyz(logger, checkers...)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "error", body["status"])

	checks, ok := body["checks"].(map[string]any)
	require.True(t, ok)
	// Ответ содержит generic статус, а не внутреннюю ошибку.
	assert.Equal(t, "unhealthy", checks["cache"])
	assert.NotContains(t, rec.Body.String(), "timeout",
		"internal error details must not leak to the client")
	// Успешные проверки не должны попадать в checks.
	_, hasDB := checks["db"]
	assert.False(t, hasDB, "successful checker should not appear in error checks")
}

func TestPreStop_Returns200AndSleeps(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	delay := 50 * time.Millisecond
	handler := server.HandlePreStop(delay, logger)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lifecycle/pre-stop", http.NoBody)

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])

	assert.GreaterOrEqual(t, elapsed, delay, "handler should sleep for at least the configured delay")
}

func TestPreStop_ZeroDelay(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	handler := server.HandlePreStop(0, logger)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lifecycle/pre-stop", http.NoBody)

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Less(t, elapsed, 50*time.Millisecond, "zero delay should return immediately")
}

func TestPreStop_LogsDelayInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf)

	handler := server.HandlePreStop(10*time.Millisecond, logger)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/lifecycle/pre-stop", http.NoBody)
	handler.ServeHTTP(rec, req)

	logs := buf.String()
	assert.Contains(t, logs, "preStop hook called")
	assert.Contains(t, logs, "preStop delay complete")
}
