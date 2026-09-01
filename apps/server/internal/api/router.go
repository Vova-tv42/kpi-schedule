package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type handlers struct {
	svc *Service
}

// NewRouter builds the full /api/v1 route tree. debugRoutes gates the
// POST /api/v1/debug/mykpi/dump fixture-capture endpoint, which must never
// be enabled in production (see .env.example).
func NewRouter(svc *Service, internalToken string, debugRoutes bool) http.Handler {
	h := &handlers{svc: svc}
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", h.getHealthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(internalTokenMiddleware(internalToken))

		r.Route("/auth", func(r chi.Router) {
			r.Post("/session", h.postAuthSession)
			r.Get("/status/{telegramId}", h.getAuthStatus)
			r.Delete("/unlink/{telegramId}", h.deleteAuthUnlink)
		})

		r.Route("/schedule", func(r chi.Router) {
			r.Get("/today", h.getScheduleToday)
			r.Get("/tomorrow", h.getScheduleTomorrow)
			r.Get("/date", h.getScheduleDate)
			r.Get("/week", h.getScheduleWeek)
			r.Post("/refresh", h.postScheduleRefresh)
		})

		r.Get("/groups", h.getGroups)
		r.Get("/time/current", h.getTimeCurrent)

		if debugRoutes {
			r.Route("/debug", func(r chi.Router) {
				r.Post("/mykpi/dump", h.postDebugMyKPIDump)
			})
		}
	})

	return r
}
