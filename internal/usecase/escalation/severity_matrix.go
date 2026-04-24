package escalation

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// SeverityMatrixKey is the system_config key that stores the per-role × per-esc
// severity tiers. Stored as JSON to keep the config flexible. Example shape:
//
//   {
//     "default": "P2",
//     "rules": {
//       "ESC-001:ae":    "P1",
//       "ESC-002:ae":    "P1",
//       "ESC-003:ae":    "P0",
//       "ESC-004:ae":    "P1",
//       "ESC-005:lead":  "P0",
//       "ESC-006:ae":    "P0"
//     }
//   }
const SeverityMatrixKey = "ESCALATION_SEVERITY_MATRIX"

// defaultSeverity is used when the key is absent or malformed.
const defaultSeverity = "P2"

// SeverityMatrix is a thin, cached reader over system_config. Not safe for
// concurrent writes; safe for concurrent reads.
type SeverityMatrix struct {
	configRepo repository.SystemConfigRepository
	logger     zerolog.Logger

	mu         sync.RWMutex
	loadedAt   time.Time
	cache      map[string]string
	defSev     string
	cacheTTL   time.Duration
}

// NewSeverityMatrix constructs a reader. The cache TTL defaults to 60s — short
// enough that admins see config edits quickly, long enough to avoid hammering
// the config table from the cron loop.
func NewSeverityMatrix(configRepo repository.SystemConfigRepository, logger zerolog.Logger) *SeverityMatrix {
	return &SeverityMatrix{
		configRepo: configRepo,
		logger:     logger,
		cacheTTL:   60 * time.Second,
	}
}

// Lookup returns the severity (P0/P1/P2) for an (escID, role) pair. Role should
// be one of "ae" | "bd" | "sdr" | "lead" | "admin". Returns the configured
// default when no exact match is found.
func (s *SeverityMatrix) Lookup(ctx context.Context, escID, role string) string {
	s.ensureFresh(ctx)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if v, ok := s.cache[escID+":"+role]; ok {
		return v
	}
	if v, ok := s.cache[escID+":*"]; ok {
		return v
	}
	if v, ok := s.cache["*:"+role]; ok {
		return v
	}
	if s.defSev != "" {
		return s.defSev
	}
	return defaultSeverity
}

func (s *SeverityMatrix) ensureFresh(ctx context.Context) {
	s.mu.RLock()
	fresh := !s.loadedAt.IsZero() && time.Since(s.loadedAt) < s.cacheTTL
	s.mu.RUnlock()
	if fresh {
		return
	}
	s.refresh(ctx)
}

func (s *SeverityMatrix) refresh(ctx context.Context) {
	raw, err := s.configRepo.GetByKey(ctx, SeverityMatrixKey)
	if err != nil {
		s.logger.Warn().Err(err).Str("key", SeverityMatrixKey).
			Msg("severity matrix config read failed — using default")
		s.mu.Lock()
		s.cache = map[string]string{}
		s.defSev = defaultSeverity
		s.loadedAt = time.Now()
		s.mu.Unlock()
		return
	}
	if raw == "" {
		s.mu.Lock()
		s.cache = map[string]string{}
		s.defSev = defaultSeverity
		s.loadedAt = time.Now()
		s.mu.Unlock()
		return
	}

	var parsed struct {
		Default string            `json:"default"`
		Rules   map[string]string `json:"rules"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		s.logger.Warn().Err(err).Msg("severity matrix config is not valid JSON — using default")
		s.mu.Lock()
		s.cache = map[string]string{}
		s.defSev = defaultSeverity
		s.loadedAt = time.Now()
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	s.cache = parsed.Rules
	if s.cache == nil {
		s.cache = map[string]string{}
	}
	if parsed.Default != "" {
		s.defSev = parsed.Default
	} else {
		s.defSev = defaultSeverity
	}
	s.loadedAt = time.Now()
	s.mu.Unlock()
}

// Invalidate forces a reload on the next Lookup. Call this after a system_config
// write for the key so downstream callers pick up the new matrix immediately.
func (s *SeverityMatrix) Invalidate() {
	s.mu.Lock()
	s.loadedAt = time.Time{}
	s.mu.Unlock()
}
