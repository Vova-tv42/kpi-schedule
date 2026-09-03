package bot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/model"
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
	URL           string
}

type dayInfo struct {
	Date             string // YYYY-MM-DD
	DayName          string
	IsDayOff         bool
	EnrichmentStatus string
	Stale            bool
	CallerName       string
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
	CallerName       string
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

// formatLessonMode renders "[Лек.|Практ., Онлайн|Оффлайн]" after a lesson name.
// If url is provided, the text is wrapped in an HTML link.
func formatLessonMode(tag, location, url string) string {
	kind := model.LocationKind(location)
	text := fmt.Sprintf("[%s, %s]", tagAbbr(tag), kind)
	if url != "" {
		return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(url), html.EscapeString(text))
	}
	return html.EscapeString(text)
}

func lessonHash(subjectNorm, tag string) string {
	h := sha256.Sum256([]byte(subjectNorm + "|" + tag))
	return hex.EncodeToString(h[:6])
}

// formatDay renders a day's schedule as an HTML-parse-mode Telegram message,
// matching the layout in docs/bot/telegram-bot-design.md §3.1. All dynamic
// text is HTML-escaped since subject/teacher/room names come from external
// sources (my.kpi.ua / Campus API) and may contain "&", "<", etc.
func formatDay(d dayInfo) string {
	var b strings.Builder

	if d.CallerName != "" {
		fmt.Fprintf(&b, "👤 Розклад: <b>%s</b>\n\n", html.EscapeString(d.CallerName))
	}

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
		fmt.Fprintf(&b, "<code>%s</code> %s <i>%s</i>\n", hhmm(l.Time), html.EscapeString(l.Name), formatLessonMode(l.Tag, location, l.URL))

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

	if w.CallerName != "" {
		fmt.Fprintf(&b, "👤 Розклад: <b>%s</b>\n\n", html.EscapeString(w.CallerName))
	}

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
			location := l.LocationTitle
			if location == "" {
				location = l.LocationRaw
			}
			fmt.Fprintf(&b, "<code>%s</code> %s <i>%s</i>\n", hhmm(l.Time), html.EscapeString(l.Name), formatLessonMode(l.Tag, location, l.URL))
		}
	}

	return b.String()
}

func formatLessonsMenu(lessons []model.UniqueLesson, notice string) string {
	var b strings.Builder
	b.WriteString("🔗 <b>Посилання на онлайн-заняття</b>\n\n")

	if notice != "" {
		b.WriteString(notice)
		b.WriteString("\n\n")
	}

	if len(lessons) == 0 {
		b.WriteString("📭 У твоєму розкладі не знайдено онлайн-занять для додавання посилань.\n")
		return b.String()
	}

	for _, l := range lessons {
		mode := formatLessonMode(l.Tag, "Онлайн", l.URL)
		fmt.Fprintf(&b, "• %s <i>%s</i>\n", html.EscapeString(l.Subject), mode)
	}

	b.WriteString("\nОбери заняття зі списку нижче, щоб додати або змінити посилання:")
	return b.String()
}

func urlsKeyboard(lessons []model.UniqueLesson) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	for _, l := range lessons {
		prefix := "➕ "
		if l.URL != "" {
			prefix = "🔗 "
		}
		title := l.Subject
		runes := []rune(title)
		if len(runes) > 30 {
			title = string(runes[:27]) + "..."
		}
		btnText := fmt.Sprintf("%s%s (%s)", prefix, title, tagAbbr(l.Tag))
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: btnText, CallbackData: urlsCallbackPrefix + "edit:" + lessonHash(l.SubjectNorm, l.Tag)},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "📅 До розкладу", CallbackData: urlsCallbackPrefix + "today"},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatURLPrompt(subjectName, tag, currentURL, errorMsg string) string {
	var b strings.Builder
	mode := formatLessonMode(tag, "Онлайн", currentURL)
	fmt.Fprintf(&b, "🔗 <b>%s</b> <i>%s</i>\n\n", html.EscapeString(subjectName), mode)

	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}

	if currentURL != "" {
		fmt.Fprintf(&b, "Поточне посилання: <a href=\"%s\">%s</a>\n\n", html.EscapeString(currentURL), html.EscapeString(currentURL))
	} else {
		b.WriteString("Поточне посилання: <i>не встановлено</i>\n\n")
	}

	b.WriteString("Надішли посилання на це заняття (Zoom, Google Meet тощо):")
	return b.String()
}

func urlPromptKeyboard(hasURL bool, hash string) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	if hasURL {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "🗑 Видалити посилання", CallbackData: urlsCallbackPrefix + "del:" + hash},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: urlsCallbackPrefix + "back"},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
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
		{
			{Text: "📥 Як встановити розширення", CallbackData: menuCallbackData("install")},
			{Text: "🔗 Прив'язати акаунт", CallbackData: menuCallbackData("link")},
		},
	}
	if state == linkStateFresh {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "📅 Розклад на сьогодні", CallbackData: menuCallbackData("today")},
		})
	}
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func installKeyboard(downloadURL string) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	if downloadURL != "" {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "📥 Встановити розширення", Url: downloadURL},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "🔑 Отримати код прив'язки", CallbackData: menuCallbackData("link")},
	})
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: menuCallbackData("back")},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func linkKeyboard() gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "📥 Як встановити розширення", CallbackData: menuCallbackData("install")},
			},
			{
				{Text: "◀️ Назад", CallbackData: menuCallbackData("back")},
				{Text: "🗓 Показати розклад", CallbackData: menuCallbackData("week")},
			},
		},
	}
}

const startScreenBase = "👋 <b>Вітаю! Я покажу твій персональний розклад КПІ.</b>\n\n" +
	"Я враховую твої вибіркові дисципліни та підгрупи завдяки швидкій синхронізації через браузерне розширення для комп'ютера.\n\n" +
	"<b>Щоб підключити розклад:</b>\n" +
	"1️⃣ Встанови розширення в браузер (Chrome, Edge, Brave, Opera).\n" +
	"2️⃣ Натисни «Прив'язати акаунт» та отримай 6-значний код.\n" +
	"3️⃣ Увійди на my.kpi.ua і синхронізуй розклад в один клік!"

// formatStartScreen appends a short status note for users who have already
// synced. The onboarding text itself never changes — the note is additive, so
// re-pairing instructions stay visible even for a linked user.
func formatStartScreen(state linkState) string {
	switch state {
	case linkStateFresh:
		return startScreenBase + "\n\n✅ <b>Твій розклад уже синхронізовано!</b> Можеш одразу відкрити його кнопкою нижче."
	case linkStateStale:
		return startScreenBase + "\n\n⚠️ <b>Розклад застарів</b> — відкрий розширення в браузері та синхронізуй ще раз."
	default:
		return startScreenBase
	}
}

func formatInstallScreen() string {
	return "📥 <b>Встановлення розширення (Chrome / Edge / Brave / Opera)</b>\n\n" +
		"Розширення працює на десктопних браузерах (Windows, macOS, Linux):\n\n" +
		"1️⃣ <b>Отримай розширення:</b> натисни кнопку <b>«Встановити розширення»</b> нижче та завантаж архів чи перейди на сторінку розширення.\n\n" +
		"2️⃣ <b>Відкрий керування розширеннями:</b> перейди в браузері за адресою:\n" +
		"<code>chrome://extensions</code>\n" +
		"<i>(для Edge: <code>edge://extensions</code>)</i>\n\n" +
		"3️⃣ <b>Увімкни режим розробника:</b> увімкни перемикач <b>«Режим розробника»</b> (<i>Developer mode</i>) у правому верхньому кутку.\n\n" +
		"4️⃣ <b>Завантаж розширення:</b> натисни кнопку <b>«Завантажити розпаковане»</b> (<i>Load unpacked</i>) ліворуч угорі та обери розпаковану папку.\n\n" +
		"Після цього повертайся сюди й тисни <b>«Отримати код прив'язки»</b>!"
}

func formatLinkText(code string, expiresIn int) string {
	return fmt.Sprintf(
		"🔑 Код прив'язки: <code>%s-%s</code>\n\nДійсний %d хвилин. Відкрий браузерне розширення KPI Schedule, увійди на my.kpi.ua і введи цей код, щоб синхронізувати розклад.",
		code[:3], code[3:], expiresIn/60,
	)
}

// formatGroupDay renders a group's schedule for a single date.
func formatGroupDay(d dayInfo, groupName string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "👥 <b>Розклад групи %s</b>\n", html.EscapeString(groupName))
	fmt.Fprintf(&b, "📅 Розклад на <b>%s</b> (%s)\n", html.EscapeString(shortDate(d.Date)), html.EscapeString(d.DayName))

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
		fmt.Fprintf(&b, "<code>%s</code> %s <i>%s</i>\n", hhmm(l.Time), html.EscapeString(l.Name), formatLessonMode(l.Tag, location, l.URL))

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

func groupDayKeyboard(date time.Time, groupID int) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "◀️ Вчора", CallbackData: fmt.Sprintf("%sprev:%s:%d", groupNavCallbackPrefix, date.Format("2006-01-02"), groupID)},
				{Text: "📅 Сьогодні", CallbackData: fmt.Sprintf("%stoday:%s:%d", groupNavCallbackPrefix, date.Format("2006-01-02"), groupID)},
				{Text: "Завтра ▶️", CallbackData: fmt.Sprintf("%snext:%s:%d", groupNavCallbackPrefix, date.Format("2006-01-02"), groupID)},
			},
		},
	}
}

// formatGroupWeek renders a group's schedule for an academic week.
func formatGroupWeek(w weekInfo, offset int, groupName string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "👥 <b>Розклад групи %s</b>\n", html.EscapeString(groupName))
	fmt.Fprintf(&b, "🗓 <b>%s</b> тиждень — %s\n", weekOrdinal(w.WeekNumber), weekOffsetLabel(offset))

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
			location := l.LocationTitle
			if location == "" {
				location = l.LocationRaw
			}
			fmt.Fprintf(&b, "<code>%s</code> %s <i>%s</i>\n", hhmm(l.Time), html.EscapeString(l.Name), formatLessonMode(l.Tag, location, l.URL))
		}
	}

	return b.String()
}

func groupWeekKeyboard(offset int, groupID int) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				groupWeekNavButton(-1, offset, groupID, "◀️ Минулий"),
				groupWeekNavButton(0, offset, groupID, "Поточний"),
				groupWeekNavButton(1, offset, groupID, "Наступний ▶️"),
			},
			{
				{Text: "📅 Розклад на сьогодні", CallbackData: fmt.Sprintf("%stoday:%d", groupWeekCallbackPrefix, groupID)},
			},
		},
	}
}

func groupWeekNavButton(target, displayed, groupID int, label string) gotgbot.InlineKeyboardButton {
	if target == displayed {
		return gotgbot.InlineKeyboardButton{
			Text:         "✅ " + weekOffsetLabel(target),
			CallbackData: fmt.Sprintf("%snoop", groupWeekCallbackPrefix),
		}
	}
	return gotgbot.InlineKeyboardButton{
		Text:         label,
		CallbackData: fmt.Sprintf("%sgoto:%d:%d", groupWeekCallbackPrefix, target, groupID),
	}
}

func formatGroupListMenu(groups []model.BotGroup, notice string) string {
	var b strings.Builder
	b.WriteString("👥 <b>Керування групами</b>\n\n")

	if notice != "" {
		b.WriteString(notice)
		b.WriteString("\n\n")
	}

	if len(groups) == 0 {
		b.WriteString("У тебе ще немає налаштованих груп для використання у чатах.\n" +
			"Натисни кнопку <b>«➕ Нова група»</b> нижче, щоб додати академічну групу.")
		return b.String()
	}

	b.WriteString("Обери групу для перегляду та налаштування, або додай нову:")
	return b.String()
}

func groupListKeyboard(groups []model.BotGroup) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	for _, g := range groups {
		title := g.AcademicGroupName
		if g.Faculty != "" {
			title += fmt.Sprintf(" (%s)", g.Faculty)
		}
		if g.TelegramChatTitle != "" {
			title += fmt.Sprintf(" — 💬 %s", g.TelegramChatTitle)
		}
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "👥 " + title, CallbackData: groupCallbackPrefix + "view:" + g.ID.String()},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "➕ Нова група", CallbackData: groupCallbackPrefix + "new"},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatGroupConfig(g model.BotGroup, notice string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "⚙️ <b>Налаштування групи: %s</b>\n\n", html.EscapeString(g.AcademicGroupName))

	if notice != "" {
		b.WriteString(notice)
		b.WriteString("\n\n")
	}

	faculty := g.Faculty
	if faculty == "" {
		faculty = "не вказано"
	}
	fmt.Fprintf(&b, "• <b>Академічна група:</b> %s (%s)\n", html.EscapeString(g.AcademicGroupName), html.EscapeString(faculty))
	fmt.Fprintf(&b, "• <b>ID в Campus:</b> <code>%d</code>\n", g.AcademicGroupID)

	if g.TelegramChatID != nil {
		chatName := g.TelegramChatTitle
		if chatName == "" {
			chatName = fmt.Sprintf("ID: %d", *g.TelegramChatID)
		}
		fmt.Fprintf(&b, "• <b>Прив'язаний чат:</b> 💬 %s\n", html.EscapeString(chatName))
	} else {
		b.WriteString("• <b>Прив'язаний чат:</b> <i>Не прив'язано</i>\n")
	}

	if g.NotificationsEnabled {
		b.WriteString("• <b>Сповіщення про пари:</b> 🔔 Увімкнено\n")
	} else {
		b.WriteString("• <b>Сповіщення про пари:</b> 🔕 Вимкнено\n")
	}

	return b.String()
}

func groupConfigKeyboard(g model.BotGroup) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	idStr := g.ID.String()

	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "🔗 Посилання на заняття", CallbackData: groupCallbackPrefix + "urls:" + idStr},
	})
	toggleNotifyText := "🔔 Сповіщення: Увімкнено"
	if !g.NotificationsEnabled {
		toggleNotifyText = "🔕 Сповіщення: Вимкнено"
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: toggleNotifyText, CallbackData: groupCallbackPrefix + "toggle_notify:" + idStr},
	})
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "✏️ Змінити академічну групу", CallbackData: groupCallbackPrefix + "edit_acad:" + idStr},
	})
	if g.TelegramChatID != nil {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "❌ Відв'язати від чату", CallbackData: groupCallbackPrefix + "unbind:" + idStr},
		})
	} else {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "🔗 Як прив'язати чат", CallbackData: groupCallbackPrefix + "bind_help:" + idStr},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "🗑 Видалити групу", CallbackData: groupCallbackPrefix + "del_ask:" + idStr},
	})
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад до списку", CallbackData: groupCallbackPrefix + "list"},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatGroupDeleteConfirm(g model.BotGroup) string {
	return fmt.Sprintf("⚠️ <b>Видалення групи</b>\n\nТи впевнений, що хочеш видалити групу <b>%s</b>?\nРозклад цієї групи більше не буде доступний у прив'язаному чаті.", html.EscapeString(g.AcademicGroupName))
}

func groupDeleteConfirmKeyboard(groupID string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: "🗑 Так, видалити", CallbackData: groupCallbackPrefix + "del_confirm:" + groupID},
				{Text: "◀️ Скасувати", CallbackData: groupCallbackPrefix + "view:" + groupID},
			},
		},
	}
}

func formatGroupCreationPrompt(errorMsg string) string {
	var b strings.Builder
	b.WriteString("➕ <b>Створення нової групи</b>\n\n")

	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}

	b.WriteString("Надішли назву академічної групи КПІ (наприклад, <code>ІП-21</code>):")
	return b.String()
}

func formatGroupEditAcadPrompt(currentName, errorMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✏️ <b>Зміна академічної групи для %s</b>\n\n", html.EscapeString(currentName))

	if errorMsg != "" {
		fmt.Fprintf(&b, "❌ <b>%s</b>\n\n", html.EscapeString(errorMsg))
	}

	b.WriteString("Надішли нову назву академічної групи КПІ (наприклад, <code>ІП-22</code>):")
	return b.String()
}

func groupPromptBackKeyboard(callback string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: "◀️ Назад", CallbackData: callback}},
		},
	}
}

func formatGroupBindPicker(chatTitle string, groups []model.BotGroup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔗 <b>Прив'язка чату «%s»</b>\n\n", html.EscapeString(chatTitle))

	if len(groups) == 0 {
		b.WriteString("У тебе ще немає створених груп. Натисни «➕ Нова група», щоб ввести академічну групу КПІ та прив'язати цей чат.")
	} else {
		b.WriteString("Обери збережену групу зі списку нижче або створи нову:")
	}
	return b.String()
}

func groupBindPickerKeyboard(chatID int64, groups []model.BotGroup) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	for _, g := range groups {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "👥 " + g.AcademicGroupName, CallbackData: fmt.Sprintf("%sbind_to:%s:%d", groupCallbackPrefix, g.ID.String(), chatID)},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "➕ Нова група", CallbackData: fmt.Sprintf("%sbind_new:%d", groupCallbackPrefix, chatID)},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatUserName(u *gotgbot.User) string {
	if u == nil {
		return "користувача"
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		if u.Username != "" {
			return "@" + u.Username
		}
		return fmt.Sprintf("ID:%d", u.Id)
	}
	if u.Username != "" {
		return fmt.Sprintf("%s (@%s)", name, u.Username)
	}
	return name
}

func formatGroupLessonsMenu(groupName string, lessons []model.UniqueLesson, notice string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔗 <b>Посилання на онлайн-заняття групи %s</b>\n\n", html.EscapeString(groupName))

	if notice != "" {
		b.WriteString(notice)
		b.WriteString("\n\n")
	}

	if len(lessons) == 0 {
		b.WriteString("📭 У розкладі групи не знайдено занять для додавання посилань.\n")
		return b.String()
	}

	for _, l := range lessons {
		mode := formatLessonMode(l.Tag, "Онлайн", l.URL)
		fmt.Fprintf(&b, "• %s <i>%s</i>\n", html.EscapeString(l.Subject), mode)
	}

	b.WriteString("\nОбери заняття зі списку нижче, щоб додати або змінити посилання:")
	return b.String()
}

func groupURLsKeyboard(groupID string, lessons []model.UniqueLesson) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	for _, l := range lessons {
		prefix := "➕ "
		if l.URL != "" {
			prefix = "🔗 "
		}
		title := l.Subject
		runes := []rune(title)
		if len(runes) > 30 {
			title = string(runes[:27]) + "..."
		}
		btnText := fmt.Sprintf("%s%s (%s)", prefix, title, tagAbbr(l.Tag))
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: btnText, CallbackData: fmt.Sprintf("%surledit:%s:%s", groupCallbackPrefix, groupID, lessonHash(l.SubjectNorm, l.Tag))},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад до налаштувань", CallbackData: groupCallbackPrefix + "view:" + groupID},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func groupURLPromptKeyboard(groupID string, hasExistingURL bool, hash string) gotgbot.InlineKeyboardMarkup {
	var rows [][]gotgbot.InlineKeyboardButton
	if hasExistingURL {
		rows = append(rows, []gotgbot.InlineKeyboardButton{
			{Text: "🗑 Видалити посилання", CallbackData: fmt.Sprintf("%surldel:%s:%s", groupCallbackPrefix, groupID, hash)},
		})
	}
	rows = append(rows, []gotgbot.InlineKeyboardButton{
		{Text: "◀️ Назад до занять", CallbackData: groupCallbackPrefix + "urls:" + groupID},
	})
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func formatUserSettings(notificationsEnabled bool) string {
	var b strings.Builder
	b.WriteString("⚙️ <b>Налаштування сповіщень</b>\n\n")
	if notificationsEnabled {
		b.WriteString("• <b>Сповіщення про пари:</b> 🔔 Увімкнено\n\n<i>Бот надсилатиме сповіщення за 10 хвилин до початку та на початку кожної пари.</i>")
	} else {
		b.WriteString("• <b>Сповіщення про пари:</b> 🔕 Вимкнено\n\n<i>Сповіщення про пари вимкнено. Ти не отримуватимеш нагадувань.</i>")
	}
	return b.String()
}

func userSettingsKeyboard(notificationsEnabled bool) gotgbot.InlineKeyboardMarkup {
	toggleText := "🔔 Сповіщення: Увімкнено"
	if !notificationsEnabled {
		toggleText = "🔕 Сповіщення: Вимкнено"
	}
	return gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{{Text: toggleText, CallbackData: "settings:toggle_notify"}},
			{{Text: "◀️ Назад", CallbackData: menuCallbackData("back")}},
		},
	}
}

