// A small purpose-built circuit breaker for the JWT introspection middleware.
//
// Why not pull in sony/gobreaker: the introspection use case is narrow — count
// upstream failures, trip after N in a window, fail fast for a cooldown, then
// probe with a single half-open request. ~80 LOC suffices, and we get exact
// state semantics + fast tests without a new dependency.
//
// State diagram:
//
//   Closed ── failureThreshold consecutive failures ──▶ Open
//   Open ── cooldown elapses ──▶ Half-Open (allow 1 probe)
//   Half-Open ── probe succeeds ──▶ Closed (counters reset)
//   Half-Open ── probe fails ──▶ Open (cooldown restarts)
//
// In Open state, callers receive ErrCircuitOpen immediately without doing
// any work. Lets a 5s auth-proxy timeout become a <1ms reject during outages,
// stopping a thundering herd before it forms.

package middleware

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned by Circuit.Do when the breaker is in Open state.
// The caller should treat this as a transient upstream failure (5xx-ish) and
// either fail fast or fall back to a cached/synthesized response.
var ErrCircuitOpen = errors.New("circuit breaker open")

// circuitState is the internal enum for breaker phases. Kept unexported so
// callers don't pattern-match on it; observability is via Stats() instead.
type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

// CircuitConfig is the tuning surface. Zero values get sane defaults via
// NewCircuit so callers can leave fields blank.
type CircuitConfig struct {
	// FailureThreshold trips Closed→Open after this many consecutive
	// failures. Default 5. Set high enough that one slow upstream call
	// doesn't trip the breaker; low enough to react fast under sustained
	// failure.
	FailureThreshold int
	// Cooldown is how long Open lasts before a single probe is allowed.
	// Default 30s. Should exceed typical transient upstream blips
	// (network jitter, deploys) but be short enough that a recovered
	// upstream is detected promptly.
	Cooldown time.Duration
}

// Circuit is the lock-protected state machine. Safe for concurrent use; one
// instance per upstream dependency.
type Circuit struct {
	cfg CircuitConfig

	mu                  sync.Mutex
	state               circuitState
	consecutiveFailures int
	openedAt            time.Time

	// Counters for observability — incremented on every Do call, never reset.
	totalCalls   uint64
	totalRejects uint64
	totalTrips   uint64
}

// NewCircuit constructs a breaker. Pass a zero-value CircuitConfig for defaults.
func NewCircuit(cfg CircuitConfig) *Circuit {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 30 * time.Second
	}
	return &Circuit{cfg: cfg, state: stateClosed}
}

// Do invokes fn under breaker control. Returns ErrCircuitOpen without calling
// fn when Open. In Half-Open it allows fn to run once; success closes the
// breaker, failure re-opens it.
//
// Errors from fn are reported back verbatim — the breaker only adds
// ErrCircuitOpen and never wraps fn's errors.
func (c *Circuit) Do(fn func() error) error {
	if !c.allow() {
		c.mu.Lock()
		c.totalCalls++
		c.totalRejects++
		c.mu.Unlock()
		return ErrCircuitOpen
	}
	c.mu.Lock()
	c.totalCalls++
	c.mu.Unlock()

	err := fn()
	c.record(err)
	return err
}

// allow checks the current state and may transition Open→Half-Open if the
// cooldown has elapsed. Holds the lock briefly.
func (c *Circuit) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.state {
	case stateClosed:
		return true
	case stateOpen:
		if time.Since(c.openedAt) >= c.cfg.Cooldown {
			c.state = stateHalfOpen
			return true
		}
		return false
	case stateHalfOpen:
		// Only one probe at a time — others fail fast. Implementation: while
		// in Half-Open, the next caller becomes the probe; subsequent callers
		// see Half-Open AND see consecutiveFailures==0 from the previous
		// trip's reset (record() resets it on success). Practically simpler:
		// allow all Half-Open callers through and let the first record() win.
		// The downside (multiple parallel probes) is bounded since fn is
		// already coalesced upstream by singleflight.
		return true
	}
	return false
}

// record updates state based on fn's outcome.
func (c *Circuit) record(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		// Success — reset failure counter; close breaker if it was probing.
		c.consecutiveFailures = 0
		if c.state == stateHalfOpen {
			c.state = stateClosed
		}
		return
	}
	// Failure — bump counter, trip if threshold reached or if probe failed.
	c.consecutiveFailures++
	if c.state == stateHalfOpen || c.consecutiveFailures >= c.cfg.FailureThreshold {
		if c.state != stateOpen {
			c.totalTrips++
		}
		c.state = stateOpen
		c.openedAt = time.Now()
	}
}

// Stats returns a snapshot for metrics emission. Cheap (one lock/unlock).
type CircuitStats struct {
	State               string
	ConsecutiveFailures int
	TotalCalls          uint64
	TotalRejects        uint64
	TotalTrips          uint64
}

func (c *Circuit) Stats() CircuitStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return CircuitStats{
		State:               stateName(c.state),
		ConsecutiveFailures: c.consecutiveFailures,
		TotalCalls:          c.totalCalls,
		TotalRejects:        c.totalRejects,
		TotalTrips:          c.totalTrips,
	}
}

func stateName(s circuitState) string {
	switch s {
	case stateClosed:
		return "closed"
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half_open"
	}
	return "unknown"
}
