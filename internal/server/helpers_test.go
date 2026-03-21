package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
)

const slogLevelDebug = slog.LevelDebug

// httpGet выполняет GET-запрос с context (noctx-совместимо).
func httpGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	return resp
}
