package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"alertmanager-webhook-relay/internal/logging"
	"alertmanager-webhook-relay/internal/server/middleware"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecovery_PanicString(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("something went wrong")
	})

	mw := middleware.Recovery(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "Internal Server Error\n", rec.Body.String())
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
	assert.Equal(t, "ERROR", logEntry["level"])
	assert.Equal(t, "panic recovered", logEntry["msg"])
	assert.Equal(t, "something went wrong", logEntry["panic"])
	assert.Contains(t, logEntry["stack"], "goroutine")
	assert.Equal(t, "GET", logEntry["method"])
	assert.Equal(t, "/boom", logEntry["path"])
}

func TestRecovery_PanicError(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic(errors.New("db connection lost"))
	})

	mw := middleware.Recovery(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api", nil)
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
	assert.Equal(t, "db connection lost", logEntry["panic"])
}

func TestRecovery_PanicArbitrary(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic(42)
	})

	mw := middleware.Recovery(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var logEntry map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logEntry))
	assert.Equal(t, "42", logEntry["panic"])
}

func TestRecovery_NoPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mw := middleware.Recovery(logger)
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
	assert.Empty(t, buf.String(), "no log output expected for normal requests")
}
