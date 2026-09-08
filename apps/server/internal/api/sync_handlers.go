package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"kpi-schedule-bot/server/internal/engine"
	"kpi-schedule-bot/server/internal/model"
	"kpi-schedule-bot/server/internal/storage"
)

type pairGenerateRequest struct {
	TelegramID int64 `json:"telegram_id"`
}

type pairGenerateResponse struct {
	PairCode  string `json:"pair_code"`
	ExpiresIn int    `json:"expires_in"`
}

type pairVerifyRequest struct {
	PairCode string `json:"pair_code"`
}

type pairVerifyResponse struct {
	Success    bool   `json:"success"`
	TelegramID int64  `json:"telegram_id"`
	AuthToken  string `json:"auth_token"`
	Status     string `json:"status"`
}

type parsedLessonDTO struct {
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Subject     string `json:"subject"`
	Tag         string `json:"tag"`
	TeacherRaw  string `json:"teacher_raw"`
	LocationRaw string `json:"location_raw"`
}

type scheduleSyncRequest struct {
	PairCode   string            `json:"pair_code,omitempty"`
	AuthToken  string            `json:"auth_token,omitempty"`
	TelegramID int64             `json:"telegram_id,omitempty"`
	GroupName  string            `json:"group_name,omitempty"`
	Lessons    []parsedLessonDTO `json:"lessons"`
}

type rawFullCalendarExtendedPropsDTO struct {
	Type        string `json:"type"`
	LocationRAW string `json:"locationRAW"`
	LocationPDF string `json:"locationPDF"`
	Groups      string `json:"groups"`
}

type rawFullCalendarEventDTO struct {
	ID             any                              `json:"id"`
	Title          string                           `json:"title"`
	Start          string                           `json:"start"`
	End            string                           `json:"end"`
	Description    string                           `json:"description"`
	DescriptionRAW string                           `json:"descriptionRAW"`
	ExtendedProps  rawFullCalendarExtendedPropsDTO `json:"extendedProps"`
}

type scheduleRawSyncRequest struct {
	PairCode   string                    `json:"pair_code,omitempty"`
	AuthToken  string                    `json:"auth_token,omitempty"`
	TelegramID int64                     `json:"telegram_id,omitempty"`
	Events     []rawFullCalendarEventDTO `json:"events"`
}

type scheduleSyncResponse struct {
	Success          bool      `json:"success"`
	LessonCount      int       `json:"lesson_count"`
	GroupName        *string   `json:"group_name,omitempty"`
	EnrichmentStatus string    `json:"enrichment_status"`
	SyncedAt         time.Time `json:"synced_at"`
}

func generate6DigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+100000), nil
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// POST /api/v1/auth/pair/generate
// Protected by X-Internal-Token (called by Telegram Bot /link handler)
func (h *handlers) postAuthPairGenerate(w http.ResponseWriter, r *http.Request) {
	var req pairGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TelegramID == 0 {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "telegram_id is required")
		return
	}

	code, expiresIn, err := h.svc.GeneratePairCode(r.Context(), req.TelegramID)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, pairGenerateResponse{
		PairCode:  code,
		ExpiresIn: expiresIn,
	})
}

// POST /api/v1/auth/pair/verify
// Public/Extension endpoint: exchanges 6-digit code for a permanent client auth_token
func (h *handlers) postAuthPairVerify(w http.ResponseWriter, r *http.Request) {
	var req pairVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.PairCode) == "" {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "pair_code is required")
		return
	}

	code := strings.ReplaceAll(strings.TrimSpace(req.PairCode), "-", "")
	telegramID, err := h.svc.db.VerifyAndConsumePairingCode(r.Context(), code)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidOrExpiredCode) {
			model.WriteError(w, http.StatusUnauthorized, model.ErrUnauthorized, "invalid or expired pairing code")
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	user, err := h.svc.db.UpsertUser(r.Context(), telegramID, nil, nil)
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	token, err := generateSecureToken()
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, "failed to generate token")
		return
	}

	if err := h.svc.db.CreateUserToken(r.Context(), user.ID, token); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, pairVerifyResponse{
		Success:    true,
		TelegramID: telegramID,
		AuthToken:  token,
		Status:     "LINKED",
	})
}

var (
	htmlTagRegex       = regexp.MustCompile(`<[^>]*>`)
	teacherPrefixRegex = regexp.MustCompile(`(?i)^Викладач(?:і|а)?:\s*`)
)

func normalizeMyKPITag(tag string) string {
	clean := strings.ToLower(strings.TrimSpace(tag))
	switch clean {
	case "lec", "лек":
		return "lec"
	case "prc", "прак", "prac":
		return "prac"
	case "lab", "лаб":
		return "lab"
	default:
		return ""
	}
}

func cleanTeacherRaw(raw string) string {
	if raw == "" {
		return ""
	}
	cleaned := htmlTagRegex.ReplaceAllString(raw, "")
	cleaned = teacherPrefixRegex.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

func cleanLocationRaw(raw string) string {
	if raw == "" {
		return ""
	}
	cleaned := htmlTagRegex.ReplaceAllString(raw, "")
	return strings.TrimSpace(cleaned)
}

func pad2(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func formatTimeHHMMSS(t string) string {
	parts := strings.Split(t, ":")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	hour := pad2(parts[0])
	min := "00"
	if len(parts) > 1 && parts[1] != "" {
		min = pad2(parts[1])
	}
	sec := "00"
	if len(parts) > 2 && parts[2] != "" {
		sec = pad2(parts[2])
	}
	return fmt.Sprintf("%s:%s:%s", hour, min, sec)
}

func parseRawFullCalendarEvents(events []rawFullCalendarEventDTO) ([]model.ParsedLesson, string) {
	lessons := make([]model.ParsedLesson, 0, len(events))
	groupOccurrences := make(map[string]int)

	for _, ev := range events {
		if strings.TrimSpace(ev.Start) == "" || strings.TrimSpace(ev.Title) == "" {
			continue
		}

		parts := strings.Split(ev.Start, "T")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			continue
		}
		datePart := parts[0]
		timePart := parts[1]

		d, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}

		startTime := formatTimeHHMMSS(timePart)
		var endTime string
		if strings.Contains(ev.End, "T") {
			endParts := strings.Split(ev.End, "T")
			if len(endParts) > 1 {
				endTime = formatTimeHHMMSS(endParts[1])
			}
		}

		teacherRaw := ev.DescriptionRAW
		if teacherRaw == "" {
			teacherRaw = ev.Description
		}
		locationRaw := ev.ExtendedProps.LocationPDF
		if locationRaw == "" {
			locationRaw = ev.ExtendedProps.LocationRAW
		}

		lessons = append(lessons, model.ParsedLesson{
			Date:        d,
			StartTime:   startTime,
			EndTime:     endTime,
			Subject:     strings.TrimSpace(ev.Title),
			Tag:         normalizeMyKPITag(ev.ExtendedProps.Type),
			TeacherRaw:  cleanTeacherRaw(teacherRaw),
			LocationRaw: cleanLocationRaw(locationRaw),
		})

		if ev.ExtendedProps.Groups != "" {
			rawGroups := strings.Split(ev.ExtendedProps.Groups, ",")
			for _, g := range rawGroups {
				trimmed := strings.TrimSpace(g)
				if trimmed != "" {
					groupOccurrences[trimmed]++
				}
			}
		}
	}

	var detectedGroup string
	maxCount := 0
	for grp, count := range groupOccurrences {
		if count > maxCount || (count == maxCount && (detectedGroup == "" || grp < detectedGroup)) {
			maxCount = count
			detectedGroup = grp
		}
	}

	return lessons, detectedGroup
}

func (h *handlers) resolveSyncUser(
	ctx context.Context,
	pairCode, authToken, headerToken string,
	telegramID int64,
	internalTokenHeader string,
) (model.User, int, string, error) {
	if pairCode != "" {
		code := strings.ReplaceAll(strings.TrimSpace(pairCode), "-", "")
		tid, pairErr := h.svc.db.VerifyAndConsumePairingCode(ctx, code)
		if pairErr != nil {
			if errors.Is(pairErr, storage.ErrInvalidOrExpiredCode) {
				return model.User{}, http.StatusUnauthorized, model.ErrUnauthorized, errors.New("invalid or expired pairing code")
			}
			return model.User{}, http.StatusInternalServerError, model.ErrInternal, pairErr
		}
		user, err := h.svc.db.UpsertUser(ctx, tid, nil, nil)
		if err != nil {
			return model.User{}, http.StatusInternalServerError, model.ErrInternal, err
		}
		return user, http.StatusOK, "", nil
	}

	if authToken != "" {
		user, err := h.svc.db.GetUserByToken(ctx, strings.TrimSpace(authToken))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrInvalidToken) {
				return model.User{}, http.StatusUnauthorized, model.ErrUnauthorized, errors.New("invalid user or token")
			}
			return model.User{}, http.StatusInternalServerError, model.ErrInternal, err
		}
		return user, http.StatusOK, "", nil
	}

	if headerToken != "" {
		user, err := h.svc.db.GetUserByToken(ctx, strings.TrimSpace(headerToken))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrInvalidToken) {
				return model.User{}, http.StatusUnauthorized, model.ErrUnauthorized, errors.New("invalid user or token")
			}
			return model.User{}, http.StatusInternalServerError, model.ErrInternal, err
		}
		return user, http.StatusOK, "", nil
	}

	if telegramID != 0 && h.internalToken != "" && subtle.ConstantTimeCompare([]byte(internalTokenHeader), []byte(h.internalToken)) == 1 {
		user, err := h.svc.db.GetUserByTelegramID(ctx, telegramID)
		if errors.Is(err, storage.ErrNotFound) {
			user, err = h.svc.db.UpsertUser(ctx, telegramID, nil, nil)
		}
		if err != nil {
			return model.User{}, http.StatusInternalServerError, model.ErrInternal, err
		}
		return user, http.StatusOK, "", nil
	}

	return model.User{}, http.StatusUnauthorized, model.ErrAuthRequired, errors.New("authentication required (pair_code, auth_token, or valid internal token)")
}

func (h *handlers) ingestAndStoreLessons(
	ctx context.Context,
	user model.User,
	groupName string,
	parsedLessons []model.ParsedLesson,
	start time.Time,
	actionName string,
) (scheduleSyncResponse, int, error) {
	groupName = strings.TrimSpace(groupName)
	if groupName != "" {
		groups, gErr := h.svc.campus.Groups(ctx)
		if gErr == nil {
			for _, g := range groups {
				if strings.EqualFold(g.Name, groupName) {
					updatedUser, uErr := h.svc.db.UpsertUser(ctx, user.TelegramID, &g.ID, &g.Name)
					if uErr == nil {
						user = updatedUser
					}
					break
				}
			}
		}
	}

	now := time.Now().UTC()
	var lessonsToStore []model.Lesson
	enrichmentStatus := model.EnrichmentDegraded

	if user.GroupID != nil {
		groupSchedule, schedErr := h.svc.campus.GroupSchedule(ctx, *user.GroupID)
		slots, slotErr := h.svc.campus.LessonSlots(ctx)
		currTime, timeErr := h.svc.campus.CurrentTime(ctx)

		if schedErr == nil && slotErr == nil && timeErr == nil {
			merged := engine.Merge(parsedLessons, groupSchedule, slots, now, currTime.CurrentWeek)
			lessonsToStore = make([]model.Lesson, 0, len(merged))
			for _, m := range merged {
				var lect *model.Lecturer
				if m.Lecturer != nil {
					lect = &model.Lecturer{ID: m.Lecturer.ID, Name: m.Lecturer.Name}
				}
				var loc *model.Location
				if m.Location != nil {
					loc = &model.Location{Title: m.Location.Title, URI: m.Location.URI}
				}
				lessonsToStore = append(lessonsToStore, model.Lesson{
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
					Lecturer:    lect,
					Location:    loc,
					Enriched:    m.Enriched,
					IsRecurring: m.IsRecurring,
				})
			}
			enrichmentStatus = model.EnrichmentFull
		}
	}

	if len(lessonsToStore) == 0 && len(parsedLessons) > 0 {
		currTime, _ := h.svc.campus.CurrentTime(ctx)
		refWeek := 1
		if currTime.CurrentWeek == 1 || currTime.CurrentWeek == 2 {
			refWeek = currTime.CurrentWeek
		}

		lessonsToStore = make([]model.Lesson, 0, len(parsedLessons))
		for _, p := range parsedLessons {
			week := engine.WeekAt(now, refWeek, p.Date)
			day := engine.ISODay(p.Date)
			norm := engine.NormalizeSubject(p.Subject)
			lessonsToStore = append(lessonsToStore, model.Lesson{
				UserID:      user.ID,
				Date:        p.Date,
				Week:        week,
				Day:         day,
				StartTime:   p.StartTime,
				EndTime:     p.EndTime,
				Subject:     p.Subject,
				SubjectNorm: norm,
				Tag:         p.Tag,
				TeacherRaw:  p.TeacherRaw,
				LocationRaw: p.LocationRaw,
				Enriched:    false,
				IsRecurring: true,
			})
		}
	}

	if err := h.svc.db.ReplaceLessons(ctx, user.ID, lessonsToStore, enrichmentStatus, nil); err != nil {
		if h.telemetry != nil {
			h.telemetry.ReportAction(actionName, "schedule_sync", http.StatusInternalServerError, time.Since(start).Milliseconds(), nil)
		}
		return scheduleSyncResponse{}, http.StatusInternalServerError, fmt.Errorf("storing lessons: %w", err)
	}

	if h.telemetry != nil {
		h.telemetry.ReportAction(actionName, "schedule_sync", http.StatusOK, time.Since(start).Milliseconds(), map[string]any{
			"lesson_count": len(lessonsToStore),
		})
	}

	return scheduleSyncResponse{
		Success:          true,
		LessonCount:      len(lessonsToStore),
		GroupName:        user.GroupName,
		EnrichmentStatus: string(enrichmentStatus),
		SyncedAt:         now,
	}, http.StatusOK, nil
}

// POST /api/v1/schedule/sync
// Ingestion endpoint for the browser extension
func (h *handlers) postScheduleSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req scheduleSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if h.telemetry != nil {
			h.telemetry.ReportAction("extension_sync", "schedule_sync", http.StatusBadRequest, time.Since(start).Milliseconds(), nil)
		}
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid json payload")
		return
	}

	user, status, errCode, err := h.resolveSyncUser(
		r.Context(),
		req.PairCode,
		req.AuthToken,
		r.Header.Get("X-User-Token"),
		req.TelegramID,
		r.Header.Get("X-Internal-Token"),
	)
	if err != nil {
		model.WriteError(w, status, errCode, err.Error())
		return
	}

	if len(req.Lessons) == 0 {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "lessons array cannot be empty")
		return
	}

	parsedLessons := make([]model.ParsedLesson, 0, len(req.Lessons))
	for _, dto := range req.Lessons {
		t, parseErr := time.Parse("2006-01-02", dto.Date)
		if parseErr != nil {
			continue
		}
		parsedLessons = append(parsedLessons, model.ParsedLesson{
			Date:        t,
			StartTime:   dto.StartTime,
			EndTime:     dto.EndTime,
			Subject:     dto.Subject,
			Tag:         dto.Tag,
			TeacherRaw:  dto.TeacherRaw,
			LocationRaw: dto.LocationRaw,
		})
	}

	if len(parsedLessons) == 0 {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid lesson dates: expected format YYYY-MM-DD")
		return
	}

	resp, statusCode, err := h.ingestAndStoreLessons(r.Context(), user, req.GroupName, parsedLessons, start, "extension_sync")
	if err != nil {
		model.WriteError(w, statusCode, model.ErrInternal, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// POST /api/v1/schedule/raw-sync
// Ingestion endpoint for browser console script (accepts raw FullCalendar event array)
func (h *handlers) postScheduleRawSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var req scheduleRawSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if h.telemetry != nil {
			h.telemetry.ReportAction("console_sync", "schedule_raw_sync", http.StatusBadRequest, time.Since(start).Milliseconds(), nil)
		}
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid json payload")
		return
	}

	user, status, errCode, err := h.resolveSyncUser(
		r.Context(),
		req.PairCode,
		req.AuthToken,
		r.Header.Get("X-User-Token"),
		req.TelegramID,
		r.Header.Get("X-Internal-Token"),
	)
	if err != nil {
		model.WriteError(w, status, errCode, err.Error())
		return
	}

	if len(req.Events) == 0 {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "events array cannot be empty")
		return
	}

	parsedLessons, detectedGroup := parseRawFullCalendarEvents(req.Events)
	if len(parsedLessons) == 0 {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "no valid events could be parsed")
		return
	}

	resp, statusCode, err := h.ingestAndStoreLessons(r.Context(), user, detectedGroup, parsedLessons, start, "console_sync")
	if err != nil {
		model.WriteError(w, statusCode, model.ErrInternal, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
