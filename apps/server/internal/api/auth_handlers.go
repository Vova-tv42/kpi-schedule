package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// GET /api/v1/auth/status/{telegramId}
//
// There is no session concept any more (see docs/architecture/data-storage.md
// §1) — this just reports whether a schedule has ever been pushed for this
// user, and how stale it is.
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

	state, err := h.svc.db.GetScheduleState(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"telegram_id": telegramID, "status": "NOT_LINKED"})
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"telegram_id":    telegramID,
		"status":         "LINKED",
		"linked_at":      user.CreatedAt,
		"last_synced_at": state.RefreshedAt,
		"group_id":       user.GroupID,
		"group_name":     user.GroupName,
	})
}

// DELETE /api/v1/auth/unlink/{telegramId}
//
// Removes the user and all their stored lessons (ON DELETE CASCADE). There
// are no credentials to delete — the server never stored any.
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

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "User unlinked and all stored lessons deleted."})
}
