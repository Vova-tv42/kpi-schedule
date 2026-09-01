package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

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

// requestsPerMinutePerIP caps how many /api/v1 requests a single client IP
// may make. This is separate from (and cannot substitute for) the per-user
// limiter the future Telegram webhook route will need — every one of a
// webhook bot's end users arrives from Telegram's own shared edge IPs, so an
// IP-keyed limit there would throttle real users instead of abusers. See
// docs/architecture/error-handling-resilience.md §5.
const requestsPerMinutePerIP = 20

// ipRateLimitMiddleware rate-limits by the client IP resolved via
// middleware.ClientIPFromRemoteAddr (see router.go) — the raw TCP peer, not
// a spoofable header, since this server isn't known to sit behind a trusted
// reverse proxy yet. Responds with the standard APIError JSON envelope
// rather than httprate's default plain-text body.
func ipRateLimitMiddleware() func(http.Handler) http.Handler {
	return httprate.LimitBy(requestsPerMinutePerIP, time.Minute,
		func(r *http.Request) (string, error) {
			return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
		},
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			model.WriteError(w, http.StatusTooManyRequests, model.ErrRateLimited,
				"too many requests, slow down")
		}),
	)
}
