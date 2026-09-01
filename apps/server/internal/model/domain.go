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

// Lesson is one merged, persisted class occurrence pattern for a user.
type Lesson struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Week          int // 1 or 2
	Day           int // 1 (Monday) .. 6 (Saturday)
	Slot          int // 1..7
	StartTime     string // "HH:MM:SS"
	Subject       string
	SubjectNorm   string
	Tag           string // "lec", "prac", "lab", or "" if unknown
	Type          string // display label, e.g. "Лекція"
	Lecturer      *Lecturer
	Location      *Location
	Dates         []string // ISO YYYY-MM-DD; empty = every cycle of this week
	Enriched      bool
}
