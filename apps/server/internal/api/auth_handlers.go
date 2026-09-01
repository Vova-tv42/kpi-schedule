package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

type sessionRequest struct {
	TelegramID int64         `json:"telegram_id"`
	GroupName  string        `json:"group_name"`
	Cookies    model.Cookies `json:"cookies"`
	UserAgent  string        `json:"user_agent"`
}

// POST /api/v1/auth/session
func (h *handlers) postAuthSession(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if err := decodeJSON(r, &req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid JSON body")
		return
	}
	if req.TelegramID == 0 || req.Cookies.PHPSESSID == "" {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "telegram_id and cookies.PHPSESSID are required")
		return
	}

	user, enrichment, err := h.svc.LinkSession(r.Context(), req.TelegramID, req.GroupName, req.Cookies, req.UserAgent)
	if err != nil {
		writeLinkError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":           true,
		"message":           "Session successfully validated and linked.",
		"telegram_id":       user.TelegramID,
		"group_name":        user.GroupName,
		"enrichment_status": enrichment,
	})
}

func writeLinkError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCookies):
		model.WriteError(w, http.StatusUnauthorized, model.ErrInvalidSessionCookies, "could not authenticate to my.kpi.ua with the provided cookies")
	default:
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
	}
}

// GET /api/v1/auth/status/{telegramId}
func (h *handlers) getAuthStatus(w http.ResponseWriter, r *http.Request) {
	telegramID, err := strconv.ParseInt(chi.URLParam(r, "telegramId"), 10, 64)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid telegramId")
		return
	}

	user, err := h.svc.db.GetUserByTelegramID(r.Context(), telegramID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"telegram_id": telegramID, "status": "NOT_LINKED"})
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	session, err := h.svc.db.GetSession(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"telegram_id": telegramID, "status": "NOT_LINKED"})
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	status := "ACTIVE"
	if session.Status == model.SessionExpired {
		status = "EXPIRED"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"telegram_id":     telegramID,
		"status":          status,
		"linked_at":       user.CreatedAt,
		"last_synced_at":  session.SyncedAt,
		"group_id":        user.GroupID,
		"group_name":      user.GroupName,
	})
}

// DELETE /api/v1/auth/unlink/{telegramId}
func (h *handlers) deleteAuthUnlink(w http.ResponseWriter, r *http.Request) {
	telegramID, err := strconv.ParseInt(chi.URLParam(r, "telegramId"), 10, 64)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid telegramId")
		return
	}

	user, err := h.svc.db.GetUserByTelegramID(r.Context(), telegramID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			model.WriteError(w, http.StatusNotFound, model.ErrUserNotFound, "user not found")
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	if err := h.svc.db.DeleteUser(r.Context(), user.ID); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Session unlinked and credentials deleted."})
}
