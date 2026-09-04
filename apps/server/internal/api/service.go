package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// staleAfter flags a stored schedule as stale for display purposes once the
// browser extension hasn't pushed an update in roughly this long (KPI's
// week-1/week-2 cycle plus margin). This is purely informational — the
// server has no way to trigger a refresh itself; only the extension can push
// new data. See docs/architecture/data-storage.md §4.
const staleAfter = 14 * 24 * time.Hour

// ErrNoScheduleData means the user exists but no schedule has ever been
// pushed for them by the browser extension.
var ErrNoScheduleData = errors.New("no schedule data stored for this user yet")

// IssueNotifier pushes issue activity back to the reporter over Telegram.
// Declared here rather than imported because internal/bot already depends on
// internal/api; *bot.Bot satisfies it and is injected in cmd/server/main.go.
type IssueNotifier interface {
	NotifyIssueComment(ctx context.Context, issue model.Issue, comment model.IssueComment) error
	NotifyIssueStatus(ctx context.Context, issue model.Issue, previous model.IssueStatus) error
	NotifyIssueThreadState(ctx context.Context, issue model.Issue, previous model.IssueThreadState) error
}

type Service struct {
	db            *storage.DB
	campus        *campus.Client
	issueNotifier IssueNotifier
}

func NewService(db *storage.DB, campusClient *campus.Client) *Service {
	return &Service{db: db, campus: campusClient}
}

// SetIssueNotifier wires up reporter notifications for the admin issue
// endpoints. Left nil when the Telegram bot is disabled, in which case the
// Notify* helpers are no-ops.
func (s *Service) SetIssueNotifier(n IssueNotifier) {
	s.issueNotifier = n
}

// NotifyIssueComment tells the reporter an admin replied. Delivery is
// best-effort: a Telegram failure is logged, never surfaced to the admin whose
// comment was already saved.
func (s *Service) NotifyIssueComment(ctx context.Context, issue model.Issue, comment model.IssueComment) {
	if s.issueNotifier == nil {
		return
	}
	if err := s.issueNotifier.NotifyIssueComment(ctx, issue, comment); err != nil {
		slog.Warn("notifying issue comment", "error", err, "issue_number", issue.Number)
	}
}

// NotifyIssueStatus tells the reporter their issue moved through triage.
// Best-effort, for the same reason as NotifyIssueComment.
func (s *Service) NotifyIssueStatus(ctx context.Context, issue model.Issue, previous model.IssueStatus) {
	if s.issueNotifier == nil {
		return
	}
	if err := s.issueNotifier.NotifyIssueStatus(ctx, issue, previous); err != nil {
		slog.Warn("notifying issue status change", "error", err, "issue_number", issue.Number)
	}
}

// NotifyIssueThreadState tells the reporter their discussion was closed or
// reopened, so a silently disabled Reply button never looks like a bug.
// Best-effort, for the same reason as NotifyIssueComment.
func (s *Service) NotifyIssueThreadState(ctx context.Context, issue model.Issue, previous model.IssueThreadState) {
	if s.issueNotifier == nil {
		return
	}
	if err := s.issueNotifier.NotifyIssueThreadState(ctx, issue, previous); err != nil {
		slog.Warn("notifying issue thread state change", "error", err, "issue_number", issue.Number)
	}
}

// Campus returns the underlying Campus API client.
func (s *Service) Campus() *campus.Client {
	return s.campus
}

// DB returns the underlying storage.DB client.
func (s *Service) DB() *storage.DB {
	return s.db
}

// GeneratePairCode creates a fresh one-time pairing code for telegramID,
// retrying on the rare code collision. Shared by the HTTP endpoint
// (sync_handlers.go, POST /api/v1/auth/pair/generate) and the Telegram
// bot's /link handler (internal/bot), which calls this in-process.
func (s *Service) GeneratePairCode(ctx context.Context, telegramID int64) (code string, expiresIn int, err error) {
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	for attempt := 0; attempt < 5; attempt++ {
		code, err = generate6DigitCode()
		if err != nil {
			return "", 0, fmt.Errorf("generating code: %w", err)
		}
		if err = s.db.CreatePairingCode(ctx, code, telegramID, expiresAt); err == nil {
			return code, 600, nil
		}
		if !errors.Is(err, storage.ErrCodeCollision) {
			return "", 0, err
		}
	}
	return "", 0, fmt.Errorf("failed to generate unique pairing code after retries")
}

// ScheduleFreshness reports whether a user has a stored schedule and how
// stale it is. It makes no network calls and triggers no refresh — the
// server cannot fetch a schedule on its own; see
// docs/architecture/data-storage.md §1 and §4. Exported so the Telegram
// bot's onboarding screen can tell an already-synced user from a new one.
func (s *Service) ScheduleFreshness(ctx context.Context, user model.User) (hasData, stale bool, enrichment model.EnrichmentStatus, err error) {
	state, stateErr := s.db.GetScheduleState(ctx, user.ID)
	if stateErr != nil {
		if errors.Is(stateErr, storage.ErrNotFound) {
			return false, false, "", nil
		}
		return false, false, "", stateErr
	}
	return true, time.Since(state.RefreshedAt) > staleAfter, state.EnrichmentStatus, nil
}
