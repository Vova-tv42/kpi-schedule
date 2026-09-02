package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

func (h *handlers) resolveUser(w http.ResponseWriter, r *http.Request) (model.User, bool) {
	telegramIDStr := r.URL.Query().Get("telegram_id")
	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "telegram_id query parameter is required")
		return model.User{}, false
	}

	user, err := h.svc.db.GetUserByTelegramID(r.Context(), telegramID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			model.WriteError(w, http.StatusUnauthorized, model.ErrAuthRequired, "user not linked; pair the browser extension first")
			return model.User{}, false
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return model.User{}, false
	}
	return user, true
}

func writeScheduleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoScheduleData):
		model.WriteError(w, http.StatusUnauthorized, model.ErrAuthRequired, "no schedule data stored yet; sync the browser extension first")
	default:
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
	}
}

// GET /api/v1/schedule/today
func (h *handlers) getScheduleToday(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	day, err := h.svc.BuildDay(r.Context(), user, time.Now())
	if err != nil {
		writeScheduleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, day)
}

// GET /api/v1/schedule/tomorrow
func (h *handlers) getScheduleTomorrow(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	day, err := h.svc.BuildDay(r.Context(), user, time.Now().AddDate(0, 0, 1))
	if err != nil {
		writeScheduleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, day)
}

// GET /api/v1/schedule/date?date=YYYY-MM-DD
func (h *handlers) getScheduleDate(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	dateStr := r.URL.Query().Get("date")
	target, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "date query parameter must be YYYY-MM-DD")
		return
	}
	day, err := h.svc.BuildDay(r.Context(), user, target)
	if err != nil {
		writeScheduleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, day)
}

// GET /api/v1/schedule/week?week=1|2
func (h *handlers) getScheduleWeek(w http.ResponseWriter, r *http.Request) {
	user, ok := h.resolveUser(w, r)
	if !ok {
		return
	}
	weekFilter := 0
	if v := r.URL.Query().Get("week"); v != "" {
		weekFilter, _ = strconv.Atoi(v)
	}
	week, err := h.svc.BuildWeek(r.Context(), user, weekFilter)
	if err != nil {
		writeScheduleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, week)
}
