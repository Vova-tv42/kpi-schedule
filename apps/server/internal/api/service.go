package api

import (
	"context"
	"errors"
	"fmt"
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

type Service struct {
	db     *storage.DB
	campus *campus.Client
}

func NewService(db *storage.DB, campusClient *campus.Client) *Service {
	return &Service{db: db, campus: campusClient}
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
