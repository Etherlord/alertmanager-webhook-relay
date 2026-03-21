// Package server provides the HTTP server with routing, health checks, and middleware chain.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"alertmanager-webhook-relay/internal/server/middleware"
)

// Config содержит настройки HTTP-сервера.
type Config struct {
	Port            int
	ShutdownTimeout time.Duration
}

// Server — HTTP-сервер с health checks и middleware chain.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
	logger     *slog.Logger
	cfg        Config
	ready      chan struct{} // закрывается после успешного Listen
}

// New создаёт новый Server с настроенными маршрутами и middleware.
func New(cfg Config, logger *slog.Logger, checkers ...Checker) *Server {
	mux := http.NewServeMux()

	mux.Handle("GET /healthz", HandleHealthz(logger))
	mux.Handle("GET /readyz", HandleReadyz(logger, checkers...))

	chain := middleware.Chain(
		middleware.Logging(logger),
		middleware.Recovery(logger),
	)

	handler := chain(mux)

	srv := &Server{
		httpServer: &http.Server{
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger,
		cfg:    cfg,
		ready:  make(chan struct{}),
	}

	return srv
}

// Start запускает HTTP-сервер. Блокирует до остановки.
// Возвращает nil при штатном завершении (http.ErrServerClosed).
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)

	s.logger.Info("starting HTTP server", "addr", addr)

	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		close(s.ready)
		s.logger.Error("failed to listen", "addr", addr, "error", err)
		return err
	}
	s.listener = ln
	close(s.ready)

	s.logger.Info("HTTP server listening", "addr", ln.Addr().String())

	err = s.httpServer.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		s.logger.Info("HTTP server stopped")
		return nil
	}
	if err != nil {
		s.logger.Error("HTTP server error", "error", err)
	}
	return err
}

// Shutdown выполняет graceful shutdown HTTP-сервера.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		s.logger.Error("HTTP server shutdown error", "error", err)
		return err
	}

	s.logger.Info("HTTP server shutdown complete")
	return nil
}

// Ready возвращает канал, который закрывается после завершения Listen (успешного или нет).
// Если Listen завершился с ошибкой, Addr() вернёт пустую строку.
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

// Addr возвращает адрес, на котором сервер слушает.
// Безопасно вызывать только после закрытия Ready().
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}
