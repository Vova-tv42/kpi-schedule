package bot

import (
	"strings"
	"testing"
	"time"

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
	if !strings.Contains(text, "💡 Feature request") || !strings.Contains(text, "🕓 On review") {
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
	if strings.Contains(joinedText, "Back") {
		t.Errorf("type picker must not offer Back, got buttons: %s", joinedText)
	}
	if !strings.Contains(joinedText, "Cancel") {
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
	if !strings.Contains(back.Text, "Back") || back.CallbackData != "iss:new" {
		t.Errorf("unexpected back button: %+v", back)
	}
	if !strings.Contains(cancel.Text, "Cancel") || cancel.CallbackData != "iss:cancel" {
		t.Errorf("unexpected cancel button: %+v", cancel)
	}
}

// The created screen is the one step with no way back — the issue already exists.
func TestIssueCreatedKeyboardHasNoBackOrCancel(t *testing.T) {
	for _, row := range issueCreatedKeyboard().InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Back") || strings.Contains(btn.Text, "Cancel") {
				t.Errorf("success screen should not offer %q", btn.Text)
			}
		}
	}
}

func TestIssueStatusAndTypeLabels(t *testing.T) {
	statuses := map[model.IssueStatus]string{
		model.IssueOnReview:      "On review",
		model.IssueReady:         "Ready for development",
		model.IssueInDevelopment: "In development",
		model.IssueImplemented:   "Implemented",
		model.IssueCancelled:     "Cancelled",
	}
	for status, want := range statuses {
		if got := issueStatusLabel(status); !strings.Contains(got, want) {
			t.Errorf("issueStatusLabel(%q) = %q, want it to contain %q", status, got, want)
		}
	}

	types := map[model.IssueType]string{
		model.IssueTypeFeature: "Feature request",
		model.IssueTypeBug:     "Bug fix",
		model.IssueTypeOther:   "Other",
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
	if !strings.Contains(text, "haven't filed any issues") {
		t.Errorf("empty list should explain there is nothing yet:\n%s", text)
	}
}

// The discussion button only exists once an admin has opened a thread.
func TestIssueViewKeyboardHidesClosedThread(t *testing.T) {
	issue := sampleIssue(5, "Week view crashes")

	for _, row := range issueViewKeyboard(issue, 0, 0).InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Discussion") {
				t.Error("discussion button should be hidden while the thread is closed")
			}
		}
	}

	issue.ThreadOpen = true
	found := false
	for _, row := range issueViewKeyboard(issue, 3, 2).InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Discussion (3)") {
				found = true
				if btn.CallbackData != "iss:thr:"+issue.ID.String() {
					t.Errorf("discussion callback = %q", btn.CallbackData)
				}
			}
			if strings.Contains(btn.Text, "Back") && btn.CallbackData != "iss:list:2" {
				t.Errorf("back should return to the originating page, got %q", btn.CallbackData)
			}
		}
	}
	if !found {
		t.Error("expected a discussion button once the thread is open")
	}
}

func TestValidateIssueText(t *testing.T) {
	if msg := validateIssueText("  ", issueTitleMaxLen, "title"); msg == "" {
		t.Error("blank title should be rejected")
	}
	if msg := validateIssueText(strings.Repeat("я", issueTitleMaxLen+1), issueTitleMaxLen, "title"); msg == "" {
		t.Error("over-long title should be rejected")
	}
	// Counted in runes, so a full-length Cyrillic title still fits.
	if msg := validateIssueText(strings.Repeat("я", issueTitleMaxLen), issueTitleMaxLen, "title"); msg != "" {
		t.Errorf("exactly-max title should be accepted, got %q", msg)
	}
	if msg := validateIssueText("Add calendar export", issueTitleMaxLen, "title"); msg != "" {
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
