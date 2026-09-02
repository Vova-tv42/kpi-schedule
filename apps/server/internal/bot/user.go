package bot

import (
	"context"
	"errors"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// ErrNotLinked means this Telegram user has never run /link + paired the
// browser extension, so no users row exists for them yet.
var ErrNotLinked = errors.New("telegram user has not linked an account yet")

// upsertUser ensures a users row exists for telegramID, creating one with no
// group binding yet on first contact (e.g. /start).
func (b *Bot) upsertUser(ctx context.Context, telegramID int64) (model.User, error) {
	return b.db.UpsertUser(ctx, telegramID, nil, nil)
}

// linkState is what the onboarding screen needs to know about a user: not
// whether a users row exists (one is created on first /start), but whether the
// browser extension has actually pushed a schedule, and how recently.
type linkState int

const (
	linkStateNone  linkState = iota // no schedule pushed yet
	linkStateStale                  // pushed, but not refreshed recently
	linkStateFresh                  // pushed and current
)

// resolveLinkState never fails softly into an error path for the caller: an
// unlinked user is a normal state, reported as linkStateNone.
func (b *Bot) resolveLinkState(ctx context.Context, telegramID int64) (linkState, error) {
	user, err := b.resolveUser(ctx, telegramID)
	if err != nil {
		if errors.Is(err, ErrNotLinked) {
			return linkStateNone, nil
		}
		return linkStateNone, err
	}

	hasData, stale, _, err := b.svc.ScheduleFreshness(ctx, user)
	switch {
	case err != nil:
		return linkStateNone, err
	case !hasData:
		return linkStateNone, nil
	case stale:
		return linkStateStale, nil
	default:
		return linkStateFresh, nil
	}
}

// resolveUser looks up an existing user by Telegram ID, returning
// ErrNotLinked (not a hard error) when none exists yet.
func (b *Bot) resolveUser(ctx context.Context, telegramID int64) (model.User, error) {
	user, err := b.db.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return model.User{}, ErrNotLinked
		}
		return model.User{}, err
	}
	return user, nil
}
