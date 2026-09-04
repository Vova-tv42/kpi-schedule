package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

func sampleIssue(number int, title string) model.Issue {
	return model.Issue{
		ID:               uuid.New(),
		Number:           number,
		AuthorTelegramID: 42,
		Type:             model.IssueTypeFeature,
		Title:            title,
		Body:             "Body text",
		Status:           model.IssueOnReview,
		ThreadState:      model.IssueThreadNone,
		CreatedAt:        time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
	}
}

// The success message must carry the "**`#N` title**" headline the feature is
// specified around, rendered as HTML because that is the bot's parse mode.
func TestFormatIssueCreatedHeadline(t *testing.T) {
	text := formatIssueCreated(sampleIssue(7, "Add calendar export"))

	if !strings.Contains(text, "<b><code>#7</code> Add calendar export</b>") {
		t.Errorf("success screen missing the #N headline:\n%s", text)
	}
	if !strings.Contains(text, "💡 Пропозиція") || !strings.Contains(text, "🕓 На розгляді") {
		t.Errorf("success screen missing type/status line:\n%s", text)
	}
}

func TestIssueHeadlineEscapesHTML(t *testing.T) {
	headline := issueHeadline(sampleIssue(3, "<script>alert(1)</script> & more"))

	if strings.Contains(headline, "<script>") {
		t.Errorf("headline did not escape user input: %s", headline)
	}
	if !strings.Contains(headline, "&lt;script&gt;") || !strings.Contains(headline, "&amp; more") {
		t.Errorf("headline escaping looks wrong: %s", headline)
	}
	if !strings.HasPrefix(headline, "<b><code>#3</code> ") {
		t.Errorf("headline lost its structure: %s", headline)
	}
}

// The type picker is the first wizard step, so going back is cancelling.
func TestIssueTypePickerHasCancelButNoBack(t *testing.T) {
	kb := issueTypePickerKeyboard()

	var texts, data []string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			texts = append(texts, btn.Text)
			data = append(data, btn.CallbackData)
		}
	}

	joinedText := strings.Join(texts, "|")
	if strings.Contains(joinedText, "Назад") {
		t.Errorf("type picker must not offer Back, got buttons: %s", joinedText)
	}
	if !strings.Contains(joinedText, "Скасувати") {
		t.Errorf("type picker must offer Cancel, got buttons: %s", joinedText)
	}

	for _, want := range []string{"iss:type:feature", "iss:type:bug", "iss:type:other", "iss:cancel"} {
		if !containsString(data, want) {
			t.Errorf("type picker missing callback %q, got %v", want, data)
		}
	}
}

func TestIssueWizardKeyboardHasBackAndCancel(t *testing.T) {
	kb := issueWizardKeyboard(issuesCallbackPrefix + "new")

	if len(kb.InlineKeyboard) != 1 || len(kb.InlineKeyboard[0]) != 2 {
		t.Fatalf("expected a single Back/Cancel row, got %+v", kb.InlineKeyboard)
	}
	back, cancel := kb.InlineKeyboard[0][0], kb.InlineKeyboard[0][1]
	if !strings.Contains(back.Text, "Назад") || back.CallbackData != "iss:new" {
		t.Errorf("unexpected back button: %+v", back)
	}
	if !strings.Contains(cancel.Text, "Скасувати") || cancel.CallbackData != "iss:cancel" {
		t.Errorf("unexpected cancel button: %+v", cancel)
	}
}

// The created screen is the one step with no way back — the issue already exists.
func TestIssueCreatedKeyboardHasNoBackOrCancel(t *testing.T) {
	for _, row := range issueCreatedKeyboard().InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Назад") || strings.Contains(btn.Text, "Скасувати") {
				t.Errorf("success screen should not offer %q", btn.Text)
			}
		}
	}
}

func TestIssueStatusAndTypeLabels(t *testing.T) {
	statuses := map[model.IssueStatus]string{
		model.IssueOnReview:      "На розгляді",
		model.IssueReady:         "Готово до розробки",
		model.IssueInDevelopment: "У розробці",
		model.IssueImplemented:   "Реалізовано",
		model.IssueCancelled:     "Скасовано",
	}
	for status, want := range statuses {
		if got := issueStatusLabel(status); !strings.Contains(got, want) {
			t.Errorf("issueStatusLabel(%q) = %q, want it to contain %q", status, got, want)
		}
	}

	types := map[model.IssueType]string{
		model.IssueTypeFeature: "Пропозиція",
		model.IssueTypeBug:     "Помилка",
		model.IssueTypeOther:   "Інше",
	}
	for issueType, want := range types {
		if got := issueTypeLabel(issueType); !strings.Contains(got, want) {
			t.Errorf("issueTypeLabel(%q) = %q, want it to contain %q", issueType, got, want)
		}
	}
}

func TestIssueListKeyboardCarriesPageAndPaginates(t *testing.T) {
	issues := []model.Issue{sampleIssue(12, "Add calendar export"), sampleIssue(11, "Dark mode")}
	kb := issueListKeyboard(issues, 1, 12)

	first := kb.InlineKeyboard[0][0]
	if !strings.HasPrefix(first.CallbackData, "iss:view:1:") {
		t.Errorf("view callback should carry the current page, got %q", first.CallbackData)
	}
	if len(first.CallbackData) > 64 {
		t.Errorf("callback data exceeds Telegram's 64-byte limit: %q", first.CallbackData)
	}

	var data []string
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			data = append(data, btn.CallbackData)
		}
	}
	if !containsString(data, "iss:list:0") || !containsString(data, "iss:list:2") {
		t.Errorf("expected prev/next pagination on page 2 of 3, got %v", data)
	}
	if !containsString(data, "iss:new") || !containsString(data, "iss:menu") {
		t.Errorf("expected New issue and Back buttons, got %v", data)
	}
}

func TestIssueListKeyboardWithoutPagination(t *testing.T) {
	kb := issueListKeyboard([]model.Issue{sampleIssue(1, "Only one")}, 0, 1)

	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "◀️" || btn.Text == "▶️" {
				t.Errorf("single-page list should have no pagination arrows, got %q", btn.Text)
			}
		}
	}
}

func TestFormatIssueListEmptyState(t *testing.T) {
	text := formatIssueList(nil, 0, 0)
	if !strings.Contains(text, "ще не створили жодного звернення") {
		t.Errorf("empty list should explain there is nothing yet:\n%s", text)
	}
}

// The discussion button only exists once an admin has started a thread.
func TestIssueViewKeyboardHidesUnstartedThread(t *testing.T) {
	issue := sampleIssue(5, "Week view crashes")

	for _, row := range issueViewKeyboard(issue, 0, 0).InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Обговорення") {
				t.Error("discussion button should be hidden until a thread is started")
			}
		}
	}

	issue.ThreadState = model.IssueThreadOpen
	found := false
	for _, row := range issueViewKeyboard(issue, 3, 2).InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Обговорення (3)") {
				found = true
				if btn.CallbackData != "iss:thr:"+issue.ID.String() {
					t.Errorf("discussion callback = %q", btn.CallbackData)
				}
			}
			if strings.Contains(btn.Text, "Назад") && btn.CallbackData != "iss:list:2" {
				t.Errorf("back should return to the originating page, got %q", btn.CallbackData)
			}
		}
	}
	if !found {
		t.Error("expected a discussion button once the thread is open")
	}
}

func TestValidateIssueText(t *testing.T) {
	if msg := validateIssueText("  ", issueTitleMaxLen, issueFieldTitle); msg == "" {
		t.Error("blank title should be rejected")
	}
	if msg := validateIssueText(strings.Repeat("я", issueTitleMaxLen+1), issueTitleMaxLen, issueFieldTitle); msg == "" {
		t.Error("over-long title should be rejected")
	}
	// Counted in runes, so a full-length Cyrillic title still fits.
	if msg := validateIssueText(strings.Repeat("я", issueTitleMaxLen), issueTitleMaxLen, issueFieldTitle); msg != "" {
		t.Errorf("exactly-max title should be accepted, got %q", msg)
	}
	if msg := validateIssueText("Add calendar export", issueTitleMaxLen, issueFieldTitle); msg != "" {
		t.Errorf("valid title rejected: %q", msg)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("short", 10); got != "short" {
		t.Errorf("truncateRunes(short) = %q", got)
	}
	got := truncateRunes(strings.Repeat("я", 40), 10)
	if len([]rune(got)) > 10 {
		t.Errorf("truncateRunes produced %d runes, want at most 10", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated label should be elided, got %q", got)
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestFormatIssueThreadAttributesBothSides(t *testing.T) {
	issue := model.Issue{
		ID:     uuid.New(),
		Number: 12,
		Title:  "Add calendar export",
		Status: model.IssueInDevelopment,
	}
	comments := []model.IssueComment{
		{AuthorRole: model.IssueCommentAdmin, AuthorLabel: "admin@example.com", Body: "Which calendar app?"},
		{AuthorRole: model.IssueCommentUser, AuthorLabel: "@student", Body: "Google Calendar <3"},
	}

	out := formatIssueThread(issue, comments)

	if !strings.Contains(out, "🛠 Команда") || !strings.Contains(out, "👤 Ви") {
		t.Errorf("expected both sides attributed, got:\n%s", out)
	}
	// The admin's identity stays inside the dashboard.
	if strings.Contains(out, "admin@example.com") {
		t.Errorf("admin email must not leak to the reporter, got:\n%s", out)
	}
	if !strings.Contains(out, "Google Calendar &lt;3") {
		t.Errorf("expected comment bodies to be HTML-escaped, got:\n%s", out)
	}
	if !strings.Contains(out, issueHeadline(issue)) {
		t.Errorf("expected the issue headline, got:\n%s", out)
	}
}

func TestFormatIssueThreadEmptyState(t *testing.T) {
	out := formatIssueThread(model.Issue{ID: uuid.New(), Number: 3, Title: "Dark mode"}, nil)
	if !strings.Contains(out, "Поки що немає повідомлень.") {
		t.Errorf("expected an empty-thread notice, got:\n%s", out)
	}
}

func TestIssueNotificationsDeepLinkToTheRightScreen(t *testing.T) {
	issue := model.Issue{ID: uuid.New(), Number: 12, Title: "Add calendar export", Status: model.IssueInDevelopment}
	id := issue.ID.String()

	comment := model.IssueComment{AuthorRole: model.IssueCommentAdmin, Body: "Which calendar app?"}
	commentText := formatIssueCommentNotification(issue, comment)
	if !strings.Contains(commentText, issueHeadline(issue)) || !strings.Contains(commentText, "Which calendar app?") {
		t.Errorf("expected the headline and the quoted reply, got:\n%s", commentText)
	}
	commentBtn := issueCommentNotificationKeyboard(issue).InlineKeyboard[0][0]
	if commentBtn.CallbackData != issuesCallbackPrefix+"thr:"+id {
		t.Errorf("expected the comment DM to open the thread, got %q", commentBtn.CallbackData)
	}

	statusText := formatIssueStatusNotification(issue, model.IssueOnReview)
	if !strings.Contains(statusText, issueStatusLabel(model.IssueOnReview)) ||
		!strings.Contains(statusText, issueStatusLabel(model.IssueInDevelopment)) {
		t.Errorf("expected both the old and the new status, got:\n%s", statusText)
	}
	statusBtn := issueStatusNotificationKeyboard(issue).InlineKeyboard[0][0]
	if statusBtn.CallbackData != issuesCallbackPrefix+"view:0:"+id {
		t.Errorf("expected the status DM to open the issue, got %q", statusBtn.CallbackData)
	}
	// Telegram caps callback_data at 64 bytes.
	for _, data := range []string{commentBtn.CallbackData, statusBtn.CallbackData} {
		if len(data) > 64 {
			t.Errorf("callback_data %q exceeds Telegram's 64-byte limit", data)
		}
	}
}

func TestIssueStatusLabelsCoverEveryStatus(t *testing.T) {
	all := []model.IssueStatus{
		model.IssueOnReview, model.IssueReady, model.IssueInDevelopment,
		model.IssueImplemented, model.IssueDuplicate, model.IssueRejected, model.IssueCancelled,
	}
	seen := map[string]bool{}
	for _, status := range all {
		label := issueStatusLabel(status)
		if seen[label] {
			t.Errorf("status %q reuses the label %q of another status", status, label)
		}
		seen[label] = true
	}
	if got := issueStatusLabel(model.IssueRejected); !strings.Contains(got, "Відхилено") {
		t.Errorf("rejected label = %q", got)
	}
	if got := issueStatusLabel(model.IssueDuplicate); !strings.Contains(got, "Дублікат") {
		t.Errorf("duplicate label = %q", got)
	}
}

func TestIssueViewShowsStatusNote(t *testing.T) {
	issue := sampleIssue(9, "Add calendar export")
	issue.Status = model.IssueRejected
	issue.StatusNote = "Out of scope <for now>"

	text := formatIssueView(issue, 0)
	if !strings.Contains(text, "Коментар команди") {
		t.Errorf("expected the status note to be shown on the issue:\n%s", text)
	}
	if !strings.Contains(text, "Out of scope &lt;for now&gt;") {
		t.Errorf("expected the note to be HTML-escaped:\n%s", text)
	}

	// An issue with no note must not grow an empty section.
	issue.StatusNote = ""
	if strings.Contains(formatIssueView(issue, 0), "Коментар команди") {
		t.Error("expected no note section when there is no note")
	}
}

func TestIssueStatusNotificationCarriesTheNote(t *testing.T) {
	issue := sampleIssue(9, "Add calendar export")
	issue.Status = model.IssueRejected
	issue.StatusNote = "Duplicate of #4"

	text := formatIssueStatusNotification(issue, model.IssueOnReview)
	if !strings.Contains(text, "Duplicate of #4") {
		t.Errorf("expected the note in the status DM:\n%s", text)
	}

	issue.StatusNote = ""
	if strings.Contains(formatIssueStatusNotification(issue, model.IssueOnReview), "Коментар команди") {
		t.Error("expected no note section in a plain status DM")
	}
}

func TestClosedThreadHidesReply(t *testing.T) {
	issue := sampleIssue(9, "Add calendar export")
	issue.ThreadState = model.IssueThreadOpen

	hasReply := func(kb gotgbot.InlineKeyboardMarkup) bool {
		for _, row := range kb.InlineKeyboard {
			for _, btn := range row {
				if strings.Contains(btn.Text, "Відповісти") {
					return true
				}
			}
		}
		return false
	}

	if !hasReply(issueThreadKeyboard(issue)) {
		t.Error("an open thread must offer Reply")
	}

	issue.ThreadState = model.IssueThreadClosed
	if hasReply(issueThreadKeyboard(issue)) {
		t.Error("a closed thread must not offer Reply")
	}

	// The transcript stays reachable, and says why writing stopped.
	text := formatIssueThread(issue, []model.IssueComment{
		{AuthorRole: model.IssueCommentAdmin, Body: "Thanks!"},
	})
	if !strings.Contains(text, "закрито") {
		t.Errorf("a closed thread should say so:\n%s", text)
	}
	if !strings.Contains(text, "Thanks!") {
		t.Errorf("a closed thread must still show its history:\n%s", text)
	}

	// The issue screen keeps a padlocked entry point into the closed thread.
	var found bool
	for _, row := range issueViewKeyboard(issue, 2, 0).InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Обговорення (2)") {
				found = true
			}
		}
	}
	if !found {
		t.Error("a closed discussion should still be reachable from the issue")
	}
}

func TestIssueDeleteConfirmation(t *testing.T) {
	issue := sampleIssue(9, "Add calendar export")

	// The issue screen offers Delete, but only via a confirmation step.
	var deleteData string
	for _, row := range issueViewKeyboard(issue, 0, 3).InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Видалити") {
				deleteData = btn.CallbackData
			}
		}
	}
	if deleteData != "iss:del:3:"+issue.ID.String() {
		t.Fatalf("delete callback = %q", deleteData)
	}

	text := formatIssueDeleteConfirm(issue)
	if !strings.Contains(text, "Скасувати цю дію неможливо") {
		t.Errorf("the confirmation should warn that deletion is permanent:\n%s", text)
	}

	kb := issueDeleteConfirmKeyboard(issue, 3)
	if got := kb.InlineKeyboard[0][0].CallbackData; got != "iss:delok:3:"+issue.ID.String() {
		t.Errorf("confirm callback = %q", got)
	}
	// Backing out returns to the issue on the page it was opened from.
	if got := kb.InlineKeyboard[1][0].CallbackData; got != "iss:view:3:"+issue.ID.String() {
		t.Errorf("keep-it callback = %q", got)
	}
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			if len(btn.CallbackData) > 64 {
				t.Errorf("callback_data %q exceeds Telegram's 64-byte limit", btn.CallbackData)
			}
		}
	}
}

// Telegram refuses a message longer than 4096 parsed characters, and a refused
// edit strands the reporter on a spinning button — so every screen built from
// user- or admin-supplied text has to fit on its own.
func TestIssueScreensFitTelegramMessageLimit(t *testing.T) {
	long := strings.Repeat("я", model.IssueCommentMaxLen)

	issue := sampleIssue(12, strings.Repeat("t", issueTitleMaxLen))
	issue.Body = long
	issue.StatusNote = long
	issue.ThreadState = model.IssueThreadOpen

	view := formatIssueView(issue, 40)
	if n := renderedLen(view); n > telegramTextMaxLen {
		t.Errorf("formatIssueView rendered %d characters, want at most %d", n, telegramTextMaxLen)
	}
	if !strings.Contains(view, "Коментар команди") {
		t.Errorf("the team's note must survive truncation, got:\n%s", view)
	}

	comments := make([]model.IssueComment, 12)
	for i := range comments {
		comments[i] = model.IssueComment{
			AuthorRole: model.IssueCommentAdmin,
			Body:       long,
			CreatedAt:  time.Date(2026, 9, 4, 12, i, 0, 0, time.UTC),
		}
	}
	comments[len(comments)-1].Body = "The newest message."

	thread := formatIssueThread(issue, comments)
	if n := renderedLen(thread); n > telegramTextMaxLen {
		t.Errorf("formatIssueThread rendered %d characters, want at most %d", n, telegramTextMaxLen)
	}
	// The tail is what the reporter is here for; the head is what gets dropped.
	if !strings.Contains(thread, "The newest message.") {
		t.Errorf("expected the newest message to survive, got:\n%s", thread)
	}
	if !strings.Contains(thread, "приховано") {
		t.Errorf("expected a notice for the dropped messages, got:\n%s", thread)
	}
}

// A short thread is shown whole, with no truncation notice.
func TestFormatIssueThreadKeepsShortTranscriptsIntact(t *testing.T) {
	issue := sampleIssue(12, "Add calendar export")
	comments := []model.IssueComment{
		{AuthorRole: model.IssueCommentAdmin, Body: "Which calendar app?"},
		{AuthorRole: model.IssueCommentUser, Body: "Google Calendar."},
	}

	out := formatIssueThread(issue, comments)
	if strings.Contains(out, "приховано") {
		t.Errorf("a two-message thread must not be truncated, got:\n%s", out)
	}
	if !strings.Contains(out, "Which calendar app?") || !strings.Contains(out, "Google Calendar.") {
		t.Errorf("expected both messages, got:\n%s", out)
	}
	if strings.Index(out, "Which calendar app?") > strings.Index(out, "Google Calendar.") {
		t.Errorf("expected oldest-first ordering, got:\n%s", out)
	}
}
