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

// adminTokenMiddleware requires the X-Admin-Secret header (or X-Internal-Token fallback)
// to match the configured secret.
func adminTokenMiddleware(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-Admin-Secret")
			if token == "" {
				token = r.Header.Get("X-Internal-Token")
			}
			if token != expected {
				model.WriteError(w, http.StatusUnauthorized, model.ErrUnauthorized, "missing or invalid X-Admin-Secret header")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// adminWritePermissionMiddleware ensures that callers with role 'read-only' cannot perform write operations.
func adminWritePermissionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := r.Header.Get("X-Admin-Role")
			if role == "read-only" {
				model.WriteError(w, http.StatusForbidden, model.ErrUnauthorized, "read-only role cannot perform write or custom query operations")
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

// corsMiddleware handles preflight OPTIONS requests and sets CORS headers
// so the browser extension (chrome-extension://*) can communicate with the server.
func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Internal-Token, X-User-Token")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

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


