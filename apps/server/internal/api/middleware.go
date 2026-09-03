package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"kpi-schedule-bot/server/internal/model"
)

// internalTokenMiddleware requires the X-Internal-Token header to match the
// configured secret. Applied to every /api/v1 route except /healthz.
func internalTokenMiddleware(expected string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Internal-Token") != expected {
				model.WriteError(w, http.StatusUnauthorized, model.ErrUnauthorized, "missing or invalid X-Internal-Token header")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestsPerMinutePerIP caps how many /api/v1 requests a single client IP
// may make in a 1-minute window.
const requestsPerMinutePerIP = 20

// corsMiddleware handles preflight OPTIONS requests and sets CORS headers
// so the browser extension (chrome-extension://*) can communicate with the server.
func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Internal-Token, X-User-Token")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type ipBucket struct {
	count   int
	resetAt time.Time
}

type ipRateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	buckets     map[string]*ipBucket
	lastCleanup time.Time
}

func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		limit:       limit,
		window:      window,
		buckets:     make(map[string]*ipBucket),
		lastCleanup: time.Now(),
	}
}

func (l *ipRateLimiter) allow(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// Purge stale IP entries periodically
	if now.Sub(l.lastCleanup) > 2*time.Minute {
		for k, b := range l.buckets {
			if now.After(b.resetAt) {
				delete(l.buckets, k)
			}
		}
		l.lastCleanup = now
	}

	b, ok := l.buckets[ip]
	if !ok || now.After(b.resetAt) {
		l.buckets[ip] = &ipBucket{
			count:   1,
			resetAt: now.Add(l.window),
		}
		return true, 0
	}

	if b.count >= l.limit {
		return false, b.resetAt.Sub(now)
	}

	b.count++
	return true, 0
}

func clientIP(r *http.Request) string {
	ip := middleware.GetClientIP(r.Context())
	if ip == "" {
		ip = r.RemoteAddr
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return strings.TrimSpace(ip)
}

// ipRateLimitMiddleware rate-limits requests per client IP. Window begins on the
// first request and resets after the configured duration.
func ipRateLimitMiddleware() func(http.Handler) http.Handler {
	limiter := newIPRateLimiter(requestsPerMinutePerIP, time.Minute)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			allowed, retryAfter := limiter.allow(ip)
			if !allowed {
				retrySec := int(retryAfter.Seconds())
				if retrySec < 1 {
					retrySec = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retrySec))
				model.WriteError(w, http.StatusTooManyRequests, model.ErrRateLimited, "too many requests, slow down")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}


