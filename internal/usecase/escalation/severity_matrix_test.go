package escalation

import (
	"context"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

type stubConfigRepo struct {
	val string
}

func (s *stubConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
	return map[string]string{SeverityMatrixKey: s.val}, nil
}
func (s *stubConfigRepo) GetByKey(ctx context.Context, key string) (string, error) {
	if key == SeverityMatrixKey {
		return s.val, nil
	}
	return "", nil
}
func (s *stubConfigRepo) ListAll(ctx context.Context) ([]entity.SystemConfig, error) {
	return nil, nil
}
func (s *stubConfigRepo) Upsert(ctx context.Context, key, value, updatedBy string) error {
	return nil
}

func TestSeverityMatrix_LookupExactMatch(t *testing.T) {
	repo := &stubConfigRepo{val: `{
        "default": "P2",
        "rules": {
            "ESC-003:ae": "P0",
            "ESC-001:ae": "P1"
        }
    }`}
	m := NewSeverityMatrix(repo, zerolog.Nop())

	if got := m.Lookup(context.Background(), "ESC-003", "ae"); got != "P0" {
		t.Errorf("ESC-003:ae expected P0, got %q", got)
	}
	if got := m.Lookup(context.Background(), "ESC-001", "ae"); got != "P1" {
		t.Errorf("ESC-001:ae expected P1, got %q", got)
	}
}

func TestSeverityMatrix_FallsBackToDefault(t *testing.T) {
	repo := &stubConfigRepo{val: `{"default": "P2", "rules": {}}`}
	m := NewSeverityMatrix(repo, zerolog.Nop())
	if got := m.Lookup(context.Background(), "ESC-999", "ae"); got != "P2" {
		t.Errorf("unknown ESC expected default P2, got %q", got)
	}
}

func TestSeverityMatrix_MissingConfigUsesHardDefault(t *testing.T) {
	repo := &stubConfigRepo{val: ""}
	m := NewSeverityMatrix(repo, zerolog.Nop())
	if got := m.Lookup(context.Background(), "ESC-001", "ae"); got != defaultSeverity {
		t.Errorf("missing config expected %q, got %q", defaultSeverity, got)
	}
}

func TestSeverityMatrix_MalformedJSONFallsBack(t *testing.T) {
	repo := &stubConfigRepo{val: `{"not valid`}
	m := NewSeverityMatrix(repo, zerolog.Nop())
	if got := m.Lookup(context.Background(), "ESC-001", "ae"); got != defaultSeverity {
		t.Errorf("malformed JSON expected %q, got %q", defaultSeverity, got)
	}
}

func TestSeverityMatrix_WildcardRole(t *testing.T) {
	repo := &stubConfigRepo{val: `{
        "default": "P2",
        "rules": {"ESC-005:*": "P0"}
    }`}
	m := NewSeverityMatrix(repo, zerolog.Nop())
	if got := m.Lookup(context.Background(), "ESC-005", "ae"); got != "P0" {
		t.Errorf("wildcard role expected P0, got %q", got)
	}
	if got := m.Lookup(context.Background(), "ESC-005", "lead"); got != "P0" {
		t.Errorf("wildcard role lead expected P0, got %q", got)
	}
}
