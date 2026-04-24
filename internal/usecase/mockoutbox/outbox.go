// Package mockoutbox is a tiny in-memory, thread-safe ring buffer that mock
// external-API clients use to record what they would have "sent". FE/QA can
// then inspect the outbox via HTTP endpoints instead of hitting real providers.
//
// Intentionally process-local — no persistence, no cross-pod replication.
// Rebooting the server clears every outbox.
package mockoutbox

import (
	"sort"
	"sync"
	"time"
)

// Provider tags — also used as the URL segment in /mock/outbox/{provider}.
const (
	ProviderClaude    = "claude"
	ProviderFireflies = "fireflies"
	ProviderHaloAI    = "haloai"
	ProviderSMTP      = "smtp"
)

// Record is one captured call. Payload/Response are free-form maps so each
// provider can record whatever is useful — FE only reads via JSON.
type Record struct {
	ID        string         `json:"id"`
	Provider  string         `json:"provider"`
	Operation string         `json:"operation"`
	Payload   map[string]any `json:"payload"`
	Response  map[string]any `json:"response,omitempty"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// Outbox is the thread-safe ring buffer. Defaults to 100 records per provider
// so memory stays bounded even on long-running dev sessions.
type Outbox struct {
	mu       sync.RWMutex
	records  []Record
	capacity int
	nextID   int64
}

// New returns an Outbox. capacity <= 0 defaults to 100.
func New(capacity int) *Outbox {
	if capacity <= 0 {
		capacity = 100
	}
	return &Outbox{capacity: capacity}
}

// Record appends a call record, evicting the oldest when the buffer is full.
// Returns the assigned Record (with ID + Timestamp populated) so callers can
// log + correlate.
func (o *Outbox) Record(provider, operation string, payload, response map[string]any, status, errMsg string) Record {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.nextID++
	r := Record{
		ID:        mockIDPrefix(provider) + "-" + itoa(o.nextID),
		Provider:  provider,
		Operation: operation,
		Payload:   copyMap(payload),
		Response:  copyMap(response),
		Status:    status,
		Error:     errMsg,
		Timestamp: time.Now().UTC(),
	}
	if len(o.records) >= o.capacity {
		// Evict oldest (head).
		o.records = append(o.records[1:], r)
	} else {
		o.records = append(o.records, r)
	}
	return r
}

// List returns the most-recent-first slice. When provider is empty, returns
// records across all providers. limit <= 0 returns everything.
func (o *Outbox) List(provider string, limit int) []Record {
	o.mu.RLock()
	defer o.mu.RUnlock()

	filtered := make([]Record, 0, len(o.records))
	for _, r := range o.records {
		if provider == "" || r.Provider == provider {
			filtered = append(filtered, r)
		}
	}
	// Newest first.
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Timestamp.After(filtered[j].Timestamp) })
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// Get returns a single record by ID.
func (o *Outbox) Get(id string) (Record, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	for _, r := range o.records {
		if r.ID == id {
			return r, true
		}
	}
	return Record{}, false
}

// Clear removes all records for one provider; pass empty to clear everything.
func (o *Outbox) Clear(provider string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	if provider == "" {
		n := len(o.records)
		o.records = nil
		return n
	}
	var kept []Record
	n := 0
	for _, r := range o.records {
		if r.Provider == provider {
			n++
			continue
		}
		kept = append(kept, r)
	}
	o.records = kept
	return n
}

// Stats returns per-provider counts.
func (o *Outbox) Stats() map[string]int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := map[string]int{}
	for _, r := range o.records {
		out[r.Provider]++
	}
	return out
}

func mockIDPrefix(p string) string {
	switch p {
	case ProviderClaude:
		return "mock-claude"
	case ProviderFireflies:
		return "mock-ff"
	case ProviderHaloAI:
		return "mock-wa"
	case ProviderSMTP:
		return "mock-smtp"
	}
	return "mock"
}

func itoa(n int64) string {
	// Small inline conversion — avoids importing strconv into a tiny package.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
