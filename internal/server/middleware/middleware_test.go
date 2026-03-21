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

func TestChain_MultipleMiddlewares(t *testing.T) {
	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	chain := middleware.Chain(mw1, mw2)
	wrapped := chain(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/test", http.NoBody)
	wrapped.ServeHTTP(rec, req)

	// Первый middleware — внешний, второй — внутренний.
	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	assert.Equal(t, expected, order)
}

func TestChain_Empty(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	chain := middleware.Chain()
	wrapped := chain(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTeapot, rec.Code)
}

func TestChain_SingleMiddleware(t *testing.T) {
	called := false

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := middleware.Chain(mw)
	wrapped := chain(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	wrapped.ServeHTTP(rec, req)

	require.True(t, called, "middleware should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestChain_LoggingRecovery_PanicAfterWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.NewWithWriter(slogDebug, &buf)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK) // статус установлен, но panic до Write
		panic("crash after setting status")
	})

	chain := middleware.Chain(
		middleware.Logging(logger),
		middleware.Recovery(logger),
	)
	wrapped := chain(handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/partial", http.NoBody)
	wrapped.ServeHTTP(rec, req)

	// Recovery должен переопределить статус на 500.
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// Должно быть две записи: "panic recovered" (Recovery) и "request completed" (Logging).
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)

	var requestEntry map[string]any
	for _, line := range lines {
		var entry map[string]any
		require.NoError(t, json.Unmarshal(line, &entry))
		if entry["msg"] == "request completed" {
			requestEntry = entry
		}
	}

	// Logging должен залогировать 500, а не 200.
	require.NotNil(t, requestEntry, "expected 'request completed' log entry")
	assert.Equal(t, float64(http.StatusInternalServerError), requestEntry["status"])
}
