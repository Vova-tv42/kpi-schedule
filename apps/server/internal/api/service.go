package api

import (
	"context"
	"errors"
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

// scheduleFreshness reports whether a user has a stored schedule and how
// stale it is. It makes no network calls and triggers no refresh — the
// server cannot fetch a schedule on its own; see
// docs/architecture/data-storage.md §1 and §4.
func (s *Service) scheduleFreshness(ctx context.Context, user model.User) (hasData, stale bool, enrichment model.EnrichmentStatus, err error) {
	state, stateErr := s.db.GetScheduleState(ctx, user.ID)
	if stateErr != nil {
		if errors.Is(stateErr, storage.ErrNotFound) {
			return false, false, "", nil
		}
		return false, false, "", stateErr
	}
	return true, time.Since(state.RefreshedAt) > staleAfter, state.EnrichmentStatus, nil
}
