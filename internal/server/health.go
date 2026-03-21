package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Checker — интерфейс проверки готовности компонента.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// HandleHealthz возвращает liveness-handler, который всегда отвечает 200 OK.
func HandleHealthz(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("liveness check called", "remote_addr", r.RemoteAddr)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

// HandleReadyz возвращает readiness-handler, который проверяет все checkers.
func HandleReadyz(logger *slog.Logger, checkers ...Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("readiness check called",
			"remote_addr", r.RemoteAddr,
			"checkers_count", len(checkers),
		)

		failed := make(map[string]string)
		for _, c := range checkers {
			if err := c.Check(r.Context()); err != nil {
				failed[c.Name()] = err.Error()
				logger.Warn("readiness check failed",
					"check_name", c.Name(),
					"error", err,
				)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if len(failed) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)

			resp := map[string]any{
				"status": "error",
				"checks": failed,
			}
			data, _ := json.Marshal(resp)
			_, _ = w.Write(data)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
