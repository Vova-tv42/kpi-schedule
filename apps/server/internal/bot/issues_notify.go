package bot

import (
	"context"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/model"
)

// NotifyIssueComment DMs the reporter that an admin replied on their issue,
// with a button that opens the thread directly. Satisfies api.IssueNotifier;
// wired up in cmd/server/main.go.
func (b *Bot) NotifyIssueComment(_ context.Context, issue model.Issue, comment model.IssueComment) error {
	_, err := b.gbot.SendMessage(issue.AuthorTelegramID,
		formatIssueCommentNotification(issue, comment),
		&gotgbot.SendMessageOpts{
			ParseMode:          "HTML",
			ReplyMarkup:        issueCommentNotificationKeyboard(issue),
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		})
	if err != nil {
		return fmt.Errorf("sending issue comment notification: %w", err)
	}
	return nil
}

// NotifyIssueStatus DMs the reporter that triage moved their issue along.
func (b *Bot) NotifyIssueStatus(_ context.Context, issue model.Issue, previous model.IssueStatus) error {
	_, err := b.gbot.SendMessage(issue.AuthorTelegramID,
		formatIssueStatusNotification(issue, previous),
		&gotgbot.SendMessageOpts{
			ParseMode:          "HTML",
			ReplyMarkup:        issueStatusNotificationKeyboard(issue),
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		})
	if err != nil {
		return fmt.Errorf("sending issue status notification: %w", err)
	}
	return nil
}
