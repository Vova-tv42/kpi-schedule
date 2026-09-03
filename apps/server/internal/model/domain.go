package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type EnrichmentStatus string

const (
	EnrichmentFull     EnrichmentStatus = "full"
	EnrichmentDegraded EnrichmentStatus = "degraded"
	EnrichmentNone     EnrichmentStatus = "none"
)

// User is a linked student, keyed by their Telegram ID.
// telegram_id is kept as the external key even without a bot wired up yet,
// so manual testing and the future bot need no schema change.
type User struct {
	ID                   uuid.UUID
	TelegramID           int64
	GroupID              *int
	GroupName            *string
	NotificationsEnabled bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ScheduleState tracks when a user's lesson set was last synced. There is no
// concept of a "session" any more — the server never sees my.kpi.ua
// credentials (see docs/architecture/data-storage.md §1); a stale
// RefreshedAt just means the browser extension hasn't pushed an update
// recently, not that anything on the server has expired.
type ScheduleState struct {
	UserID           uuid.UUID
	RefreshedAt      time.Time
	LessonCount      int
	EnrichmentStatus EnrichmentStatus
	LastError        *string
}

// Lecturer mirrors the Campus API lecturer object.
type Lecturer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Location mirrors the Campus API location object.
type Location struct {
	Title string `json:"title"`
	URI   string `json:"uri"`
}

// Lesson is one concrete, dated class occurrence for a user, as returned
// directly by my.kpi.ua's personal calendar feed (already exact-dated and
// already filtered to only this student's actual enrollment — no elective
// or subgroup ambiguity to resolve on our side). Week/Day/Slot are derived
// and stored alongside Date purely for display grouping and for matching
// against the Campus API's week-pattern schedule; Date is authoritative.
type Lesson struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Date        time.Time // calendar date (UTC midnight), authoritative
	Week        int       // 1 or 2, derived from Date at refresh time
	Day         int       // 1 (Monday) .. 7 (Sunday), derived from Date
	Slot        int       // 1..7 if resolved against Campus lesson-slot times, else 0
	StartTime   string    // "HH:MM:SS"
	EndTime     string    // "HH:MM:SS"
	Subject     string
	SubjectNorm string
	Tag         string // normalized: "lec", "prac", "lab", or "" if unknown
	TeacherRaw  string // plain-text teacher name(s) as shown by my.kpi.ua
	LocationRaw string // plain-text location/link as shown by my.kpi.ua (e.g. "lec., Онлайн Zoom")
	Lecturer    *Lecturer // Campus API enrichment: hashed ID + full name
	Location    *Location // Campus API enrichment: room title + map URI
	Enriched    bool
	// IsRecurring: true if this lesson happens every week of its Week
	// parity, false if it only occurs on a handful of specific calendar
	// dates (per the matched Campus group lesson's dates[]; defaults true
	// when unenriched). Used to decide whether a lesson belongs in the
	// generic /schedule/week template view — see
	// docs/architecture/merging-engine.md §6.
	IsRecurring bool
}

// ParsedLesson is one dated lesson occurrence in the shape a client submits
// for merging — currently produced only by engine tests, but this is the
// intended wire format for the future browser extension's schedule-sync
// payload (see docs/architecture/data-storage.md §1 and
// docs/extension/browser-extension-design.md): the extension authenticates
// to my.kpi.ua using the student's own browser session and does the
// fetch+parse client-side, so the server only ever receives this — never raw
// HTML, raw JSON, or session cookies.
type ParsedLesson struct {
	Date        time.Time // calendar date (UTC midnight)
	StartTime   string    // "HH:MM:SS"
	EndTime     string    // "HH:MM:SS"
	Subject     string
	Tag         string // normalized: "lec", "prac", "lab", or "" if unrecognized
	TeacherRaw  string
	LocationRaw string
}

// UniqueLesson represents a distinct course session (grouped by subject and tag)
// for managing online links.
type UniqueLesson struct {
	Subject     string
	SubjectNorm string
	Tag         string
	IsOnline    bool
	URL         string
}

// URLPrompt tracks an in-flight prompt asking the user to send a URL for a lesson.
type URLPrompt struct {
	UserID          uuid.UUID
	TelegramID      int64
	PromptMessageID int64
	SubjectNorm     string
	Tag             string
	SubjectName     string
	UpdatedAt       time.Time
}

// LocationKind returns "Онлайн" or "Оффлайн" based on location text.
func LocationKind(location string) string {
	if location == "" {
		return "Онлайн"
	}
	lower := strings.ToLower(location)
	if strings.Contains(lower, "онлайн") || strings.Contains(lower, "online") ||
		strings.Contains(lower, "zoom") || strings.Contains(lower, "meet") ||
		strings.Contains(lower, "teams") || strings.Contains(lower, "дистанц") ||
		strings.Contains(lower, "webex") || strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") {
		return "Онлайн"
	}
	return "Оффлайн"
}

// IsOnline reports whether a location represents an online lesson.
func IsOnline(location string) bool {
	return LocationKind(location) == "Онлайн"
}

// BotGroup represents a configured group for Telegram group chats.
type BotGroup struct {
	ID                   uuid.UUID
	CreatorTelegramID    int64
	AcademicGroupID      int
	AcademicGroupName    string
	Faculty              string
	TelegramChatID       *int64
	TelegramChatTitle    string
	NotificationsEnabled bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AlertType identifies whether a notification is sent 10 minutes prior or at the start.
type AlertType string

const (
	AlertBefore10m AlertType = "before_10m"
	AlertAtStart   AlertType = "at_start"
)

// GroupPrompt tracks an in-flight prompt for creating or configuring a group or group lesson URL.
type GroupPrompt struct {
	TelegramID      int64
	PromptMessageID int64
	Action          string // "create", "edit_academic", "set_url"
	GroupID         *uuid.UUID
	BindChatID      *int64
	BindChatTitle   string
	SubjectNorm     string
	Tag             string
	SubjectName     string
	UpdatedAt       time.Time
}

// GroupLessonURL stores a custom conference URL for a lesson in a bot group's schedule.
type GroupLessonURL struct {
	ID          uuid.UUID
	GroupID     uuid.UUID
	SubjectNorm string
	Tag         string
	URL         string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
