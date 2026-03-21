package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter оборачивает http.ResponseWriter для перехвата status code и bytes written.
// WriteHeader буферизуется до первого вызова Write или flushHeader,
// что позволяет Recovery-middleware переопределить статус при panic.
type responseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool // WriteHeader был вызван (статус зафиксирован)
	headerSent  bool // статус и заголовки отправлены клиенту
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.flushHeader()
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// flushHeader отправляет буферизованный статус клиенту, если ещё не отправлен.
func (rw *responseWriter) flushHeader() {
	if !rw.headerSent {
		if !rw.wroteHeader {
			rw.status = http.StatusOK
			rw.wroteHeader = true
		}
		rw.ResponseWriter.WriteHeader(rw.status)
		rw.headerSent = true
	}
}

// ResetStatus переопределяет HTTP-статус, если заголовки ещё не отправлены клиенту.
// Используется Recovery-middleware для установки 500 при panic.
func (rw *responseWriter) ResetStatus(code int) {
	if !rw.headerSent {
		rw.status = code
		rw.wroteHeader = true
	}
}

// Unwrap возвращает оригинальный http.ResponseWriter для http.ResponseController.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Logging возвращает middleware, который логирует HTTP-запросы.
func Logging(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(rw, r)
			rw.flushHeader()

			duration := time.Since(start)

			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"bytes", rw.bytes,
				"duration_ms", float64(duration.Microseconds())/1000.0,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		})
	}
}
