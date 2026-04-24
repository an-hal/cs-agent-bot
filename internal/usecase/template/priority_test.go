package template

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

type stubResolver struct {
	// responses keyed by templateID — value is (body, err)
	results map[string]struct {
		body string
		err  error
	}
}

func (s *stubResolver) ResolveTemplate(ctx context.Context, templateID string, client entity.Client, invoice *entity.Invoice, cfg TemplateConfig) (string, error) {
	if r, ok := s.results[templateID]; ok {
		return r.body, r.err
	}
	return "", errors.New("template not found: " + templateID)
}
func (s *stubResolver) ResolveEscalationTemplate(ctx context.Context, escID string, client entity.Client, esc entity.Escalation, extra map[string]string) (string, error) {
	return "", nil
}

func TestResolvePriority_FirstMatchWins(t *testing.T) {
	s := &stubResolver{results: map[string]struct {
		body string
		err  error
	}{
		"renewal_v2": {body: "from renewal_v2", err: nil},
		"default":    {body: "from default", err: nil},
	}}
	body, id, err := ResolvePriority(context.Background(), s,
		[]string{"missing", "renewal_v2", "default"},
		entity.Client{}, nil, TemplateConfig{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if body != "from renewal_v2" || id != "renewal_v2" {
		t.Errorf("expected renewal_v2 body, got %q (id=%q)", body, id)
	}
}

func TestResolvePriority_FallsThroughToLast(t *testing.T) {
	s := &stubResolver{results: map[string]struct {
		body string
		err  error
	}{
		"default": {body: "from default", err: nil},
	}}
	body, id, err := ResolvePriority(context.Background(), s,
		[]string{"renewal_v2", "intent_v1", "legacy", "default"},
		entity.Client{}, nil, TemplateConfig{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if id != "default" {
		t.Errorf("expected default, got %q", id)
	}
	if body != "from default" {
		t.Errorf("expected default body, got %q", body)
	}
}

func TestResolvePriority_AllFailReturnsLastError(t *testing.T) {
	s := &stubResolver{results: map[string]struct {
		body string
		err  error
	}{}}
	_, _, err := ResolvePriority(context.Background(), s,
		[]string{"a", "b", "c"},
		entity.Client{}, nil, TemplateConfig{})
	if err == nil {
		t.Fatal("expected error when all candidates fail")
	}
}

func TestResolvePriority_EmptyCandidatesRejected(t *testing.T) {
	_, _, err := ResolvePriority(context.Background(), &stubResolver{}, nil, entity.Client{}, nil, TemplateConfig{})
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
}

func TestResolvePriority_SkipsEmptyStringCandidates(t *testing.T) {
	s := &stubResolver{results: map[string]struct {
		body string
		err  error
	}{
		"real": {body: "from real", err: nil},
	}}
	_, id, err := ResolvePriority(context.Background(), s,
		[]string{"", "", "real"},
		entity.Client{}, nil, TemplateConfig{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if id != "real" {
		t.Errorf("expected real, got %q", id)
	}
}
