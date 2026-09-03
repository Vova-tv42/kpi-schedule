package idle

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWatcher_Disabled(t *testing.T) {
	w := New(0)
	defer w.Stop()

	select {
	case <-w.Done():
		t.Fatal("expected disabled watcher never to close Done()")
	case <-time.After(50 * time.Millisecond):
		// OK
	}
}

func TestWatcher_TimeoutExpires(t *testing.T) {
	timeout := 200 * time.Millisecond
	w := New(timeout)
	defer w.Stop()

	select {
	case <-w.Done():
		// OK: triggered
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for idle watcher to close Done()")
	}
}

func TestWatcher_RequestsResetTimer(t *testing.T) {
	timeout := 300 * time.Millisecond
	w := New(timeout)
	defer w.Stop()

	handler := w.Middleware(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))

	// Send requests every 100ms for 500ms (longer than the 300ms timeout)
	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/schedule/today", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		select {
		case <-w.Done():
			t.Fatalf("watcher closed Done() prematurely at iteration %d", i)
		default:
		}
	}

	// Now stop sending requests; watcher should expire after timeout
	select {
	case <-w.Done():
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("watcher should have expired after activity stopped")
	}
}

func TestWatcher_ExcludedPathsDoNotResetTimer(t *testing.T) {
	timeout := 250 * time.Millisecond
	w := New(timeout, "/healthz")
	defer w.Stop()

	handler := w.Middleware(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))

	// Send requests to /healthz every 50ms for 400ms.
	// Since /healthz is excluded, the watcher should still expire around 250ms.
	stopPinging := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopPinging:
				return
			case <-time.After(50 * time.Millisecond):
				req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
			}
		}
	}()
	defer close(stopPinging)

	select {
	case <-w.Done():
		// OK: expired despite health checks
	case <-time.After(1 * time.Second):
		t.Fatal("watcher did not expire; excluded path unexpectedly reset timer")
	}
}

func TestWatcher_InFlightRequestPreventsTimeout(t *testing.T) {
	timeout := 200 * time.Millisecond
	w := New(timeout)
	defer w.Stop()

	unblockRequest := make(chan struct{})
	requestStarted := make(chan struct{})

	handler := w.Middleware(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-unblockRequest
		rw.WriteHeader(http.StatusOK)
	}))

	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule/sync", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}()

	<-requestStarted

	// Wait 300ms (> timeout 200ms) while request is still in-flight
	time.Sleep(300 * time.Millisecond)

	select {
	case <-w.Done():
		t.Fatal("watcher closed Done() while request was still in-flight")
	default:
		// OK
	}

	// Unblock request
	close(unblockRequest)

	// Now that request finished, watcher should expire after timeout
	select {
	case <-w.Done():
		// OK
	case <-time.After(1 * time.Second):
		t.Fatal("watcher did not expire after in-flight request completed")
	}
}
