package bot

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/model"
)

// Input bounds for the /issues wizard. Telegram messages can be far longer;
// these keep an issue readable in the dashboard queue and in a thread screen.
const (
	issueTitleMaxLen = 120
	issueBodyMaxLen  = 3000
)

// issuesPageSize is how many of a user's issues fit on one /issues list screen.
const issuesPageSize = 5

// The /issues screens are the bot's only English surface, at the user's
// explicit request; every other screen in this package is Ukrainian.
const (
	issuesDMOnlyText      = "⚠️ This command is available in a direct message with the bot only."
	issuesGenericErrText  = "⚠️ Something went wrong. Please try again in a moment."
	issuesInterruptedText = "⚠️ <b>That took too long.</b> The draft expired after 10 minutes and was discarded — nothing was saved. You can start again below.\n\n"
)

func issueTypeLabel(t model.IssueType) string {
	switch t {
	case model.IssueTypeFeature:
		return "💡 Feature request"
	case model.IssueTypeBug:
		return "🐞 Bug fix"
	case model.IssueTypeOther:
		return "📝 Other"
	default:
		return "📝 Other"
	}
}

func issueStatusLabel(s model.IssueStatus) string {
	switch s {
	case model.IssueOnReview:
		return "🕓 On review"
	case model.IssueReady:
		return "📌 Ready for development"
	case model.IssueInDevelopment:
		return "🔨 In development"
	case model.IssueImplemented:
		return "✅ Implemented"
	case model.IssueDuplicate:
		return "🔁 Duplicate"
	case model.IssueRejected:
		return "⛔ Rejected"
	case model.IssueCancelled:
		return "🚫 Cancelled"
	default:
		return "🕓 On review"
	}
}

// issueHeadline renders the "**`#N` title**" line the whole feature is built
// around. The bot sends HTML, not Markdown, so bold+code map to <b>/<code>.
func issueHeadline(issue model.Issue) string {
	return fmt.Sprintf("<b><code>#%d</code> %s</b>", issue.Number, html.EscapeString(issue.Title))
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}

// telegramTextMaxLen is Telegram's hard limit for a message's text. The limit
// applies to the parsed text, so the HTML markup and the entity escapes these
// screens are built from do not count against it.
const telegramTextMaxLen = 4096

// issueScreenBudget is the length an issue screen renders within. The headroom
// under telegramTextMaxLen absorbs the wording of whatever notice a screen adds
// around the parts being measured — an overlong screen is not merely ugly, it
// is unsendable, which strands the user on a spinning button.
const issueScreenBudget = telegramTextMaxLen - 196

// minCommentPreview is the shortest excerpt of a discussion message worth
// showing; anything less and the message is dropped from the screen instead.
const minCommentPreview = 80

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// renderedLen is how long a fragment of these screens will be once Telegram has
// parsed the HTML away — the length the 4096 limit is actually measured against.
func renderedLen(s string) int {
	return len([]rune(html.UnescapeString(htmlTagPattern.ReplaceAllString(s, ""))))
}

// formatIssuesMenu is the /issues root screen. notice carries a one-off banner
// (e.g. the expired-draft warning) above the body text.
func formatIssuesMenu(notice string) string {
	var b strings.Builder
	b.WriteString("📮 <b>Issues</b>\n\n")
	if notice != "" {
		b.WriteString(notice)
	}
	b.WriteString("Found a bug or have an idea? File an issue and the team will pick it up here.\n\n")
	b.WriteString("You can follow what happens to everything you file — every issue gets a number and a status.")
	return b.String()
}

func issuesMenuKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "📋 My issues", CallbackData: issuesCallbackPrefix + "list:0"}},
			{{Text: "➕ New issue", CallbackData: issuesCallbackPrefix + "new"}},
		},
	}
}

func formatIssueTypePicker() string {
	return "➕ <b>New issue</b>\n\nWhat kind of issue is this?"
}

// issueTypePickerKeyboard deliberately has no Back button: this is the first
// step of the wizard, so going back is the same as cancelling.
func issueTypePickerKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "💡 Feature request", CallbackData: issuesCallbackPrefix + "type:feature"}},
			{{Text: "🐞 Bug fix", CallbackData: issuesCallbackPrefix + "type:bug"}},
			{{Text: "📝 Other", CallbackData: issuesCallbackPrefix + "type:other"}},
			{{Text: "✖️ Cancel", CallbackData: issuesCallbackPrefix + "cancel"}},
		},
	}
}

func formatIssueTitlePrompt(t model.IssueType, errorMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "➕ <b>New issue</b> · %s\n\n", issueTypeLabel(t))
	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}
	b.WriteString("<b>Step 1 of 2 — Title</b>\n")
	fmt.Fprintf(&b, "Send a short, descriptive title (up to %d characters).", issueTitleMaxLen)
	return b.String()
}

func formatIssueBodyPrompt(t model.IssueType, title, errorMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "➕ <b>New issue</b> · %s\n\n", issueTypeLabel(t))
	fmt.Fprintf(&b, "<b>%s</b>\n\n", html.EscapeString(title))
	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}
	b.WriteString("<b>Step 2 of 2 — Description</b>\n")
	fmt.Fprintf(&b, "Now send the details: what happens, what you expected, how to reproduce it (up to %d characters).", issueBodyMaxLen)
	return b.String()
}

// issueWizardKeyboard is the Back/Cancel pair shown on every wizard step that
// has something to go back to.
func issueWizardKeyboard(backCallback string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "◀️ Back", CallbackData: backCallback},
				{Text: "✖️ Cancel", CallbackData: issuesCallbackPrefix + "cancel"},
			},
		},
	}
}

// formatIssueCreated is the success screen. It is the only wizard screen with
// neither Back nor Cancel — the issue already exists at this point.
func formatIssueCreated(issue model.Issue) string {
	var b strings.Builder
	b.WriteString("✅ <b>Issue created</b>\n\n")
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n%s · %s\n\n", issueTypeLabel(issue.Type), issueStatusLabel(issue.Status))
	b.WriteString("Thanks! The team will review it soon.\n")
	b.WriteString("You can track its status any time with /issues.")
	return b.String()
}

func issueCreatedKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "📋 My issues", CallbackData: issuesCallbackPrefix + "list:0"}},
		},
	}
}

func formatIssueList(issues []model.Issue, page, total int) string {
	var b strings.Builder
	b.WriteString("📋 <b>My issues</b>\n\n")

	if total == 0 {
		b.WriteString("You haven't filed any issues yet.\nUse <b>«➕ New issue»</b> below to report a bug or suggest a feature.")
		return b.String()
	}

	for _, issue := range issues {
		b.WriteString(issueHeadline(issue))
		fmt.Fprintf(&b, "\n%s · %s\n\n", issueTypeLabel(issue.Type), issueStatusLabel(issue.Status))
	}

	pages := (total + issuesPageSize - 1) / issuesPageSize
	if pages > 1 {
		fmt.Fprintf(&b, "<i>Page %d of %d · %d issues total</i>", page+1, pages, total)
	} else {
		b.WriteString("Tap an issue to open it.")
	}
	return b.String()
}

func issueListKeyboard(issues []model.Issue, page, total int) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	for _, issue := range issues {
		label := fmt.Sprintf("#%d %s", issue.Number, truncateRunes(issue.Title, 28))
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			// The page travels in the callback data so the issue screen's Back
			// button returns to the page the user opened it from.
			{Text: label, CallbackData: fmt.Sprintf("%sview:%d:%s", issuesCallbackPrefix, page, issue.ID.String())},
		})
	}

	pages := (total + issuesPageSize - 1) / issuesPageSize
	if pages > 1 {
		var nav []gotgbot.InlineKeyboardButton
		if page > 0 {
			nav = append(nav, gotgbot.InlineKeyboardButton{
				Text: "◀️", CallbackData: fmt.Sprintf("%slist:%d", issuesCallbackPrefix, page-1),
			})
		}
		if page < pages-1 {
			nav = append(nav, gotgbot.InlineKeyboardButton{
				Text: "▶️", CallbackData: fmt.Sprintf("%slist:%d", issuesCallbackPrefix, page+1),
			})
		}
		if len(nav) > 0 {
			rows = append(rows, nav)
		}
	}

	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "➕ New issue", CallbackData: issuesCallbackPrefix + "new"},
	})
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Back", CallbackData: issuesCallbackPrefix + "menu"},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatIssueView(issue model.Issue, commentCount int) string {
	var b strings.Builder
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n%s · %s\n", issueTypeLabel(issue.Type), issueStatusLabel(issue.Status))
	fmt.Fprintf(&b, "🗓 Filed %s\n\n", issue.CreatedAt.Format("02.01.2006"))

	// The body and the team's note are each allowed to run to 3000 characters,
	// so together they overflow a Telegram message. Split what the header left
	// of the budget between them, giving the note at most half.
	const noteHeading = "\n\n🛠 <b>Note from the team</b>\n"
	budget := issueScreenBudget - renderedLen(b.String())
	noteBudget := 0
	if issue.StatusNote != "" {
		budget -= renderedLen(noteHeading)
		noteBudget = budget / 2
	}

	fmt.Fprintf(&b, "<blockquote>%s</blockquote>", html.EscapeString(truncateRunes(issue.Body, budget-noteBudget)))

	// The note an admin attached to the last status change. Kept on the issue
	// screen, not just in the one-off DM, so it can be re-read later.
	if issue.StatusNote != "" {
		fmt.Fprintf(&b, noteHeading+"<blockquote>%s</blockquote>",
			html.EscapeString(truncateRunes(issue.StatusNote, noteBudget)))
	}

	switch issue.ThreadState {
	case model.IssueThreadOpen:
		fmt.Fprintf(&b, "\n\n💬 The team started a discussion on this issue (%d %s).",
			commentCount, pluralEN(commentCount, "message", "messages"))
	case model.IssueThreadClosed:
		fmt.Fprintf(&b, "\n\n🔒 The discussion on this issue is closed (%d %s). You can still read it.",
			commentCount, pluralEN(commentCount, "message", "messages"))
	}
	return b.String()
}

func issueViewKeyboard(issue model.Issue, commentCount, page int) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	// A closed thread is still readable, so the button stays — only the padlock
	// and the missing Reply inside tell the user they can no longer write.
	if issue.ThreadState.Started() {
		label := fmt.Sprintf("💬 Discussion (%d)", commentCount)
		if issue.ThreadState == model.IssueThreadClosed {
			label = fmt.Sprintf("🔒 Discussion (%d)", commentCount)
		}
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: label, CallbackData: fmt.Sprintf("%sthr:%s", issuesCallbackPrefix, issue.ID.String())},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "🗑 Delete", CallbackData: fmt.Sprintf("%sdel:%d:%s", issuesCallbackPrefix, page, issue.ID.String())},
	})
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Back", CallbackData: fmt.Sprintf("%slist:%d", issuesCallbackPrefix, page)},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// formatIssueDeleteConfirm is the interstitial that keeps a stray tap from
// destroying an issue and its whole discussion.
func formatIssueDeleteConfirm(issue model.Issue) string {
	var b strings.Builder
	b.WriteString("🗑 <b>Delete this issue?</b>\n")
	b.WriteString(issueHeadline(issue))
	b.WriteString("\n\nThis removes the issue and its discussion permanently, for you and for the team. It cannot be undone.")
	return b.String()
}

func issueDeleteConfirmKeyboard(issue model.Issue, page int) gotgbot.InlineKeyboardMarkup {
	id := issue.ID.String()
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "🗑 Yes, delete it", CallbackData: fmt.Sprintf("%sdelok:%d:%s", issuesCallbackPrefix, page, id)}},
			{{Text: "◀️ Keep it", CallbackData: fmt.Sprintf("%sview:%d:%s", issuesCallbackPrefix, page, id)}},
		},
	}
}

func pluralEN(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// formatIssueThread renders the full admin↔user transcript, oldest first.
func formatIssueThread(issue model.Issue, comments []model.IssueComment) string {
	var b strings.Builder
	if issue.ThreadState == model.IssueThreadClosed {
		b.WriteString("🔒 <b>Discussion (closed)</b>\n")
	} else {
		b.WriteString("💬 <b>Discussion</b>\n")
	}
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n%s\n\n", issueStatusLabel(issue.Status))

	if issue.ThreadState == model.IssueThreadClosed {
		b.WriteString("The team closed this discussion. You can still read it, but you can't send new messages.\n\n")
	}

	if len(comments) == 0 {
		b.WriteString("No messages yet.")
		return b.String()
	}

	// A transcript grows without bound while a single message cannot, so the
	// screen is filled newest-first and the oldest messages are dropped — a
	// truncated tail beats a message Telegram refuses to send.
	const hiddenNotice = "<i>… %d earlier %s hidden. The full discussion is in the dashboard.</i>\n\n"
	budget := issueScreenBudget - renderedLen(b.String()) - renderedLen(fmt.Sprintf(hiddenNotice, len(comments), "messages"))

	blocks := make([]string, 0, len(comments))
	hidden := 0
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]
		author := "🛠 Team"
		if c.AuthorRole == model.IssueCommentUser {
			author = "👤 You"
		}
		head := fmt.Sprintf("%s · <i>%s</i>\n", author, c.CreatedAt.Format("02.01.2006 15:04"))
		room := budget - renderedLen(head) - len("\n\n")
		if room < minCommentPreview {
			hidden = i + 1
			break
		}
		body := truncateRunes(c.Body, room)
		budget -= renderedLen(head) + len([]rune(body)) + len("\n\n")
		blocks = append(blocks, fmt.Sprintf("%s<blockquote>%s</blockquote>\n\n", head, html.EscapeString(body)))
	}

	if hidden > 0 {
		fmt.Fprintf(&b, hiddenNotice, hidden, pluralEN(hidden, "message", "messages"))
	}
	for i := len(blocks) - 1; i >= 0; i-- {
		b.WriteString(blocks[i])
	}
	return strings.TrimRight(b.String(), "\n")
}

func issueThreadKeyboard(issue model.Issue) gotgbot.InlineKeyboardMarkup {
	id := issue.ID.String()
	var rows [][]gotgbot.InlineKeyboardButton
	// Reply disappears once the team closes the discussion; the transcript and
	// its Refresh stay, because a closed thread is still worth re-reading.
	if issue.ThreadState == model.IssueThreadOpen {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "✍️ Reply", CallbackData: issuesCallbackPrefix + "reply:" + id},
			{Text: "🔄 Refresh", CallbackData: issuesCallbackPrefix + "thr:" + id},
		})
	} else {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "🔄 Refresh", CallbackData: issuesCallbackPrefix + "thr:" + id},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Back", CallbackData: issuesCallbackPrefix + "view:0:" + id},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatIssueReplyPrompt(issue model.Issue, errorMsg string) string {
	var b strings.Builder
	b.WriteString("✍️ <b>Reply</b>\n")
	b.WriteString(issueHeadline(issue))
	b.WriteString("\n\n")
	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}
	fmt.Fprintf(&b, "Send your reply as a message (up to %d characters). The team will see it on this issue.", model.IssueCommentMaxLen)
	return b.String()
}

// formatIssueCommentNotification is the DM the reporter gets when an admin
// replies. It quotes the reply so the message is useful on its own.
func formatIssueCommentNotification(issue model.Issue, comment model.IssueComment) string {
	var b strings.Builder
	b.WriteString("💬 <b>New reply on your issue</b>\n")
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n\n<blockquote>%s</blockquote>", html.EscapeString(comment.Body))
	return b.String()
}

func issueCommentNotificationKeyboard(issue model.Issue) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "💬 Open discussion", CallbackData: issuesCallbackPrefix + "thr:" + issue.ID.String()}},
		},
	}
}

// formatIssueStatusNotification is the DM the reporter gets when triage moves
// their issue along.
func formatIssueStatusNotification(issue model.Issue, previous model.IssueStatus) string {
	var b strings.Builder
	b.WriteString("🔄 <b>Issue status changed</b>\n")
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n\n%s → <b>%s</b>", issueStatusLabel(previous), issueStatusLabel(issue.Status))
	// The admin's optional explanation, e.g. why the issue was rejected.
	if issue.StatusNote != "" {
		fmt.Fprintf(&b, "\n\n🛠 <b>Note from the team</b>\n<blockquote>%s</blockquote>",
			html.EscapeString(issue.StatusNote))
	}
	return b.String()
}

// formatIssueThreadStateNotification tells the reporter their discussion was
// closed or reopened, so a Reply button that vanishes is never a mystery.
func formatIssueThreadStateNotification(issue model.Issue, _ model.IssueThreadState) string {
	var b strings.Builder
	if issue.ThreadState == model.IssueThreadClosed {
		b.WriteString("🔒 <b>Discussion closed</b>\n")
		b.WriteString(issueHeadline(issue))
		b.WriteString("\n\nThe team closed the discussion on this issue. You can still read the history, but you can't send new messages.")
		return b.String()
	}
	b.WriteString("💬 <b>Discussion reopened</b>\n")
	b.WriteString(issueHeadline(issue))
	b.WriteString("\n\nThe team reopened the discussion on this issue. You can reply again.")
	return b.String()
}

func issueThreadStateNotificationKeyboard(issue model.Issue) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "💬 Open discussion", CallbackData: issuesCallbackPrefix + "thr:" + issue.ID.String()}},
		},
	}
}

func issueStatusNotificationKeyboard(issue model.Issue) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "📄 Open issue", CallbackData: issuesCallbackPrefix + "view:0:" + issue.ID.String()}},
		},
	}
}
