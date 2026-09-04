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
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// cmdIssues opens the /issues root screen. Unlike the schedule commands it does
// not require a linked account — anyone who can talk to the bot can file an
// issue. It is private-chat only.
func (b *Bot) cmdIssues(bot *gotgbot.Bot, ctx *ext.Context) error {
	if isGroupChat(ctx.EffectiveChat) {
		_, err := bot.SendMessage(ctx.EffectiveChat.Id, issuesDMOnlyText, nil)
		return err
	}

	reqCtx := context.Background()
	// Typing /issues abandons any wizard that was already in flight, the same
	// way /urls drops a pending URL prompt.
	_ = b.db.ClearIssueDraft(reqCtx, ctx.EffectiveUser.Id)

	return sendScreen(bot, ctx.EffectiveChat.Id, formatIssuesMenu(""), issuesMenuKeyboard(), true)
}

// onIssues routes every "iss:" button. One screen per action; each edits the
// single wizard message in place rather than posting a new one.
func (b *Bot) onIssues(bot *gotgbot.Bot, ctx *ext.Context) error {
	cq := ctx.CallbackQuery
	if cq == nil {
		return nil
	}
	action := strings.TrimPrefix(cq.Data, issuesCallbackPrefix)
	reqCtx := context.Background()

	switch {
	case action == "menu":
		_ = b.db.ClearIssueDraft(reqCtx, cq.From.Id)
		return b.applyScreen(bot, cq, formatIssuesMenu(""), issuesMenuKeyboard(), true)

	case action == "new":
		// Re-entering the type picker discards whatever the draft held so far;
		// the picker is step one, so there is nothing left to keep.
		_ = b.db.ClearIssueDraft(reqCtx, cq.From.Id)
		return b.applyScreen(bot, cq, formatIssueTypePicker(), issueTypePickerKeyboard(), true)

	case action == "cancel":
		return b.cancelIssueWizard(bot, cq)

	case strings.HasPrefix(action, "type:"):
		return b.startIssueDraft(bot, cq, strings.TrimPrefix(action, "type:"))

	case action == "back:title":
		return b.backToIssueTitle(bot, cq)

	case strings.HasPrefix(action, "list:"):
		page, err := strconv.Atoi(strings.TrimPrefix(action, "list:"))
		if err != nil || page < 0 {
			page = 0
		}
		return b.editToIssueList(bot, cq, page)

	case strings.HasPrefix(action, "view:"):
		return b.editToIssueView(bot, cq, strings.TrimPrefix(action, "view:"))

	case strings.HasPrefix(action, "thr:"):
		return b.editToIssueThread(bot, cq, strings.TrimPrefix(action, "thr:"))

	case strings.HasPrefix(action, "reply:"):
		return b.startIssueReplyDraft(bot, cq, strings.TrimPrefix(action, "reply:"))

	default:
		return answerSilently(bot, cq)
	}
}

// cancelIssueWizard drops the draft and removes the bot's own message, leaving
// no trace of the abandoned flow in the chat.
func (b *Bot) cancelIssueWizard(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery) error {
	_ = b.db.ClearIssueDraft(context.Background(), cq.From.Id)

	msg := cq.Message
	if msg != nil {
		if _, err := bot.DeleteMessage(msg.GetChat().Id, msg.GetMessageId(), nil); err != nil {
			slog.Warn("could not delete cancelled issue wizard message", "error", err, "telegram_id", cq.From.Id)
		}
	}
	return answerSilently(bot, cq)
}

// startIssueDraft records the chosen type and asks for a title. The message the
// button lives on becomes the wizard message that every later step edits.
func (b *Bot) startIssueDraft(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, rawType string) error {
	if !model.ValidIssueType(rawType) {
		return answerSilently(bot, cq)
	}
	issueType := model.IssueType(rawType)

	msg := cq.Message
	if msg == nil {
		return answerSilently(bot, cq)
	}

	reqCtx := context.Background()
	// Free text now belongs to this wizard, so any other prompt waiting for a
	// typed answer is stale.
	_ = b.db.ClearURLPrompt(reqCtx, cq.From.Id)
	_ = b.db.ClearGroupPrompt(reqCtx, cq.From.Id)

	err := b.db.SetIssueDraft(reqCtx, model.IssueDraft{
		TelegramID:      cq.From.Id,
		ChatID:          msg.GetChat().Id,
		PromptMessageID: msg.GetMessageId(),
		Step:            model.IssueStepTitle,
		IssueType:       issueType,
	})
	if err != nil {
		slog.Error("starting issue draft", "error", err, "telegram_id", cq.From.Id)
		return answerIssueError(bot, cq)
	}

	return b.applyScreen(bot, cq, formatIssueTitlePrompt(issueType, ""),
		issueWizardKeyboard(issuesCallbackPrefix+"new"), true)
}

// backToIssueTitle steps the wizard back from the description to the title.
// The previously entered title is dropped, since the user is about to retype it.
func (b *Bot) backToIssueTitle(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery) error {
	reqCtx := context.Background()
	draft, err := b.db.GetIssueDraft(reqCtx, cq.From.Id)
	if err != nil || draft == nil {
		// No cleanup needed on this path: the message carrying the button is
		// the wizard message, and the recovery screen edits it in place.
		return b.issueDraftInterrupted(bot, cq, err)
	}

	draft.Step = model.IssueStepTitle
	draft.Title = ""
	if err := b.db.SetIssueDraft(reqCtx, *draft); err != nil {
		slog.Error("rewinding issue draft", "error", err, "telegram_id", cq.From.Id)
		return answerIssueError(bot, cq)
	}

	return b.applyScreen(bot, cq, formatIssueTitlePrompt(draft.IssueType, ""),
		issueWizardKeyboard(issuesCallbackPrefix+"new"), true)
}

// issueDraftInterrupted renders the root menu with the expiry banner. It is the
// recovery path for any step that needs a draft which is gone — either timed
// out (ErrIssueDraftExpired) or never there.
func (b *Bot) issueDraftInterrupted(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, cause error) error {
	if cause != nil && !errors.Is(cause, storage.ErrIssueDraftExpired) {
		slog.Error("loading issue draft", "error", cause, "telegram_id", cq.From.Id)
		return answerIssueError(bot, cq)
	}
	return b.applyScreen(bot, cq, formatIssuesMenu(issuesInterruptedText), issuesMenuKeyboard(), true)
}

func (b *Bot) editToIssueList(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, page int) error {
	reqCtx := context.Background()
	_ = b.db.ClearIssueDraft(reqCtx, cq.From.Id)

	total, err := b.db.CountIssuesByAuthor(reqCtx, cq.From.Id)
	if err != nil {
		slog.Error("counting user issues", "error", err, "telegram_id", cq.From.Id)
		return answerIssueError(bot, cq)
	}
	// A page can fall off the end if issues were listed from a stale keyboard.
	if page*issuesPageSize >= total {
		page = 0
	}

	issues, err := b.db.ListIssuesByAuthor(reqCtx, cq.From.Id, issuesPageSize, page*issuesPageSize)
	if err != nil {
		slog.Error("listing user issues", "error", err, "telegram_id", cq.From.Id)
		return answerIssueError(bot, cq)
	}

	return b.applyScreen(bot, cq, formatIssueList(issues, page, total), issueListKeyboard(issues, page, total), true)
}

// editToIssueView opens one issue. arg is "<page>:<uuid>" so the Back button
// can return to the list page the user came from.
func (b *Bot) editToIssueView(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, arg string) error {
	page, idStr := 0, arg
	if left, right, found := strings.Cut(arg, ":"); found {
		if n, err := strconv.Atoi(left); err == nil && n >= 0 {
			page = n
		}
		idStr = right
	}

	issue, ok, err := b.authoredIssue(bot, cq, idStr)
	if !ok {
		return err
	}

	count := 0
	if issue.ThreadOpen {
		if count, err = b.db.CountIssueComments(context.Background(), issue.ID); err != nil {
			slog.Error("counting issue comments", "error", err, "issue_id", issue.ID)
			count = 0
		}
	}

	return b.applyScreen(bot, cq, formatIssueView(issue, count), issueViewKeyboard(issue, count, page), true)
}

// authoredIssue loads the issue named by idStr and confirms the caller filed
// it. It answers the callback itself and returns ok=false when it cannot.
func (b *Bot) authoredIssue(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, idStr string) (model.Issue, bool, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return model.Issue{}, false, answerSilently(bot, cq)
	}

	issue, err := b.db.GetIssueByID(context.Background(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return model.Issue{}, false, answerSilently(bot, cq)
		}
		slog.Error("loading issue", "error", err, "issue_id", id)
		return model.Issue{}, false, answerIssueError(bot, cq)
	}
	if issue.AuthorTelegramID != cq.From.Id {
		return model.Issue{}, false, answerSilently(bot, cq)
	}
	return issue, true, nil
}

// editToIssueThread shows the discussion transcript. Threads are opened by
// admins, so a closed one has nothing to show.
func (b *Bot) editToIssueThread(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, idStr string) error {
	_ = b.db.ClearIssueDraft(context.Background(), cq.From.Id)

	issue, ok, err := b.authoredIssue(bot, cq, idStr)
	if !ok {
		return err
	}
	if !issue.ThreadOpen {
		return answerSilently(bot, cq)
	}

	comments, err := b.db.ListIssueComments(context.Background(), issue.ID)
	if err != nil {
		slog.Error("loading issue thread", "error", err, "issue_id", issue.ID)
		return answerIssueError(bot, cq)
	}

	return b.applyScreen(bot, cq, formatIssueThread(issue, comments), issueThreadKeyboard(issue), true)
}

// startIssueReplyDraft asks the user for a thread reply, reusing the wizard's
// draft row (and its 10-minute TTL) with the issue attached.
func (b *Bot) startIssueReplyDraft(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery, idStr string) error {
	issue, ok, err := b.authoredIssue(bot, cq, idStr)
	if !ok {
		return err
	}
	if !issue.ThreadOpen {
		return answerSilently(bot, cq)
	}

	msg := cq.Message
	if msg == nil {
		return answerSilently(bot, cq)
	}

	reqCtx := context.Background()
	_ = b.db.ClearURLPrompt(reqCtx, cq.From.Id)
	_ = b.db.ClearGroupPrompt(reqCtx, cq.From.Id)

	issueID := issue.ID
	if err := b.db.SetIssueDraft(reqCtx, model.IssueDraft{
		TelegramID:      cq.From.Id,
		ChatID:          msg.GetChat().Id,
		PromptMessageID: msg.GetMessageId(),
		Step:            model.IssueStepReply,
		IssueType:       issue.Type,
		Title:           issue.Title,
		IssueID:         &issueID,
	}); err != nil {
		slog.Error("starting issue reply draft", "error", err, "issue_id", issue.ID)
		return answerIssueError(bot, cq)
	}

	return b.applyScreen(bot, cq, formatIssueReplyPrompt(issue, ""),
		issueWizardKeyboard(issuesCallbackPrefix+"thr:"+issue.ID.String()), true)
}

// handleIssueInput consumes the user's typed answer for the current wizard
// step. The caller (onTextMessage) has already deleted the user's message; this
// only ever edits the stored wizard message.
func (b *Bot) handleIssueInput(bot *gotgbot.Bot, ctx *ext.Context, draft *model.IssueDraft, input string) error {
	switch draft.Step {
	case model.IssueStepTitle:
		return b.handleIssueTitleInput(bot, ctx, draft, input)
	case model.IssueStepBody:
		return b.handleIssueBodyInput(bot, ctx, draft, input)
	case model.IssueStepReply:
		return b.handleIssueReplyInput(bot, ctx, draft, input)
	default:
		return nil
	}
}

func (b *Bot) handleIssueTitleInput(bot *gotgbot.Bot, ctx *ext.Context, draft *model.IssueDraft, input string) error {
	reqCtx := context.Background()

	if msg := validateIssueText(input, issueTitleMaxLen, "title"); msg != "" {
		return b.editIssuePrompt(bot, draft, formatIssueTitlePrompt(draft.IssueType, msg),
			issueWizardKeyboard(issuesCallbackPrefix+"new"))
	}

	draft.Title = input
	draft.Step = model.IssueStepBody
	if err := b.db.SetIssueDraft(reqCtx, *draft); err != nil {
		slog.Error("saving issue title", "error", err, "telegram_id", draft.TelegramID)
		return nil
	}

	return b.editIssuePrompt(bot, draft, formatIssueBodyPrompt(draft.IssueType, draft.Title, ""),
		issueWizardKeyboard(issuesCallbackPrefix+"back:title"))
}

func (b *Bot) handleIssueBodyInput(bot *gotgbot.Bot, ctx *ext.Context, draft *model.IssueDraft, input string) error {
	reqCtx := context.Background()

	if msg := validateIssueText(input, issueBodyMaxLen, "description"); msg != "" {
		return b.editIssuePrompt(bot, draft, formatIssueBodyPrompt(draft.IssueType, draft.Title, msg),
			issueWizardKeyboard(issuesCallbackPrefix+"back:title"))
	}

	from := ctx.EffectiveUser
	issue, err := b.db.CreateIssue(reqCtx, model.Issue{
		AuthorTelegramID: draft.TelegramID,
		AuthorUsername:   from.Username,
		AuthorFirstName:  from.FirstName,
		Type:             draft.IssueType,
		Title:            draft.Title,
		Body:             input,
	})
	if err != nil {
		slog.Error("creating issue", "error", err, "telegram_id", draft.TelegramID)
		return b.editIssuePrompt(bot, draft, formatIssueBodyPrompt(draft.IssueType, draft.Title,
			"Could not save the issue. Please send the description again."),
			issueWizardKeyboard(issuesCallbackPrefix+"back:title"))
	}

	_ = b.db.ClearIssueDraft(reqCtx, draft.TelegramID)

	return b.editIssuePrompt(bot, draft, formatIssueCreated(issue), issueCreatedKeyboard())
}

func (b *Bot) handleIssueReplyInput(bot *gotgbot.Bot, ctx *ext.Context, draft *model.IssueDraft, input string) error {
	reqCtx := context.Background()
	if draft.IssueID == nil {
		_ = b.db.ClearIssueDraft(reqCtx, draft.TelegramID)
		return nil
	}

	issue, err := b.db.GetIssueByID(reqCtx, *draft.IssueID)
	if err != nil {
		slog.Error("loading issue for reply", "error", err, "issue_id", draft.IssueID)
		_ = b.db.ClearIssueDraft(reqCtx, draft.TelegramID)
		return nil
	}
	if issue.AuthorTelegramID != draft.TelegramID {
		_ = b.db.ClearIssueDraft(reqCtx, draft.TelegramID)
		return nil
	}

	if msg := validateIssueText(input, model.IssueCommentMaxLen, "reply"); msg != "" {
		return b.editIssuePrompt(bot, draft, formatIssueReplyPrompt(issue, msg),
			issueWizardKeyboard(issuesCallbackPrefix+"thr:"+issue.ID.String()))
	}

	label := ctx.EffectiveUser.FirstName
	if ctx.EffectiveUser.Username != "" {
		label = "@" + ctx.EffectiveUser.Username
	}
	if _, err := b.db.AddIssueComment(reqCtx, model.IssueComment{
		IssueID:     issue.ID,
		AuthorRole:  model.IssueCommentUser,
		AuthorLabel: label,
		Body:        input,
	}); err != nil {
		slog.Error("saving issue reply", "error", err, "issue_id", issue.ID)
		return b.editIssuePrompt(bot, draft, formatIssueReplyPrompt(issue, "Could not send the reply. Please try again."),
			issueWizardKeyboard(issuesCallbackPrefix+"thr:"+issue.ID.String()))
	}

	_ = b.db.ClearIssueDraft(reqCtx, draft.TelegramID)

	comments, err := b.db.ListIssueComments(reqCtx, issue.ID)
	if err != nil {
		slog.Error("reloading issue thread", "error", err, "issue_id", issue.ID)
		return nil
	}

	return b.editIssuePrompt(bot, draft, formatIssueThread(issue, comments), issueThreadKeyboard(issue))
}

// validateIssueText returns an empty string when input is acceptable, or the
// error line to show above the prompt.
func validateIssueText(input string, max int, field string) string {
	if strings.TrimSpace(input) == "" {
		return fmt.Sprintf("The %s cannot be empty.", field)
	}
	if len([]rune(input)) > max {
		return fmt.Sprintf("That %s is too long — keep it under %d characters.", field, max)
	}
	return ""
}

// editIssuePrompt rewrites the wizard message recorded in the draft. There is
// no callback query here (the trigger was a plain text message), so this cannot
// go through applyScreen.
func (b *Bot) editIssuePrompt(bot *gotgbot.Bot, draft *model.IssueDraft, text string, kb gotgbot.InlineKeyboardMarkup) error {
	_, _, err := bot.EditMessageText(&gotgbot.EditMessageTextOpts{
		ChatId:             draft.ChatID,
		MessageId:          draft.PromptMessageID,
		Text:               text,
		ParseMode:          "HTML",
		ReplyMarkup:        kb,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
	})
	if err != nil && !isNotModified(err) {
		slog.Warn("could not edit issue wizard message", "error", err, "telegram_id", draft.TelegramID)
	}
	return nil
}

// SweepExpiredIssueDrafts discards drafts whose 10 minutes are up and removes
// the wizard messages they left behind. The lazy expiry in GetIssueDraft cannot
// do the cleanup on its own: an abandoned draft is, by definition, never
// touched again. Wired into the same per-minute heartbeats as lesson alerts, so
// it also runs after the Fly.io machine wakes from scale-to-zero.
func (b *Bot) SweepExpiredIssueDrafts(ctx context.Context, now time.Time) error {
	drafts, err := b.db.ListExpiredIssueDrafts(ctx, now)
	if err != nil {
		return fmt.Errorf("listing expired issue drafts: %w", err)
	}

	for _, draft := range drafts {
		if _, err := b.gbot.DeleteMessage(draft.ChatID, draft.PromptMessageID, nil); err != nil {
			// Best effort: the message may be older than Telegram's deletion
			// window, or already gone. The draft is dropped either way.
			slog.Warn("could not delete expired issue wizard message", "error", err, "telegram_id", draft.TelegramID)
		}
		if err := b.db.ClearIssueDraft(ctx, draft.TelegramID); err != nil {
			slog.Error("clearing expired issue draft", "error", err, "telegram_id", draft.TelegramID)
		}
	}
	return nil
}

func answerIssueError(bot *gotgbot.Bot, cq *gotgbot.CallbackQuery) error {
	_, err := bot.AnswerCallbackQuery(cq.Id, &gotgbot.AnswerCallbackQueryOpts{
		Text:      issuesGenericErrText,
		ShowAlert: true,
	})
	return err
}
