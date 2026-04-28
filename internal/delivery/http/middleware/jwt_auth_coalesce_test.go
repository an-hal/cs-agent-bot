package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestJWTAuth_CoalescesConcurrentMisses fires N concurrent requests with the
// SAME token against a fake auth proxy and asserts the proxy was hit exactly
// once. Without singleflight this would be N calls; with it, the first call
// populates the cache and the rest piggyback.
func TestJWTAuth_CoalescesConcurrentMisses(t *testing.T) {
	const N = 50
	var upstreamHits int32

	// Slow fake auth proxy — the slowness widens the race window so a
	// non-coalesced implementation reliably fans out to N hits.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","message":"ok","data":{"user_id":42,"email":"u@example.com","platform":"test","expires_at":"2099-01-01T00:00:00Z"}}`))
	}))
	defer upstream.Close()

	mw := JWTAuthMiddleware(upstream.URL, "production", false, zerolog.Nop())
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	var wg sync.WaitGroup
	wg.Add(N)
	startGate := make(chan struct{})
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			<-startGate
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set("Authorization", "Bearer test-token-coalesced")
			rec := httptest.NewRecorder()
			if err := handler(rec, req); err != nil {
				t.Errorf("handler returned error: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		}()
	}
	close(startGate)
	wg.Wait()

	hits := atomic.LoadInt32(&upstreamHits)
	if hits != 1 {
		t.Fatalf("expected exactly 1 upstream hit (singleflight), got %d (%d concurrent requests)", hits, N)
	}
	t.Logf("OK: %d concurrent requests → %d upstream call (coalesced)", N, hits)
}

// TestJWTAuth_CoalescesNegativeOutcome verifies that a 401 from the proxy is
// also coalesced — concurrent invalid-token requests should hit the proxy
// once, not N times.
func TestJWTAuth_CoalescesNegativeOutcome(t *testing.T) {
	const N = 30
	var upstreamHits int32

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		time.Sleep(40 * time.Millisecond)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"error","message":"token expired"}`))
	}))
	defer upstream.Close()

	mw := JWTAuthMiddleware(upstream.URL, "production", false, zerolog.Nop())
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	var wg sync.WaitGroup
	wg.Add(N)
	startGate := make(chan struct{})
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			<-startGate
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set("Authorization", "Bearer expired-token")
			rec := httptest.NewRecorder()
			_ = handler(rec, req)
		}()
	}
	close(startGate)
	wg.Wait()

	hits := atomic.LoadInt32(&upstreamHits)
	if hits != 1 {
		t.Fatalf("expected 1 upstream hit for negative outcome, got %d", hits)
	}
}

// TestJWTAuth_CacheServesAfterFirstCall verifies the cache works:
// the second call (well after the first completes) hits the cache, not the proxy.
func TestJWTAuth_CacheServesAfterFirstCall(t *testing.T) {
	var upstreamHits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","message":"ok","data":{"user_id":42,"email":"u@example.com","platform":"test","expires_at":"2099-01-01T00:00:00Z"}}`))
	}))
	defer upstream.Close()

	mw := JWTAuthMiddleware(upstream.URL, "production", false, zerolog.Nop())
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer cached-token")
		rec := httptest.NewRecorder()
		if err := handler(rec, req); err != nil {
			t.Fatalf("handler error: %v", err)
		}
	}
	hits := atomic.LoadInt32(&upstreamHits)
	if hits != 1 {
		t.Fatalf("expected 1 upstream hit (cached), got %d", hits)
	}
}

// TestJWTAuth_DifferentTokensNotCoalesced — sanity check that singleflight
// keys correctly: different tokens must NOT be merged.
func TestJWTAuth_DifferentTokensNotCoalesced(t *testing.T) {
	var upstreamHits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","message":"ok","data":{"user_id":42,"email":"u@example.com","platform":"test","expires_at":"2099-01-01T00:00:00Z"}}`))
	}))
	defer upstream.Close()

	mw := JWTAuthMiddleware(upstream.URL, "production", false, zerolog.Nop())
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	for i, tok := range []string{"token-A", "token-B", "token-C"} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		if err := handler(rec, req); err != nil {
			t.Fatalf("handler error on token %d: %v", i, err)
		}
	}
	hits := atomic.LoadInt32(&upstreamHits)
	if hits != 3 {
		t.Fatalf("expected 3 upstream hits (one per distinct token), got %d", hits)
	}
}
