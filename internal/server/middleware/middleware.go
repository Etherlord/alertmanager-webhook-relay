// Package middleware provides HTTP middleware chain: Metrics, Logging, Recovery.
package middleware

import "net/http"

// Middleware — функция-обёртка для HTTP-обработчика.
type Middleware func(http.Handler) http.Handler

// Chain последовательно применяет middleware.
// Порядок: первый middleware в списке — самый внешний.
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}
