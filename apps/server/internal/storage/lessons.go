package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

// ReplaceLessons atomically swaps a user's whole lesson set and records the
// refresh outcome, so a partial scrape can never leave half-old, half-new data.
func (db *DB) ReplaceLessons(ctx context.Context, userID uuid.UUID, lessons []model.Lesson, enrichment model.EnrichmentStatus, lastError *string) error {
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_lessons WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("clearing old lessons: %w", err)
	}

	for _, l := range lessons {
		var lecturerID, lecturerName *string
		if l.Lecturer != nil {
			lecturerID, lecturerName = &l.Lecturer.ID, &l.Lecturer.Name
		}
		var locTitle, locURI *string
		if l.Location != nil {
			locTitle, locURI = &l.Location.Title, &l.Location.URI
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO user_lessons
				(id, user_id, date, week, day, slot, start_time, end_time, subject, subject_norm, tag,
				 teacher_raw, location_raw, lecturer_id, lecturer_name, location_title, location_uri, enriched, is_recurring)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (user_id, date, start_time, subject_norm) DO UPDATE
			SET end_time = excluded.end_time, tag = excluded.tag, teacher_raw = excluded.teacher_raw,
			    location_raw = excluded.location_raw, lecturer_id = excluded.lecturer_id,
			    lecturer_name = excluded.lecturer_name, location_title = excluded.location_title,
			    location_uri = excluded.location_uri, enriched = excluded.enriched, is_recurring = excluded.is_recurring
		`, uuid.New(), userID, l.Date, l.Week, l.Day, l.Slot, l.StartTime, l.EndTime, l.Subject, l.SubjectNorm, l.Tag,
			l.TeacherRaw, l.LocationRaw, lecturerID, lecturerName, locTitle, locURI, l.Enriched, l.IsRecurring)
		if err != nil {
			return fmt.Errorf("inserting lesson %q: %w", l.Subject, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_schedule_state (user_id, refreshed_at, lesson_count, enrichment_status, last_error)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE
		SET refreshed_at = excluded.refreshed_at, lesson_count = excluded.lesson_count,
		    enrichment_status = excluded.enrichment_status, last_error = excluded.last_error
	`, userID, time.Now().UTC(), len(lessons), string(enrichment), lastError)
	if err != nil {
		return fmt.Errorf("updating schedule state: %w", err)
	}

	return tx.Commit()
}

func (db *DB) GetScheduleState(ctx context.Context, userID uuid.UUID) (model.ScheduleState, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT user_id, refreshed_at, lesson_count, enrichment_status, last_error
		FROM user_schedule_state WHERE user_id = ?
	`, userID)

	var s model.ScheduleState
	var status string
	if err := row.Scan(&s.UserID, &s.RefreshedAt, &s.LessonCount, &status, &s.LastError); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ScheduleState{}, ErrNotFound
		}
		return model.ScheduleState{}, fmt.Errorf("fetching schedule state: %w", err)
	}
	s.EnrichmentStatus = model.EnrichmentStatus(status)
	return s, nil
}

const lessonColumns = `id, user_id, date, week, day, slot, start_time, end_time, subject, subject_norm, tag,
	       teacher_raw, location_raw, lecturer_id, lecturer_name, location_title, location_uri, enriched, is_recurring`

func scanLesson(rows *sql.Rows) (model.Lesson, error) {
	var l model.Lesson
	var lecturerID, lecturerName, locTitle, locURI *string
	err := rows.Scan(&l.ID, &l.UserID, &l.Date, &l.Week, &l.Day, &l.Slot, &l.StartTime, &l.EndTime,
		&l.Subject, &l.SubjectNorm, &l.Tag, &l.TeacherRaw, &l.LocationRaw,
		&lecturerID, &lecturerName, &locTitle, &locURI, &l.Enriched, &l.IsRecurring)
	if err != nil {
		return model.Lesson{}, fmt.Errorf("scanning lesson: %w", err)
	}
	if lecturerID != nil {
		l.Lecturer = &model.Lecturer{ID: *lecturerID, Name: *lecturerName}
	}
	if locTitle != nil {
		l.Location = &model.Location{Title: *locTitle, URI: *locURI}
	}
	return l, nil
}

// GetLessonsByDateRange returns all stored lessons for a user with a date in
// [start, end] (inclusive), ordered chronologically.
func (db *DB) GetLessonsByDateRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]model.Lesson, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT `+lessonColumns+`
		FROM user_lessons WHERE user_id = ? AND date BETWEEN ? AND ?
		ORDER BY date, start_time
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("querying lessons: %w", err)
	}
	defer rows.Close()

	var lessons []model.Lesson
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}

// SetLessonURL saves or updates a custom URL for a lesson identified by user, normalized subject, and tag.
func (db *DB) SetLessonURL(ctx context.Context, userID uuid.UUID, subjectNorm, tag, url string) error {
	now := time.Now().UTC()
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO user_lesson_urls (id, user_id, subject_norm, tag, url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, subject_norm, tag) DO UPDATE
		SET url = excluded.url, updated_at = excluded.updated_at
	`, uuid.New(), userID, subjectNorm, tag, url, now, now)
	if err != nil {
		return fmt.Errorf("saving lesson url: %w", err)
	}
	return nil
}

// GetLessonURLs returns all stored custom URLs for user's lessons as a map keyed by "subject_norm|tag".
func (db *DB) GetLessonURLs(ctx context.Context, userID uuid.UUID) (map[string]string, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT subject_norm, tag, url FROM user_lesson_urls WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying lesson urls: %w", err)
	}
	defer rows.Close()

	urls := make(map[string]string)
	for rows.Next() {
		var sn, tag, url string
		if err := rows.Scan(&sn, &tag, &url); err != nil {
			return nil, fmt.Errorf("scanning lesson url: %w", err)
		}
		urls[sn+"|"+tag] = url
	}
	return urls, rows.Err()
}

// DeleteLessonURL deletes a stored custom URL for a lesson.
func (db *DB) DeleteLessonURL(ctx context.Context, userID uuid.UUID, subjectNorm, tag string) error {
	_, err := db.SQL.ExecContext(ctx, `
		DELETE FROM user_lesson_urls WHERE user_id = ? AND subject_norm = ? AND tag = ?
	`, userID, subjectNorm, tag)
	if err != nil {
		return fmt.Errorf("deleting lesson url: %w", err)
	}
	return nil
}

// SetURLPrompt records an active prompt for a user entering a URL.
func (db *DB) SetURLPrompt(ctx context.Context, userID uuid.UUID, telegramID, promptMsgID int64, subjectNorm, tag, subjectName string) error {
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO user_url_prompts (user_id, telegram_id, prompt_message_id, subject_norm, tag, subject_name, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (telegram_id) DO UPDATE
		SET prompt_message_id = excluded.prompt_message_id,
		    subject_norm = excluded.subject_norm,
		    tag = excluded.tag,
		    subject_name = excluded.subject_name,
		    updated_at = excluded.updated_at
	`, userID, telegramID, promptMsgID, subjectNorm, tag, subjectName, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("saving url prompt: %w", err)
	}
	return nil
}

// GetURLPrompt retrieves the active URL entry prompt for a Telegram user.
func (db *DB) GetURLPrompt(ctx context.Context, telegramID int64) (*model.URLPrompt, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT user_id, telegram_id, prompt_message_id, subject_norm, tag, subject_name, updated_at
		FROM user_url_prompts WHERE telegram_id = ?
	`, telegramID)

	var p model.URLPrompt
	if err := row.Scan(&p.UserID, &p.TelegramID, &p.PromptMessageID, &p.SubjectNorm, &p.Tag, &p.SubjectName, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("fetching url prompt: %w", err)
	}
	return &p, nil
}

// ClearURLPrompt removes the active URL entry prompt for a Telegram user.
func (db *DB) ClearURLPrompt(ctx context.Context, telegramID int64) error {
	_, err := db.SQL.ExecContext(ctx, `DELETE FROM user_url_prompts WHERE telegram_id = ?`, telegramID)
	if err != nil {
		return fmt.Errorf("clearing url prompt: %w", err)
	}
	return nil
}

// GetUniqueScheduleLessons returns deduplicated online lessons from user_lessons,
// excluding offline classes, and populated with existing custom URLs.
func (db *DB) GetUniqueScheduleLessons(ctx context.Context, userID uuid.UUID) ([]model.UniqueLesson, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT subject, subject_norm, tag, location_raw, location_title
		FROM user_lessons
		WHERE user_id = ?
		ORDER BY subject, tag
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying unique lessons: %w", err)
	}
	defer rows.Close()

	type groupData struct {
		subject     string
		subjectNorm string
		tag         string
		hasOnline   bool
	}
	groups := make(map[string]*groupData)
	var groupKeys []string

	for rows.Next() {
		var subject, subjectNorm, tag, locRaw string
		var locTitle *string
		if err := rows.Scan(&subject, &subjectNorm, &tag, &locRaw, &locTitle); err != nil {
			return nil, fmt.Errorf("scanning unique lesson row: %w", err)
		}
		key := subjectNorm + "|" + tag
		loc := locRaw
		if locTitle != nil && *locTitle != "" {
			loc = *locTitle
		}
		isOnline := model.IsOnline(loc)

		g, exists := groups[key]
		if !exists {
			g = &groupData{
				subject:     subject,
				subjectNorm: subjectNorm,
				tag:         tag,
				hasOnline:   isOnline,
			}
			groups[key] = g
			groupKeys = append(groupKeys, key)
		} else {
			if isOnline {
				g.hasOnline = true
			}
			if g.subject == "" && subject != "" {
				g.subject = subject
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	urls, err := db.GetLessonURLs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting lesson urls: %w", err)
	}

	var unique []model.UniqueLesson
	for _, key := range groupKeys {
		g := groups[key]
		if !g.hasOnline {
			continue
		}
		unique = append(unique, model.UniqueLesson{
			Subject:     g.subject,
			SubjectNorm: g.subjectNorm,
			Tag:         g.tag,
			IsOnline:    true,
			URL:         urls[key],
		})
	}

	sort.Slice(unique, func(i, j int) bool {
		if unique[i].Subject != unique[j].Subject {
			return unique[i].Subject < unique[j].Subject
		}
		return unique[i].Tag < unique[j].Tag
	})

	return unique, nil
}

