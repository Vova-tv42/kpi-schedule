package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// navCallbackPrefix namespaces the day-navigation buttons' CallbackData so
// the callback handler only fires for them, not future button types.
const navCallbackPrefix = "nav:"

// navCallbackData encodes an action ("prev"/"next"/"refresh") plus the
// *currently displayed* date. No message state is persisted server-side —
// the callback carries everything needed to re-derive the target date, per
// docs/bot/telegram-bot-design.md §5.
func navCallbackData(action string, date time.Time) string {
	return fmt.Sprintf("%s%s:%s", navCallbackPrefix, action, date.Format("2006-01-02"))
}

// onNav handles taps on the ◀️/🔄/▶️ day-navigation row by editing the
// existing message in place (never sending a new one) and clearing the
// button's loading spinner via AnswerCallbackQuery.
func (b *Bot) onNav(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	reqCtx := context.Background()

	parts := strings.SplitN(strings.TrimPrefix(cq.Data, navCallbackPrefix), ":", 2)
	if len(parts) != 2 {
		_, err := bot.AnswerCallbackQuery(cq.Id, nil)
		return err
	}
	action, dateStr := parts[0], parts[1]

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		_, aErr := bot.AnswerCallbackQuery(cq.Id, nil)
		return aErr
	}

	switch action {
	case "prev":
		date = date.AddDate(0, 0, -1)
	case "next":
		date = date.AddDate(0, 0, 1)
	case "refresh":
		// keep date as-is
	}

	text, kb, hasKeyboard, err := b.renderDay(reqCtx, cq.From.Id, date)
	if err != nil {
		slog.Error("rendering nav callback", "error", err, "telegram_id", cq.From.Id)
		_, aErr := bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
			Text:      "⚠️ Помилка, спробуй пізніше",
			ShowAlert: true,
		})
		return aErr
	}

	msg := cq.Message
	editOpts := &gotgbot.EditMessageTextOpts{
		ChatId:    msg.GetChat().Id,
		MessageId: msg.GetMessageId(),
		Text:      text,
		ParseMode: "HTML",
	}
	if hasKeyboard {
		editOpts.ReplyMarkup = kb
	}
	if _, _, err := bot.EditMessageText(editOpts); err != nil {
		return fmt.Errorf("editing message for nav callback: %w", err)
	}

	answerOpts := &gotgbot.AnswerCallbackQueryOpts{}
	if action == "refresh" {
		answerOpts.Text = "Оновлено ✅"
	}
	_, err = bot.AnswerCallbackQuery(cq.Id, answerOpts)
	return err
}
