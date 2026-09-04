package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// IngestPayload models the anonymous action event sent to the admin dashboard.
type IngestPayload struct {
	ActionType string         `json:"action_type"`
	ActionName string         `json:"action_name"`
	StatusCode int            `json:"status_code"`
	DurationMs int64          `json:"duration_ms"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Client delivers anonymous action events to the admin dashboard ingestion endpoint.
type Client struct {
	ingestURL  string
	ingestKey  string
	httpClient *http.Client
}

// NewClient constructs a new telemetry Client. If ingestURL is empty, reporting is disabled.
func NewClient(ingestURL, ingestKey string) *Client {
	return &Client{
		ingestURL: ingestURL,
		ingestKey: ingestKey,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// ReportAction dispatches an action metric in an asynchronous background goroutine.
// It never blocks the caller and never fails the request lifecycle.
func (c *Client) ReportAction(actionType, actionName string, statusCode int, durationMs int64, metadata map[string]any) {
	if c == nil || c.ingestURL == "" {
		return
	}

	payload := IngestPayload{
		ActionType: actionType,
		ActionName: actionName,
		StatusCode: statusCode,
		DurationMs: durationMs,
		Metadata:   metadata,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Debug("telemetry panic recovered", "panic", r)
			}
		}()

		data, err := json.Marshal(payload)
		if err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.ingestURL, bytes.NewReader(data))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if c.ingestKey != "" {
			req.Header.Set("X-Ingest-Key", c.ingestKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			slog.Debug("telemetry ingest request failed", "error", err)
			return
		}
		_ = resp.Body.Close()
	}()
}
