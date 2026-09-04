package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestTelemetryReportAction(t *testing.T) {
	var mu sync.Mutex
	var receivedPayload IngestPayload
	var receivedKey string
	received := make(chan struct{}, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		receivedKey = r.Header.Get("X-Ingest-Key")
		_ = json.NewDecoder(r.Body).Decode(&receivedPayload)
		w.WriteHeader(http.StatusOK)
		select {
		case received <- struct{}{}:
		default:
		}
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-secret-key")
	client.ReportAction("telegram_command", "/today", 200, 42, map[string]any{"source": "test"})

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for telemetry report")
	}

	mu.Lock()
	defer mu.Unlock()

	if receivedKey != "test-secret-key" {
		t.Errorf("expected X-Ingest-Key 'test-secret-key', got %q", receivedKey)
	}
	if receivedPayload.ActionType != "telegram_command" {
		t.Errorf("expected ActionType 'telegram_command', got %q", receivedPayload.ActionType)
	}
	if receivedPayload.ActionName != "/today" {
		t.Errorf("expected ActionName '/today', got %q", receivedPayload.ActionName)
	}
	if receivedPayload.StatusCode != 200 {
		t.Errorf("expected StatusCode 200, got %d", receivedPayload.StatusCode)
	}
	if receivedPayload.DurationMs != 42 {
		t.Errorf("expected DurationMs 42, got %d", receivedPayload.DurationMs)
	}
}

func TestTelemetryNilOrDisabled(t *testing.T) {
	// Should not panic or error when client is nil or URL is empty
	var nilClient *Client
	nilClient.ReportAction("test", "test", 200, 0, nil)

	emptyClient := NewClient("", "")
	emptyClient.ReportAction("test", "test", 200, 0, nil)
}
