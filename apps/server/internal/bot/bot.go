// Package bot implements the Telegram-facing UI. It runs in-process
// alongside the HTTP API server rather than as a separate service (see
// docs/project-repository.md §4.1) and calls internal/api and
// internal/storage directly instead of looping back through HTTP.
package bot

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/storage"
)

// Bot wraps the Telegram bot's lifecycle: construction, update dispatch, and
// start/stop. See docs/bot/telegram-bot-design.md for the intended UX.
type Bot struct {
	gbot       *gotgbot.Bot
	updater    *ext.Updater
	dispatcher *ext.Dispatcher
	svc        *api.Service
	db         *storage.DB
}

// New builds a Bot and registers its command/callback handlers. It does not
// start receiving updates yet — call StartPolling for that. NewBot validates
// the token with a GetMe call, so a bad token fails here, loudly.
func New(token string, svc *api.Service, db *storage.DB) (*Bot, error) {
	gbot, err := gotgbot.NewBot(token, nil)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}

	b := &Bot{gbot: gbot, svc: svc, db: db}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(_ *gotgbot.Bot, _ *ext.Context, err error) ext.DispatcherAction {
			slog.Error("telegram handler error", "error", err)
			return ext.DispatcherActionNoop
		},
	})
	dispatcher.AddHandler(handlers.NewCommand("start", b.cmdStart))
	dispatcher.AddHandler(handlers.NewCommand("link", b.cmdLink))
	dispatcher.AddHandler(handlers.NewCommand("today", b.cmdToday))
	dispatcher.AddHandler(handlers.NewCommand("week", b.cmdWeek))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(navCallbackPrefix), b.onNav))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(weekCallbackPrefix), b.onWeek))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(menuCallbackPrefix), b.onMenu))

	b.dispatcher = dispatcher
	b.updater = ext.NewUpdater(dispatcher, nil)

	return b, nil
}

// pollTimeoutSeconds is how long we ask Telegram to hold each getUpdates
// call open server-side while waiting for a new update (the actual
// "long" part of long polling).
const pollTimeoutSeconds = 9

// StartPolling begins long-polling for updates. This is the local-dev
// delivery mode; production is meant to use a webhook instead (not
// implemented yet — see docs/bot/telegram-bot-design.md). Non-blocking: the
// updater runs polling in its own background goroutine.
func (b *Bot) StartPolling() error {
	return b.updater.StartPolling(b.gbot, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout:        pollTimeoutSeconds,
			AllowedUpdates: []string{"message", "callback_query"},
			// The HTTP client's own request timeout must exceed
			// pollTimeoutSeconds, or it cancels the request before
			// Telegram's long-poll window can resolve server-side —
			// gotgbot's default RequestOpts.Timeout is only 5s.
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: (pollTimeoutSeconds + 5) * time.Second,
			},
		},
	})
}

// Stop shuts the bot down. Deliberately does not use updater.Idle(), which
// installs its own OS-signal handler — main.go already owns graceful
// shutdown, so Stop is just wired into that existing signal-wait/shutdown
// sequence instead of running a second one.
func (b *Bot) Stop() {
	if err := b.updater.Stop(); err != nil {
		slog.Error("stopping telegram bot", "error", err)
	}
}
