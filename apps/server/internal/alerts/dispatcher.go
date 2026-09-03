package alerts

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/engine"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// TelegramSender is an interface for sending Telegram messages (satisfied by *gotgbot.Bot).
type TelegramSender interface {
	SendMessage(chatID int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error)
}

type DispatchResult struct {
	PersonalAlertsSent int `json:"personal_alerts_sent"`
	GroupAlertsSent    int `json:"group_alerts_sent"`
}

type Dispatcher struct {
	db     *storage.DB
	campus *campus.Client
	sender TelegramSender
	loc    *time.Location
}

func NewDispatcher(db *storage.DB, campus *campus.Client, sender TelegramSender) *Dispatcher {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		loc = time.FixedZone("EEST", 3*3600)
	}
	return &Dispatcher{
		db:     db,
		campus: campus,
		sender: sender,
		loc:    loc,
	}
}

// Dispatch evaluates pending lesson alerts for personal users and group chats at time `now`.
func (d *Dispatcher) Dispatch(ctx context.Context, now time.Time) (DispatchResult, error) {
	var result DispatchResult
	if d.sender == nil {
		return result, nil
	}

	_ = d.db.CleanOldAlerts(ctx, 7*24*time.Hour)

	nowKyiv := now.In(d.loc)
	todayUTC := time.Date(nowKyiv.Year(), nowKyiv.Month(), nowKyiv.Day(), 0, 0, 0, 0, time.UTC)
	dateStr := todayUTC.Format("2006-01-02")
	isoDay := engine.ISODay(todayUTC)

	// 1. Dispatch Personal User Alerts
	users, err := d.db.GetUsersWithNotifications(ctx)
	if err != nil {
		slog.Error("getting users with notifications", "error", err)
	} else {
		for _, u := range users {
			lessons, err := d.db.GetLessonsByDateRange(ctx, u.ID, todayUTC, todayUTC)
			if err != nil || len(lessons) == 0 {
				continue
			}

			urls, _ := d.db.GetLessonURLs(ctx, u.ID)

			for _, l := range lessons {
				alertType, matches := matchAlertWindow(nowKyiv, l.StartTime)
				if !matches {
					continue
				}

				sent, err := d.db.HasAlertBeenSent(ctx, "user", u.ID.String(), dateStr, l.StartTime, alertType)
				if err != nil || sent {
					continue
				}

				url := ""
				if urls != nil {
					url = urls[l.SubjectNorm+"|"+l.Tag]
				}

				msg := formatPersonalAlert(alertType, l, url)
				_, sendErr := d.sender.SendMessage(u.TelegramID, msg, &gotgbot.SendMessageOpts{
					ParseMode:          "HTML",
					LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
				})
				if sendErr != nil {
					slog.Warn("sending personal lesson alert", "error", sendErr, "telegram_id", u.TelegramID)
				} else {
					result.PersonalAlertsSent++
				}

				_ = d.db.RecordAlertSent(ctx, "user", u.ID.String(), dateStr, l.StartTime, alertType)
			}
		}
	}

	// 2. Dispatch Group Chat Alerts
	groups, err := d.db.GetActiveBotGroupsWithNotifications(ctx)
	if err != nil {
		slog.Error("getting active bot groups with notifications", "error", err)
	} else if len(groups) > 0 && d.campus != nil {
		currentTime, cErr := d.campus.CurrentTime(ctx)
		if cErr != nil {
			slog.Error("resolving current campus week for alerts", "error", cErr)
		} else {
			week := engine.WeekAt(time.Now(), currentTime.CurrentWeek, todayUTC)
			dayShort := dayShortUA[isoDay]

			for _, g := range groups {
				if g.TelegramChatID == nil {
					continue
				}

				sched, sErr := d.campus.GroupSchedule(ctx, g.AcademicGroupID)
				if sErr != nil {
					slog.Warn("fetching group schedule for alerts", "error", sErr, "group", g.AcademicGroupName)
					continue
				}

				var dayScheds []campus.DaySchedule
				if week == 1 {
					dayScheds = sched.ScheduleFirstWeek
				} else {
					dayScheds = sched.ScheduleSecondWeek
				}

				var todayPairs []campus.Pair
				for _, ds := range dayScheds {
					if ds.Day == dayShort {
						todayPairs = ds.Pairs
						break
					}
				}

				if len(todayPairs) == 0 {
					continue
				}

				urls, _ := d.db.GetGroupLessonURLs(ctx, g.ID)

				for _, p := range todayPairs {
					if len(p.Dates) > 0 && !containsDate(p.Dates, dateStr) {
						continue
					}

					alertType, matches := matchAlertWindow(nowKyiv, p.Time)
					if !matches {
						continue
					}

					sent, err := d.db.HasAlertBeenSent(ctx, "group", g.ID.String(), dateStr, p.Time, alertType)
					if err != nil || sent {
						continue
					}

					norm := engine.NormalizeSubject(p.Name)
					tag := engine.NormalizeTag(p.Tag)
					url := ""
					if urls != nil {
						url = urls[norm+"|"+tag]
					}

					msg := formatGroupAlert(alertType, g.AcademicGroupName, p, url)
					_, sendErr := d.sender.SendMessage(*g.TelegramChatID, msg, &gotgbot.SendMessageOpts{
						ParseMode:          "HTML",
						LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
					})
					if sendErr != nil {
						slog.Warn("sending group lesson alert", "error", sendErr, "chat_id", *g.TelegramChatID)
					} else {
						result.GroupAlertsSent++
					}

					_ = d.db.RecordAlertSent(ctx, "group", g.ID.String(), dateStr, p.Time, alertType)
				}
			}
		}
	}

	return result, nil
}

func matchAlertWindow(nowKyiv time.Time, startTime string) (model.AlertType, bool) {
	h, m, err := parseTimeHHMM(startTime)
	if err != nil {
		return "", false
	}

	lessonTime := time.Date(nowKyiv.Year(), nowKyiv.Month(), nowKyiv.Day(), h, m, 0, 0, nowKyiv.Location())
	diff := lessonTime.Sub(nowKyiv).Minutes()

	// 10 minutes before (window [8.0 .. 12.0] minutes)
	if diff >= 8.0 && diff <= 12.0 {
		return model.AlertBefore10m, true
	}
	// At start (window [-2.0 .. 2.0] minutes)
	if diff >= -2.0 && diff <= 2.0 {
		return model.AlertAtStart, true
	}

	return "", false
}

func parseTimeHHMM(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid time %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return h, m, nil
}

func containsDate(dates []string, target string) bool {
	for _, d := range dates {
		if d == target {
			return true
		}
	}
	return false
}

var dayShortUA = map[int]string{
	1: "Пн", 2: "Вв", 3: "Ср", 4: "Чт", 5: "Пт", 6: "Сб", 7: "Нд",
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

func hhmm(t string) string {
	if len(t) >= 5 {
		return t[:5]
	}
	return t
}

func formatPersonalAlert(alertType model.AlertType, lesson model.Lesson, url string) string {
	timeDisplay := hhmm(lesson.StartTime)
	var header string
	if alertType == model.AlertBefore10m {
		header = fmt.Sprintf("⏰ <b>Через 10 хвилин пара! (%s)</b>", timeDisplay)
	} else {
		header = fmt.Sprintf("🚀 <b>Пара розпочалася! (%s)</b>", timeDisplay)
	}

	kind := model.LocationKind(lesson.LocationRaw)
	modeText := fmt.Sprintf("[%s, %s]", tagAbbr(lesson.Tag), kind)

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "📖 <b>%s</b> <i>%s</i>\n", html.EscapeString(lesson.Subject), html.EscapeString(modeText))

	teacher := lesson.TeacherRaw
	if lesson.Lecturer != nil && lesson.Lecturer.Name != "" {
		teacher = lesson.Lecturer.Name
	}
	if teacher != "" {
		fmt.Fprintf(&sb, "👨‍🏫 Викладач: %s\n", html.EscapeString(teacher))
	}

	location := lesson.LocationRaw
	if lesson.Location != nil && lesson.Location.Title != "" {
		location = lesson.Location.Title
	}
	if location != "" && !model.IsOnline(location) {
		fmt.Fprintf(&sb, "📍 Аудиторія: %s\n", html.EscapeString(location))
	}

	if url != "" {
		fmt.Fprintf(&sb, "\n🔗 <a href=\"%s\">Приєднатися до заняття</a>\n", html.EscapeString(url))
	}

	return sb.String()
}

func formatGroupAlert(alertType model.AlertType, groupName string, pair campus.Pair, url string) string {
	timeDisplay := hhmm(pair.Time)
	var header string
	if alertType == model.AlertBefore10m {
		header = fmt.Sprintf("⏰ <b>Через 10 хвилин пара! (%s)</b>", timeDisplay)
	} else {
		header = fmt.Sprintf("🚀 <b>Пара розпочалася! (%s)</b>", timeDisplay)
	}

	locTitle := ""
	if pair.Location != nil {
		locTitle = pair.Location.Title
	}
	kind := model.LocationKind(locTitle)
	modeText := fmt.Sprintf("[%s, %s]", tagAbbr(pair.Tag), kind)

	var sb strings.Builder
	fmt.Fprintf(&sb, "👥 <b>Група %s</b>\n", html.EscapeString(groupName))
	sb.WriteString(header)
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "📖 <b>%s</b> <i>%s</i>\n", html.EscapeString(pair.Name), html.EscapeString(modeText))

	teacher := ""
	if pair.Lecturer != nil && pair.Lecturer.Name != "" {
		teacher = pair.Lecturer.Name
	}
	if teacher != "" {
		fmt.Fprintf(&sb, "👨‍🏫 Викладач: %s\n", html.EscapeString(teacher))
	}

	if locTitle != "" && !model.IsOnline(locTitle) {
		fmt.Fprintf(&sb, "📍 Аудиторія: %s\n", html.EscapeString(locTitle))
	}

	if url != "" {
		fmt.Fprintf(&sb, "\n🔗 <a href=\"%s\">Приєднатися до заняття</a>\n", html.EscapeString(url))
	}

	return sb.String()
}
