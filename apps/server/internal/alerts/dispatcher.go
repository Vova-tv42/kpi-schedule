package alerts

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/engine"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

// TelegramSender is an interface for sending and deleting Telegram messages (satisfied by *gotgbot.Bot).
type TelegramSender interface {
	SendMessage(chatID int64, text string, opts *gotgbot.SendMessageOpts) (*gotgbot.Message, error)
	DeleteMessage(chatID int64, messageID int64, opts *gotgbot.DeleteMessageOpts) (bool, error)
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

	// Snapshot the alert messages left by earlier runs before sending anything,
	// so alerts sent within this run never delete one another.
	previous, err := d.db.ListLiveAlertMessages(ctx)
	if err != nil {
		slog.Error("listing previous lesson alert messages", "error", err)
	}
	// Recipients ("user|<id>" / "group|<id>") that received a new alert in this run.
	notified := make(map[string]bool)
	defer d.deletePreviousAlerts(ctx, now, previous, notified)

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
				alertType, minsBefore, matches := matchAlertWindow(nowKyiv, l.StartTime)
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

				location := l.LocationRaw
				if l.Location != nil && l.Location.Title != "" {
					location = l.Location.Title
				}

				msg := formatAlertMessage(alertType, minsBefore, l.StartTime, l.Subject, l.Tag, location, url)
				opts := &gotgbot.SendMessageOpts{
					ParseMode:          "HTML",
					LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
				}
				if kb := buildAlertKeyboard(l.Subject, url); kb != nil {
					opts.ReplyMarkup = *kb
				}

				var messageID int64
				sentMsg, sendErr := d.sender.SendMessage(u.TelegramID, msg, opts)
				if sendErr != nil {
					slog.Warn("sending personal lesson alert", "error", sendErr, "telegram_id", u.TelegramID)
				} else {
					result.PersonalAlertsSent++
					notified[recipientKey("user", u.ID.String())] = true
					if sentMsg != nil {
						messageID = sentMsg.MessageId
					}
				}

				_ = d.db.RecordAlertSent(ctx, "user", u.ID.String(), dateStr, l.StartTime, alertType, u.TelegramID, messageID)
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

					alertType, minsBefore, matches := matchAlertWindow(nowKyiv, p.Time)
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

					location := ""
					if p.Location != nil {
						location = p.Location.Title
					}

					msg := formatAlertMessage(alertType, minsBefore, p.Time, p.Name, p.Tag, location, url)
					opts := &gotgbot.SendMessageOpts{
						ParseMode:          "HTML",
						LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
					}
					if kb := buildAlertKeyboard(p.Name, url); kb != nil {
						opts.ReplyMarkup = *kb
					}

					var messageID int64
					sentMsg, sendErr := d.sender.SendMessage(*g.TelegramChatID, msg, opts)
					if sendErr != nil {
						slog.Warn("sending group lesson alert", "error", sendErr, "chat_id", *g.TelegramChatID)
					} else {
						result.GroupAlertsSent++
						notified[recipientKey("group", g.ID.String())] = true
						if sentMsg != nil {
							messageID = sentMsg.MessageId
						}
					}

					_ = d.db.RecordAlertSent(ctx, "group", g.ID.String(), dateStr, p.Time, alertType, *g.TelegramChatID, messageID)
				}
			}
		}
	}

	return result, nil
}

func recipientKey(recipientType, recipientID string) string {
	return recipientType + "|" + recipientID
}

// deletePreviousAlerts removes lesson alert messages sent by earlier runs:
//   - every alert of a recipient that just received a new alert (the new one
//     replaces them),
//   - every "lesson started" alert, even when this run sent nothing — it is
//     only relevant until the next cron tick, and
//   - every pre-lesson alert whose lesson's "lesson started" window is over —
//     its follow-up was never sent (cron missed the window, notifications were
//     turned off, the lesson left the schedule), so nothing would replace it.
//
// Otherwise a pre-lesson alert stays in the chat until its follow-up arrives.
//
// The stored message reference is cleared whatever Telegram answers: the
// message may already be gone (deleted by a user, chat left, >48h old), and a
// failed delete is not worth retrying on every tick.
func (d *Dispatcher) deletePreviousAlerts(ctx context.Context, now time.Time, previous []storage.AlertMessage, notified map[string]bool) {
	for _, m := range previous {
		if m.AlertType != model.AlertAtStart &&
			!notified[recipientKey(m.RecipientType, m.RecipientID)] &&
			!d.startWindowOver(m, now) {
			continue
		}
		if _, err := d.sender.DeleteMessage(m.ChatID, m.MessageID, nil); err != nil {
			slog.Warn("deleting previous lesson alert", "error", err, "chat_id", m.ChatID, "message_id", m.MessageID)
		}
		if err := d.db.ClearAlertMessage(ctx, m.ID); err != nil {
			slog.Warn("clearing previous lesson alert message", "error", err, "alert_id", m.ID)
		}
	}
}

// startWindowOver reports whether the "lesson started" window of the alert's
// lesson has closed at `now`. A malformed date or time counts as over, so a
// bad row cannot keep a message in the chat forever.
func (d *Dispatcher) startWindowOver(m storage.AlertMessage, now time.Time) bool {
	day, err := time.ParseInLocation("2006-01-02", m.LessonDate, d.loc)
	if err != nil {
		return true
	}
	h, min, err := parseTimeHHMM(m.LessonTime)
	if err != nil {
		return true
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), h, min, 0, 0, d.loc)
	return now.After(start.Add(atStartWindowAfter))
}

// atStartWindowAfter is how long after a lesson starts its "lesson started"
// alert may still be sent.
const atStartWindowAfter = 5 * time.Minute

// matchAlertWindow checks if nowKyiv falls into the 15-5m before window or 5m before to 5m after start window.
func matchAlertWindow(nowKyiv time.Time, startTime string) (model.AlertType, int, bool) {
	h, m, err := parseTimeHHMM(startTime)
	if err != nil {
		return "", 0, false
	}

	lessonTime := time.Date(nowKyiv.Year(), nowKyiv.Month(), nowKyiv.Day(), h, m, 0, 0, nowKyiv.Location())
	diff := lessonTime.Sub(nowKyiv).Minutes()

	// 15 to 5 minutes before lesson start: AlertBefore10m
	if diff > 5.0 && diff <= 15.0 {
		mins := int(math.Round(diff))
		if mins <= 0 {
			mins = 1
		}
		return model.AlertBefore10m, mins, true
	}

	// 5 minutes before to 5 minutes after start: AlertAtStart
	if diff >= -atStartWindowAfter.Minutes() && diff <= 5.0 {
		return model.AlertAtStart, 0, true
	}

	return "", 0, false
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

func tagAbbr(tag string) string {
	switch strings.ToLower(tag) {
	case "lec", "лек":
		return "Лек."
	case "prac", "прак":
		return "Практ."
	case "lab", "лаб":
		return "Лаб."
	default:
		if tag != "" {
			return tag
		}
		return "Пара"
	}
}

func formatLessonMode(tag, location, rawURL string) string {
	kind := model.LocationKind(location)
	text := fmt.Sprintf("[%s, %s]", tagAbbr(tag), kind)
	if rawURL != "" {
		return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(rawURL), html.EscapeString(text))
	}
	return html.EscapeString(text)
}

func hhmm(t string) string {
	if len(t) >= 5 {
		return t[:5]
	}
	return t
}

func minutesPluralUA(n int) string {
	n100 := n % 100
	if n100 >= 11 && n100 <= 19 {
		return "хвилин"
	}
	switch n % 10 {
	case 1:
		return "хвилину"
	case 2, 3, 4:
		return "хвилини"
	default:
		return "хвилин"
	}
}

func platformLabel(rawURL string) string {
	lower := strings.ToLower(rawURL)
	if strings.Contains(lower, "zoom") {
		return "Zoom"
	}
	if strings.Contains(lower, "meet.google") || strings.Contains(lower, "meet") {
		return "Meet"
	}
	if strings.Contains(lower, "teams.microsoft") || strings.Contains(lower, "teams") {
		return "Teams"
	}
	if strings.Contains(lower, "webex") {
		return "Webex"
	}
	if strings.Contains(lower, "youtube") || strings.Contains(lower, "youtu.be") {
		return "YouTube"
	}
	return "Онлайн"
}

func formatAlertMessage(alertType model.AlertType, minutesBefore int, startTime, subject, tag, location, rawURL string) string {
	var header string
	if alertType == model.AlertBefore10m {
		header = fmt.Sprintf("<blockquote>🔔 Пара почнеться через %d %s</blockquote>", minutesBefore, minutesPluralUA(minutesBefore))
	} else {
		header = "<blockquote>🔔 Почалась пара</blockquote>"
	}

	timeStr := hhmm(startTime)
	mode := formatLessonMode(tag, location, rawURL)

	return fmt.Sprintf("%s\n\n<code>%s</code>  %s <i>%s</i>", header, html.EscapeString(timeStr), html.EscapeString(subject), mode)
}

func buildAlertKeyboard(subject, rawURL string) *gotgbot.InlineKeyboardMarkup {
	if rawURL == "" {
		return nil
	}
	btnText := fmt.Sprintf("🤙 %s (%s)", subject, platformLabel(rawURL))
	return &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
			{
				{Text: btnText, Url: rawURL},
			},
		},
	}
}
