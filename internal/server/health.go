package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
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
				failed[c.Name()] = "unhealthy"
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

// HandlePreStop returns a handler for Kubernetes preStop lifecycle hook.
// It sleeps for the given delay, allowing kube-proxy to update iptables
// and remove the pod from endpoints before SIGTERM is sent.
// Distroless images don't have a shell, so exec sleep is not an option.
func HandlePreStop(delay time.Duration, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("preStop hook called, waiting before shutdown",
			"delay", delay,
			"remote_addr", r.RemoteAddr,
		)

		time.Sleep(delay)

		logger.Info("preStop delay complete")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
