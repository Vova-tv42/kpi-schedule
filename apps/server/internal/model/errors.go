package model

import (
	"encoding/json"
	"net/http"
	"time"
)

// Error codes, taken verbatim from docs/architecture/error-handling-resilience.md §4.
const (
	ErrAuthRequired         = "ERR_AUTH_REQUIRED"
	ErrCampusAPIUnavailable = "ERR_CAMPUS_API_UNAVAILABLE"
	ErrGroupNotFound        = "ERR_GROUP_NOT_FOUND"
	ErrUserNotFound         = "ERR_USER_NOT_FOUND"
	ErrIssueNotFound        = "ERR_ISSUE_NOT_FOUND"
	ErrInvalidRequest       = "ERR_INVALID_REQUEST"
	ErrInternal             = "ERR_INTERNAL"
	ErrUnauthorized         = "ERR_UNAUTHORIZED"
	ErrRateLimited          = "ERR_RATE_LIMITED"
)

// APIError is the standard error envelope for every non-2xx JSON response.
type APIError struct {
	Success   bool      `json:"success"`
	ErrorCode string    `json:"error_code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func NewAPIError(code, message string) APIError {
	return APIError{Success: false, ErrorCode: code, Message: message, Timestamp: time.Now().UTC()}
}

// WriteError writes the APIError envelope with the given HTTP status.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(NewAPIError(code, message))
}
