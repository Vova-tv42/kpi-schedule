package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/crypto"
	"kpi-schedule-bot/server/internal/engine"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/mykpi"
	"kpi-schedule-bot/server/internal/storage"
)

// refreshInterval matches KPI's week-1/week-2 cycle: a stored schedule is
// considered fresh enough for two full cycles before an inline re-scrape is
// forced (docs/architecture/merging-engine.md; refresh policy agreed with the
// user — no platform cron in this iteration).
const refreshInterval = 14 * 24 * time.Hour

// fetchWindowPast/fetchWindowFuture bound the date range requested from
// my.kpi.ua's events endpoint on each refresh — wide enough to cover
// today/tomorrow/week views and a full semester's worth of upcoming lessons
// without pulling unbounded history. The endpoint requires an explicit
// start/end range and silently returns no events without one.
const (
	fetchWindowPast   = 14 * 24 * time.Hour
	fetchWindowFuture = 120 * 24 * time.Hour
)

var ErrInvalidCookies = errors.New("cookies rejected by my.kpi.ua")

type Service struct {
	db     *storage.DB
	campus *campus.Client
	mykpi  *mykpi.Client
	key    []byte
}

func NewService(db *storage.DB, campusClient *campus.Client, mykpiClient *mykpi.Client, key []byte) *Service {
	return &Service{db: db, campus: campusClient, mykpi: mykpiClient, key: key}
}

// LinkSession validates the cookies against my.kpi.ua, resolves the group
// name to a Campus groupId, stores the encrypted session, and performs the
// first schedule refresh.
func (s *Service) LinkSession(ctx context.Context, telegramID int64, groupName string, cookies model.Cookies, userAgent string) (model.User, model.EnrichmentStatus, error) {
	var groupIDPtr *int
	if groupName != "" {
		groupID, err := s.campus.ResolveGroupID(ctx, groupName)
		if err != nil {
			return model.User{}, "", fmt.Errorf("resolving group %q: %w", groupName, err)
		}
		groupIDPtr = &groupID
	}

	user, err := s.db.UpsertUser(ctx, telegramID, groupIDPtr, nonEmptyPtr(groupName))
	if err != nil {
		return model.User{}, "", err
	}

	// Probe before storing: an invalid cookie set must never be persisted as "active".
	if _, err := s.mykpi.FetchCalendarPage(ctx, cookies, userAgent); err != nil {
		if errors.Is(err, mykpi.ErrSessionExpired) {
			return model.User{}, "", ErrInvalidCookies
		}
		return model.User{}, "", fmt.Errorf("probing my.kpi.ua: %w", err)
	}

	if err := s.storeCookies(ctx, user.ID, cookies, userAgent); err != nil {
		return model.User{}, "", err
	}

	status, refreshErr := s.refreshSchedule(ctx, user)
	if refreshErr != nil && !errors.Is(refreshErr, mykpi.ErrSessionExpired) {
		return model.User{}, "", refreshErr
	}
	return user, status, nil
}

func (s *Service) storeCookies(ctx context.Context, userID uuid.UUID, cookies model.Cookies, userAgent string) error {
	plaintext, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("marshaling cookies: %w", err)
	}
	ciphertext, err := crypto.Seal(s.key, userID[:], plaintext)
	if err != nil {
		return fmt.Errorf("encrypting cookies: %w", err)
	}
	return s.db.UpsertSession(ctx, userID, ciphertext, userAgent)
}

// refreshSchedule performs one full scrape+merge+store cycle for a user.
// On session expiry it marks the session expired but returns the sentinel
// error so callers can decide whether to serve stale data.
func (s *Service) refreshSchedule(ctx context.Context, user model.User) (model.EnrichmentStatus, error) {
	session, err := s.db.GetSession(ctx, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return "", fmt.Errorf("%w: no session stored for user", ErrInvalidCookies)
		}
		return "", err
	}

	plaintext, err := crypto.Open(s.key, user.ID[:], session.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypting stored cookies: %w", err)
	}
	var cookies model.Cookies
	if err := json.Unmarshal(plaintext, &cookies); err != nil {
		return "", fmt.Errorf("unmarshaling stored cookies: %w", err)
	}

	now := time.Now().UTC()
	eventsJSON, err := s.mykpi.FetchStudentEventsRange(ctx, cookies, session.UserAgent, now.Add(-fetchWindowPast), now.Add(fetchWindowFuture))
	if err != nil {
		if errors.Is(err, mykpi.ErrSessionExpired) {
			msg := "my.kpi.ua rejected the stored session cookies"
			_ = s.db.MarkSessionStatus(ctx, user.ID, model.SessionExpired, &msg)
			return "", mykpi.ErrSessionExpired
		}
		return "", fmt.Errorf("fetching calendar events: %w", err)
	}
	_ = s.db.MarkSessionStatus(ctx, user.ID, model.SessionActive, nil)

	personal, err := mykpi.ParseEventsJSON(eventsJSON)
	if err != nil {
		return "", fmt.Errorf("parsing calendar events: %w", err)
	}

	enrichmentStatus := model.EnrichmentFull
	var groupSchedule campus.GroupScheduleResponse
	var slotTimes map[string]string
	var groupErrMsg *string
	referenceWeek := 1

	if user.GroupID != nil {
		var groupErr error
		groupSchedule, groupErr = s.campus.GroupSchedule(ctx, *user.GroupID)
		if groupErr == nil {
			slotTimes, groupErr = s.campus.LessonSlots(ctx)
		}
		if groupErr == nil {
			var currentTime campus.CurrentAcademicTime
			currentTime, groupErr = s.campus.CurrentTime(ctx)
			if groupErr == nil {
				referenceWeek = currentTime.CurrentWeek
			}
		}
		if groupErr != nil {
			enrichmentStatus = model.EnrichmentDegraded
			msg := groupErr.Error()
			groupErrMsg = &msg
		}
	} else {
		enrichmentStatus = model.EnrichmentDegraded
		msg := "no group linked; personal schedule stored without enrichment"
		groupErrMsg = &msg
	}

	merged := engine.Merge(personal, groupSchedule, slotTimes, now, referenceWeek)
	lessons := make([]model.Lesson, 0, len(merged))
	for _, m := range merged {
		lessons = append(lessons, model.Lesson{
			UserID:      user.ID,
			Date:        m.Date,
			Week:        m.Week,
			Day:         m.Day,
			Slot:        m.Slot,
			StartTime:   m.StartTime,
			EndTime:     m.EndTime,
			Subject:     m.Subject,
			SubjectNorm: m.SubjectNorm,
			Tag:         m.Tag,
			TeacherRaw:  m.TeacherRaw,
			LocationRaw: m.LocationRaw,
			Lecturer:    toModelLecturer(m.Lecturer),
			Location:    toModelLocation(m.Location),
			Enriched:    m.Enriched,
		})
	}

	if err := s.db.ReplaceLessons(ctx, user.ID, lessons, enrichmentStatus, groupErrMsg); err != nil {
		return "", fmt.Errorf("storing lessons: %w", err)
	}

	return enrichmentStatus, nil
}

func toModelLecturer(l *campus.Lecturer) *model.Lecturer {
	if l == nil {
		return nil
	}
	return &model.Lecturer{ID: l.ID, Name: l.Name}
}

func toModelLocation(l *campus.Location) *model.Location {
	if l == nil {
		return nil
	}
	return &model.Location{Title: l.Title, URI: l.URI}
}

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ensureFresh applies the no-cron refresh policy: refresh inline if there is
// no stored schedule yet, if it is older than refreshInterval, or if
// forceRefresh is set. A session-expiry during refresh is not an error here
// when a stored snapshot already exists — the caller serves it stale instead.
func (s *Service) ensureFresh(ctx context.Context, user model.User, forceRefresh bool) (stale bool, sessionStatus model.SessionStatus, enrichment model.EnrichmentStatus, err error) {
	state, stateErr := s.db.GetScheduleState(ctx, user.ID)
	hasStoredData := stateErr == nil

	needsRefresh := forceRefresh || !hasStoredData || time.Since(state.RefreshedAt) > refreshInterval
	if !needsRefresh {
		session, sessErr := s.db.GetSession(ctx, user.ID)
		if sessErr == nil {
			sessionStatus = session.Status
		}
		return false, sessionStatus, state.EnrichmentStatus, nil
	}

	newStatus, refreshErr := s.refreshSchedule(ctx, user)
	if refreshErr != nil {
		if errors.Is(refreshErr, mykpi.ErrSessionExpired) {
			if !hasStoredData {
				return false, model.SessionExpired, "", ErrInvalidCookies
			}
			// Serve the stale snapshot rather than failing the request.
			return true, model.SessionExpired, state.EnrichmentStatus, nil
		}
		if !hasStoredData {
			return false, "", "", refreshErr
		}
		// Any other refresh failure (e.g. Campus API + my.kpi.ua both down)
		// also degrades to stale-serve rather than a hard error.
		return true, "", state.EnrichmentStatus, nil
	}

	return false, model.SessionActive, newStatus, nil
}
