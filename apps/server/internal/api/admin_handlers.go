package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"kpi-schedule-bot/server/internal/model"
)

type updateRowRequest struct {
	PrimaryKeyColumn string         `json:"primary_key_column"`
	PrimaryKeyValue  any            `json:"primary_key_value"`
	Updates          map[string]any `json:"updates"`
}

type executeQueryRequest struct {
	Query string `json:"query"`
}

func (h *handlers) getAdminTables(w http.ResponseWriter, r *http.Request) {
	tables, err := h.svc.DB().GetTables(r.Context())
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tables": tables,
	})
}

func (h *handlers) getAdminTableRows(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")
	if table == "" {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "table parameter is required")
		return
	}

	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if rawOffset := r.URL.Query().Get("offset"); rawOffset != "" {
		if parsed, err := strconv.Atoi(rawOffset); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	data, err := h.svc.DB().GetTableRows(r.Context(), table, limit, offset, sortBy, sortOrder)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *handlers) putAdminTableRow(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")
	if table == "" {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "table parameter is required")
		return
	}

	var req updateRowRequest
	if err := decodeJSON(r, &req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid JSON payload")
		return
	}

	if req.PrimaryKeyColumn == "" || req.PrimaryKeyValue == nil || len(req.Updates) == 0 {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "primary_key_column, primary_key_value and updates are required")
		return
	}

	if err := h.svc.DB().UpdateTableRow(r.Context(), table, req.PrimaryKeyColumn, req.PrimaryKeyValue, req.Updates); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (h *handlers) postAdminQuery(w http.ResponseWriter, r *http.Request) {
	var req executeQueryRequest
	if err := decodeJSON(r, &req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid JSON payload")
		return
	}

	if req.Query == "" {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "query is required")
		return
	}

	result, err := h.svc.DB().ExecuteAdminQuery(r.Context(), req.Query)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
