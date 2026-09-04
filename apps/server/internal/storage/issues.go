package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"kpi-schedule-bot/server/internal/model"
)

// IssueDraftTTL bounds how long an in-flight /issues wizard survives. After it
// the draft is discarded and the bot's prompt message is deleted by the sweeper
// (see (*bot.Bot).SweepExpiredIssueDrafts).
const IssueDraftTTL = 10 * time.Minute

// ErrIssueDraftExpired is returned by GetIssueDraft when a draft existed but
// its TTL had elapsed. The row is consumed (deleted) in the process, so callers
// get this exactly once and should tell the user the flow was interrupted.
var ErrIssueDraftExpired = errors.New("issue draft expired")

const issueColumns = `id, number, author_telegram_id, author_username, author_first_name, type, title, body, status, status_by, status_note, thread_state, created_at, updated_at`

func scanIssue(row interface{ Scan(...any) error }) (model.Issue, error) {
	var i model.Issue
	var idStr string
	err := row.Scan(&idStr, &i.Number, &i.AuthorTelegramID, &i.AuthorUsername, &i.AuthorFirstName,
		&i.Type, &i.Title, &i.Body, &i.Status, &i.StatusBy, &i.StatusNote, &i.ThreadState,
		&i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Issue{}, ErrNotFound
		}
		return model.Issue{}, fmt.Errorf("scanning issue: %w", err)
	}
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return model.Issue{}, fmt.Errorf("parsing issue uuid: %w", err)
	}
	i.ID = parsedID
	return i, nil
}

// CreateIssue assigns the next public number and inserts the issue. The number
// is derived inside a transaction with MAX(number)+1; db.SQL is capped at a
// single open connection (see Open), so writers are already serialized, and the
// UNIQUE constraint on number is the backstop.
func (db *DB) CreateIssue(ctx context.Context, issue model.Issue) (model.Issue, error) {
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return model.Issue{}, fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	var next int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(number), 0) + 1 FROM issues`).Scan(&next); err != nil {
		return model.Issue{}, fmt.Errorf("allocating issue number: %w", err)
	}

	now := time.Now().UTC()
	issue.ID = uuid.New()
	issue.Number = next
	issue.Status = model.IssueOnReview
	issue.ThreadState = model.IssueThreadNone
	issue.CreatedAt = now
	issue.UpdatedAt = now

	_, err = tx.ExecContext(ctx, `
		INSERT INTO issues (`+issueColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', ?, ?, ?)
	`, issue.ID.String(), issue.Number, issue.AuthorTelegramID, issue.AuthorUsername, issue.AuthorFirstName,
		string(issue.Type), issue.Title, issue.Body, string(issue.Status),
		string(issue.ThreadState), now, now)
	if err != nil {
		return model.Issue{}, fmt.Errorf("inserting issue: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.Issue{}, fmt.Errorf("committing issue: %w", err)
	}
	return issue, nil
}

// GetIssueByID returns a single issue, or ErrNotFound.
func (db *DB) GetIssueByID(ctx context.Context, id uuid.UUID) (model.Issue, error) {
	row := db.SQL.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE id = ?`, id.String())
	return scanIssue(row)
}

// GetIssueByNumber returns a single issue by its public #N, or ErrNotFound.
func (db *DB) GetIssueByNumber(ctx context.Context, number int) (model.Issue, error) {
	row := db.SQL.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE number = ?`, number)
	return scanIssue(row)
}

func collectIssues(rows *sql.Rows) ([]model.Issue, error) {
	defer rows.Close()
	var out []model.Issue
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, issue)
	}
	return out, rows.Err()
}

// ListIssuesByAuthor returns one page of a user's own issues, newest first.
func (db *DB) ListIssuesByAuthor(ctx context.Context, telegramID int64, limit, offset int) ([]model.Issue, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT `+issueColumns+` FROM issues
		WHERE author_telegram_id = ?
		ORDER BY number DESC
		LIMIT ? OFFSET ?
	`, telegramID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing issues by author: %w", err)
	}
	return collectIssues(rows)
}

// CountIssuesByAuthor returns how many issues a user has filed.
func (db *DB) CountIssuesByAuthor(ctx context.Context, telegramID int64) (int, error) {
	var n int
	err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE author_telegram_id = ?`, telegramID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting issues by author: %w", err)
	}
	return n, nil
}

// IssueFilter narrows the admin dashboard's issue queue. Zero values mean "no
// filter"; Limit defaults to 50 and is capped at 200.
type IssueFilter struct {
	Status string
	Type   string
	Query  string
	Limit  int
	Offset int
}

// buildIssueWhere renders the shared WHERE clause for ListIssues/CountIssues so
// the list and its total can never drift apart.
func buildIssueWhere(f IssueFilter) (string, []any) {
	var clauses []string
	var args []any
	if f.Status != "" {
		clauses = append(clauses, `status = ?`)
		args = append(args, f.Status)
	}
	if f.Type != "" {
		clauses = append(clauses, `type = ?`)
		args = append(args, f.Type)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		clauses = append(clauses, `(title LIKE ? OR body LIKE ? OR author_username LIKE ?)`)
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// ListIssues returns one page of the whole issue queue, newest first.
func (db *DB) ListIssues(ctx context.Context, f IssueFilter) ([]model.Issue, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	where, args := buildIssueWhere(f)
	args = append(args, limit, f.Offset)

	rows, err := db.SQL.QueryContext(ctx, `SELECT `+issueColumns+` FROM issues`+where+` ORDER BY number DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing issues: %w", err)
	}
	return collectIssues(rows)
}

// CountIssues returns the total number of issues matching the filter.
func (db *DB) CountIssues(ctx context.Context, f IssueFilter) (int, error) {
	where, args := buildIssueWhere(f)
	var n int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues`+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting issues: %w", err)
	}
	return n, nil
}

// CountIssuesByStatus returns totals per status for the dashboard's filter tabs.
func (db *DB) CountIssuesByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := db.SQL.QueryContext(ctx, `SELECT status, COUNT(*) FROM issues GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("counting issues by status: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("scanning status count: %w", err)
		}
		out[status] = n
	}
	return out, rows.Err()
}

// UpdateIssueStatus moves an issue through triage and records which admin did
// it. note is the optional explanation the admin attaches to the change (say,
// why an issue was rejected); it is delivered to the reporter with the status
// DM and shown on their issue screen. Passing an empty note clears the previous
// one, so a stale explanation never outlives the status it explained.
// Returns ErrNotFound if no such issue exists.
func (db *DB) UpdateIssueStatus(ctx context.Context, id uuid.UUID, status model.IssueStatus, note, adminEmail string) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE issues SET status = ?, status_by = ?, status_note = ?, updated_at = ? WHERE id = ?
	`, string(status), adminEmail, note, now, id.String())
	if err != nil {
		return fmt.Errorf("updating issue status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetIssueThreadState opens, closes or resets an issue's discussion. A user
// sees the thread once it leaves IssueThreadNone and can reply only while it is
// IssueThreadOpen; IssueThreadClosed keeps the transcript readable but stops
// new messages from their side.
func (db *DB) SetIssueThreadState(ctx context.Context, id uuid.UUID, state model.IssueThreadState) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `UPDATE issues SET thread_state = ?, updated_at = ? WHERE id = ?`,
		string(state), now, id.String())
	if err != nil {
		return fmt.Errorf("updating issue thread state: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteIssue removes an issue permanently. Its comments go with it via the
// ON DELETE CASCADE on issue_comments; any reply draft pointing at it is
// cleared in the same transaction so the bot never prompts for a reply to an
// issue that no longer exists.
func (db *DB) DeleteIssue(ctx context.Context, id uuid.UUID) error {
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_issue_drafts WHERE issue_id = ?`, id.String()); err != nil {
		return fmt.Errorf("clearing drafts for issue: %w", err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM issues WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("deleting issue: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing issue deletion: %w", err)
	}
	return nil
}

const issueCommentColumns = `id, issue_id, author_role, author_label, body, created_at`

func scanIssueComment(row interface{ Scan(...any) error }) (model.IssueComment, error) {
	var c model.IssueComment
	var idStr, issueIDStr string
	err := row.Scan(&idStr, &issueIDStr, &c.AuthorRole, &c.AuthorLabel, &c.Body, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.IssueComment{}, ErrNotFound
		}
		return model.IssueComment{}, fmt.Errorf("scanning issue comment: %w", err)
	}
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return model.IssueComment{}, fmt.Errorf("parsing comment uuid: %w", err)
	}
	parsedIssueID, err := uuid.Parse(issueIDStr)
	if err != nil {
		return model.IssueComment{}, fmt.Errorf("parsing comment issue uuid: %w", err)
	}
	c.ID = parsedID
	c.IssueID = parsedIssueID
	return c, nil
}

// AddIssueComment appends one message to an issue's discussion thread and
// bumps the issue's updated_at so the dashboard can sort by recent activity.
func (db *DB) AddIssueComment(ctx context.Context, comment model.IssueComment) (model.IssueComment, error) {
	now := time.Now().UTC()
	comment.ID = uuid.New()
	comment.CreatedAt = now

	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return model.IssueComment{}, fmt.Errorf("beginning tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO issue_comments (`+issueCommentColumns+`)
		VALUES (?, ?, ?, ?, ?, ?)
	`, comment.ID.String(), comment.IssueID.String(), string(comment.AuthorRole), comment.AuthorLabel, comment.Body, now)
	if err != nil {
		return model.IssueComment{}, fmt.Errorf("inserting issue comment: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, comment.IssueID.String()); err != nil {
		return model.IssueComment{}, fmt.Errorf("touching issue: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.IssueComment{}, fmt.Errorf("committing issue comment: %w", err)
	}
	return comment, nil
}

// ListIssueComments returns a thread's full history, oldest first.
func (db *DB) ListIssueComments(ctx context.Context, issueID uuid.UUID) ([]model.IssueComment, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT `+issueCommentColumns+` FROM issue_comments
		WHERE issue_id = ?
		ORDER BY created_at ASC
	`, issueID.String())
	if err != nil {
		return nil, fmt.Errorf("listing issue comments: %w", err)
	}
	defer rows.Close()

	var out []model.IssueComment
	for rows.Next() {
		c, err := scanIssueComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountIssueComments returns how many messages a thread holds.
func (db *DB) CountIssueComments(ctx context.Context, issueID uuid.UUID) (int, error) {
	var n int
	err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue_comments WHERE issue_id = ?`, issueID.String()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting issue comments: %w", err)
	}
	return n, nil
}

// SetIssueDraft upserts the caller's in-flight wizard state, (re)starting the
// TTL clock. One draft per user: starting a new flow replaces the old one, the
// same way user_url_prompts / user_group_prompts behave.
func (db *DB) SetIssueDraft(ctx context.Context, draft model.IssueDraft) error {
	now := time.Now().UTC()
	var issueID any
	if draft.IssueID != nil {
		issueID = draft.IssueID.String()
	}
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO user_issue_drafts (telegram_id, chat_id, prompt_message_id, step, issue_type, title, issue_id, expires_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (telegram_id) DO UPDATE
		SET chat_id = excluded.chat_id,
		    prompt_message_id = excluded.prompt_message_id,
		    step = excluded.step,
		    issue_type = excluded.issue_type,
		    title = excluded.title,
		    issue_id = excluded.issue_id,
		    expires_at = excluded.expires_at,
		    updated_at = excluded.updated_at
	`, draft.TelegramID, draft.ChatID, draft.PromptMessageID, draft.Step, string(draft.IssueType),
		draft.Title, issueID, now.Add(IssueDraftTTL), now)
	if err != nil {
		return fmt.Errorf("saving issue draft: %w", err)
	}
	return nil
}

// GetIssueDraft returns the caller's active draft, or (nil, nil) when there is
// none. When one existed but timed out it returns the expired draft *and*
// ErrIssueDraftExpired: the row is consumed, so the caller sees the expiry
// exactly once, and gets the prompt_message_id it needs to clean up the wizard
// message the abandoned flow left behind.
func (db *DB) GetIssueDraft(ctx context.Context, telegramID int64) (*model.IssueDraft, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT telegram_id, chat_id, prompt_message_id, step, issue_type, title, issue_id, expires_at, updated_at
		FROM user_issue_drafts WHERE telegram_id = ?
	`, telegramID)

	draft, err := scanIssueDraft(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if time.Now().UTC().After(draft.ExpiresAt) {
		_ = db.ClearIssueDraft(ctx, telegramID)
		return &draft, ErrIssueDraftExpired
	}
	return &draft, nil
}

func scanIssueDraft(row interface{ Scan(...any) error }) (model.IssueDraft, error) {
	var d model.IssueDraft
	var issueID sql.NullString
	err := row.Scan(&d.TelegramID, &d.ChatID, &d.PromptMessageID, &d.Step, &d.IssueType, &d.Title, &issueID, &d.ExpiresAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.IssueDraft{}, ErrNotFound
		}
		return model.IssueDraft{}, fmt.Errorf("scanning issue draft: %w", err)
	}
	if issueID.Valid && issueID.String != "" {
		parsed, err := uuid.Parse(issueID.String)
		if err != nil {
			return model.IssueDraft{}, fmt.Errorf("parsing draft issue uuid: %w", err)
		}
		d.IssueID = &parsed
	}
	return d, nil
}

// ClearIssueDraft removes the caller's active draft, if any.
func (db *DB) ClearIssueDraft(ctx context.Context, telegramID int64) error {
	_, err := db.SQL.ExecContext(ctx, `DELETE FROM user_issue_drafts WHERE telegram_id = ?`, telegramID)
	if err != nil {
		return fmt.Errorf("clearing issue draft: %w", err)
	}
	return nil
}

// ListExpiredIssueDrafts returns drafts whose TTL has elapsed. The sweeper uses
// it to delete the bot's own prompt messages before clearing the rows — the
// lazy expiry in GetIssueDraft cannot do that, since nothing may ever touch the
// draft again.
func (db *DB) ListExpiredIssueDrafts(ctx context.Context, now time.Time) ([]model.IssueDraft, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT telegram_id, chat_id, prompt_message_id, step, issue_type, title, issue_id, expires_at, updated_at
		FROM user_issue_drafts WHERE expires_at < ?
	`, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("listing expired issue drafts: %w", err)
	}
	defer rows.Close()

	var out []model.IssueDraft
	for rows.Next() {
		d, err := scanIssueDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
