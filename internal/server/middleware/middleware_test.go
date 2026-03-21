package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped.ServeHTTP(rec, req)

	require.True(t, called, "middleware should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)
}
