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
	Name          string
	Tag           string
	TeacherRaw    string
	LocationRaw   string
	LecturerName  string
	LocationTitle string
}

type dayInfo struct {
	Date             string // YYYY-MM-DD
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
	WeekNumber       int
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

var tagShortLabels = map[string]string{
	"lec":  "лек.",
	"prac": "прак.",
	"lab":  "лаб.",
}

func tagShort(tag string) string {
	return tagShortLabels[tag]
}

var tagAbbrLabels = map[string]string{
	"lec":  "Лек.",
	"prac": "Практ.",
	"lab":  "Лаб.",
}

func tagAbbr(tag string) string {
	if v, ok := tagAbbrLabels[tag]; ok {
		return v
	}
	return "Заняття"
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

func weekOrdinal(week int) string {
	switch week {
	case 1:
		return "Перший"
	case 2:
		return "Другий"
	default:
		return ""
	}
}

// hhmm truncates a stored "HH:MM:SS" time to "HH:MM" for display.
func hhmm(t string) string {
	if len(t) >= 5 {
		return t[:5]
	}
	return t
}

// shortDate reformats a stored "YYYY-MM-DD" date to "DD.MM" for display —
// the year adds nothing a student needs to see on a day/week screen.
func shortDate(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("02.01")
}

// locationKind classifies a lesson's raw/enriched location as online or
// on-campus by keyword, since that's the only thing worth surfacing inline
// (see lessonModeSuffix) — the exact room number is dropped as clutter.
func locationKind(location string) string {
	if location == "" {
		return ""
	}
	lower := strings.ToLower(location)
	if strings.Contains(lower, "онлайн") || strings.Contains(lower, "zoom") {
		return "Online"
	}
	return "Offline"
}

// lessonModeSuffix renders "(Лек., Online)"-style trailer text appended after
// a lesson's name, replacing the separate location row it used to get.
func lessonModeSuffix(location, tag string) string {
	parts := []string{tagAbbr(tag)}
	if kind := locationKind(location); kind != "" {
		parts = append(parts, kind)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// formatDay renders a day's schedule as an HTML-parse-mode Telegram message,
// matching the layout in docs/bot/telegram-bot-design.md §3.1. All dynamic
// text is HTML-escaped since subject/teacher/room names come from external
// sources (my.kpi.ua / Campus API) and may contain "&", "<", etc.
func formatDay(d dayInfo) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📅 Розклад на <b>%s</b> (%s)\n", html.EscapeString(shortDate(d.Date)), html.EscapeString(d.DayName))

	switch {
	case d.Stale:
		b.WriteString("⚠️ Розклад міг застаріти — синхронізуй розширення ще раз.\n")
	case d.EnrichmentStatus == "degraded":
		b.WriteString("⚠️ Деякі деталі тимчасово недоступні.\n")
	}

	if d.IsDayOff || len(d.Lessons) == 0 {
		b.WriteString("\n🎉 Пар немає — вихідний!\n")
		return b.String()
	}

	for _, l := range d.Lessons {
		b.WriteString("\n")

		location := l.LocationTitle
		if location == "" {
			location = l.LocationRaw
		}
		fmt.Fprintf(&b, "<code>%s</code> %s <i>%s</i>\n", hhmm(l.Time), html.EscapeString(l.Name), lessonModeSuffix(location, l.Tag))

		teacher := l.LecturerName
		if teacher == "" {
			teacher = l.TeacherRaw
		}
		if teacher != "" {
			fmt.Fprintf(&b, "<b>Викладач:</b> %s\n", html.EscapeString(teacher))
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

	fmt.Fprintf(&b, "🗓 <b>%s</b> тиждень — %s\n", weekOrdinal(w.WeekNumber), weekOffsetLabel(offset))
	if group != nil && *group != "" {
		fmt.Fprintf(&b, "<b>Група:</b> %s\n", html.EscapeString(*group))
	}

	switch {
	case w.Stale:
		b.WriteString("⚠️ Розклад міг застаріти — синхронізуй розширення ще раз.\n")
	case w.EnrichmentStatus == "degraded":
		b.WriteString("⚠️ Деякі деталі тимчасово недоступні.\n")
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
		b.WriteString("\n<blockquote><b>")
		b.WriteString(html.EscapeString(d.DayName))
		b.WriteString("</b>")
		switch d.DayName {
		case todayName:
			b.WriteString(" - <i>Сьогодні</i>")
		case tomorrowName:
			b.WriteString(" - <i>Завтра</i>")
		}
		b.WriteString("</blockquote>\n")

		for _, l := range d.Lessons {
			fmt.Fprintf(&b, "<code>%s</code> %s", hhmm(l.Time), html.EscapeString(l.Name))
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
