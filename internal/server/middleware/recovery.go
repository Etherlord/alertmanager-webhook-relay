package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery возвращает middleware, который перехватывает panic,
// логирует ошибку со стектрейсом и отвечает 500 Internal Server Error.
func Recovery(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := string(debug.Stack())

					var panicStr string
					switch v := rec.(type) {
					case string:
						panicStr = v
					case error:
						panicStr = v.Error()
					default:
						panicStr = fmt.Sprintf("%v", v)
					}

					logger.Error("panic recovered",
						"panic", panicStr,
						"stack", stack,
						"method", r.Method,
						"path", r.URL.Path,
						"remote_addr", r.RemoteAddr,
					)

					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal Server Error\n"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
