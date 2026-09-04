package bot

import (
	"fmt"
	"html"
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
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
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
	fmt.Fprintf(&b, "<blockquote>%s</blockquote>", html.EscapeString(issue.Body))

	if issue.ThreadOpen {
		fmt.Fprintf(&b, "\n\n💬 The team started a discussion on this issue (%d %s).",
			commentCount, pluralEN(commentCount, "message", "messages"))
	}
	return b.String()
}

func issueViewKeyboard(issue model.Issue, commentCount, page int) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	if issue.ThreadOpen {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{
				Text:         fmt.Sprintf("💬 Discussion (%d)", commentCount),
				CallbackData: fmt.Sprintf("%sthr:%s", issuesCallbackPrefix, issue.ID.String()),
			},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Back", CallbackData: fmt.Sprintf("%slist:%d", issuesCallbackPrefix, page)},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func pluralEN(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
