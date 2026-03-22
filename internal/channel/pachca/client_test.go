package pachca

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// messageRequest mirrors the JSON structure sent to Pachca API.
type messageRequest struct {
	Message struct {
		EntityType string `json:"entity_type"`
		EntityID   int    `json:"entity_id"`
		Content    string `json:"content"`
	} `json:"message"`
}

func TestClient_SendMessage_Success(t *testing.T) {
	var received messageRequest
	var authHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		contentType := r.Header.Get("Content-Type")
		assert.Equal(t, "application/json", contentType)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/shared/v1/messages", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &received))

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", WithHTTPClient(srv.Client()))
	err := client.SendMessage(context.Background(), 42, "Hello, Pachca!")

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token", authHeader)
	assert.Equal(t, "discussion", received.Message.EntityType)
	assert.Equal(t, 42, received.Message.EntityID)
	assert.Equal(t, "Hello, Pachca!", received.Message.Content)

	t.Logf("sent message: entity_type=%s, entity_id=%d, content=%q",
		received.Message.EntityType, received.Message.EntityID, received.Message.Content)
}

func TestClient_SendMessage_Non2xxStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"bad request", http.StatusBadRequest},
		{"unauthorized", http.StatusUnauthorized},
		{"forbidden", http.StatusForbidden},
		{"not found", http.StatusNotFound},
		{"internal server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"error": "something went wrong"}`))
			}))
			defer srv.Close()

			client := NewClient(srv.URL, "test-token", WithHTTPClient(srv.Client()))
			err := client.SendMessage(context.Background(), 42, "Hello")

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "HTTP")

			t.Logf("status=%d → error: %v", tt.statusCode, err)
		})
	}
}

func TestClient_SendMessage_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := NewClient(srv.URL, "test-token", WithHTTPClient(srv.Client()))
	err := client.SendMessage(ctx, 42, "Hello")

	assert.Error(t, err)
	t.Logf("canceled context → error: %v", err)
}

func TestClient_SendMessage_200AlsoAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token", WithHTTPClient(srv.Client()))
	err := client.SendMessage(context.Background(), 42, "Hello")

	assert.NoError(t, err)
}

func TestNewClient_DefaultHTTPClient(t *testing.T) {
	client := NewClient("https://api.pachca.com", "test-token")
	assert.NotNil(t, client)
	t.Logf("default client created with base_url=%s", "https://api.pachca.com")
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	client := NewClient("https://api.pachca.com", "test-token", WithHTTPClient(custom))
	assert.NotNil(t, client)
}

func TestClient_SendMessage_BaseURLTrailingSlash(t *testing.T) {
	var requestPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	// Base URL with trailing slash
	client := NewClient(srv.URL+"/", "test-token", WithHTTPClient(srv.Client()))
	err := client.SendMessage(context.Background(), 42, "Hello")

	require.NoError(t, err)
	assert.Equal(t, "/api/shared/v1/messages", requestPath)

	t.Logf("path: %s (no double slash)", requestPath)
}
