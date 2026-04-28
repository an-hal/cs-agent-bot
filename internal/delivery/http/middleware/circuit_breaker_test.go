package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// ── Unit tests for the breaker primitive ────────────────────────────

func TestCircuit_OpensAfterThresholdFailures(t *testing.T) {
	c := NewCircuit(CircuitConfig{FailureThreshold: 3, Cooldown: 100 * time.Millisecond})

	// Below threshold — breaker stays Closed.
	for i := 0; i < 2; i++ {
		err := c.Do(func() error { return errors.New("upstream down") })
		if errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("breaker tripped early at failure %d", i+1)
		}
	}
	if got := c.Stats().State; got != "closed" {
		t.Fatalf("expected closed, got %q", got)
	}

	// Third failure trips the breaker.
	_ = c.Do(func() error { return errors.New("upstream down") })
	if got := c.Stats().State; got != "open" {
		t.Fatalf("expected open after threshold reached, got %q", got)
	}

	// Subsequent calls fail fast WITHOUT invoking fn.
	var invoked int32
	err := c.Do(func() error { atomic.AddInt32(&invoked, 1); return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if invoked != 0 {
		t.Fatalf("fn was invoked while breaker open (invocations=%d)", invoked)
	}
}

func TestCircuit_RecoversAfterCooldown(t *testing.T) {
	c := NewCircuit(CircuitConfig{FailureThreshold: 2, Cooldown: 50 * time.Millisecond})
	// Trip the breaker.
	_ = c.Do(func() error { return errors.New("x") })
	_ = c.Do(func() error { return errors.New("x") })
	if c.Stats().State != "open" {
		t.Fatalf("expected open")
	}

	// Wait for cooldown + a probe success → breaker closes.
	time.Sleep(60 * time.Millisecond)
	err := c.Do(func() error { return nil })
	if err != nil {
		t.Fatalf("probe call returned %v", err)
	}
	if c.Stats().State != "closed" {
		t.Fatalf("expected closed after successful probe, got %q", c.Stats().State)
	}
}

func TestCircuit_ProbeFailureReopens(t *testing.T) {
	c := NewCircuit(CircuitConfig{FailureThreshold: 2, Cooldown: 50 * time.Millisecond})
	_ = c.Do(func() error { return errors.New("x") })
	_ = c.Do(func() error { return errors.New("x") })
	time.Sleep(60 * time.Millisecond)

	// Probe fails → breaker re-opens immediately.
	_ = c.Do(func() error { return errors.New("still down") })
	if c.Stats().State != "open" {
		t.Fatalf("expected open after failed probe, got %q", c.Stats().State)
	}
}

func TestCircuit_SuccessResetsFailureCounter(t *testing.T) {
	c := NewCircuit(CircuitConfig{FailureThreshold: 3, Cooldown: time.Second})
	_ = c.Do(func() error { return errors.New("x") })
	_ = c.Do(func() error { return errors.New("x") })
	if c.Stats().ConsecutiveFailures != 2 {
		t.Fatalf("expected 2 failures, got %d", c.Stats().ConsecutiveFailures)
	}
	_ = c.Do(func() error { return nil })
	if c.Stats().ConsecutiveFailures != 0 {
		t.Fatalf("success should reset counter, got %d", c.Stats().ConsecutiveFailures)
	}
	if c.Stats().State != "closed" {
		t.Fatalf("expected closed")
	}
}

// ── Integration with JWTAuthMiddleware ──────────────────────────────

// TestJWTAuth_TokenRejectionsDoNotTripBreaker verifies the critical guard:
// a flood of users with expired tokens (legitimate 401s from upstream)
// must NOT trip the breaker. Otherwise a few stale browsers would lock
// out the entire API.
//
// Verification strategy: count upstream hits. With breaker correctly
// distinguishing rejection vs failure, all 10 calls reach the upstream.
// If the breaker counted rejections as failures, only 5 (= FailureThreshold)
// would reach the upstream and the rest would fail-fast.
func TestJWTAuth_TokenRejectionsDoNotTripBreaker(t *testing.T) {
	var upstreamHits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":"error","message":"token expired"}`))
	}))
	defer upstream.Close()

	mw := JWTAuthMiddleware(upstream.URL, "production", false, zerolog.Nop())
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	// Each iteration uses a distinct token so the cache doesn't short-circuit.
	const N = 10
	for i := 0; i < N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer expired-"+itoa(i))
		rec := httptest.NewRecorder()
		err := handler(rec, req)
		// Middleware returns an error (not a status code via the recorder).
		if err == nil {
			t.Fatalf("token %d: expected auth error, got nil", i)
		}
	}
	hits := atomic.LoadInt32(&upstreamHits)
	if hits != N {
		t.Fatalf("expected all %d requests to reach upstream (breaker stays closed); only %d did", N, hits)
	}
}

// TestJWTAuth_UpstreamDownTripsBreaker confirms the inverse: when the
// upstream returns 5xx (or network errors) the breaker trips after
// FailureThreshold and subsequent calls fail fast WITHOUT hitting upstream.
func TestJWTAuth_UpstreamDownTripsBreaker(t *testing.T) {
	var upstreamHits int32
	// 5xx on every call simulates a broken upstream. Slight sleep so that
	// "fast-fail" after trip is unambiguously faster than "actually called".
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error","message":"down"}`))
	}))
	defer upstream.Close()

	mw := JWTAuthMiddleware(upstream.URL, "production", false, zerolog.Nop())
	handler := mw(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	// Fire 5 distinct tokens — exactly the FailureThreshold. Breaker should
	// trip on the 5th. Subsequent calls return ErrCircuitOpen → handler
	// surfaces as Unauthorized but DOES NOT touch upstream.
	const trip = 5
	for i := 0; i < trip; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer trip-"+itoa(i))
		_ = handler(httptest.NewRecorder(), req)
	}
	beforeProbe := atomic.LoadInt32(&upstreamHits)
	if beforeProbe != trip {
		t.Fatalf("expected %d upstream hits while tripping, got %d", trip, beforeProbe)
	}

	// Post-trip: 5 more calls — none should reach upstream.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer post-"+itoa(i))
		err := handler(httptest.NewRecorder(), req)
		if err == nil {
			t.Fatalf("expected error after breaker open, got nil")
		}
	}
	afterProbe := atomic.LoadInt32(&upstreamHits)
	if afterProbe != trip {
		t.Fatalf("breaker should have prevented upstream calls; got %d (was %d)", afterProbe, trip)
	}
}

// itoa: tiny stdlib wrapper to avoid importing "strconv" just for one call.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	const digits = "0123456789"
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = digits[i%10]
		i /= 10
	}
	return string(b[pos:])
}
