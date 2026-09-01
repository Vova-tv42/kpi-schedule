package api

import (
	"net/http"

	"kpi-schedule-bot/server/internal/model"
)

// GET /api/v1/groups?query=
func (h *handlers) getGroups(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	groups, err := h.svc.campus.SearchGroups(r.Context(), query)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrCampusAPIUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

// GET /api/v1/time/current
func (h *handlers) getTimeCurrent(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.campus.CurrentTime(r.Context())
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrCampusAPIUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// GET /healthz
func (h *handlers) getHealthz(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.db.SQL.PingContext(r.Context()); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, "database unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
