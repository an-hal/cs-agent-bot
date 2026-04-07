package dashboard_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// noopTracer wraps the OpenTelemetry noop tracer provider so it satisfies
// the internal tracer.Tracer interface.
type noopTracer struct {
	t trace.Tracer
}

func newNoopTracer() noopTracer {
	return noopTracer{t: noop.NewTracerProvider().Tracer("test")}
}

func (n noopTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return n.t.Start(ctx, spanName, opts...)
}

func (n noopTracer) Shutdown(_ context.Context) error { return nil }

// ─── mock LogRepository ───────────────────────────────────────────────────────

type mockLogRepo struct {
	repository.LogRepository // embed so only overridden methods need implementing

	appendActivityCalled bool
	appendActivityEntry  entity.ActivityLog
	appendActivityErr    error

	getActivitiesResult []entity.ActivityLog
	getActivitiesTotal  int
	getActivitiesFilter entity.ActivityFilter
	getActivitiesErr    error
}

func (m *mockLogRepo) AppendActivity(_ context.Context, entry entity.ActivityLog) error {
	m.appendActivityCalled = true
	m.appendActivityEntry = entry
	return m.appendActivityErr
}

func (m *mockLogRepo) GetActivities(_ context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	m.getActivitiesFilter = filter
	return m.getActivitiesResult, m.getActivitiesTotal, m.getActivitiesErr
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func newUsecase(logRepo repository.LogRepository) dashboard.DashboardUsecase {
	return dashboard.NewDashboardUsecase(
		nil, nil, nil, nil, // unused repos for these tests
		logRepo,
		newNoopTracer(),
		zerolog.Nop(),
	)
}

// ─── RecordActivity ───────────────────────────────────────────────────────────

func TestRecordActivity_DelegatesToRepo(t *testing.T) {
	mock := &mockLogRepo{}
	uc := newUsecase(mock)

	entry := entity.ActivityLog{
		Category:  entity.ActivityCategoryData,
		ActorType: entity.ActivityActorHuman,
		Actor:     "user@example.com",
		Action:    "edit_client",
		Target:    "PT Maju Digital",
		RefID:     "C01",
	}

	if err := uc.RecordActivity(context.Background(), entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mock.appendActivityCalled {
		t.Fatal("expected AppendActivity to be called")
	}
	if mock.appendActivityEntry.Actor != "user@example.com" {
		t.Errorf("actor mismatch: got %q", mock.appendActivityEntry.Actor)
	}
	if mock.appendActivityEntry.Action != "edit_client" {
		t.Errorf("action mismatch: got %q", mock.appendActivityEntry.Action)
	}
}

func TestRecordActivity_PropagatesRepoError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	mock := &mockLogRepo{appendActivityErr: repoErr}
	uc := newUsecase(mock)

	err := uc.RecordActivity(context.Background(), entity.ActivityLog{Action: "add_client"})
	if !errors.Is(err, repoErr) {
		t.Errorf("expected repo error, got: %v", err)
	}
}

// ─── GetActivityLogs ──────────────────────────────────────────────────────────

func TestGetActivityLogs_FilterPassedToRepo(t *testing.T) {
	now := time.Now()
	mock := &mockLogRepo{
		getActivitiesResult: []entity.ActivityLog{
			{ID: 1, Category: "bot", Action: "RENEWAL"},
		},
		getActivitiesTotal: 1,
	}
	uc := newUsecase(mock)

	filter := entity.ActivityFilter{
		WorkspaceID: "dealls",
		Category:    entity.ActivityCategoryBot,
		Since:       &now,
		Limit:       25,
		Offset:      10,
	}

	logs, total, err := uc.GetActivityLogs(context.Background(), filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Action != "RENEWAL" {
		t.Errorf("unexpected action: %q", logs[0].Action)
	}

	// Verify filter was forwarded unchanged
	if mock.getActivitiesFilter.WorkspaceID != "dealls" {
		t.Errorf("workspace_id not forwarded: %q", mock.getActivitiesFilter.WorkspaceID)
	}
	if mock.getActivitiesFilter.Limit != 25 {
		t.Errorf("limit not forwarded: %d", mock.getActivitiesFilter.Limit)
	}
	if mock.getActivitiesFilter.Offset != 10 {
		t.Errorf("offset not forwarded: %d", mock.getActivitiesFilter.Offset)
	}
}

func TestGetActivityLogs_PropagatesRepoError(t *testing.T) {
	repoErr := errors.New("query failed")
	mock := &mockLogRepo{getActivitiesErr: repoErr}
	uc := newUsecase(mock)

	_, _, err := uc.GetActivityLogs(context.Background(), entity.ActivityFilter{Limit: 10})
	if !errors.Is(err, repoErr) {
		t.Errorf("expected repo error, got: %v", err)
	}
}
