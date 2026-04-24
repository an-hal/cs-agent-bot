package manualaction

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
)

type stubRepo struct {
	inserted    *entity.ManualAction
	getRes      *entity.ManualAction
	updated     *entity.ManualAction
	listRes     []entity.ManualAction
	listTotal   int64
	pastDueRes  []entity.ManualAction
	updateCalls int
}

func (s *stubRepo) Insert(ctx context.Context, m *entity.ManualAction) (*entity.ManualAction, error) {
	s.inserted = m
	out := *m
	out.ID = "maq-1"
	out.CreatedAt = time.Now()
	out.UpdatedAt = out.CreatedAt
	return &out, nil
}
func (s *stubRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ManualAction, error) {
	return s.getRes, nil
}
func (s *stubRepo) List(ctx context.Context, f entity.ManualActionFilter) ([]entity.ManualAction, int64, error) {
	return s.listRes, s.listTotal, nil
}
func (s *stubRepo) Update(ctx context.Context, m *entity.ManualAction) (*entity.ManualAction, error) {
	s.updateCalls++
	s.updated = m
	return m, nil
}
func (s *stubRepo) ListPastDue(ctx context.Context, cutoff time.Time, limit int) ([]entity.ManualAction, error) {
	return s.pastDueRes, nil
}

type notifierSpy struct {
	queuedCalls     int
	escalationCalls int
}

func (n *notifierSpy) NotifyManualQueued(_ context.Context, _ *entity.ManualAction) error {
	n.queuedCalls++
	return nil
}
func (n *notifierSpy) NotifyManualEscalation(_ context.Context, _ *entity.ManualAction) error {
	n.escalationCalls++
	return nil
}

type activitySpy struct{ calls int }

func (a *activitySpy) LogManualHumanSend(_ context.Context, _ *entity.ManualAction, _ string) error {
	a.calls++
	return nil
}

type masterDataSpy struct{ calls int }

func (m *masterDataSpy) StampSentFlag(_ context.Context, _, _, _ string) error {
	m.calls++
	return nil
}

func newUC(repo *stubRepo, opts ...func(*usecase)) Usecase {
	u := &usecase{repo: repo, logger: zerolog.Nop()}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func TestCreatePending_ValidatesAndInserts(t *testing.T) {
	repo := &stubRepo{}
	notif := &notifierSpy{}
	uc := New(repo, notif, nil, nil, zerolog.Nop())

	in := CreatePendingInput{
		WorkspaceID:    "ws-1",
		MasterDataID:   "md-1",
		TriggerID:      "AE_P4_REN90_OPENER",
		FlowCategory:   "renewal_opener",
		Role:           "ae",
		AssignedToUser: "Owner@Example.COM",
		DueAt:          time.Now().Add(24 * time.Hour),
	}
	out, err := uc.CreatePending(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.ID != "maq-1" {
		t.Errorf("expected id=maq-1, got %q", out.ID)
	}
	if repo.inserted.AssignedToUser != "owner@example.com" {
		t.Errorf("assignee not lowercased: %q", repo.inserted.AssignedToUser)
	}
	if repo.inserted.Priority != entity.ManualActionPriorityP2 {
		t.Errorf("expected default priority P2, got %q", repo.inserted.Priority)
	}
	if notif.queuedCalls != 1 {
		t.Errorf("expected 1 telegram notify, got %d", notif.queuedCalls)
	}
}

func TestCreatePending_ValidationErrors(t *testing.T) {
	uc := New(&stubRepo{}, nil, nil, nil, zerolog.Nop())
	_, err := uc.CreatePending(context.Background(), CreatePendingInput{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if ae := apperror.GetAppError(err); ae == nil {
		t.Fatalf("expected AppError, got %v", err)
	}
}

func TestMarkSent_RequiresValidChannel(t *testing.T) {
	repo := &stubRepo{
		getRes: &entity.ManualAction{
			ID:          "maq-1",
			WorkspaceID: "ws-1",
			Status:      entity.ManualActionStatusPending,
		},
	}
	uc := New(repo, nil, nil, nil, zerolog.Nop())

	_, err := uc.MarkSent(context.Background(), "ws-1", "maq-1", MarkSentRequest{
		Channel:       "fax",
		ActualMessage: "hi",
	})
	if err == nil {
		t.Fatal("expected error for invalid channel")
	}
	if !strings.Contains(err.Error(), "channel") {
		t.Errorf("expected channel error, got %v", err)
	}
}

func TestMarkSent_HappyPath_CallsSideEffects(t *testing.T) {
	repo := &stubRepo{
		getRes: &entity.ManualAction{
			ID:           "maq-1",
			WorkspaceID:  "ws-1",
			MasterDataID: "md-1",
			Status:       entity.ManualActionStatusPending,
		},
	}
	activity := &activitySpy{}
	md := &masterDataSpy{}
	uc := New(repo, nil, activity, md, zerolog.Nop())

	out, err := uc.MarkSent(context.Background(), "ws-1", "maq-1", MarkSentRequest{
		Channel:       entity.ManualActionChannelWA,
		ActualMessage: "Thanks for your time yesterday...",
		Notes:         "positive tone",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.Status != entity.ManualActionStatusSent {
		t.Errorf("expected sent status, got %q", out.Status)
	}
	if out.SentAt == nil {
		t.Error("expected SentAt to be set")
	}
	if activity.calls != 1 {
		t.Errorf("expected activity log call, got %d", activity.calls)
	}
	if md.calls != 1 {
		t.Errorf("expected master-data stamp call, got %d", md.calls)
	}
}

func TestMarkSent_RejectsTerminalStatus(t *testing.T) {
	repo := &stubRepo{
		getRes: &entity.ManualAction{
			ID:     "maq-1",
			Status: entity.ManualActionStatusSent,
		},
	}
	uc := New(repo, nil, nil, nil, zerolog.Nop())
	_, err := uc.MarkSent(context.Background(), "ws-1", "maq-1", MarkSentRequest{
		Channel:       entity.ManualActionChannelWA,
		ActualMessage: "x",
	})
	if err == nil {
		t.Fatal("expected error for terminal status")
	}
}

func TestSkip_RequiresReasonMin5Chars(t *testing.T) {
	repo := &stubRepo{
		getRes: &entity.ManualAction{Status: entity.ManualActionStatusPending},
	}
	uc := New(repo, nil, nil, nil, zerolog.Nop())
	_, err := uc.Skip(context.Background(), "ws-1", "maq-1", "nope")
	if err == nil {
		t.Fatal("expected error for short reason")
	}
}

func TestExpirePastDue_TransitionsAndEscalates(t *testing.T) {
	repo := &stubRepo{
		pastDueRes: []entity.ManualAction{
			{ID: "a", Status: entity.ManualActionStatusPending},
			{ID: "b", Status: entity.ManualActionStatusPending},
		},
	}
	notif := &notifierSpy{}
	uc := New(repo, notif, nil, nil, zerolog.Nop())
	n, err := uc.ExpirePastDue(context.Background(), 48*time.Hour)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 expired, got %d", n)
	}
	if notif.escalationCalls != 2 {
		t.Errorf("expected 2 escalations, got %d", notif.escalationCalls)
	}
}
