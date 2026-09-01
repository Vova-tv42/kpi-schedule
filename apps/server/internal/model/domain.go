package model

import (
	"time"

	"github.com/google/uuid"
)

type SessionStatus string

const (
	SessionActive  SessionStatus = "active"
	SessionExpired SessionStatus = "expired"
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
	ID         uuid.UUID
	TelegramID int64
	GroupID    *int
	GroupName  *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UserSession holds the encrypted my.kpi.ua cookies for a user.
type UserSession struct {
	UserID         uuid.UUID
	Ciphertext     []byte
	UserAgent      string
	Status         SessionStatus
	SyncedAt       time.Time
	LastCheckedAt  time.Time
	LastError      *string
}

// Cookies is the decrypted cookie set posted by the client.
type Cookies struct {
	PHPSESSID string `json:"PHPSESSID"`
	Identity  string `json:"_identity"`
	Language  string `json:"language,omitempty"`
}

// ScheduleState tracks when a user's lesson set was last refreshed.
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
}
