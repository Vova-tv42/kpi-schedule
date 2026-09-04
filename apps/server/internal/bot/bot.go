// Package bot implements the Telegram-facing UI. It runs in-process
// alongside the HTTP API server rather than as a separate service (see
// docs/project-repository.md §4.1) and calls internal/api and
// internal/storage directly instead of looping back through HTTP.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/storage"
	"kpi-schedule-bot/server/internal/telemetry"
)

// Bot wraps the Telegram bot's lifecycle: construction, update dispatch, and
// start/stop. See docs/bot/telegram-bot-design.md for the intended UX.
type Bot struct {
	gbot                *gotgbot.Bot
	updater             *ext.Updater
	dispatcher          *ext.Dispatcher
	svc                 *api.Service
	db                  *storage.DB
	extensionInstallURL string
	telemetry           *telemetry.Client
}

// SetTelemetry registers a telemetry client for anonymous action logging.
func (b *Bot) SetTelemetry(t *telemetry.Client) {
	b.telemetry = t
}

func (b *Bot) wrap(actionType, actionName string, fn handlers.Response) handlers.Response {
	return func(bot *gotgbot.Bot, ctx *ext.Context) error {
		start := time.Now()
		err := fn(bot, ctx)
		duration := time.Since(start).Milliseconds()
		status := 200
		if err != nil {
			status = 500
		}
		if b.telemetry != nil {
			b.telemetry.ReportAction(actionType, actionName, status, duration, nil)
		}
		return err
	}
}

// SetExtensionInstallURL overrides the external URL for installing the extension.
func (b *Bot) SetExtensionInstallURL(url string) {
	b.extensionInstallURL = url
}

// ExtensionInstallURL returns the external URL for installing the extension.
func (b *Bot) ExtensionInstallURL() string {
	return b.extensionInstallURL
}

// SetExtensionDownloadURL is an alias for SetExtensionInstallURL for backward compatibility.
func (b *Bot) SetExtensionDownloadURL(url string) {
	b.SetExtensionInstallURL(url)
}

// ExtensionDownloadURL is an alias for ExtensionInstallURL for backward compatibility.
func (b *Bot) ExtensionDownloadURL() string {
	return b.ExtensionInstallURL()
}

// GBot returns the underlying gotgbot.Bot client.
func (b *Bot) GBot() *gotgbot.Bot {
	return b.gbot
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
	dispatcher.AddHandler(handlers.NewCommand("start", b.wrap("telegram_command", "/start", b.cmdStart)))
	dispatcher.AddHandler(handlers.NewCommand("install", b.wrap("telegram_command", "/install", b.cmdInstall)))
	dispatcher.AddHandler(handlers.NewCommand("link", b.wrap("telegram_command", "/link", b.cmdLink)))
	dispatcher.AddHandler(handlers.NewCommand("today", b.wrap("telegram_command", "/today", b.cmdToday)))
	dispatcher.AddHandler(handlers.NewCommand("week", b.wrap("telegram_command", "/week", b.cmdWeek)))
	dispatcher.AddHandler(handlers.NewCommand("urls", b.wrap("telegram_command", "/urls", b.cmdURLs)))
	dispatcher.AddHandler(handlers.NewCommand("group", b.wrap("telegram_command", "/group", b.cmdGroup)))
	dispatcher.AddHandler(handlers.NewCommand("group_today", b.wrap("telegram_command", "/group_today", b.cmdGroupToday)))
	dispatcher.AddHandler(handlers.NewCommand("group_week", b.wrap("telegram_command", "/group_week", b.cmdGroupWeek)))
	dispatcher.AddHandler(handlers.NewCommand("settings", b.wrap("telegram_command", "/settings", b.cmdSettings)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(navCallbackPrefix), b.wrap("telegram_callback", "nav", b.onNav)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(weekCallbackPrefix), b.wrap("telegram_callback", "week", b.onWeek)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(menuCallbackPrefix), b.wrap("telegram_callback", "menu", b.onMenu)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(urlsCallbackPrefix), b.wrap("telegram_callback", "urls", b.onURLs)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(groupCallbackPrefix), b.wrap("telegram_callback", "group", b.onGroup)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(groupNavCallbackPrefix), b.wrap("telegram_callback", "group_nav", b.onGroupNav)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(groupWeekCallbackPrefix), b.wrap("telegram_callback", "group_week", b.onGroupWeek)))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("settings:"), b.wrap("telegram_callback", "settings", b.onSettings)))
	dispatcher.AddHandler(handlers.NewMessage(message.All, b.wrap("telegram_message", "text", b.onTextMessage)))

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
		{Command: "settings", Description: "Налаштування сповіщень"},
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

	adminCommands := []gotgbot.BotCommand{
		{Command: "today", Description: "Показати персональний розклад на сьогодні"},
		{Command: "week", Description: "Показати персональний розклад на тиждень"},
		{Command: "group_today", Description: "Показати розклад групи на сьогодні"},
		{Command: "group_week", Description: "Показати розклад групи на тиждень"},
		{Command: "group", Description: "Керування академічною групою"},
	}
	if _, err := b.gbot.SetMyCommands(adminCommands, &gotgbot.SetMyCommandsOpts{
		Scope: gotgbot.BotCommandScopeAllChatAdministrators{},
	}); err != nil {
		slog.Warn("setting chat administrators commands", "error", err)
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
// registers the public webhookURL with Telegram's Bot API. To prevent cold-start
// latency and avoid dropping pending updates when waking up on Fly.io, it checks
// whether the webhook is already registered before issuing Telegram API calls.
func (b *Bot) RegisterWebhook(webhookURL, secretToken string) error {
	cleanURL := strings.TrimRight(webhookURL, "/")
	if !strings.HasSuffix(cleanURL, WebhookPath) {
		cleanURL += WebhookPath
	}

	if err := b.AddWebhook(secretToken); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. Check local persistent disk cache (fastest: < 1ms on wake)
	if b.db != nil {
		var cachedURL string
		if ok, _ := b.db.CacheGet(ctx, "telegram_webhook_registration", 30*24*time.Hour, &cachedURL); ok && cachedURL == cleanURL {
			slog.Info("telegram webhook already registered (cached)", "url", cleanURL)
			return nil
		}
	}

	// 2. Check Telegram API if cache was missing or stale
	info, err := b.gbot.GetWebhookInfo(&gotgbot.GetWebhookInfoOpts{
		RequestOpts: &gotgbot.RequestOpts{Timeout: 3 * time.Second},
	})
	if err == nil && info != nil && info.Url == cleanURL {
		slog.Info("telegram webhook verified with telegram", "url", cleanURL)
		if b.db != nil {
			_ = b.db.CacheSet(ctx, "telegram_webhook_registration", cleanURL)
		}
		return nil
	}

	// 3. Setup commands and register webhook if URL changed or unset
	_ = b.SetupCommands()

	_, err = b.gbot.SetWebhook(cleanURL, &gotgbot.SetWebhookOpts{
		SecretToken:        secretToken,
		AllowedUpdates:     []string{"message", "callback_query"},
		DropPendingUpdates: false,
		RequestOpts:        &gotgbot.RequestOpts{Timeout: 5 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("setting telegram webhook: %w", err)
	}

	if b.db != nil {
		_ = b.db.CacheSet(ctx, "telegram_webhook_registration", cleanURL)
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
