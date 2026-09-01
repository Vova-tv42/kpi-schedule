package api

import (
	"net/http"

	"kpi-schedule-bot/server/internal/model"
)

// internalTokenMiddleware requires the X-Internal-Token header to match the
// configured secret. Applied to every /api/v1 route except /healthz.
func internalTokenMiddleware(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Internal-Token") != expected {
				model.WriteError(w, http.StatusUnauthorized, model.ErrUnauthorized, "missing or invalid X-Internal-Token header")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
