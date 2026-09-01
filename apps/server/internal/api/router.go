package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type handlers struct {
	svc *Service
}

// NewRouter builds the full /api/v1 route tree.
func NewRouter(svc *Service, internalToken string) http.Handler {
	h := &handlers{svc: svc}
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	// ClientIPFromRemoteAddr, not the deprecated RealIP: this server isn't
	// known to sit behind any particular reverse proxy yet (hosting is
	// deliberately undecided, see docs/project-repository.md §4.2), and
	// RealIP blindly trusts X-Forwarded-For/X-Real-IP from anyone, which
	// would let a client spoof its way around ipRateLimitMiddleware
	// entirely. Switch to middleware.ClientIPFromXFF(trustedProxyCIDR) (or
	// ClientIPFromHeader for a known CDN) once a specific host is chosen.
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", h.getHealthz)

	r.Route("/api/v1", func(r chi.Router) {
		// Rate-limit before the token check, so guessing at X-Internal-Token
		// is itself throttled rather than exempt from the limit.
		r.Use(ipRateLimitMiddleware())
		r.Use(internalTokenMiddleware(internalToken))

		r.Route("/auth", func(r chi.Router) {
			r.Get("/status/{telegramId}", h.getAuthStatus)
			r.Delete("/unlink/{telegramId}", h.deleteAuthUnlink)
		})

		r.Route("/schedule", func(r chi.Router) {
			r.Get("/today", h.getScheduleToday)
			r.Get("/tomorrow", h.getScheduleTomorrow)
			r.Get("/date", h.getScheduleDate)
			r.Get("/week", h.getScheduleWeek)
		})

		r.Get("/groups", h.getGroups)
		r.Get("/time/current", h.getTimeCurrent)
	})

	return r
}
