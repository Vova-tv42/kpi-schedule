package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

const (
	genericErrorText = "⚠️ Щось пішло не так. Спробуй ще раз трохи пізніше."
	notLinkedText    = "🔒 Акаунт ще не прив'язано. Надішли /start, щоб отримати код і синхронізувати браузерне розширення."
	noScheduleText   = "📭 Розкладу ще немає. Синхронізуй браузерне розширення (після прив'язки) і спробуй ще раз."
)

func isGroupChat(chat *gotgbot.Chat) bool {
	if chat == nil {
		return false
	}
	return chat.Type != "private"
}

func isChatAdmin(bot *gotgbot.Bot, chatID, userID int64) bool {
	member, err := bot.GetChatMember(chatID, userID, nil)
	if err == nil {
		st := member.GetStatus()
		return st == "administrator" || st == "creator"
	}
	admins, aErr := bot.GetChatAdministrators(chatID, nil)
	if aErr == nil {
		for _, a := range admins {
			if a.GetUser().Id == userID {
				return true
			}
		}
	}
	return false
}

// renderDay resolves telegramID's user and schedule for date, returning a
// ready-to-send message.
func (b *Bot) renderDay(ctx context.Context, telegramID int64, date time.Time, callerName string) (text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool, err error) {
	user, uErr := b.resolveUser(ctx, telegramID)
	if uErr != nil {
		if errors.Is(uErr, ErrNotLinked) {
			return notLinkedText, gotgbot.InlineKeyboardMarkup{}, false, ErrNotLinked
		}
		return "", gotgbot.InlineKeyboardMarkup{}, false, uErr
	}

	view, vErr := b.svc.BuildDay(ctx, user, date)
	if vErr != nil {
		if errors.Is(vErr, api.ErrNoScheduleData) {
			return noScheduleText, gotgbot.InlineKeyboardMarkup{}, false, api.ErrNoScheduleData
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
		CallerName:       callerName,
		Lessons:          lines,
	}

	return formatDay(info), dayKeyboard(date), true, nil
}

// renderWeek is renderDay's counterpart for the week screen.
func (b *Bot) renderWeek(ctx context.Context, telegramID int64, offset int, callerName string) (text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool, err error) {
	user, uErr := b.resolveUser(ctx, telegramID)
	if uErr != nil {
		if errors.Is(uErr, ErrNotLinked) {
			return notLinkedText, gotgbot.InlineKeyboardMarkup{}, false, ErrNotLinked
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
			return noScheduleText, gotgbot.InlineKeyboardMarkup{}, false, api.ErrNoScheduleData
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
		CallerName:       callerName,
		Days:             days,
	}

	return formatWeek(info, offset, user.GroupName), weekKeyboard(offset), true, nil
}

// renderGroupDay renders a group's schedule for a single date.
func (b *Bot) renderGroupDay(ctx context.Context, group model.BotGroup, date time.Time) (text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool, err error) {
	var groupIDPtr *uuid.UUID
	if group.ID != uuid.Nil {
		groupIDPtr = &group.ID
	}
	view, vErr := b.svc.BuildGroupDay(ctx, groupIDPtr, group.AcademicGroupID, date)
	if vErr != nil {
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

	return formatGroupDay(info, group.AcademicGroupName), groupDayKeyboard(date, group.AcademicGroupID), true, nil
}

// renderGroupWeek renders a group's schedule for a full academic week.
func (b *Bot) renderGroupWeek(ctx context.Context, group model.BotGroup, offset int) (text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool, err error) {
	parity, pErr := b.svc.ResolveWeekParity(ctx, time.Now().AddDate(0, 0, 7*offset))
	if pErr != nil {
		return "", gotgbot.InlineKeyboardMarkup{}, false, pErr
	}

	var groupIDPtr *uuid.UUID
	if group.ID != uuid.Nil {
		groupIDPtr = &group.ID
	}
	view, vErr := b.svc.BuildGroupWeek(ctx, groupIDPtr, group.AcademicGroupID, parity)
	if vErr != nil {
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

	return formatGroupWeek(info, offset, group.AcademicGroupName), groupWeekKeyboard(offset, group.AcademicGroupID), true, nil
}

func (b *Bot) cmdStart(bot *gotgbot.Bot, ctx *ext.Context) error {
	reqCtx := context.Background()

	if isGroupChat(ctx.EffectiveChat) {
		welcome := "👋 <b>Вітаю! Я бот розкладу КПІ.</b>\n\n" +
			"У цьому чаті доступні команди:\n" +
			"• /today — твій персональний розклад на сьогодні\n" +
			"• /week — твій персональний розклад на тиждень\n" +
			"• /group_today — загальний розклад групи на сьогодні\n" +
			"• /group_week — загальний розклад групи на тиждень\n\n" +
			"⚙️ <b>Адміністраторам:</b> надішліть /group, щоб налаштувати академічну групу для цього чату."
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, welcome, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
		return err
	}

	if _, err := b.upsertUser(reqCtx, ctx.EffectiveUser.Id); err != nil {
		return fmt.Errorf("upserting user on /start: %w", err)
	}

	msgText := strings.TrimSpace(ctx.EffectiveMessage.Text)
	parts := strings.Fields(msgText)
	if len(parts) > 1 {
		payload := parts[1]
		if strings.HasPrefix(payload, "bind_") {
			chatIDStr := strings.TrimPrefix(payload, "bind_")
			if chatID, err := strconv.ParseInt(chatIDStr, 10, 64); err == nil {
				return b.showGroupBindPicker(bot, ctx, chatID)
			}
		}
		if strings.HasPrefix(payload, "cfg_") {
			groupIDStr := strings.TrimPrefix(payload, "cfg_")
			if groupID, err := uuid.Parse(groupIDStr); err == nil {
				return b.showGroupConfigScreen(bot, ctx.EffectiveChat.Id, groupID, "")
			}
		}
	}

	state, err := b.resolveLinkState(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		slog.Error("resolving link state for /start", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		state = linkStateNone
	}

	_, err = bot.SendMessage(ctx.EffectiveChat.Id, formatStartScreen(state), &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		ReplyMarkup:        startKeyboard(state),
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	return err
}

func (b *Bot) showGroupBindPicker(bot *gotgbot.Bot, ctx *ext.Context, chatID int64) error {
	reqCtx := context.Background()
	chatTitle := fmt.Sprintf("ID: %d", chatID)
	tgChat, err := bot.GetChat(chatID, nil)
	if err == nil && tgChat.Title != "" {
		chatTitle = tgChat.Title
	}

	groups, err := b.db.GetBotGroupsByCreator(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		slog.Error("getting creator groups for bind picker", "error", err)
	}

	text := formatGroupBindPicker(chatTitle, groups)
	kb := groupBindPickerKeyboard(chatID, groups)
	_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		ReplyMarkup:        kb,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	return sendErr
}

func (b *Bot) showGroupConfigScreen(bot *gotgbot.Bot, chatID int64, groupID uuid.UUID, notice string) error {
	reqCtx := context.Background()
	group, err := b.db.GetBotGroupByID(reqCtx, groupID)
	if err != nil {
		_, sendErr := bot.SendMessage(chatID, genericErrorText, nil)
		return sendErr
	}
	text := formatGroupConfig(group, notice)
	kb := groupConfigKeyboard(group)
	_, sendErr := bot.SendMessage(chatID, text, &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		ReplyMarkup:        kb,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	return sendErr
}

func (b *Bot) cmdInstall(bot *gotgbot.Bot, ctx *ext.Context) error {
	if isGroupChat(ctx.EffectiveChat) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "⚠️ Ця команда доступна лише в особистих повідомленнях з ботом.", nil)
		return err
	}

	reqCtx := context.Background()
	if _, err := b.upsertUser(reqCtx, ctx.EffectiveUser.Id); err != nil {
		return fmt.Errorf("upserting user on /install: %w", err)
	}

	_, err := bot.SendMessage(ctx.EffectiveChat.Id, formatInstallScreen(), &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		ReplyMarkup:        installKeyboard(b.ExtensionDownloadURL()),
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	return err
}

func (b *Bot) cmdLink(bot *gotgbot.Bot, ctx *ext.Context) error {
	if isGroupChat(ctx.EffectiveChat) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "⚠️ Ця команда доступна лише в особистих повідомленнях з ботом.", nil)
		return err
	}

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
		ParseMode:          "HTML",
		ReplyMarkup:        linkKeyboard(),
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	return err
}

func (b *Bot) cmdToday(bot *gotgbot.Bot, ctx *ext.Context) error {
	callerName := ""
	if isGroupChat(ctx.EffectiveChat) {
		callerName = formatUserName(ctx.EffectiveUser)
	}

	text, kb, hasKeyboard, err := b.renderDay(context.Background(), ctx.EffectiveUser.Id, time.Now(), callerName)
	if err != nil {
		if errors.Is(err, ErrNotLinked) {
			msg := notLinkedText
			if isGroupChat(ctx.EffectiveChat) {
				msg = fmt.Sprintf("🔒 <b>%s</b>, твій акаунт ще не прив'язано до бота. Напиши боту в особисті повідомлення /start, щоб підключити розклад.", html.EscapeString(callerName))
			}
			_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, msg, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
			return sendErr
		}
		if errors.Is(err, api.ErrNoScheduleData) {
			msg := noScheduleText
			if isGroupChat(ctx.EffectiveChat) {
				msg = fmt.Sprintf("📭 <b>%s</b>, твій розклад ще не синхронізовано з браузерного розширення.", html.EscapeString(callerName))
			}
			_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, msg, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
			return sendErr
		}
		slog.Error("rendering /today", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}
	return sendScreen(bot, ctx.EffectiveChat.Id, text, kb, hasKeyboard)
}

func (b *Bot) cmdWeek(bot *gotgbot.Bot, ctx *ext.Context) error {
	callerName := ""
	if isGroupChat(ctx.EffectiveChat) {
		callerName = formatUserName(ctx.EffectiveUser)
	}

	text, kb, hasKeyboard, err := b.renderWeek(context.Background(), ctx.EffectiveUser.Id, 0, callerName)
	if err != nil {
		if errors.Is(err, ErrNotLinked) {
			msg := notLinkedText
			if isGroupChat(ctx.EffectiveChat) {
				msg = fmt.Sprintf("🔒 <b>%s</b>, твій акаунт ще не прив'язано до бота. Напиши боту в особисті повідомлення /start, щоб підключити розклад.", html.EscapeString(callerName))
			}
			_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, msg, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
			return sendErr
		}
		if errors.Is(err, api.ErrNoScheduleData) {
			msg := noScheduleText
			if isGroupChat(ctx.EffectiveChat) {
				msg = fmt.Sprintf("📭 <b>%s</b>, твій розклад ще не синхронізовано з браузерного розширення.", html.EscapeString(callerName))
			}
			_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, msg, &gotgbot.SendMessageOpts{ParseMode: "HTML"})
			return sendErr
		}
		slog.Error("rendering /week", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}
	return sendScreen(bot, ctx.EffectiveChat.Id, text, kb, hasKeyboard)
}

func (b *Bot) cmdURLs(bot *gotgbot.Bot, ctx *ext.Context) error {
	if isGroupChat(ctx.EffectiveChat) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "⚠️ Ця команда доступна лише в особистих повідомленнях з ботом.", nil)
		return err
	}

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
		ParseMode:          "HTML",
		ReplyMarkup:        kb,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	return err
}

func (b *Bot) cmdGroup(bot *gotgbot.Bot, ctx *ext.Context) error {
	reqCtx := context.Background()
	if isGroupChat(ctx.EffectiveChat) {
		chatID := ctx.EffectiveChat.Id
		userID := ctx.EffectiveUser.Id
		if !isChatAdmin(bot, chatID, userID) {
			_, err := bot.SendMessage(chatID, "⚠️ Тільки адміністратори цього чату можуть налаштовувати групу.", nil)
			return err
		}

		group, err := b.db.GetBotGroupByChatID(reqCtx, chatID)
		botUsername := bot.Username
		if group.AcademicGroupName != "" && err == nil {
			msg := fmt.Sprintf("👥 <b>Налаштування групи</b>\n\nЦей чат прив'язано до: <b>%s</b>.\n\nКерування налаштуваннями здійснюється в особистих повідомленнях з ботом.", html.EscapeString(group.AcademicGroupName))
			kb := gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{{Text: "⚙️ Відкрити налаштування в особистих", Url: fmt.Sprintf("https://t.me/%s?start=cfg_%s", botUsername, group.ID.String())}},
				},
			}
			_, sendErr := bot.SendMessage(chatID, msg, &gotgbot.SendMessageOpts{
				ParseMode:          "HTML",
				ReplyMarkup:        kb,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
			})
			return sendErr
		}

		msg := "⚙️ <b>Налаштування групи</b>\n\nЦей чат ще не прив'язано до жодної академічної групи КПІ.\n\nНалаштування здійснюються в особистих повідомленнях з ботом, щоб не захаращувати чат."
		kb := gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{{Text: "⚙️ Налаштувати в особистих", Url: fmt.Sprintf("https://t.me/%s?start=bind_%d", botUsername, chatID)}},
			},
		}
		_, sendErr := bot.SendMessage(chatID, msg, &gotgbot.SendMessageOpts{
			ParseMode:          "HTML",
			ReplyMarkup:        kb,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		})
		return sendErr
	}

	_ = b.db.ClearGroupPrompt(reqCtx, ctx.EffectiveUser.Id)
	groups, err := b.db.GetBotGroupsByCreator(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		slog.Error("getting bot groups for creator", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}

	text := formatGroupListMenu(groups, "")
	kb := groupListKeyboard(groups)
	_, err = bot.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		ReplyMarkup:        kb,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	return err
}

func (b *Bot) cmdGroupToday(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !isGroupChat(ctx.EffectiveChat) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "⚠️ Ця команда доступна лише у групових чатах.", nil)
		return err
	}

	reqCtx := context.Background()
	group, err := b.db.GetBotGroupByChatID(reqCtx, ctx.EffectiveChat.Id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, "⚙️ Для цього чату ще не налаштовано академічну групу. Адміністратор може налаштувати її за допомогою команди /group.", nil)
			return sendErr
		}
		slog.Error("fetching group for /group_today", "error", err, "chat_id", ctx.EffectiveChat.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}

	text, kb, hasKeyboard, rErr := b.renderGroupDay(reqCtx, group, time.Now())
	if rErr != nil {
		slog.Error("rendering group day", "error", rErr, "group", group.AcademicGroupName)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}
	return sendScreen(bot, ctx.EffectiveChat.Id, text, kb, hasKeyboard)
}

func (b *Bot) cmdGroupWeek(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !isGroupChat(ctx.EffectiveChat) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "⚠️ Ця команда доступна лише у групових чатах.", nil)
		return err
	}

	reqCtx := context.Background()
	group, err := b.db.GetBotGroupByChatID(reqCtx, ctx.EffectiveChat.Id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, "⚙️ Для цього чату ще не налаштовано академічну групу. Адміністратор може налаштувати її за допомогою команди /group.", nil)
			return sendErr
		}
		slog.Error("fetching group for /group_week", "error", err, "chat_id", ctx.EffectiveChat.Id)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}

	text, kb, hasKeyboard, rErr := b.renderGroupWeek(reqCtx, group, 0)
	if rErr != nil {
		slog.Error("rendering group week", "error", rErr, "group", group.AcademicGroupName)
		_, sendErr := bot.SendMessage(ctx.EffectiveChat.Id, genericErrorText, nil)
		return sendErr
	}
	return sendScreen(bot, ctx.EffectiveChat.Id, text, kb, hasKeyboard)
}

func (b *Bot) handleGroupInput(bot *gotgbot.Bot, ctx *ext.Context, prompt *model.GroupPrompt, rawInput string) error {
	reqCtx := context.Background()

	if prompt.Action == "set_url" && prompt.GroupID != nil {
		hash := lessonHash(prompt.SubjectNorm, prompt.Tag)
		groupIDStr := prompt.GroupID.String()
		if !isValidURL(rawInput) {
			text := formatURLPrompt(prompt.SubjectName, prompt.Tag, "", "Некоректне посилання. Будь ласка, надішли дійсне посилання (наприклад: https://zoom.us/j/...):")
			kb := groupURLPromptKeyboard(groupIDStr, false, hash)
			opts := &gotgbot.EditMessageTextOpts{
				ChatId:             ctx.EffectiveChat.Id,
				MessageId:          prompt.PromptMessageID,
				Text:               text,
				ParseMode:          "HTML",
				ReplyMarkup:        kb,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
			}
			_, _, _ = bot.EditMessageText(opts)
			return nil
		}

		if err := b.db.SetGroupLessonURL(reqCtx, *prompt.GroupID, prompt.SubjectNorm, prompt.Tag, rawInput); err != nil {
			slog.Error("saving group lesson url", "error", err, "group_id", prompt.GroupID)
			return nil
		}
		_ = b.db.ClearGroupPrompt(reqCtx, ctx.EffectiveUser.Id)

		group, gErr := b.db.GetBotGroupByID(reqCtx, *prompt.GroupID)
		if gErr != nil {
			return nil
		}
		lessons, lErr := b.svc.GetUniqueGroupLessons(reqCtx, group.ID, group.AcademicGroupID)
		if lErr != nil {
			slog.Error("fetching group lessons after url save", "error", lErr)
			return nil
		}

		notice := fmt.Sprintf("✅ Посилання для «<b>%s (%s)</b>» збережено!", html.EscapeString(prompt.SubjectName), tagAbbr(prompt.Tag))
		text := formatGroupLessonsMenu(group.AcademicGroupName, lessons, notice)
		kb := groupURLsKeyboard(groupIDStr, lessons)
		opts := &gotgbot.EditMessageTextOpts{
			ChatId:             ctx.EffectiveChat.Id,
			MessageId:          prompt.PromptMessageID,
			Text:               text,
			ParseMode:          "HTML",
			ReplyMarkup:        kb,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		}
		_, _, _ = bot.EditMessageText(opts)
		return nil
	}

	groupID, err := b.svc.Campus().ResolveGroupID(reqCtx, rawInput)
	if err != nil {
		groups, sErr := b.svc.Campus().SearchGroups(reqCtx, rawInput)
		if sErr == nil && len(groups) == 1 {
			groupID = groups[0].ID
			rawInput = groups[0].Name
		} else {
			errMsg := fmt.Sprintf("Групу «%s» не знайдено в базі КПІ. Перевір назву та введи ще раз (наприклад, ІП-21):", rawInput)
			var text string
			var kb gotgbot.InlineKeyboardMarkup
			if prompt.Action == "create" {
				text = formatGroupCreationPrompt(errMsg)
				kb = groupPromptBackKeyboard(groupCallbackPrefix + "list")
			} else {
				text = formatGroupEditAcadPrompt("", errMsg)
				groupIDStr := ""
				if prompt.GroupID != nil {
					groupIDStr = prompt.GroupID.String()
				}
				kb = groupPromptBackKeyboard(groupCallbackPrefix + "view:" + groupIDStr)
			}
			opts := &gotgbot.EditMessageTextOpts{
				ChatId:             ctx.EffectiveChat.Id,
				MessageId:          prompt.PromptMessageID,
				Text:               text,
				ParseMode:          "HTML",
				ReplyMarkup:        kb,
				LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
			}
			_, _, _ = bot.EditMessageText(opts)
			return nil
		}
	}

	faculty := ""
	allGroups, _ := b.svc.Campus().Groups(reqCtx)
	for _, ag := range allGroups {
		if ag.ID == groupID {
			rawInput = ag.Name
			faculty = ag.Faculty
			break
		}
	}

	_ = b.db.ClearGroupPrompt(reqCtx, ctx.EffectiveUser.Id)

	if prompt.Action == "create" {
		var chatID *int64
		chatTitle := prompt.BindChatTitle
		if prompt.BindChatID != nil {
			chatID = prompt.BindChatID
		}
		created, cErr := b.db.CreateBotGroup(reqCtx, ctx.EffectiveUser.Id, groupID, rawInput, faculty, chatID, chatTitle)
		if cErr != nil {
			slog.Error("creating bot group", "error", cErr)
			return nil
		}
		notice := fmt.Sprintf("✅ Групу <b>%s</b> успішно створено!", html.EscapeString(created.AcademicGroupName))
		text := formatGroupConfig(created, notice)
		kb := groupConfigKeyboard(created)
		opts := &gotgbot.EditMessageTextOpts{
			ChatId:             ctx.EffectiveChat.Id,
			MessageId:          prompt.PromptMessageID,
			Text:               text,
			ParseMode:          "HTML",
			ReplyMarkup:        kb,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		}
		_, _, _ = bot.EditMessageText(opts)
		return nil
	}

	if prompt.Action == "edit_academic" && prompt.GroupID != nil {
		uErr := b.db.UpdateBotGroupAcademic(reqCtx, *prompt.GroupID, groupID, rawInput, faculty)
		if uErr != nil {
			slog.Error("updating bot group academic", "error", uErr)
			return nil
		}
		updated, fErr := b.db.GetBotGroupByID(reqCtx, *prompt.GroupID)
		if fErr != nil {
			return nil
		}
		notice := fmt.Sprintf("✅ Академічну групу змінено на <b>%s</b>!", html.EscapeString(updated.AcademicGroupName))
		text := formatGroupConfig(updated, notice)
		kb := groupConfigKeyboard(updated)
		opts := &gotgbot.EditMessageTextOpts{
			ChatId:             ctx.EffectiveChat.Id,
			MessageId:          prompt.PromptMessageID,
			Text:               text,
			ParseMode:          "HTML",
			ReplyMarkup:        kb,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		}
		_, _, _ = bot.EditMessageText(opts)
		return nil
	}

	return nil
}

func (b *Bot) onTextMessage(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return nil
	}

	reqCtx := context.Background()

	// Check for dash command aliases in group chats
	if isGroupChat(ctx.EffectiveChat) {
		if strings.HasPrefix(msg.Text, "/group-today") {
			return b.cmdGroupToday(bot, ctx)
		}
		if strings.HasPrefix(msg.Text, "/group-week") {
			return b.cmdGroupWeek(bot, ctx)
		}
	}

	// 1. Check active group input prompt
	grpPrompt, err := b.db.GetGroupPrompt(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		slog.Error("checking group prompt", "error", err, "telegram_id", ctx.EffectiveUser.Id)
		return nil
	}
	if grpPrompt != nil {
		if _, delErr := bot.DeleteMessage(ctx.EffectiveChat.Id, msg.MessageId, nil); delErr != nil {
			slog.Warn("could not delete user group prompt message", "error", delErr)
		}
		if strings.HasPrefix(msg.Text, "/") {
			_ = b.db.ClearGroupPrompt(reqCtx, ctx.EffectiveUser.Id)
			return nil
		}
		return b.handleGroupInput(bot, ctx, grpPrompt, strings.TrimSpace(msg.Text))
	}

	// 2. Check active URL input prompt
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
			ChatId:             ctx.EffectiveChat.Id,
			MessageId:          prompt.PromptMessageID,
			Text:               text,
			ParseMode:          "HTML",
			ReplyMarkup:        kb,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
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
		ChatId:             ctx.EffectiveChat.Id,
		MessageId:          prompt.PromptMessageID,
		Text:               text,
		ParseMode:          "HTML",
		ReplyMarkup:        kb,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
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

// sendScreen posts a screen as a new message.
func sendScreen(bot *gotgbot.Bot, chatID int64, text string, kb gotgbot.InlineKeyboardMarkup, hasKeyboard bool) error {
	opts := &gotgbot.SendMessageOpts{
		ParseMode:          "HTML",
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	}
	if hasKeyboard {
		opts.ReplyMarkup = kb
	}
	_, err := bot.SendMessage(chatID, text, opts)
	return err
}

func (b *Bot) cmdSettings(bot *gotgbot.Bot, ctx *ext.Context) error {
	if isGroupChat(ctx.EffectiveChat) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, "⚠️ Команда /settings доступна лише в особистих повідомленнях з ботом.", nil)
		return err
	}

	reqCtx := context.Background()
	user, err := b.db.GetUserByTelegramID(reqCtx, ctx.EffectiveUser.Id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			user, err = b.db.UpsertUser(reqCtx, ctx.EffectiveUser.Id, nil, nil)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	text := formatUserSettings(user.NotificationsEnabled)
	kb := userSettingsKeyboard(user.NotificationsEnabled)
	return sendScreen(bot, ctx.EffectiveChat.Id, text, kb, true)
}

