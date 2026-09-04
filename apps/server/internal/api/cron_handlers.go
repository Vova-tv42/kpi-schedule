package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"kpi-schedule-bot/server/internal/alerts"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/telemetry"
)

type CronHandler struct {
	dispatcher *alerts.Dispatcher
	cronSecret string
	telemetry  *telemetry.Client
}

func NewCronHandler(dispatcher *alerts.Dispatcher, cronSecret string) *CronHandler {
	return &CronHandler{
		dispatcher: dispatcher,
		cronSecret: cronSecret,
	}
}

// SetTelemetry attaches a telemetry client to report cron alert runs.
func (h *CronHandler) SetTelemetry(t *telemetry.Client) {
	h.telemetry = t
}

// HandleLessonAlerts triggers the scheduled lesson alerts check for personal users and group chats.
func (h *CronHandler) HandleLessonAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		model.WriteError(w, http.StatusMethodNotAllowed, model.ErrInvalidRequest, "Method not allowed")
		return
	}

	authHeader := r.Header.Get("Authorization")
	secretHeader := r.Header.Get("X-Cron-Secret")
	querySecret := r.URL.Query().Get("secret")

	var providedSecret string
	if strings.HasPrefix(authHeader, "Bearer ") {
		providedSecret = strings.TrimPrefix(authHeader, "Bearer ")
	} else if secretHeader != "" {
		providedSecret = secretHeader
	} else if querySecret != "" {
		providedSecret = querySecret
	}

	if h.cronSecret == "" || subtle.ConstantTimeCompare([]byte(providedSecret), []byte(h.cronSecret)) != 1 {
		model.WriteError(w, http.StatusUnauthorized, model.ErrUnauthorized, "Invalid or missing cron authorization")
		return
	}

	start := time.Now()
	if h.dispatcher == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "alerts dispatcher not configured",
		})
		return
	}

	result, err := h.dispatcher.Dispatch(r.Context(), time.Now())
	duration := time.Since(start).Milliseconds()
	if err != nil {
		if h.telemetry != nil {
			h.telemetry.ReportAction("cron_alert", "lesson_alerts", http.StatusInternalServerError, duration, nil)
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	if h.telemetry != nil {
		h.telemetry.ReportAction("cron_alert", "lesson_alerts", http.StatusOK, duration, map[string]any{
			"personal_alerts": result.PersonalAlertsSent,
			"group_alerts":    result.GroupAlertsSent,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":              true,
		"personal_alerts_sent": result.PersonalAlertsSent,
		"group_alerts_sent":    result.GroupAlertsSent,
		"dispatched_at":        time.Now().UTC(),
	})
}
