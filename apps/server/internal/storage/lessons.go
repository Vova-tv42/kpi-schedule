package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
