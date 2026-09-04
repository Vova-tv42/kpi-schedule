package api

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"kpi-schedule-bot/server/internal/alerts"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/telemetry"
)

// DraftSweeper discards /issues wizard drafts whose 10-minute TTL has elapsed
// and deletes the bot messages they left behind. Satisfied by
// (*bot.Bot).SweepExpiredIssueDrafts; nil when the Telegram bot is disabled.
type DraftSweeper func(ctx context.Context, now time.Time) error

type CronHandler struct {
	dispatcher   *alerts.Dispatcher
	cronSecret   string
	telemetry    *telemetry.Client
	draftSweeper DraftSweeper
}

func NewCronHandler(dispatcher *alerts.Dispatcher, cronSecret string) *CronHandler {
	return &CronHandler{
		dispatcher: dispatcher,
		cronSecret: cronSecret,
	}
}

// SetDraftSweeper attaches the expired-/issues-draft cleanup. It rides on this
// cron tick because it is the only per-minute heartbeat that survives the
// Fly.io scale-to-zero shutdown — an in-process ticker stops when the machine
// sleeps, which is exactly when drafts are most likely to go stale.
func (h *CronHandler) SetDraftSweeper(s DraftSweeper) {
	h.draftSweeper = s
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

	if h.draftSweeper != nil {
		if err := h.draftSweeper(r.Context(), time.Now()); err != nil {
			// Never fail the alerts run over draft cleanup.
			slog.Error("sweeping expired issue drafts", "error", err)
		}
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
