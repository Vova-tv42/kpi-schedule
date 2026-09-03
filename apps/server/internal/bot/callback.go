package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/api"
	"kpi-schedule-bot/server/internal/model"
)

// Each screen namespaces its buttons with its own CallbackData prefix, so one
// dispatcher handler is registered per screen rather than one that has to
// demultiplex every button in the bot.
const (
	navCallbackPrefix       = "nav:"   // day screen: prev / today / next
	weekCallbackPrefix      = "week:"  // week screen: week slots + jump to today
	menuCallbackPrefix      = "menu:"  // onboarding screens: link / back / week
	urlsCallbackPrefix      = "urls:"  // lesson URLs screens: edit / back / del / today
	groupCallbackPrefix     = "grp:"   // group admin screens: list / new / view / edit / unbind / delete / bind
	groupNavCallbackPrefix  = "gnav:"  // group schedule day screen: prev / today / next
	groupWeekCallbackPrefix = "gweek:" // group schedule week screen: slots / today
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

// onURLs handles callbacks from the lesson URLs screens.
func (b *Bot) onURLs(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	action := strings.TrimPrefix(cq.Data, urlsCallbackPrefix)

	switch {
	case action == "today":
		_ = b.db.ClearURLPrompt(context.Background(), cq.From.Id)
		return b.editToDay(bot, cq, time.Now())
	case action == "back":
		_ = b.db.ClearURLPrompt(context.Background(), cq.From.Id)
		return b.editToLessonsMenu(bot, cq, "")
	case strings.HasPrefix(action, "edit:"):
		hash := strings.TrimPrefix(action, "edit:")
		return b.editToURLPrompt(bot, cq, hash)
	case strings.HasPrefix(action, "del:"):
		hash := strings.TrimPrefix(action, "del:")
		return b.handleDeleteURL(bot, cq, hash)
	default:
		return answerSilently(bot, cq)
	}
}

func (b *Bot) editToLessonsMenu(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, notice string) error {
	reqCtx := context.Background()
	user, err := b.resolveUser(reqCtx, cq.From.Id)
	if err != nil {
		return answerWithError(bot, cq)
	}

	lessons, err := b.db.GetUniqueScheduleLessons(reqCtx, user.ID)
	if err != nil {
		slog.Error("fetching unique lessons for callback", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}

	text := formatLessonsMenu(lessons, notice)
	kb := urlsKeyboard(lessons)
	return b.applyScreen(bot, cq, text, kb, true)
}

func (b *Bot) editToURLPrompt(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, hash string) error {
	reqCtx := context.Background()
	user, err := b.resolveUser(reqCtx, cq.From.Id)
	if err != nil {
		return answerWithError(bot, cq)
	}

	lessons, err := b.db.GetUniqueScheduleLessons(reqCtx, user.ID)
	if err != nil {
		slog.Error("fetching unique lessons for prompt", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}

	var target *model.UniqueLesson
	for _, l := range lessons {
		if lessonHash(l.SubjectNorm, l.Tag) == hash {
			lCopy := l
			target = &lCopy
			break
		}
	}
	if target == nil {
		return answerWithError(bot, cq)
	}

	msgID := cq.Message.GetMessageId()
	if err := b.db.SetURLPrompt(reqCtx, user.ID, cq.From.Id, msgID, target.SubjectNorm, target.Tag, target.Subject); err != nil {
		slog.Error("setting url prompt", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}

	text := formatURLPrompt(target.Subject, target.Tag, target.URL, "")
	kb := urlPromptKeyboard(target.URL != "", hash)
	return b.applyScreen(bot, cq, text, kb, true)
}

func (b *Bot) handleDeleteURL(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, hash string) error {
	reqCtx := context.Background()
	user, err := b.resolveUser(reqCtx, cq.From.Id)
	if err != nil {
		return answerWithError(bot, cq)
	}

	lessons, err := b.db.GetUniqueScheduleLessons(reqCtx, user.ID)
	if err != nil {
		slog.Error("fetching unique lessons for delete", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}

	var target *model.UniqueLesson
	for _, l := range lessons {
		if lessonHash(l.SubjectNorm, l.Tag) == hash {
			lCopy := l
			target = &lCopy
			break
		}
	}
	if target != nil {
		if err := b.db.DeleteLessonURL(reqCtx, user.ID, target.SubjectNorm, target.Tag); err != nil {
			slog.Error("deleting lesson url", "error", err, "telegram_id", cq.From.Id)
			return answerWithError(bot, cq)
		}
	}
	_ = b.db.ClearURLPrompt(reqCtx, cq.From.Id)

	subjectLabel := ""
	if target != nil {
		subjectLabel = fmt.Sprintf(" «%s (%s)»", target.Subject, tagAbbr(target.Tag))
	}
	return b.editToLessonsMenu(bot, cq, fmt.Sprintf("🗑 Посилання для%s видалено.", subjectLabel))
}


func (b *Bot) editToDay(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, date time.Time) error {
	callerName := ""
	isGroup := cq.Message != nil && cq.Message.GetChat().Type != "private"
	if isGroup {
		callerName = formatUserName(&cq.From)
	}

	text, kb, hasKeyboard, err := b.renderDay(context.Background(), cq.From.Id, date, callerName)
	if err != nil {
		if isGroup && errors.Is(err, ErrNotLinked) {
			_, answerErr := bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "🔒 Твій акаунт ще не прив'язано. Напиши боту в особисті повідомлення /start, щоб підключити розклад.",
				ShowAlert: true,
			})
			return answerErr
		}
		if isGroup && errors.Is(err, api.ErrNoScheduleData) {
			_, answerErr := bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "📭 Твій розклад ще не синхронізовано з браузерного розширення.",
				ShowAlert: true,
			})
			return answerErr
		}
		slog.Error("rendering day callback", "error", err, "telegram_id", cq.From.Id)
		return answerWithError(bot, cq)
	}
	return b.applyScreen(bot, cq, text, kb, hasKeyboard)
}

func (b *Bot) editToWeek(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, offset int) error {
	callerName := ""
	isGroup := cq.Message != nil && cq.Message.GetChat().Type != "private"
	if isGroup {
		callerName = formatUserName(&cq.From)
	}

	text, kb, hasKeyboard, err := b.renderWeek(context.Background(), cq.From.Id, offset, callerName)
	if err != nil {
		if isGroup && errors.Is(err, ErrNotLinked) {
			_, answerErr := bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "🔒 Твій акаунт ще не прив'язано. Напиши боту в особисті повідомлення /start, щоб підключити розклад.",
				ShowAlert: true,
			})
			return answerErr
		}
		if isGroup && errors.Is(err, api.ErrNoScheduleData) {
			_, answerErr := bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
				Text:      "📭 Твій розклад ще не синхронізовано з браузерного розширення.",
				ShowAlert: true,
			})
			return answerErr
		}
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
		ChatId:             msg.GetChat().Id,
		MessageId:          msg.GetMessageId(),
		Text:               text,
		ParseMode:          "HTML",
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
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

// onGroupNav handles day navigation for group schedules (gnav:action:date:groupID).
func (b *Bot) onGroupNav(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	parts := strings.Split(strings.TrimPrefix(cq.Data, groupNavCallbackPrefix), ":")
	if len(parts) != 3 {
		return answerSilently(bot, cq)
	}
	action, dateStr, groupIDStr := parts[0], parts[1], parts[2]

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return answerSilently(bot, cq)
	}
	groupID, err := strconv.Atoi(groupIDStr)
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

	group := model.BotGroup{AcademicGroupID: groupID, AcademicGroupName: fmt.Sprintf("ID:%d", groupID)}
	if cq.Message != nil {
		if g, err := b.db.GetBotGroupByChatID(context.Background(), cq.Message.GetChat().Id); err == nil {
			group = g
		}
	}

	text, kb, hasKeyboard, rErr := b.renderGroupDay(context.Background(), group, date)
	if rErr != nil {
		slog.Error("rendering group day callback", "error", rErr, "group_id", groupID)
		return answerWithError(bot, cq)
	}
	return b.applyScreen(bot, cq, text, kb, hasKeyboard)
}

// onGroupWeek handles week navigation for group schedules (gweek:action...).
func (b *Bot) onGroupWeek(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	action := strings.TrimPrefix(cq.Data, groupWeekCallbackPrefix)

	if action == "noop" {
		return answerSilently(bot, cq)
	}

	parts := strings.Split(action, ":")
	if len(parts) < 2 {
		return answerSilently(bot, cq)
	}

	act := parts[0]
	switch act {
	case "today":
		groupID, err := strconv.Atoi(parts[1])
		if err != nil {
			return answerSilently(bot, cq)
		}
		group := model.BotGroup{AcademicGroupID: groupID, AcademicGroupName: fmt.Sprintf("ID:%d", groupID)}
		if cq.Message != nil {
			if g, err := b.db.GetBotGroupByChatID(context.Background(), cq.Message.GetChat().Id); err == nil {
				group = g
			}
		}
		text, kb, hasKeyboard, rErr := b.renderGroupDay(context.Background(), group, time.Now())
		if rErr != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, text, kb, hasKeyboard)

	case "goto":
		if len(parts) != 3 {
			return answerSilently(bot, cq)
		}
		offset, err := strconv.Atoi(parts[1])
		if err != nil {
			return answerSilently(bot, cq)
		}
		groupID, err := strconv.Atoi(parts[2])
		if err != nil {
			return answerSilently(bot, cq)
		}
		group := model.BotGroup{AcademicGroupID: groupID, AcademicGroupName: fmt.Sprintf("ID:%d", groupID)}
		if cq.Message != nil {
			if g, err := b.db.GetBotGroupByChatID(context.Background(), cq.Message.GetChat().Id); err == nil {
				group = g
			}
		}
		text, kb, hasKeyboard, rErr := b.renderGroupWeek(context.Background(), group, offset)
		if rErr != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, text, kb, hasKeyboard)

	default:
		return answerSilently(bot, cq)
	}
}

// onGroup handles callbacks from group administration screens (grp:...).
func (b *Bot) onGroup(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	action := strings.TrimPrefix(cq.Data, groupCallbackPrefix)
	reqCtx := context.Background()

	switch {
	case action == "list":
		_ = b.db.ClearGroupPrompt(reqCtx, cq.From.Id)
		groups, err := b.db.GetBotGroupsByCreator(reqCtx, cq.From.Id)
		if err != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, formatGroupListMenu(groups, ""), groupListKeyboard(groups), true)

	case action == "new":
		msgID := cq.Message.GetMessageId()
		err := b.db.SetGroupPrompt(reqCtx, model.GroupPrompt{
			TelegramID:      cq.From.Id,
			PromptMessageID: msgID,
			Action:          "create",
		})
		if err != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, formatGroupCreationPrompt(""), groupPromptBackKeyboard(groupCallbackPrefix+"list"), true)

	case strings.HasPrefix(action, "view:"):
		idStr := strings.TrimPrefix(action, "view:")
		gid, err := uuid.Parse(idStr)
		if err != nil {
			return answerWithError(bot, cq)
		}
		_ = b.db.ClearGroupPrompt(reqCtx, cq.From.Id)
		g, err := b.db.GetBotGroupByID(reqCtx, gid)
		if err != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, formatGroupConfig(g, ""), groupConfigKeyboard(g), true)

	case strings.HasPrefix(action, "edit_acad:"):
		idStr := strings.TrimPrefix(action, "edit_acad:")
		gid, err := uuid.Parse(idStr)
		if err != nil {
			return answerWithError(bot, cq)
		}
		g, err := b.db.GetBotGroupByID(reqCtx, gid)
		if err != nil {
			return answerWithError(bot, cq)
		}
		msgID := cq.Message.GetMessageId()
		err = b.db.SetGroupPrompt(reqCtx, model.GroupPrompt{
			TelegramID:      cq.From.Id,
			PromptMessageID: msgID,
			Action:          "edit_academic",
			GroupID:         &gid,
		})
		if err != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, formatGroupEditAcadPrompt(g.AcademicGroupName, ""), groupPromptBackKeyboard(groupCallbackPrefix+"view:"+idStr), true)

	case strings.HasPrefix(action, "unbind:"):
		idStr := strings.TrimPrefix(action, "unbind:")
		gid, err := uuid.Parse(idStr)
		if err != nil {
			return answerWithError(bot, cq)
		}
		if err := b.db.UnbindBotGroupChat(reqCtx, gid); err != nil {
			return answerWithError(bot, cq)
		}
		g, err := b.db.GetBotGroupByID(reqCtx, gid)
		if err != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, formatGroupConfig(g, "✅ Чат успішно відв'язано."), groupConfigKeyboard(g), true)

	case strings.HasPrefix(action, "bind_help:"):
		idStr := strings.TrimPrefix(action, "bind_help:")
		helpText := "🔗 <b>Як прив'язати цю групу до Telegram-чату:</b>\n\n" +
			"1. Додай цього бота до свого групового чату.\n" +
			"2. Напиши в чаті команду <code>/group</code>.\n" +
			"3. Натисни кнопку налаштування — бот надішле кнопку переходу в особисті повідомлення для прив'язки чату."
		kb := groupPromptBackKeyboard(groupCallbackPrefix + "view:" + idStr)
		return b.applyScreen(bot, cq, helpText, kb, true)

	case strings.HasPrefix(action, "del_ask:"):
		idStr := strings.TrimPrefix(action, "del_ask:")
		gid, err := uuid.Parse(idStr)
		if err != nil {
			return answerWithError(bot, cq)
		}
		g, err := b.db.GetBotGroupByID(reqCtx, gid)
		if err != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, formatGroupDeleteConfirm(g), groupDeleteConfirmKeyboard(idStr), true)

	case strings.HasPrefix(action, "del_confirm:"):
		idStr := strings.TrimPrefix(action, "del_confirm:")
		gid, err := uuid.Parse(idStr)
		if err != nil {
			return answerWithError(bot, cq)
		}
		g, _ := b.db.GetBotGroupByID(reqCtx, gid)
		if err := b.db.DeleteBotGroup(reqCtx, gid); err != nil {
			return answerWithError(bot, cq)
		}
		groups, err := b.db.GetBotGroupsByCreator(reqCtx, cq.From.Id)
		if err != nil {
			return answerWithError(bot, cq)
		}
		notice := fmt.Sprintf("🗑 Групу «<b>%s</b>» успішно видалено.", html.EscapeString(g.AcademicGroupName))
		return b.applyScreen(bot, cq, formatGroupListMenu(groups, notice), groupListKeyboard(groups), true)

	case strings.HasPrefix(action, "bind_to:"):
		rest := strings.TrimPrefix(action, "bind_to:")
		parts := strings.Split(rest, ":")
		if len(parts) != 2 {
			return answerSilently(bot, cq)
		}
		gid, err := uuid.Parse(parts[0])
		if err != nil {
			return answerWithError(bot, cq)
		}
		chatID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return answerWithError(bot, cq)
		}
		chatTitle := fmt.Sprintf("ID: %d", chatID)
		if tgChat, err := bot.GetChat(chatID, nil); err == nil && tgChat.Title != "" {
			chatTitle = tgChat.Title
		}
		if err := b.db.BindBotGroupChat(reqCtx, gid, chatID, chatTitle); err != nil {
			return answerWithError(bot, cq)
		}
		g, err := b.db.GetBotGroupByID(reqCtx, gid)
		if err != nil {
			return answerWithError(bot, cq)
		}
		notice := fmt.Sprintf("✅ Чат «<b>%s</b>» успішно прив'язано до групи <b>%s</b>!", html.EscapeString(chatTitle), html.EscapeString(g.AcademicGroupName))
		return b.applyScreen(bot, cq, formatGroupConfig(g, notice), groupConfigKeyboard(g), true)

	case strings.HasPrefix(action, "bind_new:"):
		chatIDStr := strings.TrimPrefix(action, "bind_new:")
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			return answerWithError(bot, cq)
		}
		chatTitle := fmt.Sprintf("ID: %d", chatID)
		if tgChat, err := bot.GetChat(chatID, nil); err == nil && tgChat.Title != "" {
			chatTitle = tgChat.Title
		}
		msgID := cq.Message.GetMessageId()
		err = b.db.SetGroupPrompt(reqCtx, model.GroupPrompt{
			TelegramID:      cq.From.Id,
			PromptMessageID: msgID,
			Action:          "create",
			BindChatID:      &chatID,
			BindChatTitle:   chatTitle,
		})
		if err != nil {
			return answerWithError(bot, cq)
		}
		return b.applyScreen(bot, cq, formatGroupCreationPrompt(""), groupPromptBackKeyboard(groupCallbackPrefix+"list"), true)

	default:
		return answerSilently(bot, cq)
	}
}
