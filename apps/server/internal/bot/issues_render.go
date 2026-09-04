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

// Every /issues screen is Ukrainian, like the rest of the bot.
const (
	issuesDMOnlyText      = "⚠️ Ця команда доступна лише в приватному чаті з ботом."
	issuesGenericErrText  = "⚠️ Щось пішло не так. Спробуйте ще раз за мить."
	issuesInterruptedText = "⚠️ <b>Це зайняло забагато часу.</b> Чернетку втрачено через 10 хвилин — нічого не збережено. Можете почати знову нижче.\n\n"
)

func issueTypeLabel(t model.IssueType) string {
	switch t {
	case model.IssueTypeFeature:
		return "💡 Пропозиція"
	case model.IssueTypeBug:
		return "🐞 Помилка"
	case model.IssueTypeOther:
		return "📝 Інше"
	default:
		return "📝 Інше"
	}
}

func issueStatusLabel(s model.IssueStatus) string {
	switch s {
	case model.IssueOnReview:
		return "🕓 На розгляді"
	case model.IssueReady:
		return "📌 Готово до розробки"
	case model.IssueInDevelopment:
		return "🔨 У розробці"
	case model.IssueImplemented:
		return "✅ Реалізовано"
	case model.IssueDuplicate:
		return "🔁 Дублікат"
	case model.IssueRejected:
		return "⛔ Відхилено"
	case model.IssueCancelled:
		return "🚫 Скасовано"
	default:
		return "🕓 На розгляді"
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
	b.WriteString("📮 <b>Звернення</b>\n\n")
	if notice != "" {
		b.WriteString(notice)
	}
	b.WriteString("Знайшли помилку або маєте ідею? Створіть звернення — команда розбереться з ним тут.\n\n")
	b.WriteString("Ви можете стежити за всім, що надсилали, — кожне звернення отримує номер і статус.")
	return b.String()
}

func issuesMenuKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "📋 Мої звернення", CallbackData: issuesCallbackPrefix + "list:0"}},
			{{Text: "➕ Нове звернення", CallbackData: issuesCallbackPrefix + "new"}},
		},
	}
}

func formatIssueTypePicker() string {
	return "➕ <b>Нове звернення</b>\n\nЯкого типу це звернення?"
}

// issueTypePickerKeyboard deliberately has no Back button: this is the first
// step of the wizard, so going back is the same as cancelling.
func issueTypePickerKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "💡 Пропозиція", CallbackData: issuesCallbackPrefix + "type:feature"}},
			{{Text: "🐞 Помилка", CallbackData: issuesCallbackPrefix + "type:bug"}},
			{{Text: "📝 Інше", CallbackData: issuesCallbackPrefix + "type:other"}},
			{{Text: "✖️ Скасувати", CallbackData: issuesCallbackPrefix + "cancel"}},
		},
	}
}

func formatIssueTitlePrompt(t model.IssueType, errorMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "➕ <b>Нове звернення</b> · %s\n\n", issueTypeLabel(t))
	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}
	b.WriteString("<b>Крок 1 з 2 — Заголовок</b>\n")
	fmt.Fprintf(&b, "Надішліть короткий і зрозумілий заголовок (до %d символів).", issueTitleMaxLen)
	return b.String()
}

func formatIssueBodyPrompt(t model.IssueType, title, errorMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "➕ <b>Нове звернення</b> · %s\n\n", issueTypeLabel(t))
	fmt.Fprintf(&b, "<b>%s</b>\n\n", html.EscapeString(title))
	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}
	b.WriteString("<b>Крок 2 з 2 — Опис</b>\n")
	fmt.Fprintf(&b, "Тепер надішліть деталі: що відбувається, чого ви очікували, як це відтворити (до %d символів).", issueBodyMaxLen)
	return b.String()
}

// issueWizardKeyboard is the Back/Cancel pair shown on every wizard step that
// has something to go back to.
func issueWizardKeyboard(backCallback string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "◀️ Назад", CallbackData: backCallback},
				{Text: "✖️ Скасувати", CallbackData: issuesCallbackPrefix + "cancel"},
			},
		},
	}
}

// formatIssueCreated is the success screen. It is the only wizard screen with
// neither Back nor Cancel — the issue already exists at this point.
func formatIssueCreated(issue model.Issue) string {
	var b strings.Builder
	b.WriteString("✅ <b>Звернення створено</b>\n\n")
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n%s · %s\n\n", issueTypeLabel(issue.Type), issueStatusLabel(issue.Status))
	b.WriteString("Дякуємо! Команда невдовзі його розгляне.\n")
	b.WriteString("Перевірити статус можна будь-коли командою /issues.")
	return b.String()
}

func issueCreatedKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "📋 Мої звернення", CallbackData: issuesCallbackPrefix + "list:0"}},
		},
	}
}

func formatIssueList(issues []model.Issue, page, total int) string {
	var b strings.Builder
	b.WriteString("📋 <b>Мої звернення</b>\n\n")

	if total == 0 {
		b.WriteString("Ви ще не створили жодного звернення.\nНатисніть <b>«➕ Нове звернення»</b> нижче, щоб повідомити про помилку або запропонувати ідею.")
		return b.String()
	}

	for _, issue := range issues {
		b.WriteString(issueHeadline(issue))
		fmt.Fprintf(&b, "\n%s · %s\n\n", issueTypeLabel(issue.Type), issueStatusLabel(issue.Status))
	}

	pages := (total + issuesPageSize - 1) / issuesPageSize
	if pages > 1 {
		fmt.Fprintf(&b, "<i>Сторінка %d з %d · усього звернень: %d</i>", page+1, pages, total)
	} else {
		b.WriteString("Натисніть на звернення, щоб відкрити його.")
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
		{Text: "➕ Нове звернення", CallbackData: issuesCallbackPrefix + "new"},
	})
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: issuesCallbackPrefix + "menu"},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatIssueView(issue model.Issue, commentCount int) string {
	var b strings.Builder
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n%s · %s\n", issueTypeLabel(issue.Type), issueStatusLabel(issue.Status))
	fmt.Fprintf(&b, "🗓 Створено %s\n\n", issue.CreatedAt.Format("02.01.2006"))

	// The body and the team's note are each allowed to run to 3000 characters,
	// so together they overflow a Telegram message. Split what the header left
	// of the budget between them, giving the note at most half.
	const noteHeading = "\n\n🛠 <b>Коментар команди</b>\n"
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
		fmt.Fprintf(&b, "\n\n💬 Команда розпочала обговорення цього звернення (%d %s).",
			commentCount, pluralUA(commentCount, "повідомлення", "повідомлення", "повідомлень"))
	case model.IssueThreadClosed:
		fmt.Fprintf(&b, "\n\n🔒 Обговорення цього звернення закрито (%d %s). Його все ще можна читати.",
			commentCount, pluralUA(commentCount, "повідомлення", "повідомлення", "повідомлень"))
	}
	return b.String()
}

func issueViewKeyboard(issue model.Issue, commentCount, page int) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	// A closed thread is still readable, so the button stays — only the padlock
	// and the missing Reply inside tell the user they can no longer write.
	if issue.ThreadState.Started() {
		label := fmt.Sprintf("💬 Обговорення (%d)", commentCount)
		if issue.ThreadState == model.IssueThreadClosed {
			label = fmt.Sprintf("🔒 Обговорення (%d)", commentCount)
		}
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: label, CallbackData: fmt.Sprintf("%sthr:%s", issuesCallbackPrefix, issue.ID.String())},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "🗑 Видалити", CallbackData: fmt.Sprintf("%sdel:%d:%s", issuesCallbackPrefix, page, issue.ID.String())},
	})
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf("%slist:%d", issuesCallbackPrefix, page)},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// formatIssueDeleteConfirm is the interstitial that keeps a stray tap from
// destroying an issue and its whole discussion.
func formatIssueDeleteConfirm(issue model.Issue) string {
	var b strings.Builder
	b.WriteString("🗑 <b>Видалити це звернення?</b>\n")
	b.WriteString(issueHeadline(issue))
	b.WriteString("\n\nЦе назавжди видалить звернення та його обговорення — і для вас, і для команди. Скасувати цю дію неможливо.")
	return b.String()
}

func issueDeleteConfirmKeyboard(issue model.Issue, page int) gotgbot.InlineKeyboardMarkup {
	id := issue.ID.String()
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "🗑 Так, видалити", CallbackData: fmt.Sprintf("%sdelok:%d:%s", issuesCallbackPrefix, page, id)}},
			{{Text: "◀️ Залишити", CallbackData: fmt.Sprintf("%sview:%d:%s", issuesCallbackPrefix, page, id)}},
		},
	}
}

// pluralUA picks the Ukrainian plural form for n: "1 повідомлення",
// "2 повідомлення", "5 повідомлень".
func pluralUA(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}
	if n%100 >= 11 && n%100 <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

// formatIssueThread renders the full admin↔user transcript, oldest first.
func formatIssueThread(issue model.Issue, comments []model.IssueComment) string {
	var b strings.Builder
	if issue.ThreadState == model.IssueThreadClosed {
		b.WriteString("🔒 <b>Обговорення (закрито)</b>\n")
	} else {
		b.WriteString("💬 <b>Обговорення</b>\n")
	}
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n%s\n\n", issueStatusLabel(issue.Status))

	if issue.ThreadState == model.IssueThreadClosed {
		b.WriteString("Команда закрила це обговорення. Його ще можна читати, але надсилати нові повідомлення вже не можна.\n\n")
	}

	if len(comments) == 0 {
		b.WriteString("Поки що немає повідомлень.")
		return b.String()
	}

	// A transcript grows without bound while a single message cannot, so the
	// screen is filled newest-first and the oldest messages are dropped — a
	// truncated tail beats a message Telegram refuses to send.
	const hiddenNotice = "<i>… приховано %d попередніх %s. Повне обговорення — у панелі команди.</i>\n\n"
	budget := issueScreenBudget - renderedLen(b.String()) - renderedLen(fmt.Sprintf(hiddenNotice, len(comments), "повідомлень"))

	blocks := make([]string, 0, len(comments))
	hidden := 0
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]
		author := "🛠 Команда"
		if c.AuthorRole == model.IssueCommentUser {
			author = "👤 Ви"
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
		fmt.Fprintf(&b, hiddenNotice, hidden, pluralUA(hidden, "повідомлення", "повідомлень", "повідомлень"))
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
			{Text: "✍️ Відповісти", CallbackData: issuesCallbackPrefix + "reply:" + id},
			{Text: "🔄 Оновити", CallbackData: issuesCallbackPrefix + "thr:" + id},
		})
	} else {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "🔄 Оновити", CallbackData: issuesCallbackPrefix + "thr:" + id},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: issuesCallbackPrefix + "view:0:" + id},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatIssueReplyPrompt(issue model.Issue, errorMsg string) string {
	var b strings.Builder
	b.WriteString("✍️ <b>Відповідь</b>\n")
	b.WriteString(issueHeadline(issue))
	b.WriteString("\n\n")
	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}
	fmt.Fprintf(&b, "Надішліть відповідь повідомленням (до %d символів). Команда побачить її в цьому зверненні.", model.IssueCommentMaxLen)
	return b.String()
}

// formatIssueCommentNotification is the DM the reporter gets when an admin
// replies. It quotes the reply so the message is useful on its own.
func formatIssueCommentNotification(issue model.Issue, comment model.IssueComment) string {
	var b strings.Builder
	b.WriteString("💬 <b>Нова відповідь у вашому зверненні</b>\n")
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n\n<blockquote>%s</blockquote>", html.EscapeString(comment.Body))
	return b.String()
}

func issueCommentNotificationKeyboard(issue model.Issue) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "💬 Відкрити обговорення", CallbackData: issuesCallbackPrefix + "thr:" + issue.ID.String()}},
		},
	}
}

// formatIssueStatusNotification is the DM the reporter gets when triage moves
// their issue along.
func formatIssueStatusNotification(issue model.Issue, previous model.IssueStatus) string {
	var b strings.Builder
	b.WriteString("🔄 <b>Статус звернення змінився</b>\n")
	b.WriteString(issueHeadline(issue))
	fmt.Fprintf(&b, "\n\n%s → <b>%s</b>", issueStatusLabel(previous), issueStatusLabel(issue.Status))
	// The admin's optional explanation, e.g. why the issue was rejected.
	if issue.StatusNote != "" {
		fmt.Fprintf(&b, "\n\n🛠 <b>Коментар команди</b>\n<blockquote>%s</blockquote>",
			html.EscapeString(issue.StatusNote))
	}
	return b.String()
}

// formatIssueThreadStateNotification tells the reporter their discussion was
// closed or reopened, so a Reply button that vanishes is never a mystery.
func formatIssueThreadStateNotification(issue model.Issue, _ model.IssueThreadState) string {
	var b strings.Builder
	if issue.ThreadState == model.IssueThreadClosed {
		b.WriteString("🔒 <b>Обговорення закрито</b>\n")
		b.WriteString(issueHeadline(issue))
		b.WriteString("\n\nКоманда закрила обговорення цього звернення. Історію можна читати, але надсилати нові повідомлення вже не можна.")
		return b.String()
	}
	b.WriteString("💬 <b>Обговорення відновлено</b>\n")
	b.WriteString(issueHeadline(issue))
	b.WriteString("\n\nКоманда відновила обговорення цього звернення. Ви знову можете відповідати.")
	return b.String()
}

func issueThreadStateNotificationKeyboard(issue model.Issue) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "💬 Відкрити обговорення", CallbackData: issuesCallbackPrefix + "thr:" + issue.ID.String()}},
		},
	}
}

func issueStatusNotificationKeyboard(issue model.Issue) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "📄 Відкрити звернення", CallbackData: issuesCallbackPrefix + "view:0:" + issue.ID.String()}},
		},
	}
}
