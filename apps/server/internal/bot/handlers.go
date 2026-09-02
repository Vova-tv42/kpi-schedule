package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"kpi-schedule-bot/server/internal/api"
)

const genericErrorText = "⚠️ Щось пішло не так. Спробуй ще раз трохи пізніше."

// renderDay resolves telegramID's user and schedule for date, returning a
// ready-to-send message. err is non-nil only for unexpected failures the
// caller should log and surface generically — "not linked yet" and "no
// schedule synced yet" are expected states, reported via text/hasKeyboard
// instead of err.
func (b *Bot) renderDay(ctx context.Context, telegramID int64, date time.Time) (text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool, err error) {
	user, uErr := b.resolveUser(ctx, telegramID)
	if uErr != nil {
		if errors.Is(uErr, ErrNotLinked) {
			return "🔒 Акаунт ще не прив'язано. Надішли /link, щоб отримати код і синхронізувати браузерне розширення.", gotgbot.InlineKeyboardMarkup{}, false, nil
		}
		return "", gotgbot.InlineKeyboardMarkup{}, false, uErr
	}

	view, vErr := b.svc.BuildDay(ctx, user, date)
	if vErr != nil {
		if errors.Is(vErr, api.ErrNoScheduleData) {
			return "📭 Розкладу ще немає. Синхронізуй браузерне розширення (після /link) і спробуй ще раз.", gotgbot.InlineKeyboardMarkup{}, false, nil
		}
		return "", gotgbot.InlineKeyboardMarkup{}, false, vErr
	}

	lines := make([]lessonLine, 0, len(view.Lessons))
	for _, l := range view.Lessons {
		line := lessonLine{
			Time:        l.Time,
			EndTime:     l.EndTime,
			Name:        l.Name,
			Tag:         l.Tag,
			TeacherRaw:  l.TeacherRaw,
			LocationRaw: l.LocationRaw,
			Enriched:    l.Enriched,
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
		Week:             view.Week,
		DayName:          view.DayName,
		IsDayOff:         view.IsDayOff,
		EnrichmentStatus: view.EnrichmentStatus,
		Stale:            view.Stale,
		Lessons:          lines,
	}

	return formatDay(info, user.GroupName), dayKeyboard(date), true, nil
}

func (b *Bot) cmdStart(bot *gotgbot.Bot, ctx *ext.Context) error {
	if _, err := b.upsertUser(context.Background(), ctx.EffectiveUser.Id); err != nil {
		return fmt.Errorf("upserting user on /start: %w", err)
	}

	text := "👋 Вітаю! Я покажу твій персональний розклад занять КПІ.\n\n" +
		"Розклад поєднує дві частини: твої обрані дисципліни з my.kpi.ua та дані групи з Campus API. " +
		"Щоб я міг їх зв'язати, спочатку прив'яжи акаунт:\n\n" +
		"1️⃣ Надішли /link — отримаєш код прив'язки.\n" +
		"2️⃣ Відкрий браузерне розширення, увійди на my.kpi.ua і введи код.\n" +
		"3️⃣ Надішли /today — побачиш розклад із кнопками навігації."

	_, err := bot.SendMessage(ctx.EffectiveChat.Id, text, nil)
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

	formattedCode := code[:3] + "-" + code[3:]
	text := fmt.Sprintf(
		"🔑 Код прив'язки: <code>%s</code>\n\nДійсний %d хвилин. Відкрий браузерне розширення KPI Schedule, увійди на my.kpi.ua і введи цей код, щоб синхронізувати розклад.",
		formattedCode, expiresIn/60,
	)
	_, err = bot.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	return err
}

func (b *Bot) cmdToday(bot *gotgbot.Bot, ctx *ext.Context) error {
	text, kb, hasKeyboard, err := b.renderDay(context.Background(), ctx.EffectiveUser.Id, time.Now())
	if err != nil {
		slog.Error("rendering /today", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}

	opts := &gotgbot.SendMessageOpts{ParseMode: "HTML"}
	if hasKeyboard {
		opts.ReplyMarkup = kb
	}
	_, err = bot.SendMessage(ctx.EffectiveChat.Id, text, opts)
	return err
}
