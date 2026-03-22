package pachca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 10 * time.Second
	maxErrorBodySize   = 8 << 10 // 8 KB
	messagesPath       = "/api/shared/v1/messages"
)

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client (useful for testing).
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		cl.httpClient = c
	}
}

// Client is an HTTP client for the Pachca API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	logger     *slog.Logger
}

// NewClient creates a new Pachca API client.
func NewClient(baseURL, token string, logger *slog.Logger, opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		logger:     logger,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.logger.Debug("pachca client created", "base_url", c.baseURL)
	return c
}

// messagePayload is the JSON structure for the Pachca messages API.
type messagePayload struct {
	Message messageBody `json:"message"`
}

type messageBody struct {
	EntityType string `json:"entity_type"`
	EntityID   int    `json:"entity_id"`
	Content    string `json:"content"`
}

// SendMessage sends a message to a Pachca chat.
func (c *Client) SendMessage(ctx context.Context, chatID int, content string) error {
	c.logger.Debug("sending pachca message", "chat_id", chatID, "content_len", len(content))

	payload := messagePayload{
		Message: messageBody{
			EntityType: "discussion",
			EntityID:   chatID,
			Content:    content,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pachca marshal request: %w", err)
	}

	url := c.baseURL + messagesPath

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pachca create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pachca send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read limited error body for diagnostics, then drain for connection reuse.
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		c.logger.Warn("pachca API error",
			"status_code", resp.StatusCode,
			"response_body", string(errBody),
		)
		return fmt.Errorf("pachca API returned HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	// Drain body for TCP connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	c.logger.Debug("pachca message sent", "chat_id", chatID, "status_code", resp.StatusCode)
	return nil
}
