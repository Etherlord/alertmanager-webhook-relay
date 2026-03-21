package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockService implements the receiver interface for handler tests.
type mockService struct {
	receiveFunc func(ctx context.Context, group AlertGroup) error
}

func (m *mockService) Receive(ctx context.Context, group AlertGroup) error {
	if m.receiveFunc != nil {
		return m.receiveFunc(ctx, group)
	}
	return nil
}

func handlerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func validPayloadJSON(t *testing.T) []byte {
	t.Helper()
	group := validGroup()
	data, err := json.Marshal(group)
	require.NoError(t, err)
	return data
}

func TestHandleWebhook_Success(t *testing.T) {
	svc := &mockService{}
	handler := HandleWebhook(handlerLogger(), svc, 1<<20)

	body := validPayloadJSON(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])

	t.Logf("success: status=%d, body=%s", w.Code, w.Body.String())
}

func TestHandleWebhook_WrongContentType(t *testing.T) {
	svc := &mockService{}
	handler := HandleWebhook(handlerLogger(), svc, 1<<20)

	body := validPayloadJSON(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)

	t.Logf("wrong content-type: status=%d, body=%s", w.Code, w.Body.String())
}

func TestHandleWebhook_BodyTooLarge(t *testing.T) {
	svc := &mockService{}
	handler := HandleWebhook(handlerLogger(), svc, 10) // 10 bytes max

	body := validPayloadJSON(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	t.Logf("body too large: status=%d, body=%s", w.Code, w.Body.String())
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	svc := &mockService{}
	handler := HandleWebhook(handlerLogger(), svc, 1<<20)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	t.Logf("invalid JSON: status=%d, body=%s", w.Code, w.Body.String())
}

func TestHandleWebhook_ValidationError(t *testing.T) {
	svc := &mockService{
		receiveFunc: func(_ context.Context, _ AlertGroup) error {
			return ErrInvalidPayload
		},
	}
	handler := HandleWebhook(handlerLogger(), svc, 1<<20)

	body := validPayloadJSON(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	t.Logf("validation error: status=%d, body=%s", w.Code, w.Body.String())
}

func TestHandleWebhook_PayloadTooLargeError(t *testing.T) {
	svc := &mockService{
		receiveFunc: func(_ context.Context, _ AlertGroup) error {
			return ErrPayloadTooLarge
		},
	}
	handler := HandleWebhook(handlerLogger(), svc, 1<<20)

	body := validPayloadJSON(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	t.Logf("payload too large from service: status=%d, body=%s", w.Code, w.Body.String())
}

func TestHandleWebhook_InternalError(t *testing.T) {
	svc := &mockService{
		receiveFunc: func(_ context.Context, _ AlertGroup) error {
			return errors.New("database error")
		},
	}
	handler := HandleWebhook(handlerLogger(), svc, 1<<20)

	body := validPayloadJSON(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	t.Logf("internal error: status=%d, body=%s", w.Code, w.Body.String())
}

func TestHandleWebhook_EmptyBody(t *testing.T) {
	svc := &mockService{}
	handler := HandleWebhook(handlerLogger(), svc, 1<<20)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	t.Logf("empty body: status=%d, body=%s", w.Code, w.Body.String())
}
