package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"kpi-schedule-bot/server/internal/campus"
	"kpi-schedule-bot/server/internal/storage"
)

func setupTestServer(t *testing.T) (http.Handler, *storage.DB, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := storage.Migrate(dbPath); err != nil {
		t.Fatalf("migrating db: %v", err)
	}

	db, err := storage.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	campusClient := campus.NewClient(db)
	svc := NewService(db, campusClient)
	token := "test-internal-token"
	router := NewRouter(svc, token)

	return router, db, token
}

func TestAuthPairGenerateAndVerify(t *testing.T) {
	router, _, internalToken := setupTestServer(t)

	// 1. Generate pairing code (as Bot)
	genReqBody, _ := json.Marshal(map[string]any{"telegram_id": 987654321})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/pair/generate", bytes.NewReader(genReqBody))
	req.Header.Set("X-Internal-Token", internalToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("generate code failed: %d, body: %s", w.Code, w.Body.String())
	}

	var genResp struct {
		PairCode  string `json:"pair_code"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(w.Body).Decode(&genResp); err != nil {
		t.Fatalf("decoding generate response: %v", err)
	}
	if len(genResp.PairCode) != 6 {
		t.Errorf("expected 6-digit code, got %q", genResp.PairCode)
	}

	// 2. Verify code (as Extension)
	verifyReqBody, _ := json.Marshal(map[string]any{"pair_code": genResp.PairCode})
	vReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/pair/verify", bytes.NewReader(verifyReqBody))
	vReq.Header.Set("Content-Type", "application/json")
	vW := httptest.NewRecorder()

	router.ServeHTTP(vW, vReq)
	if vW.Code != http.StatusOK {
		t.Fatalf("verify code failed: %d, body: %s", vW.Code, vW.Body.String())
	}

	var verifyResp struct {
		Success    bool   `json:"success"`
		TelegramID int64  `json:"telegram_id"`
		AuthToken  string `json:"auth_token"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(vW.Body).Decode(&verifyResp); err != nil {
		t.Fatalf("decoding verify response: %v", err)
	}
	if !verifyResp.Success || verifyResp.TelegramID != 987654321 || verifyResp.AuthToken == "" {
		t.Errorf("unexpected verify response: %+v", verifyResp)
	}

	// 3. Re-verify consumed code must fail
	vReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/pair/verify", bytes.NewReader(verifyReqBody))
	vReq2.Header.Set("Content-Type", "application/json")
	vW2 := httptest.NewRecorder()

	router.ServeHTTP(vW2, vReq2)
	if vW2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for already-consumed code, got %d", vW2.Code)
	}
}

func TestScheduleSyncWithToken(t *testing.T) {
	router, db, internalToken := setupTestServer(t)

	// Setup user & token
	user, err := db.UpsertUser(context.Background(), 111222333, nil, nil)
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	testToken := "secret-client-token-12345"
	if err := db.CreateUserToken(context.Background(), user.ID, testToken); err != nil {
		t.Fatalf("create user token: %v", err)
	}

	// Send schedule sync
	syncReq := map[string]any{
		"auth_token": testToken,
		"group_name": "ІП-21",
		"lessons": []map[string]any{
			{
				"date":         "2026-09-01",
				"start_time":   "08:30:00",
				"end_time":     "10:05:00",
				"subject":      "Технології DevOps",
				"tag":          "lec",
				"teacher_raw":  "Колумбет В. П.",
				"location_raw": "Онлайн Zoom",
			},
		},
	}
	body, _ := json.Marshal(syncReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("schedule sync failed: %d, body: %s", w.Code, w.Body.String())
	}

	var syncResp struct {
		Success     bool   `json:"success"`
		LessonCount int    `json:"lesson_count"`
		GroupName   string `json:"group_name"`
	}
	if err := json.NewDecoder(w.Body).Decode(&syncResp); err != nil {
		t.Fatalf("decode sync response: %v", err)
	}
	if !syncResp.Success || syncResp.LessonCount != 1 {
		t.Errorf("unexpected sync response: %+v", syncResp)
	}

	// Verify query /today
	todayReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedule/date?telegram_id=111222333&date=2026-09-01", nil)
	todayReq.Header.Set("X-Internal-Token", internalToken)
	tW := httptest.NewRecorder()

	router.ServeHTTP(tW, todayReq)
	if tW.Code != http.StatusOK {
		t.Fatalf("query schedule date failed: %d, body: %s", tW.Code, tW.Body.String())
	}
}

func TestScheduleSyncRejectsRawTelegramIDWithoutInternalToken(t *testing.T) {
	router, _, _ := setupTestServer(t)

	// Attempt to push schedule using only raw Telegram ID without pair code or token
	syncReq := map[string]any{
		"telegram_id": 999888777,
		"lessons": []map[string]any{
			{
				"date":       "2026-09-01",
				"start_time": "08:30:00",
				"subject":    "Spoofed Course",
			},
		},
	}
	body, _ := json.Marshal(syncReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated telegram_id push, got %d", w.Code)
	}
}

