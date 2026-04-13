package auth_test

import (
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
)

func TestRateLimiter_AllowsWithinBudget(t *testing.T) {
	t.Parallel()
	lim := auth.NewRateLimiter(time.Minute, 3)

	for i := 0; i < 3; i++ {
		ok, remaining, _ := lim.Allow("user-1")
		if !ok {
			t.Fatalf("call %d should be allowed", i)
		}
		if remaining != 3-i-1 {
			t.Errorf("remaining: got %d want %d", remaining, 3-i-1)
		}
	}
}

func TestRateLimiter_BlocksWhenExceeded(t *testing.T) {
	t.Parallel()
	lim := auth.NewRateLimiter(time.Minute, 2)

	_, _, _ = lim.Allow("k")
	_, _, _ = lim.Allow("k")
	ok, remaining, reset := lim.Allow("k")
	if ok {
		t.Fatal("third call should be blocked")
	}
	if remaining != 0 {
		t.Errorf("remaining: got %d want 0", remaining)
	}
	if reset == 0 {
		t.Errorf("reset should be non-zero")
	}
}

func TestRateLimiter_PerKeyIsolation(t *testing.T) {
	t.Parallel()
	lim := auth.NewRateLimiter(time.Minute, 1)

	if ok, _, _ := lim.Allow("a"); !ok {
		t.Fatal("a:1 should be allowed")
	}
	if ok, _, _ := lim.Allow("b"); !ok {
		t.Fatal("b:1 should be allowed (separate key)")
	}
	if ok, _, _ := lim.Allow("a"); ok {
		t.Fatal("a:2 should be blocked")
	}
}

func TestRateLimiter_LimitGetter(t *testing.T) {
	t.Parallel()
	lim := auth.NewRateLimiter(time.Second, 7)
	if lim.Limit() != 7 {
		t.Errorf("Limit(): got %d want 7", lim.Limit())
	}
}
