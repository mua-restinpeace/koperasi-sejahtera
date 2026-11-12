// Package middleware berisi middleware HTTP.
package middleware

import (
	"log/slog"
	"net/http"
)

// Logging mencatat setiap permintaan yang masuk.
func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("request", "method", r.Method, "path", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
