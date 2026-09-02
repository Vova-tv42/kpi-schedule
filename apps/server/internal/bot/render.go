package bot

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// lessonLine and dayInfo are plain, bot-local mirrors of the (unexported)
// api.lessonView/dayView shape. Handlers copy the fields they need out of
// api.Service.BuildDay's result into these before rendering, rather than
// this package trying to name those unexported types directly.
type lessonLine struct {
	Time          string
	EndTime       string
	Name          string
	Tag           string
	TeacherRaw    string
	LocationRaw   string
	LecturerName  string
	LocationTitle string
	Enriched      bool
}

type dayInfo struct {
	Date             string // YYYY-MM-DD
	Week             int
	DayName          string
	IsDayOff         bool
	EnrichmentStatus string
	Stale            bool
	Lessons          []lessonLine
}

var numberEmoji = []string{"", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣"}

func lessonNumber(i int) string {
	if i < len(numberEmoji) {
		return numberEmoji[i]
	}
	return fmt.Sprintf("%d.", i)
}

var tagLabels = map[string]string{
	"lec":  "Лекція",
	"prac": "Практика",
	"lab":  "Лабораторна",
}

func tagLabel(tag string) string {
	if label, ok := tagLabels[tag]; ok {
		return label
	}
	return "Заняття"
}

func weekLabel(week int) string {
	switch week {
	case 1:
		return "1-й тиждень (Чисельник)"
	case 2:
		return "2-й тиждень (Знаменник)"
	default:
		return ""
	}
}

// formatDay renders a day's schedule as an HTML-parse-mode Telegram message,
// matching the layout in docs/bot/telegram-bot-design.md §3.1. All dynamic
// text is HTML-escaped since subject/teacher/room names come from external
// sources (my.kpi.ua / Campus API) and may contain "&", "<", etc.
func formatDay(d dayInfo, group *string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📅 Розклад на %s (%s)\n", html.EscapeString(d.Date), html.EscapeString(d.DayName))

	header := "🔹 " + weekLabel(d.Week)
	if group != nil && *group != "" {
		header += " | Група: " + html.EscapeString(*group)
	}
	b.WriteString(header)
	b.WriteString("\n")

	switch {
	case d.Stale:
		b.WriteString("⚠️ Розклад міг застаріти — синхронізуй розширення ще раз.\n")
	case d.EnrichmentStatus == "degraded":
		b.WriteString("⚠️ Деякі деталі (аудиторія/викладач) тимчасово недоступні.\n")
	}

	if d.IsDayOff || len(d.Lessons) == 0 {
		b.WriteString("\n🎉 Пар немає — вихідний!\n")
		return b.String()
	}

	for i, l := range d.Lessons {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s %s — %s | %s\n", lessonNumber(i+1), l.Time, l.EndTime, tagLabel(l.Tag))
		fmt.Fprintf(&b, "📖 %s\n", html.EscapeString(l.Name))

		teacher := l.LecturerName
		if teacher == "" {
			teacher = l.TeacherRaw
		}
		if teacher != "" {
			fmt.Fprintf(&b, "👨‍🏫 %s\n", html.EscapeString(teacher))
		}

		location := l.LocationTitle
		if location == "" {
			location = l.LocationRaw
		}
		if location != "" {
			fmt.Fprintf(&b, "📍 %s\n", html.EscapeString(location))
		}
	}

	return b.String()
}

// dayKeyboard builds the ◀️/🔄/▶️ day-navigation row. CallbackData encodes
// the action plus the *currently displayed* date (nav:<action>:<date>); the
// callback handler derives the target date from it — no message state needs
// to be persisted server-side, per docs/bot/telegram-bot-design.md §5.
func dayKeyboard(date time.Time) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "◀️ Вчора", CallbackData: navCallbackData("prev", date)},
				{Text: "🔄 Оновити", CallbackData: navCallbackData("refresh", date)},
				{Text: "Завтра ▶️", CallbackData: navCallbackData("next", date)},
			},
		},
	}
}
