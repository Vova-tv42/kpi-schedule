package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"kpi-schedule-bot/server/internal/model"
)

func TestIPRateLimiter_AllowsUpToLimit(t *testing.T) {
	limiter := newIPRateLimiter(3, 100*time.Millisecond)

	for i := 1; i <= 3; i++ {
		allowed, _ := limiter.allow("1.2.3.4")
		if !allowed {
			t.Fatalf("request %d should be allowed, got blocked", i)
		}
	}

	allowed, retryAfter := limiter.allow("1.2.3.4")
	if allowed {
		t.Fatalf("request 4 should be blocked, got allowed")
	}
	if retryAfter <= 0 || retryAfter > 100*time.Millisecond {
		t.Errorf("unexpected retryAfter: %v", retryAfter)
	}
}

func TestIPRateLimiter_ResetsAfterWindow(t *testing.T) {
	limiter := newIPRateLimiter(2, 50*time.Millisecond)

	// Consume limit
	limiter.allow("10.0.0.1")
	limiter.allow("10.0.0.1")

	// Next request is blocked
	allowed, _ := limiter.allow("10.0.0.1")
	if allowed {
		t.Fatalf("expected request to be blocked")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Window has reset: first request starts new window and succeeds
	allowed, _ = limiter.allow("10.0.0.1")
	if !allowed {
		t.Fatalf("expected request after window reset to be allowed")
	}
}

func TestIPRateLimiter_DifferentIPsIndependent(t *testing.T) {
	limiter := newIPRateLimiter(1, 100*time.Millisecond)

	allowedA, _ := limiter.allow("1.1.1.1")
	if !allowedA {
		t.Fatal("expected IP A first request to be allowed")
	}

	// IP A is now blocked
	allowedA2, _ := limiter.allow("1.1.1.1")
	if allowedA2 {
		t.Fatal("expected IP A second request to be blocked")
	}

	// IP B is not affected by IP A
	allowedB, _ := limiter.allow("2.2.2.2")
	if !allowedB {
		t.Fatal("expected IP B first request to be allowed")
	}
}

func TestIPRateLimitMiddleware_HTTPIntegration(t *testing.T) {
	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromRemoteAddr)

	// Custom small limit for testing HTTP responses
	testLimiter := newIPRateLimiter(2, 80*time.Millisecond)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			allowed, _ := testLimiter.allow(ip)
			if !allowed {
				w.Header().Set("Retry-After", "1")
				model.WriteError(w, http.StatusTooManyRequests, model.ErrRateLimited, "too many requests, slow down")
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	makeReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.50:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// First 2 requests succeed
	w1 := makeReq()
	if w1.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w1.Code)
	}

	w2 := makeReq()
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}

	// 3rd request blocked with 429
	w3 := makeReq()
	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w3.Code)
	}

	var errResp model.APIError
	if err := json.NewDecoder(w3.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if errResp.ErrorCode != model.ErrRateLimited {
		t.Errorf("expected %s, got %s", model.ErrRateLimited, errResp.ErrorCode)
	}
	if w3.Header().Get("Retry-After") == "" {
		t.Errorf("expected Retry-After header to be set")
	}

	// After wait, requests succeed again
	time.Sleep(90 * time.Millisecond)
	w4 := makeReq()
	if w4.Code != http.StatusOK {
		t.Errorf("expected 200 after window reset, got %d", w4.Code)
	}
}
