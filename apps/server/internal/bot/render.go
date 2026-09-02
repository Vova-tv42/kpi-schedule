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

// weekDayLine and weekInfo mirror api.weekDayView/weekBlockView the same way,
// flattened to the single week block the /week screen shows at a time.
type weekDayLine struct {
	DayName string
	Lessons []lessonLine
}

type weekInfo struct {
	WeekName         string
	EnrichmentStatus string
	Stale            bool
	Days             []weekDayLine
}

var isoDayNamesUA = map[int]string{
	1: "Понеділок", 2: "Вівторок", 3: "Середа", 4: "Четвер", 5: "П'ятниця", 6: "Субота", 7: "Неділя",
}

// isoWeekday returns the ISO weekday number (Monday=1 … Sunday=7), matching
// how days are numbered in stored lessons and in api's day-name maps.
func isoWeekday(t time.Time) int {
	if wd := int(t.Weekday()); wd != 0 {
		return wd
	}
	return 7
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

var tagShortLabels = map[string]string{
	"lec":  "лек.",
	"prac": "прак.",
	"lab":  "лаб.",
}

func tagShort(tag string) string {
	return tagShortLabels[tag]
}

func weekOffsetLabel(offset int) string {
	switch offset {
	case -1:
		return "Минулий"
	case 0:
		return "Поточний"
	case 1:
		return "Наступний"
	default:
		return ""
	}
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

// formatWeek renders one academic week as a compact HTML message: a line per
// lesson (time, subject, short tag) grouped under day headers, since a full
// per-lesson block for six days would not fit a Telegram message. Days that
// fall on today/tomorrow are marked, but only when the displayed week is the
// current one (offset 0) — otherwise no day in it is "today".
func formatWeek(w weekInfo, offset int, group *string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "🗓 %s — %s\n", html.EscapeString(w.WeekName), weekOffsetLabel(offset))
	if group != nil && *group != "" {
		fmt.Fprintf(&b, "🔹 Група: %s\n", html.EscapeString(*group))
	}

	switch {
	case w.Stale:
		b.WriteString("⚠️ Розклад міг застаріти — синхронізуй розширення ще раз.\n")
	case w.EnrichmentStatus == "degraded":
		b.WriteString("⚠️ Деякі деталі (аудиторія/викладач) тимчасово недоступні.\n")
	}

	if len(w.Days) == 0 {
		b.WriteString("\n🎉 Занять цього тижня немає.\n")
		return b.String()
	}

	var todayName, tomorrowName string
	if offset == 0 {
		today := isoWeekday(time.Now())
		todayName = isoDayNamesUA[today]
		tomorrowName = isoDayNamesUA[today%7+1]
	}

	for _, d := range w.Days {
		fmt.Fprintf(&b, "\n<b>%s</b>", html.EscapeString(d.DayName))
		switch d.DayName {
		case todayName:
			b.WriteString(" — <i>Сьогодні</i>")
		case tomorrowName:
			b.WriteString(" — <i>Завтра</i>")
		}
		b.WriteString("\n")

		for _, l := range d.Lessons {
			fmt.Fprintf(&b, "%s %s", l.Time, html.EscapeString(l.Name))
			if tag := tagShort(l.Tag); tag != "" {
				fmt.Fprintf(&b, " <i>(%s)</i>", tag)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

// dayKeyboard builds the ◀️/📅/▶️ day-navigation row. CallbackData encodes
// the action plus the *currently displayed* date (nav:<action>:<date>); the
// callback handler derives the target date from it — no message state needs
// to be persisted server-side, per docs/bot/telegram-bot-design.md §5.
func dayKeyboard(date time.Time) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "◀️ Вчора", CallbackData: navCallbackData("prev", date)},
				{Text: "📅 Сьогодні", CallbackData: navCallbackData("today", date)},
				{Text: "Завтра ▶️", CallbackData: navCallbackData("next", date)},
			},
		},
	}
}

// weekKeyboard builds the week screen's three fixed week slots (previous /
// current / next, all relative to the real current week, not to what is
// displayed) plus a jump to today's schedule. Telegram has no disabled-button
// state, so the slot already being displayed is rendered as a marked, inert
// button instead of being removed — the row keeps its shape as you navigate.
func weekKeyboard(offset int) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				weekNavButton(-1, offset, "◀️ Минулий"),
				weekNavButton(0, offset, "Поточний"),
				weekNavButton(1, offset, "Наступний ▶️"),
			},
			{
				{Text: "📅 Розклад на сьогодні", CallbackData: weekTodayCallbackData()},
			},
		},
	}
}

func weekNavButton(target, displayed int, label string) gotgbot.InlineKeyboardButton {
	if target == displayed {
		return gotgbot.InlineKeyboardButton{
			Text:         "✅ " + weekOffsetLabel(target),
			CallbackData: weekNoopCallbackData(),
		}
	}
	return gotgbot.InlineKeyboardButton{Text: label, CallbackData: weekCallbackData(target)}
}

// startKeyboard and linkKeyboard are the two onboarding screens. Both live in
// a single message that is edited in place: the start screen's one button
// swaps the message over to the pairing code, whose own buttons go back or
// forward into the schedule. The schedule screens deliberately have no route
// back here — onboarding is a one-way path.
// startKeyboard keeps the link button in every state — re-pairing stays
// available — and adds a direct route into the schedule once one has actually
// been synced and is still current.
func startKeyboard(state linkState) gotgbot.InlineKeyboardMarkup {
	rows := [][]gotgbot.InlineKeyboardButton{
		{{Text: "🔗 Прив'язати акаунт", CallbackData: menuCallbackData("link")}},
	}
	if state == linkStateFresh {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "📅 Розклад на сьогодні", CallbackData: menuCallbackData("today")},
		})
	}
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func linkKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "◀️ Назад", CallbackData: menuCallbackData("back")},
				{Text: "🗓 Показати розклад", CallbackData: menuCallbackData("week")},
			},
		},
	}
}

const startScreenBase = "👋 Вітаю! Я покажу твій персональний розклад занять КПІ.\n\n" +
	"Спочатку встанови браузерне розширення — інструкція буде тут пізніше.\n\n" +
	"Коли розширення встановлено, натисни кнопку нижче, щоб отримати код прив'язки."

// formatStartScreen appends a short status note for users who have already
// synced. The onboarding text itself never changes — the note is additive, so
// re-pairing instructions stay visible even for a linked user.
func formatStartScreen(state linkState) string {
	switch state {
	case linkStateFresh:
		return startScreenBase + "\n\n✅ Твій розклад уже синхронізовано — можеш одразу відкрити його кнопкою нижче."
	case linkStateStale:
		return startScreenBase + "\n\n⚠️ Розклад уже синхронізовано, але міг застаріти — відкрий розширення і синхронізуй ще раз."
	default:
		return startScreenBase
	}
}

func formatLinkText(code string, expiresIn int) string {
	return fmt.Sprintf(
		"🔑 Код прив'язки: <code>%s-%s</code>\n\nДійсний %d хвилин. Відкрий браузерне розширення KPI Schedule, увійди на my.kpi.ua і введи цей код, щоб синхронізувати розклад.",
		code[:3], code[3:], expiresIn/60,
	)
}
