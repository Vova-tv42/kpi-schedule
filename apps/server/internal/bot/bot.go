// Package bot implements the Telegram-facing UI. It runs in-process
// alongside the HTTP API server rather than as a separate service (see
// docs/project-repository.md §4.1) and calls internal/api and
// internal/storage directly instead of looping back through HTTP.
package bot

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

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
// start receiving updates yet — call RegisterWebhook for that. New validates
// the token with a GetMe call unless DisableTokenCheck is set in optional botOpts.
func New(token string, svc *api.Service, db *storage.DB, botOpts ...*gotgbot.BotOpts) (*Bot, error) {
	var opts *gotgbot.BotOpts
	if len(botOpts) > 0 {
		opts = botOpts[0]
	}
	gbot, err := gotgbot.NewBot(token, opts)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}

	b := &Bot{gbot: gbot, svc: svc, db: db}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		MaxRoutines: -1,
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

// WebhookPath is the URL path where the HTTP server receives Telegram updates.
const WebhookPath = "/api/v1/telegram/webhook"

// AddWebhook prepares the updater to receive updates on WebhookPath with the
// given secretToken authentication.
func (b *Bot) AddWebhook(secretToken string) error {
	if err := b.updater.AddWebhook(b.gbot, WebhookPath, &ext.AddWebhookOpts{
		SecretToken: secretToken,
	}); err != nil {
		return fmt.Errorf("adding webhook to updater: %w", err)
	}
	return nil
}

// RegisterWebhook prepares the updater to receive updates on WebhookPath and
// registers the public webhookURL with Telegram's Bot API.
func (b *Bot) RegisterWebhook(webhookURL, secretToken string) error {
	if err := b.AddWebhook(secretToken); err != nil {
		return err
	}

	cleanURL := strings.TrimRight(webhookURL, "/")
	if !strings.HasSuffix(cleanURL, WebhookPath) {
		cleanURL += WebhookPath
	}

	_, err := b.gbot.SetWebhook(cleanURL, &gotgbot.SetWebhookOpts{
		SecretToken:        secretToken,
		AllowedUpdates:     []string{"message", "callback_query"},
		DropPendingUpdates: true,
	})
	if err != nil {
		return fmt.Errorf("setting telegram webhook: %w", err)
	}

	return nil
}

// WebhookHandler returns an http.HandlerFunc that routes incoming Telegram webhook
// updates to the dispatcher, verifying the secret token.
func (b *Bot) WebhookHandler() http.HandlerFunc {
	return b.updater.GetHandlerFunc("/")
}

// Stop shuts the bot down: closes update channels and stops the dispatcher.
// Deliberately does not use updater.Idle(), which installs its own OS-signal
// handler — main.go already owns graceful shutdown, so Stop is just wired into
// that existing signal-wait/shutdown sequence instead of running a second one.
func (b *Bot) Stop() {
	if err := b.updater.Stop(); err != nil {
		slog.Error("stopping telegram bot", "error", err)
	}
}
