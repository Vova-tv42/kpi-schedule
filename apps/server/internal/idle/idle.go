// Package idle provides an activity tracker and middleware that triggers a
// shutdown signal when the HTTP server has been idle (no requests received or
// in-flight) for a configurable duration. Used for scale-to-zero hosting (e.g.
// Fly.io Fly Machines).
package idle

import (
	"net/http"
	"sync/atomic"
	"time"
)

// Watcher monitors server activity and signals when the idle timeout expires.
type Watcher struct {
	timeout        time.Duration
	activeRequests atomic.Int32
	lastActivity   atomic.Int64
	done           chan struct{}
	closed         atomic.Bool
	stopTicker     chan struct{}
	excludedPaths  map[string]struct{}
}

// New creates a Watcher and starts its background ticker. If timeout <= 0, the
// watcher is disabled and Done() will never receive.
func New(timeout time.Duration, excludedPaths ...string) *Watcher {
	w := &Watcher{
		timeout:       timeout,
		done:          make(chan struct{}),
		stopTicker:    make(chan struct{}),
		excludedPaths: make(map[string]struct{}, len(excludedPaths)),
	}
	for _, p := range excludedPaths {
		w.excludedPaths[p] = struct{}{}
	}
	w.touch()

	if timeout > 0 {
		go w.run()
	}

	return w
}

func (w *Watcher) touch() {
	w.lastActivity.Store(time.Now().UnixNano())
}

func (w *Watcher) run() {
	// Check frequently enough to react promptly, but no faster than 100ms.
	interval := w.timeout / 4
	if interval > 15*time.Second {
		interval = 15 * time.Second
	} else if interval < 100*time.Millisecond {
		interval = 100 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopTicker:
			return
		case <-ticker.C:
			if w.activeRequests.Load() > 0 {
				continue
			}
			last := time.Unix(0, w.lastActivity.Load())
			if time.Since(last) >= w.timeout {
				if w.closed.CompareAndSwap(false, true) {
					close(w.done)
				}
				return
			}
		}
	}
}

// Middleware wraps an http.Handler to update the last activity timestamp and
// track in-flight requests. Excluded paths (e.g. /healthz) are ignored so
// health probes do not keep the server awake.
func (w *Watcher) Middleware(next http.Handler) http.Handler {
	if w == nil || w.timeout <= 0 {
		return next
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if _, excluded := w.excludedPaths[r.URL.Path]; excluded {
			next.ServeHTTP(rw, r)
			return
		}

		w.activeRequests.Add(1)
		defer w.activeRequests.Add(-1)

		w.touch()
		next.ServeHTTP(rw, r)
		w.touch()
	})
}

// Done returns a channel that is closed when the idle timeout has elapsed.
func (w *Watcher) Done() <-chan struct{} {
	return w.done
}

// Stop terminates the background ticker.
func (w *Watcher) Stop() {
	if w.timeout > 0 && w.closed.CompareAndSwap(false, true) {
		close(w.stopTicker)
	}
}
