package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"kpi-schedule-bot/server/internal/api"
)

const (
	genericErrorText = "⚠️ Щось пішло не так. Спробуй ще раз трохи пізніше."
	notLinkedText    = "🔒 Акаунт ще не прив'язано. Надішли /start, щоб отримати код і синхронізувати браузерне розширення."
	noScheduleText   = "📭 Розкладу ще немає. Синхронізуй браузерне розширення (після прив'язки) і спробуй ще раз."
)

// renderDay resolves telegramID's user and schedule for date, returning a
// ready-to-send message. err is non-nil only for unexpected failures the
// caller should log and surface generically — "not linked yet" and "no
// schedule synced yet" are expected states, reported via text/hasKeyboard
// instead of err.
func (b *Bot) renderDay(ctx context.Context, telegramID int64, date time.Time) (text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool, err error) {
	user, uErr := b.resolveUser(ctx, telegramID)
	if uErr != nil {
		if errors.Is(uErr, ErrNotLinked) {
			return notLinkedText, gotgbot.InlineKeyboardMarkup{}, false, nil
		}
		return "", gotgbot.InlineKeyboardMarkup{}, false, uErr
	}

	view, vErr := b.svc.BuildDay(ctx, user, date)
	if vErr != nil {
		if errors.Is(vErr, api.ErrNoScheduleData) {
			return noScheduleText, gotgbot.InlineKeyboardMarkup{}, false, nil
		}
		return "", gotgbot.InlineKeyboardMarkup{}, false, vErr
	}

	lines := make([]lessonLine, 0, len(view.Lessons))
	for _, l := range view.Lessons {
		line := lessonLine{
			Time:        l.Time,
			Name:        l.Name,
			Tag:         l.Tag,
			TeacherRaw:  l.TeacherRaw,
			LocationRaw: l.LocationRaw,
			URL:         l.URL,
		}
		if l.Lecturer != nil {
			line.LecturerName = l.Lecturer.Name
		}
		if l.Location != nil {
			line.LocationTitle = l.Location.Title
		}
		lines = append(lines, line)
	}

	info := dayInfo{
		Date:             view.Date,
		DayName:          view.DayName,
		IsDayOff:         view.IsDayOff,
		EnrichmentStatus: view.EnrichmentStatus,
		Stale:            view.Stale,
		Lessons:          lines,
	}

	return formatDay(info), dayKeyboard(date), true, nil
}

// renderWeek is renderDay's counterpart for the week screen. offset is in
// calendar weeks from today (-1/0/+1); the parity BuildWeek needs is derived
// from the date that offset lands on, so the two-week rotation stays the
// service's concern rather than the bot's.
func (b *Bot) renderWeek(ctx context.Context, telegramID int64, offset int) (text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool, err error) {
	user, uErr := b.resolveUser(ctx, telegramID)
	if uErr != nil {
		if errors.Is(uErr, ErrNotLinked) {
			return notLinkedText, gotgbot.InlineKeyboardMarkup{}, false, nil
		}
		return "", gotgbot.InlineKeyboardMarkup{}, false, uErr
	}

	parity, pErr := b.svc.ResolveWeekParity(ctx, time.Now().AddDate(0, 0, 7*offset))
	if pErr != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, false, pErr
	}

	view, vErr := b.svc.BuildWeek(ctx, user, parity)
	if vErr != nil {
		if errors.Is(vErr, api.ErrNoScheduleData) {
			return noScheduleText, gotgbot.InlineKeyboardMarkup{}, false, nil
		}
		return "", gotgbot.InlineKeyboardMarkup{}, false, vErr
	}
	if len(view.Weeks) == 0 {
		return "", gotgbot.InlineKeyboardMarkup{}, false, fmt.Errorf("no week block built for parity %d", parity)
	}
	block := view.Weeks[0]

	days := make([]weekDayLine, 0, len(block.Days))
	for _, d := range block.Days {
		lessons := make([]lessonLine, 0, len(d.Lessons))
		for _, l := range d.Lessons {
			line := lessonLine{
				Time:        l.Time,
				Name:        l.Name,
				Tag:         l.Tag,
				TeacherRaw:  l.TeacherRaw,
				LocationRaw: l.LocationRaw,
				URL:         l.URL,
			}
			if l.Lecturer != nil {
				line.LecturerName = l.Lecturer.Name
			}
			if l.Location != nil {
				line.LocationTitle = l.Location.Title
			}
			lessons = append(lessons, line)
		}
		days = append(days, weekDayLine{DayName: d.DayName, Lessons: lessons})
	}

	info := weekInfo{
		WeekNumber:       block.WeekNumber,
		EnrichmentStatus: view.EnrichmentStatus,
		Stale:            view.Stale,
		Days:             days,
	}

	return formatWeek(info, offset, user.GroupName), weekKeyboard(offset), true, nil
}

func (b *Bot) cmdStart(bot *gotgbot.Bot, ctx *ext.Context) error {
	reqCtx := context.Background()

	if _, err := b.upsertUser(reqCtx, ctx.EffectiveUser.Id); err != nil {
		return fmt.Errorf("upserting user on /start: %w", err)
	}

	state, err := b.resolveLinkState(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		// A status lookup failing is no reason to withhold onboarding: fall
		// back to the plain first-time screen.
		slog.Error("resolving link state for /start", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		state = linkStateNone
	}

	_, err = bot.SendMessage(ctx.EffectiveChat.Id, formatStartScreen(state), &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: startKeyboard(state),
	})
	return err
}

func (b *Bot) cmdLink(bot *gotgbot.Bot, ctx *ext.Context) error {
	reqCtx := context.Background()

	if _, err := b.upsertUser(reqCtx, ctx.EffectiveUser.Id); err != nil {
		return fmt.Errorf("upserting user on /link: %w", err)
	}

	code, expiresIn, err := b.svc.GeneratePairCode(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		slog.Error("generating pair code", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}

	_, err = bot.SendMessage(ctx.EffectiveChat.Id, formatLinkText(code, expiresIn), &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: linkKeyboard(),
	})
	return err
}

func (b *Bot) cmdToday(bot *gotgbot.Bot, ctx *ext.Context) error {
	text, kb, hasKeyboard, err := b.renderDay(context.Background(), ctx.EffectiveUser.Id, time.Now())
	if err != nil {
		slog.Error("rendering /today", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}
	return sendScreen(bot, ctx.EffectiveChat.Id, text, kb, hasKeyboard)
}

func (b *Bot) cmdWeek(bot *gotgbot.Bot, ctx *ext.Context) error {
	text, kb, hasKeyboard, err := b.renderWeek(context.Background(), ctx.EffectiveUser.Id, 0)
	if err != nil {
		slog.Error("rendering /week", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}
	return sendScreen(bot, ctx.EffectiveChat.Id, text, kb, hasKeyboard)
}

func (b *Bot) cmdURLs(bot *gotgbot.Bot, ctx *ext.Context) error {
	reqCtx := context.Background()

	user, err := b.resolveUser(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		if errors.Is(err, ErrNotLinked) {
			_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, notLinkedText, nil)
			return sendErr
		}
		slog.Error("resolving user for /urls", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}

	_ = b.db.ClearURLPrompt(reqCtx, ctx.EffectiveUser.Id)

	lessons, err := b.db.GetUniqueScheduleLessons(reqCtx, user.ID)
	if err != nil {
		slog.Error("getting unique lessons for /urls", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}

	text := formatLessonsMenu(lessons, "")
	kb := urlsKeyboard(lessons)

	_, err = bot.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: kb,
	})
	return err
}

func (b *Bot) onTextMessage(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return nil
	}

	reqCtx := context.Background()
	prompt, err := b.db.GetURLPrompt(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		slog.Error("checking url prompt", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		return nil
	}
	if prompt == nil {
		return nil
	}

	// Delete user message immediately to avoid chat pollution
	if _, err := bot.DeleteMessage(ctx.EffectiveChat.Id, msg.MessageId, nil); err != nil {
		slog.Warn("could not delete user url message", "error", err, "chat_id", ctx.EffectiveChat.Id, "message_id", msg.MessageId)
	}

	if strings.HasPrefix(msg.Text, "/") {
		_ = b.db.ClearURLPrompt(reqCtx, ctx.EffectiveUser.Id)
		return nil
	}

	rawURL := strings.TrimSpace(msg.Text)
	hash := lessonHash(prompt.SubjectNorm, prompt.Tag)
	if !isValidURL(rawURL) {
		text := formatURLPrompt(prompt.SubjectName, prompt.Tag, "", "Некоректне посилання. Будь ласка, надішли дійсне посилання (наприклад: https://zoom.us/j/...):")
		kb := urlPromptKeyboard(false, hash)
		opts := &gotgbot.EditMessageTextOpts{
			ChatId:      ctx.EffectiveChat.Id,
			MessageId:   prompt.PromptMessageID,
			Text:        text,
			ParseMode:   "HTML",
			ReplyMarkup: kb,
		}
		_, _, _ = bot.EditMessageText(opts)
		return nil
	}

	if err := b.db.SetLessonURL(reqCtx, prompt.UserID, prompt.SubjectNorm, prompt.Tag, rawURL); err != nil {
		slog.Error("saving lesson url", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		return nil
	}
	_ = b.db.ClearURLPrompt(reqCtx, ctx.EffectiveUser.Id)

	lessons, err := b.db.GetUniqueScheduleLessons(reqCtx, prompt.UserID)
	if err != nil {
		slog.Error("fetching unique lessons after url save", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		return nil
	}

	notice := fmt.Sprintf("✅ Посилання для «<b>%s (%s)</b>» збережено!", html.EscapeString(prompt.SubjectName), tagAbbr(prompt.Tag))
	text := formatLessonsMenu(lessons, notice)
	kb := urlsKeyboard(lessons)

	opts := &gotgbot.EditMessageTextOpts{
		ChatId:      ctx.EffectiveChat.Id,
		MessageId:   prompt.PromptMessageID,
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: kb,
	}
	_, _, _ = bot.EditMessageText(opts)
	return nil
}

func isValidURL(raw string) bool {
	if len(raw) < 10 || len(raw) > 2048 {
		return false
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	if u.Host == "" || !strings.Contains(u.Host, ".") {
		return false
	}
	return true
}

// sendScreen posts a screen as a new message. Only typed commands do this —
// button taps edit the existing message instead (see applyScreen).
func sendScreen(bot *gotgbot.Bot, chatID int64, text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool) error {
	opts := &gotgbot.SendMessageOpts{ParseMode: "HTML"}
	if hasKeyboard {
		opts.ReplyMarkup = kb
	}
	_, err := bot.SendMessage(chatID, text, opts)
	return err
}

