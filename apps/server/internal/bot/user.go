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
