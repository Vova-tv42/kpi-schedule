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
	router, _, internalToken := setupTestServer(t)

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

	// 1. No token header -> 401
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/sync", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for no token header, got %d", w1.Code)
	}

	// 2. Invalid token header -> 401
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/sync", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Token", "wrong-secret-token")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid X-Internal-Token, got %d", w2.Code)
	}

	// 3. Valid token header -> 200 OK
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/sync", bytes.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Internal-Token", internalToken)
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid X-Internal-Token, got %d, body: %s", w3.Code, w3.Body.String())
	}
}

func TestScheduleRawSync(t *testing.T) {
	router, _, internalToken := setupTestServer(t)

	// 1. Generate pairing code
	genReqBody, _ := json.Marshal(map[string]any{"telegram_id": 555444333})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/pair/generate", bytes.NewReader(genReqBody))
	req.Header.Set("X-Internal-Token", internalToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("generate code failed: %d", w.Code)
	}
	var genResp struct {
		PairCode string `json:"pair_code"`
	}
	_ = json.NewDecoder(w.Body).Decode(&genResp)

	// 2. Raw sync with raw FullCalendar events fixture
	rawPayload := map[string]any{
		"pair_code": genResp.PairCode,
		"events": []map[string]any{
			{
				"id":             1019849,
				"title":          "Технології DevOps",
				"start":          "2026-09-01T08:30:00",
				"end":            "2026-09-01T10:05:00",
				"description":    "<i>Колумбет В. П.</i>",
				"descriptionRAW": "Викладачі: Колумбет В. П.",
				"extendedProps": map[string]any{
					"type":        "prc",
					"locationRAW": ", URL: Не вказано",
					"locationPDF": "Online Zoom",
					"groups":      "ТВ-41, ТВ-42",
				},
			},
			{
				"id":             1019850,
				"title":          "Архітектура ПЗ",
				"start":          "2026-09-01T10:20:00",
				"end":            "2026-09-01T11:55:00",
				"description":    "Петренко П. П.",
				"descriptionRAW": "Викладач: Петренко П. П.",
				"extendedProps": map[string]any{
					"type":        "lec",
					"locationRAW": ", URL: Не вказано",
					"locationPDF": "Online Teams",
					"groups":      "ТВ-42",
				},
			},
		},
	}
	rawBytes, _ := json.Marshal(rawPayload)

	syncReq := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/raw-sync", bytes.NewReader(rawBytes))
	syncReq.Header.Set("Content-Type", "application/json")
	syncW := httptest.NewRecorder()
	router.ServeHTTP(syncW, syncReq)

	if syncW.Code != http.StatusOK {
		t.Fatalf("raw-sync failed: %d, body: %s", syncW.Code, syncW.Body.String())
	}

	var syncResp struct {
		Success     bool    `json:"success"`
		LessonCount int     `json:"lesson_count"`
		GroupName   *string `json:"group_name"`
	}
	if err := json.NewDecoder(syncW.Body).Decode(&syncResp); err != nil {
		t.Fatalf("decoding sync response: %v", err)
	}
	if !syncResp.Success || syncResp.LessonCount != 2 {
		t.Errorf("expected 2 lessons stored, got %+v", syncResp)
	}
	// "ТВ-42" appeared in both events, so it should be detected as the primary group
	if syncResp.GroupName == nil || *syncResp.GroupName != "ТВ-42" {
		t.Errorf("expected detected group ТВ-42, got %v", syncResp.GroupName)
	}

	// 3. Verify querying the schedule
	schedReq := httptest.NewRequest(http.MethodGet, "/api/v1/schedule/date?telegram_id=555444333&date=2026-09-01", nil)
	schedReq.Header.Set("X-Internal-Token", internalToken)
	schedW := httptest.NewRecorder()
	router.ServeHTTP(schedW, schedReq)

	if schedW.Code != http.StatusOK {
		t.Fatalf("query schedule date failed: %d, body: %s", schedW.Code, schedW.Body.String())
	}

	var dateResp struct {
		Lessons []struct {
			Name       string `json:"name"`
			Tag        string `json:"tag"`
			TeacherRaw string `json:"teacher_raw"`
		} `json:"lessons"`
	}
	_ = json.NewDecoder(schedW.Body).Decode(&dateResp)
	if len(dateResp.Lessons) != 2 {
		t.Fatalf("expected 2 lessons in date response, got %d", len(dateResp.Lessons))
	}
	if dateResp.Lessons[0].Tag != "prac" { // "prc" was normalized to "prac"
		t.Errorf("expected tag prac, got %q", dateResp.Lessons[0].Tag)
	}
	if dateResp.Lessons[0].TeacherRaw != "Колумбет В. П." {
		t.Errorf("expected stripped teacher, got %q", dateResp.Lessons[0].TeacherRaw)
	}

	// 4. Repeated sync with same consumed pair_code must fail (401)
	repeatReq := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/raw-sync", bytes.NewReader(rawBytes))
	repeatReq.Header.Set("Content-Type", "application/json")
	repeatW := httptest.NewRecorder()
	router.ServeHTTP(repeatW, repeatReq)
	if repeatW.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for consumed pair code, got %d", repeatW.Code)
	}
}

func TestFormatTimeHHMMSS(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"8:30", "08:30:00"},
		{"08:30:00", "08:30:00"},
		{"8:30:00", "08:30:00"},
		{" 8:30:00 ", "08:30:00"},
		{"14:15", "14:15:00"},
		{"14:15:20", "14:15:20"},
		{"", ""},
	}
	for _, tc := range tests {
		got := formatTimeHHMMSS(tc.input)
		if got != tc.expected {
			t.Errorf("formatTimeHHMMSS(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestCleanTeacherRaw(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<i>Викладач: Колумбет В. П.</i>", "Колумбет В. П."},
		{"Викладачі: Іванов І. І.", "Іванов І. І."},
		{"Викладача: Сидоров С. С.", "Сидоров С. С."},
		{"Колумбет В. П.", "Колумбет В. П."},
		{"", ""},
	}
	for _, tc := range tests {
		got := cleanTeacherRaw(tc.input)
		if got != tc.expected {
			t.Errorf("cleanTeacherRaw(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSyncEmptyPayloadValidation(t *testing.T) {
	router, _, internalToken := setupTestServer(t)

	// 1. Extension sync with empty lessons array -> 400
	req1Body, _ := json.Marshal(map[string]any{
		"telegram_id": 12345,
		"lessons":     []any{},
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/sync", bytes.NewReader(req1Body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Token", internalToken)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty lessons, got %d", w1.Code)
	}

	// 2. Console sync with empty events array -> 400
	req2Body, _ := json.Marshal(map[string]any{
		"telegram_id": 12345,
		"events":      []any{},
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/raw-sync", bytes.NewReader(req2Body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Token", internalToken)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty events, got %d", w2.Code)
	}
}



