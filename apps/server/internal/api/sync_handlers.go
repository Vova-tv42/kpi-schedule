package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
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

	code, err := generate6DigitCode()
	if err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, "failed to generate pairing code")
		return
	}

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	if err := h.svc.db.CreatePairingCode(r.Context(), code, req.TelegramID, expiresAt); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, pairGenerateResponse{
		PairCode:  code,
		ExpiresIn: 600,
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

// POST /api/v1/schedule/sync
// Ingestion endpoint for the browser extension
func (h *handlers) postScheduleSync(w http.ResponseWriter, r *http.Request) {
	var req scheduleSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		model.WriteError(w, http.StatusBadRequest, model.ErrInvalidRequest, "invalid json payload")
		return
	}

	var user model.User
	var err error

	// 1. Resolve user by pair_code, auth_token, or telegram_id
	if req.PairCode != "" {
		code := strings.ReplaceAll(strings.TrimSpace(req.PairCode), "-", "")
		telegramID, pairErr := h.svc.db.VerifyAndConsumePairingCode(r.Context(), code)
		if pairErr != nil {
			model.WriteError(w, http.StatusUnauthorized, model.ErrUnauthorized, "invalid or expired pairing code")
			return
		}
		user, err = h.svc.db.UpsertUser(r.Context(), telegramID, nil, nil)
	} else if req.AuthToken != "" {
		user, err = h.svc.db.GetUserByToken(r.Context(), strings.TrimSpace(req.AuthToken))
	} else if req.TelegramID != 0 {
		user, err = h.svc.db.GetUserByTelegramID(r.Context(), req.TelegramID)
		if errors.Is(err, storage.ErrNotFound) {
			user, err = h.svc.db.UpsertUser(r.Context(), req.TelegramID, nil, nil)
		}
	} else {
		// Check X-User-Token header as fallback
		tokenHeader := r.Header.Get("X-User-Token")
		if tokenHeader != "" {
			user, err = h.svc.db.GetUserByToken(r.Context(), strings.TrimSpace(tokenHeader))
		} else {
			model.WriteError(w, http.StatusUnauthorized, model.ErrAuthRequired, "authentication required (pair_code or auth_token)")
			return
		}
	}

	if err != nil {
		if errors.Is(err, storage.ErrInvalidToken) || errors.Is(err, storage.ErrNotFound) {
			model.WriteError(w, http.StatusUnauthorized, model.ErrUnauthorized, "invalid user or token")
			return
		}
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, err.Error())
		return
	}

	// 2. Parse lessons from DTO
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

	// 3. Resolve group name and group ID if provided
	groupName := strings.TrimSpace(req.GroupName)
	if groupName != "" {
		groups, gErr := h.svc.campus.Groups(r.Context())
		if gErr == nil {
			for _, g := range groups {
				if strings.EqualFold(g.Name, groupName) {
					user, _ = h.svc.db.UpsertUser(r.Context(), user.TelegramID, &g.ID, &g.Name)
					break
				}
			}
		}
	}

	// 4. Enrich lessons with Campus API
	now := time.Now().UTC()
	var lessonsToStore []model.Lesson
	enrichmentStatus := model.EnrichmentDegraded

	if user.GroupID != nil {
		groupSchedule, schedErr := h.svc.campus.GroupSchedule(r.Context(), *user.GroupID)
		slots, slotErr := h.svc.campus.LessonSlots(r.Context())
		currTime, timeErr := h.svc.campus.CurrentTime(r.Context())

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

	// Fallback to unenriched lessons if Campus API was unavailable
	if len(lessonsToStore) == 0 && len(parsedLessons) > 0 {
		currTime, _ := h.svc.campus.CurrentTime(r.Context())
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

	// 5. Replace lessons in DB
	if err := h.svc.db.ReplaceLessons(r.Context(), user.ID, lessonsToStore, enrichmentStatus, nil); err != nil {
		model.WriteError(w, http.StatusInternalServerError, model.ErrInternal, fmt.Sprintf("storing lessons: %s", err))
		return
	}

	writeJSON(w, http.StatusOK, scheduleSyncResponse{
		Success:          true,
		LessonCount:      len(lessonsToStore),
		GroupName:        user.GroupName,
		EnrichmentStatus: string(enrichmentStatus),
		SyncedAt:         now,
	})
}
