package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"kpi-schedule-bot/server/internal/model"
)

// ReplaceLessons atomically swaps a user's whole lesson set and records the
// refresh outcome, so a partial scrape can never leave half-old, half-new data.
func (db *DB) ReplaceLessons(ctx context.Context, userID uuid.UUID, lessons []model.Lesson, enrichment model.EnrichmentStatus, lastError *string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM user_lessons WHERE user_id = $1`, userID); err != nil {
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

		_, err := tx.Exec(ctx, `
			INSERT INTO user_lessons
				(user_id, week, day, slot, start_time, subject, subject_norm, tag, type,
				 lecturer_id, lecturer_name, location_title, location_uri, dates, enriched)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		`, userID, l.Week, l.Day, l.Slot, l.StartTime, l.Subject, l.SubjectNorm, l.Tag, l.Type,
			lecturerID, lecturerName, locTitle, locURI, l.Dates, l.Enriched)
		if err != nil {
			return fmt.Errorf("inserting lesson %q: %w", l.Subject, err)
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_schedule_state (user_id, refreshed_at, lesson_count, enrichment_status, last_error)
		VALUES ($1, now(), $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET refreshed_at = now(), lesson_count = $2, enrichment_status = $3, last_error = $4
	`, userID, len(lessons), string(enrichment), lastError)
	if err != nil {
		return fmt.Errorf("updating schedule state: %w", err)
	}

	return tx.Commit(ctx)
}

func (db *DB) GetScheduleState(ctx context.Context, userID uuid.UUID) (model.ScheduleState, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT user_id, refreshed_at, lesson_count, enrichment_status, last_error
		FROM user_schedule_state WHERE user_id = $1
	`, userID)

	var s model.ScheduleState
	var status string
	if err := row.Scan(&s.UserID, &s.RefreshedAt, &s.LessonCount, &status, &s.LastError); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ScheduleState{}, ErrNotFound
		}
		return model.ScheduleState{}, fmt.Errorf("fetching schedule state: %w", err)
	}
	s.EnrichmentStatus = model.EnrichmentStatus(status)
	return s, nil
}

// GetLessons returns all stored lessons for a user, optionally filtered to one week (1 or 2).
// Pass week=0 to fetch both weeks.
func (db *DB) GetLessons(ctx context.Context, userID uuid.UUID, week int) ([]model.Lesson, error) {
	var rows pgx.Rows
	var err error
	if week == 0 {
		rows, err = db.Pool.Query(ctx, `
			SELECT id, user_id, week, day, slot, start_time, subject, subject_norm, tag, type,
			       lecturer_id, lecturer_name, location_title, location_uri, dates, enriched
			FROM user_lessons WHERE user_id = $1
			ORDER BY week, day, slot
		`, userID)
	} else {
		rows, err = db.Pool.Query(ctx, `
			SELECT id, user_id, week, day, slot, start_time, subject, subject_norm, tag, type,
			       lecturer_id, lecturer_name, location_title, location_uri, dates, enriched
			FROM user_lessons WHERE user_id = $1 AND week = $2
			ORDER BY day, slot
		`, userID, week)
	}
	if err != nil {
		return nil, fmt.Errorf("querying lessons: %w", err)
	}
	defer rows.Close()

	var lessons []model.Lesson
	for rows.Next() {
		var l model.Lesson
		var lecturerID, lecturerName, locTitle, locURI *string
		if err := rows.Scan(&l.ID, &l.UserID, &l.Week, &l.Day, &l.Slot, &l.StartTime,
			&l.Subject, &l.SubjectNorm, &l.Tag, &l.Type,
			&lecturerID, &lecturerName, &locTitle, &locURI, &l.Dates, &l.Enriched); err != nil {
			return nil, fmt.Errorf("scanning lesson: %w", err)
		}
		if lecturerID != nil {
			l.Lecturer = &model.Lecturer{ID: *lecturerID, Name: *lecturerName}
		}
		if locTitle != nil {
			l.Location = &model.Location{Title: *locTitle, URI: *locURI}
		}
		lessons = append(lessons, l)
	}
	return lessons, rows.Err()
}
