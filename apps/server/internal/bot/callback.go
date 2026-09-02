package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Each screen namespaces its buttons with its own CallbackData prefix, so one
// dispatcher handler is registered per screen rather than one that has to
// demultiplex every button in the bot.
const (
	navCallbackPrefix  = "nav:"  // day screen: prev / today / next
	weekCallbackPrefix = "week:" // week screen: week slots + jump to today
	menuCallbackPrefix = "menu:" // onboarding screens: link / back / week
)

// navCallbackData encodes an action ("prev"/"next"/"today") plus the
// *currently displayed* date. No message state is persisted server-side —
// the callback carries everything needed to re-derive the target date, per
// docs/bot/telegram-bot-design.md §5.
func navCallbackData(action string, date time.Time) string {
	return fmt.Sprintf("%s%s:%s", navCallbackPrefix, action, date.Format("2006-01-02"))
}

// weekCallbackData targets a week by its offset from the real current week
// (-1/0/+1), not by an offset relative to what is on screen — the three week
// buttons are fixed slots, so navigation never drifts further than one week
// out from today.
func weekCallbackData(offset int) string {
	return fmt.Sprintf("%sgoto:%d", weekCallbackPrefix, offset)
}

func weekNoopCallbackData() string { return weekCallbackPrefix + "noop" }

func weekTodayCallbackData() string { return weekCallbackPrefix + "today" }

func menuCallbackData(action string) string { return menuCallbackPrefix + action }

// onNav handles the day screen's ◀️/📅/▶️ row.
func (b *Bot) onNav(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery

	parts := strings.SplitN(strings.TrimPrefix(cq.Data, navCallbackPrefix), ":", 2)
	if len(parts) != 2 {
		return answerSilently(bot, cq)
	}
	action, dateStr := parts[0], parts[1]

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return answerSilently(bot, cq)
	}

	switch action {
	case "prev":
		date = date.AddDate(0, 0, -1)
	case "next":
		date = date.AddDate(0, 0, 1)
	case "today":
		date = time.Now()
	default:
		return answerSilently(bot, cq)
	}

	return b.editToDay(bot, cq, date)
}

// onWeek handles the week screen's week slots and its jump to today.
func (b *Bot) onWeek(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	action := strings.TrimPrefix(cq.Data, weekCallbackPrefix)

	switch {
	case action == "today":
		return b.editToDay(bot, cq, time.Now())
	case strings.HasPrefix(action, "goto:"):
		offset, err := strconv.Atoi(strings.TrimPrefix(action, "goto:"))
		if err != nil {
			return answerSilently(bot, cq)
		}
		return b.editToWeek(bot, cq, offset)
	default:
		// "noop": the slot already on screen. Just clear the spinner.
		return answerSilently(bot, cq)
	}
}

// onMenu handles the onboarding screens' buttons.
func (b *Bot) onMenu(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery

	switch strings.TrimPrefix(cq.Data, menuCallbackPrefix) {
	case "link":
		return b.editToLinkScreen(bot, cq)
	case "back":
		return b.editToStartScreen(bot, cq)
	case "week":
		return b.editToWeek(bot, cq, 0)
	case "today":
		return b.editToDay(bot, cq, time.Now())
	default:
		return answerSilently(bot, cq)
	}
}

func (b *Bot) editToDay(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, date time.Time) error {
	text, kb, hasKeyboard, err := b.renderDay(context.Background(), cq.From.Id, date)
	if err != nil {
		slog.Error("rendering day callback", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}
	return b.applyScreen(bot, cq, text, kb, hasKeyboard)
}

func (b *Bot) editToWeek(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, offset int) error {
	text, kb, hasKeyboard, err := b.renderWeek(context.Background(), cq.From.Id, offset)
	if err != nil {
		slog.Error("rendering week callback", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}
	return b.applyScreen(bot, cq, text, kb, hasKeyboard)
}

func (b *Bot) editToStartScreen(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery) error {
	state, err := b.resolveLinkState(context.Background(), cq.From.Id)
	if err != nil {
		slog.Error("resolving link state for back button", "error", err, "telegram_id", cq.From.Id)
		state = linkStateNone
	}
	return b.applyScreen(bot, cq, formatStartScreen(state), startKeyboard(state), true)
}

func (b *Bot) editToLinkScreen(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery) error {
	code, expiresIn, err := b.svc.GeneratePairCode(context.Background(), cq.From.Id)
	if err != nil {
		slog.Error("generating pair code", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}
	return b.applyScreen(bot, cq, formatLinkText(code, expiresIn), linkKeyboard(), true)
}

// applyScreen swaps the tapped message over to a new screen in place — never
// sending a new message — and always clears the button's loading spinner.
func (b *Bot) applyScreen(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool) error {
	msg := cq.Message
	opts := &gotgbot.EditMessageTextOpts{
		ChatId:    msg.GetChat().Id,
		MessageId: msg.GetMessageId(),
		Text:      text,
		ParseMode: "HTML",
	}
	if hasKeyboard {
		opts.ReplyMarkup = kb
	}
	if _, _, err := bot.EditMessageText(opts); err != nil && !isNotModified(err) {
		return fmt.Errorf("editing message for callback: %w", err)
	}
	return answerSilently(bot, cq)
}

// isNotModified reports the 400 Telegram returns when an edit would leave the
// message byte-identical — which is the normal outcome of tapping 📅 Сьогодні
// while today is already on screen, not a failure.
func isNotModified(err error) bool {
	var tgErr *gotgbot.TelegramError
	return errors.As(err, &tgErr) && strings.Contains(tgErr.Description, "message is not modified")
}

func answerSilently(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery) error {
	_, err := bot.AnswerCallbackQuery(cq.Id, nil)
	return err
}

func answerWithError(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery) error {
	_, err := bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
		Text:      "⚠️ Помилка, спробуй пізніше",
		ShowAlert: true,
	})
	return err
}
