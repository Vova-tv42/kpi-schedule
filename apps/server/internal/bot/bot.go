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
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/storage"
)

// Bot wraps the Telegram bot's lifecycle: construction, update dispatch, and
// start/stop. See docs/bot/telegram-bot-design.md for the intended UX.
type Bot struct {
	gbot                 *gotgbot.Bot
	updater              *ext.Updater
	dispatcher           *ext.Dispatcher
	svc                  *api.Service
	db                   *storage.DB
	extensionDownloadURL string
}

// SetExtensionDownloadURL overrides the public URL for downloading the extension.
func (b *Bot) SetExtensionDownloadURL(url string) {
	b.extensionDownloadURL = url
}

// ExtensionDownloadURL returns the public URL for downloading the extension.
func (b *Bot) ExtensionDownloadURL() string {
	return b.extensionDownloadURL
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
	dispatcher.AddHandler(handlers.NewCommand("install", b.cmdInstall))
	dispatcher.AddHandler(handlers.NewCommand("link", b.cmdLink))
	dispatcher.AddHandler(handlers.NewCommand("today", b.cmdToday))
	dispatcher.AddHandler(handlers.NewCommand("week", b.cmdWeek))
	dispatcher.AddHandler(handlers.NewCommand("urls", b.cmdURLs))
	dispatcher.AddHandler(handlers.NewCommand("group", b.cmdGroup))
	dispatcher.AddHandler(handlers.NewCommand("group_today", b.cmdGroupToday))
	dispatcher.AddHandler(handlers.NewCommand("group_week", b.cmdGroupWeek))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(navCallbackPrefix), b.onNav))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(weekCallbackPrefix), b.onWeek))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(menuCallbackPrefix), b.onMenu))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(urlsCallbackPrefix), b.onURLs))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(groupCallbackPrefix), b.onGroup))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(groupNavCallbackPrefix), b.onGroupNav))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(groupWeekCallbackPrefix), b.onGroupWeek))
	dispatcher.AddHandler(handlers.NewMessage(message.All, b.onTextMessage))

	b.dispatcher = dispatcher
	b.updater = ext.NewUpdater(dispatcher, nil)

	return b, nil
}

// SetupCommands registers command menus separately for private and group chats.
func (b *Bot) SetupCommands() error {
	privateCommands := []gotgbot.BotCommand{
		{Command: "today", Description: "Показати розклад на сьогодні"},
		{Command: "week", Description: "Показати розклад на тиждень"},
		{Command: "urls", Description: "Посилання на онлайн-заняття"},
		{Command: "group", Description: "Керування академічними групами"},
		{Command: "install", Description: "Інструкція та завантаження розширення"},
		{Command: "link", Description: "Отримати код прив'язки браузерного розширення"},
		{Command: "start", Description: "Знайомство та головне меню"},
	}
	if _, err := b.gbot.SetMyCommands(privateCommands, &gotgbot.SetMyCommandsOpts{
		Scope: gotgbot.BotCommandScopeAllPrivateChats{},
	}); err != nil {
		slog.Warn("setting private chat commands", "error", err)
	}

	groupCommands := []gotgbot.BotCommand{
		{Command: "today", Description: "Показати персональний розклад на сьогодні"},
		{Command: "week", Description: "Показати персональний розклад на тиждень"},
		{Command: "group_today", Description: "Показати розклад групи на сьогодні"},
		{Command: "group_week", Description: "Показати розклад групи на тиждень"},
	}
	if _, err := b.gbot.SetMyCommands(groupCommands, &gotgbot.SetMyCommandsOpts{
		Scope: gotgbot.BotCommandScopeAllGroupChats{},
	}); err != nil {
		slog.Warn("setting group chat commands", "error", err)
	}

	return nil
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
	_ = b.SetupCommands()

	if err := b.AddWebhook(secretToken); err != nil {
		return err
	}

	cleanURL := strings.TrimRight(webhookURL, "/")
	if b.extensionDownloadURL == "" {
		baseURL := strings.TrimSuffix(cleanURL, WebhookPath)
		b.extensionDownloadURL = baseURL + "/api/v1/extension/download"
	}
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
