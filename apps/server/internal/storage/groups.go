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

const botGroupColumns = `id, creator_telegram_id, academic_group_id, academic_group_name, faculty, telegram_chat_id, telegram_chat_title, notifications_enabled, created_at, updated_at`

func scanBotGroup(row interface{ Scan(...any) error }) (model.BotGroup, error) {
	var g model.BotGroup
	var idStr string
	var chatID sql.NullInt64
	err := row.Scan(&idStr, &g.CreatorTelegramID, &g.AcademicGroupID, &g.AcademicGroupName, &g.Faculty, &chatID, &g.TelegramChatTitle, &g.NotificationsEnabled, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.BotGroup{}, ErrNotFound
		}
		return model.BotGroup{}, fmt.Errorf("scanning bot group: %w", err)
	}
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		return model.BotGroup{}, fmt.Errorf("parsing group uuid: %w", err)
	}
	g.ID = parsedID
	if chatID.Valid {
		g.TelegramChatID = &chatID.Int64
	}
	return g, nil
}

// CreateBotGroup inserts a new bot group record.
func (db *DB) CreateBotGroup(ctx context.Context, creatorTelegramID int64, academicGroupID int, academicGroupName, faculty string, telegramChatID *int64, telegramChatTitle string) (model.BotGroup, error) {
	now := time.Now().UTC()
	id := uuid.New()

	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO bot_groups (id, creator_telegram_id, academic_group_id, academic_group_name, faculty, telegram_chat_id, telegram_chat_title, notifications_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
	`, id.String(), creatorTelegramID, academicGroupID, academicGroupName, faculty, telegramChatID, telegramChatTitle, now, now)
	if err != nil {
		return model.BotGroup{}, fmt.Errorf("inserting bot group: %w", err)
	}

	return model.BotGroup{
		ID:                   id,
		CreatorTelegramID:    creatorTelegramID,
		AcademicGroupID:      academicGroupID,
		AcademicGroupName:    academicGroupName,
		Faculty:              faculty,
		TelegramChatID:       telegramChatID,
		TelegramChatTitle:    telegramChatTitle,
		NotificationsEnabled: true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

// GetBotGroupByID returns a bot group by its UUID.
func (db *DB) GetBotGroupByID(ctx context.Context, id uuid.UUID) (model.BotGroup, error) {
	row := db.SQL.QueryRowContext(ctx, `SELECT `+botGroupColumns+` FROM bot_groups WHERE id = ?`, id.String())
	return scanBotGroup(row)
}

// GetBotGroupByChatID finds the bot group configured for a Telegram group chat.
func (db *DB) GetBotGroupByChatID(ctx context.Context, chatID int64) (model.BotGroup, error) {
	row := db.SQL.QueryRowContext(ctx, `SELECT `+botGroupColumns+` FROM bot_groups WHERE telegram_chat_id = ?`, chatID)
	return scanBotGroup(row)
}

// GetBotGroupsByCreator returns all bot groups created by a Telegram user.
func (db *DB) GetBotGroupsByCreator(ctx context.Context, creatorTelegramID int64) ([]model.BotGroup, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT `+botGroupColumns+` FROM bot_groups
		WHERE creator_telegram_id = ?
		ORDER BY created_at DESC
	`, creatorTelegramID)
	if err != nil {
		return nil, fmt.Errorf("querying creator bot groups: %w", err)
	}
	defer rows.Close()

	var groups []model.BotGroup
	for rows.Next() {
		g, err := scanBotGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// UpdateBotGroupAcademic updates the academic group info for an existing bot group.
func (db *DB) UpdateBotGroupAcademic(ctx context.Context, id uuid.UUID, academicGroupID int, academicGroupName, faculty string) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE bot_groups
		SET academic_group_id = ?, academic_group_name = ?, faculty = ?, updated_at = ?
		WHERE id = ?
	`, academicGroupID, academicGroupName, faculty, now, id.String())
	if err != nil {
		return fmt.Errorf("updating bot group academic info: %w", err)
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

// BindBotGroupChat associates a Telegram chat ID and title with a bot group.
func (db *DB) BindBotGroupChat(ctx context.Context, id uuid.UUID, chatID int64, chatTitle string) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE bot_groups
		SET telegram_chat_id = ?, telegram_chat_title = ?, updated_at = ?
		WHERE id = ?
	`, chatID, chatTitle, now, id.String())
	if err != nil {
		return fmt.Errorf("binding bot group chat: %w", err)
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

// UnbindBotGroupChat removes the Telegram chat association from a bot group.
func (db *DB) UnbindBotGroupChat(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE bot_groups
		SET telegram_chat_id = NULL, telegram_chat_title = '', updated_at = ?
		WHERE id = ?
	`, now, id.String())
	if err != nil {
		return fmt.Errorf("unbinding bot group chat: %w", err)
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

// DeleteBotGroup deletes a bot group and its associated records.
func (db *DB) DeleteBotGroup(ctx context.Context, id uuid.UUID) error {
	_, _ = db.SQL.ExecContext(ctx, `DELETE FROM bot_group_lesson_urls WHERE group_id = ?`, id.String())
	_, _ = db.SQL.ExecContext(ctx, `DELETE FROM bot_group_admins WHERE group_id = ?`, id.String())
	_, _ = db.SQL.ExecContext(ctx, `DELETE FROM user_group_prompts WHERE group_id = ?`, id.String())
	res, err := db.SQL.ExecContext(ctx, `DELETE FROM bot_groups WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("deleting bot group: %w", err)
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


// SetGroupPrompt records an active group input prompt for a user.
func (db *DB) SetGroupPrompt(ctx context.Context, p model.GroupPrompt) error {
	var groupIDStr *string
	if p.GroupID != nil {
		s := p.GroupID.String()
		groupIDStr = &s
	}
	now := time.Now().UTC()

	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO user_group_prompts (telegram_id, prompt_message_id, action, group_id, bind_chat_id, bind_chat_title, subject_norm, tag, subject_name, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (telegram_id) DO UPDATE
		SET prompt_message_id = excluded.prompt_message_id,
		    action = excluded.action,
		    group_id = excluded.group_id,
		    bind_chat_id = excluded.bind_chat_id,
		    bind_chat_title = excluded.bind_chat_title,
		    subject_norm = excluded.subject_norm,
		    tag = excluded.tag,
		    subject_name = excluded.subject_name,
		    updated_at = excluded.updated_at
	`, p.TelegramID, p.PromptMessageID, p.Action, groupIDStr, p.BindChatID, p.BindChatTitle, p.SubjectNorm, p.Tag, p.SubjectName, now)
	if err != nil {
		return fmt.Errorf("saving group prompt: %w", err)
	}
	return nil
}

// GetGroupPrompt retrieves an active group input prompt for a user.
func (db *DB) GetGroupPrompt(ctx context.Context, telegramID int64) (*model.GroupPrompt, error) {
	row := db.SQL.QueryRowContext(ctx, `
		SELECT telegram_id, prompt_message_id, action, group_id, bind_chat_id, bind_chat_title, subject_norm, tag, subject_name, updated_at
		FROM user_group_prompts WHERE telegram_id = ?
	`, telegramID)

	var p model.GroupPrompt
	var groupIDStr sql.NullString
	var bindChatID sql.NullInt64

	if err := row.Scan(&p.TelegramID, &p.PromptMessageID, &p.Action, &groupIDStr, &bindChatID, &p.BindChatTitle, &p.SubjectNorm, &p.Tag, &p.SubjectName, &p.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("fetching group prompt: %w", err)
	}

	if groupIDStr.Valid {
		id, err := uuid.Parse(groupIDStr.String)
		if err == nil {
			p.GroupID = &id
		}
	}
	if bindChatID.Valid {
		p.BindChatID = &bindChatID.Int64
	}

	return &p, nil
}

// ClearGroupPrompt removes an active group input prompt for a user.
func (db *DB) ClearGroupPrompt(ctx context.Context, telegramID int64) error {
	_, err := db.SQL.ExecContext(ctx, `DELETE FROM user_group_prompts WHERE telegram_id = ?`, telegramID)
	if err != nil {
		return fmt.Errorf("clearing group prompt: %w", err)
	}
	return nil
}

// SetGroupLessonURL upserts a custom URL for a group's lesson.
func (db *DB) SetGroupLessonURL(ctx context.Context, groupID uuid.UUID, subjectNorm, tag, rawURL string) error {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO bot_group_lesson_urls (id, group_id, subject_norm, tag, url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (group_id, subject_norm, tag) DO UPDATE
		SET url = excluded.url,
		    updated_at = excluded.updated_at
	`, id, groupID.String(), subjectNorm, tag, rawURL, now, now)
	if err != nil {
		return fmt.Errorf("upserting group lesson url: %w", err)
	}
	return nil
}

// GetGroupLessonURLs retrieves all custom URLs for a bot group, returned as a map of "subjectNorm:tag" -> url.
func (db *DB) GetGroupLessonURLs(ctx context.Context, groupID uuid.UUID) (map[string]string, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT subject_norm, tag, url
		FROM bot_group_lesson_urls
		WHERE group_id = ?
	`, groupID.String())
	if err != nil {
		return nil, fmt.Errorf("querying group lesson urls: %w", err)
	}
	defer rows.Close()

	urls := make(map[string]string)
	for rows.Next() {
		var norm, tag, u string
		if err := rows.Scan(&norm, &tag, &u); err != nil {
			return nil, fmt.Errorf("scanning group lesson url: %w", err)
		}
		urls[norm+"|"+tag] = u
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating group lesson urls: %w", err)
	}
	return urls, nil
}

// DeleteGroupLessonURL removes a custom URL for a group's lesson.
func (db *DB) DeleteGroupLessonURL(ctx context.Context, groupID uuid.UUID, subjectNorm, tag string) error {
	_, err := db.SQL.ExecContext(ctx, `
		DELETE FROM bot_group_lesson_urls
		WHERE group_id = ? AND subject_norm = ? AND tag = ?
	`, groupID.String(), subjectNorm, tag)
	if err != nil {
		return fmt.Errorf("deleting group lesson url: %w", err)
	}
	return nil
}

// SetBotGroupNotifications toggles notifications_enabled for a bot group.
func (db *DB) SetBotGroupNotifications(ctx context.Context, id uuid.UUID, enabled bool) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE bot_groups
		SET notifications_enabled = ?, updated_at = ?
		WHERE id = ?
	`, enabled, now, id.String())
	if err != nil {
		return fmt.Errorf("updating bot group notifications: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetActiveBotGroupsWithNotifications returns all groups with a bound Telegram chat and notifications enabled.
func (db *DB) GetActiveBotGroupsWithNotifications(ctx context.Context) ([]model.BotGroup, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT `+botGroupColumns+`
		FROM bot_groups
		WHERE telegram_chat_id IS NOT NULL AND notifications_enabled = 1
	`)
	if err != nil {
		return nil, fmt.Errorf("querying active bot groups with notifications: %w", err)
	}
	defer rows.Close()

	var groups []model.BotGroup
	for rows.Next() {
		g, err := scanBotGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetBotGroupsForUser returns all bot groups where the user is the creator or an accepted admin.
func (db *DB) GetBotGroupsForUser(ctx context.Context, telegramID int64) ([]model.BotGroup, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT `+botGroupColumns+` FROM bot_groups
		WHERE creator_telegram_id = ?
		   OR id IN (SELECT group_id FROM bot_group_admins WHERE telegram_id = ? AND status = 'accepted')
		ORDER BY created_at DESC
	`, telegramID, telegramID)
	if err != nil {
		return nil, fmt.Errorf("querying user bot groups: %w", err)
	}
	defer rows.Close()

	var groups []model.BotGroup
	for rows.Next() {
		g, err := scanBotGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetGroupAdmins returns all invited and accepted administrators for a group.
func (db *DB) GetGroupAdmins(ctx context.Context, groupID uuid.UUID) ([]model.GroupAdmin, error) {
	rows, err := db.SQL.QueryContext(ctx, `
		SELECT group_id, telegram_id, username, first_name, status, created_at, updated_at
		FROM bot_group_admins
		WHERE group_id = ?
		ORDER BY created_at ASC
	`, groupID.String())
	if err != nil {
		return nil, fmt.Errorf("querying group admins: %w", err)
	}
	defer rows.Close()

	var admins []model.GroupAdmin
	for rows.Next() {
		var a model.GroupAdmin
		var gidStr string
		var statusStr string
		if err := rows.Scan(&gidStr, &a.TelegramID, &a.Username, &a.FirstName, &statusStr, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning group admin: %w", err)
		}
		parsedID, err := uuid.Parse(gidStr)
		if err != nil {
			return nil, fmt.Errorf("parsing group admin uuid: %w", err)
		}
		a.GroupID = parsedID
		a.Status = model.GroupAdminStatus(statusStr)
		admins = append(admins, a)
	}
	return admins, rows.Err()
}

// AddGroupAdmin invites or records a user as an administrator for a group.
func (db *DB) AddGroupAdmin(ctx context.Context, groupID uuid.UUID, telegramID int64, username, firstName string) error {
	now := time.Now().UTC()
	_, err := db.SQL.ExecContext(ctx, `
		INSERT INTO bot_group_admins (group_id, telegram_id, username, first_name, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'invited', ?, ?)
		ON CONFLICT (group_id, telegram_id) DO UPDATE
		SET username = excluded.username,
		    first_name = excluded.first_name,
		    updated_at = excluded.updated_at
	`, groupID.String(), telegramID, username, firstName, now, now)
	if err != nil {
		return fmt.Errorf("adding group admin: %w", err)
	}
	return nil
}

// RemoveGroupAdmin removes an administrator record for a group.
func (db *DB) RemoveGroupAdmin(ctx context.Context, groupID uuid.UUID, telegramID int64) error {
	_, err := db.SQL.ExecContext(ctx, `
		DELETE FROM bot_group_admins
		WHERE group_id = ? AND telegram_id = ?
	`, groupID.String(), telegramID)
	if err != nil {
		return fmt.Errorf("removing group admin: %w", err)
	}
	return nil
}

// AcceptGroupAdmin marks an invited administrator as accepted.
func (db *DB) AcceptGroupAdmin(ctx context.Context, groupID uuid.UUID, telegramID int64) error {
	now := time.Now().UTC()
	res, err := db.SQL.ExecContext(ctx, `
		UPDATE bot_group_admins
		SET status = 'accepted', updated_at = ?
		WHERE group_id = ? AND telegram_id = ?
	`, now, groupID.String(), telegramID)
	if err != nil {
		return fmt.Errorf("accepting group admin: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetGroupAdminRelation returns "creator", "accepted", "invited", or "" for a user.
func (db *DB) GetGroupAdminRelation(ctx context.Context, groupID uuid.UUID, telegramID int64) (string, error) {
	group, err := db.GetBotGroupByID(ctx, groupID)
	if err != nil {
		return "", err
	}
	if group.CreatorTelegramID == telegramID {
		return "creator", nil
	}

	var status string
	err = db.SQL.QueryRowContext(ctx, `
		SELECT status FROM bot_group_admins
		WHERE group_id = ? AND telegram_id = ?
	`, groupID.String(), telegramID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("checking group admin relation: %w", err)
	}
	return status, nil
}

// DeleteOrTransferGroupOwnership handles group deletion:
// If telegramID is a co-admin: removes them from group admins.
// If telegramID is the creator:
//   - If another accepted co-admin exists, transfers ownership to that admin.
//   - If no other accepted co-admins exist, completely deletes the group.
func (db *DB) DeleteOrTransferGroupOwnership(ctx context.Context, groupID uuid.UUID, telegramID int64) (transferred bool, newOwnerID int64, err error) {
	group, err := db.GetBotGroupByID(ctx, groupID)
	if err != nil {
		return false, 0, err
	}

	if group.CreatorTelegramID != telegramID {
		if err := db.RemoveGroupAdmin(ctx, groupID, telegramID); err != nil {
			return false, 0, err
		}
		return false, 0, nil
	}

	var nextAdminID int64
	err = db.SQL.QueryRowContext(ctx, `
		SELECT telegram_id FROM bot_group_admins
		WHERE group_id = ? AND status = 'accepted'
		ORDER BY created_at ASC LIMIT 1
	`, groupID.String()).Scan(&nextAdminID)

	if err == nil {
		now := time.Now().UTC()
		_, err = db.SQL.ExecContext(ctx, `
			UPDATE bot_groups
			SET creator_telegram_id = ?, updated_at = ?
			WHERE id = ?
		`, nextAdminID, now, groupID.String())
		if err != nil {
			return false, 0, fmt.Errorf("transferring creator ownership: %w", err)
		}
		_ = db.RemoveGroupAdmin(ctx, groupID, nextAdminID)
		return true, nextAdminID, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return false, 0, fmt.Errorf("checking next admin for transfer: %w", err)
	}

	if err := db.DeleteBotGroup(ctx, groupID); err != nil {
		return false, 0, err
	}
	return false, 0, nil
}

