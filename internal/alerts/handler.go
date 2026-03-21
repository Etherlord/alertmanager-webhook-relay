package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// Receiver defines the interface for the alert service used by the handler.
type Receiver interface {
	Receive(ctx context.Context, group AlertGroup) error
}

// HandleWebhook returns an http.HandlerFunc that accepts Alertmanager webhook payloads.
// maxBodySize limits the request body size in bytes (DoS protection).
func HandleWebhook(logger *slog.Logger, svc Receiver, maxBodySize int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("webhook request received",
			"remote_addr", r.RemoteAddr,
			"content_length", r.ContentLength,
		)

		// Content-Type check.
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			logger.Warn("unsupported content type", "content_type", ct)
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		// Limit body size.
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		var group AlertGroup
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				logger.Warn("request body too large",
					"max_bytes", maxBodySize,
				)
				writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			logger.Warn("invalid JSON payload", "error", err)
			writeError(w, http.StatusBadRequest, "invalid JSON payload")
			return
		}

		logger.Debug("webhook payload decoded",
			"group_key", group.GroupKey,
			"receiver", group.Receiver,
			"alerts_count", len(group.Alerts),
		)

		if err := svc.Receive(r.Context(), group); err != nil {
			switch {
			case errors.Is(err, ErrInvalidPayload):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, ErrPayloadTooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, err.Error())
			default:
				logger.Error("failed to receive alert group", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))

		logger.Debug("webhook request processed", "group_key", group.GroupKey)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp, _ := json.Marshal(map[string]string{"error": message})
	_, _ = w.Write(resp)
}
