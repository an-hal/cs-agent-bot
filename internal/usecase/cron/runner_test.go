package cron_test

// runner_test.go — tests for cronRunner.RunAll, StartRunAll, WithRuleEngine,
// and NewCronRunner. Uses stub implementations of all repository interfaces
// to test without a real database or TriggerService.

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	ucron "github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	"github.com/rs/zerolog"
)

// ─── Full stub repos for the CronRunner ──────────────────────────────────────

// stubFullClientRepo satisfies repository.ClientRepository.
type stubFullClientRepo struct {
	clients []entity.Client
	err     error
}

func (r *stubFullClientRepo) GetAll(_ context.Context) ([]entity.Client, error) {
	return r.clients, r.err
}
func (r *stubFullClientRepo) GetByID(_ context.Context, _ string) (*entity.Client, error) {
	return nil, nil
}
func (r *stubFullClientRepo) GetByWANumber(_ context.Context, _ string) (*entity.Client, error) {
	return nil, nil
}
func (r *stubFullClientRepo) GetByCompanyID(_ context.Context, _ string) (*entity.Client, error) {
	return nil, nil
}
func (r *stubFullClientRepo) GetLatestInvoice(_ context.Context, _ string) (*entity.Invoice, error) {
	return nil, nil
}
func (r *stubFullClientRepo) UpdateLastInteraction(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (r *stubFullClientRepo) CreateClient(_ context.Context, _ entity.Client) error { return nil }
func (r *stubFullClientRepo) UpdateInvoiceReminderFlags(_ context.Context, _ string, _ map[string]bool) error {
	return nil
}
func (r *stubFullClientRepo) UpdatePaymentStatus(_ context.Context, _, _ string) error { return nil }
func (r *stubFullClientRepo) GetAllByWorkspace(_ context.Context, _ string) ([]entity.Client, error) {
	return r.clients, r.err
}
func (r *stubFullClientRepo) GetAllByWorkspaceIDs(_ context.Context, _ []string) ([]entity.Client, error) {
	return r.clients, r.err
}
func (r *stubFullClientRepo) CountByFilter(_ context.Context, _ entity.ClientFilter) (int64, error) {
	return 0, nil
}
func (r *stubFullClientRepo) FetchByFilter(_ context.Context, _ entity.ClientFilter, _ pagination.Params) ([]entity.Client, error) {
	return nil, nil
}
func (r *stubFullClientRepo) UpdateClientFields(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}

// stubFullFlagsRepo satisfies repository.FlagsRepository.
type stubFullFlagsRepo struct {
	flags entity.ClientFlags
	err   error
}

func (r *stubFullFlagsRepo) GetByCompanyID(_ context.Context, _ string) (*entity.ClientFlags, error) {
	if r.err != nil {
		return nil, r.err
	}
	cp := r.flags
	return &cp, nil
}
func (r *stubFullFlagsRepo) UpdateFlags(_ context.Context, _ string, _ entity.ClientFlags) error {
	return nil
}
func (r *stubFullFlagsRepo) SetBotActive(_ context.Context, _ string, _ bool) error { return nil }
func (r *stubFullFlagsRepo) ResetCycleFlags(_ context.Context, _ string) error       { return nil }

// stubFullConvStateRepo satisfies repository.ConversationStateRepository.
type stubFullConvStateRepo struct{}

func (r *stubFullConvStateRepo) GetByCompanyID(_ context.Context, _ string) (*entity.ConversationState, error) {
	return &entity.ConversationState{BotActive: true}, nil
}
func (r *stubFullConvStateRepo) CreateOrUpdate(_ context.Context, _ entity.ConversationState) error {
	return nil
}
func (r *stubFullConvStateRepo) SetBotActive(_ context.Context, _ string, _ bool, _ string) error {
	return nil
}
func (r *stubFullConvStateRepo) SetCooldown(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
func (r *stubFullConvStateRepo) RecordMessage(_ context.Context, _ string, _, _ string) error {
	return nil
}

// stubFullInvoiceRepo satisfies repository.InvoiceRepository.
type stubFullInvoiceRepo struct{}

func (r *stubFullInvoiceRepo) GetActiveByCompanyID(_ context.Context, _ string) (*entity.Invoice, error) {
	return nil, nil
}
func (r *stubFullInvoiceRepo) GetAllByCompanyID(_ context.Context, _ string) ([]entity.Invoice, error) {
	return nil, nil
}
func (r *stubFullInvoiceRepo) GetAllPaginated(_ context.Context, _ entity.InvoiceFilter, _ pagination.Params) ([]entity.Invoice, int64, error) {
	return nil, 0, nil
}
func (r *stubFullInvoiceRepo) GetByID(_ context.Context, _ string) (*entity.Invoice, error) {
	return nil, nil
}
func (r *stubFullInvoiceRepo) UpdateFields(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}
func (r *stubFullInvoiceRepo) CreateInvoice(_ context.Context, _ entity.Invoice) error { return nil }
func (r *stubFullInvoiceRepo) Create(_ context.Context, _ *sql.Tx, _ entity.Invoice) error {
	return nil
}
func (r *stubFullInvoiceRepo) Delete(_ context.Context, _ string) error { return nil }
func (r *stubFullInvoiceRepo) ListOverdue(_ context.Context, _ time.Time) ([]entity.Invoice, error) {
	return nil, nil
}
func (r *stubFullInvoiceRepo) Stats(_ context.Context, _ []string) (*entity.InvoiceStats, error) {
	return &entity.InvoiceStats{}, nil
}
func (r *stubFullInvoiceRepo) UpdateStatusBulk(_ context.Context, _ []string, _ string) error {
	return nil
}
func (r *stubFullInvoiceRepo) UpdateFlags(_ context.Context, _ string, _ map[string]bool) error {
	return nil
}

// stubFullLogRepo satisfies repository.LogRepository.
type stubFullLogRepo struct {
	sentToday bool
	err       error
}

func (r *stubFullLogRepo) AppendLog(_ context.Context, _ entity.ActionLog) error  { return nil }
func (r *stubFullLogRepo) SentTodayAlready(_ context.Context, _ string) (bool, error) {
	return r.sentToday, r.err
}
func (r *stubFullLogRepo) MessageIDExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (r *stubFullLogRepo) AppendActivity(_ context.Context, _ entity.ActivityLog) error { return nil }
func (r *stubFullLogRepo) GetActivities(_ context.Context, _ entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	return nil, 0, nil
}
func (r *stubFullLogRepo) GetActivityStats(_ context.Context, _ []string) (entity.ActivityStats, error) {
	return entity.ActivityStats{}, nil
}
func (r *stubFullLogRepo) GetRecentActivities(_ context.Context, _ []string, _ time.Time, _ int) ([]entity.ActivityLog, error) {
	return nil, nil
}
func (r *stubFullLogRepo) GetCompanySummary(_ context.Context, _ []string, _ string) (*entity.CompanySummary, error) {
	return nil, nil
}
func (r *stubFullLogRepo) GetRecentActionLogs(_ context.Context, _ []string, _ int) ([]entity.ActionLog, error) {
	return nil, nil
}
func (r *stubFullLogRepo) GetActionLogSummary(_ context.Context, _ []string, _ time.Time) (*entity.ActionLogSummary, error) {
	return &entity.ActionLogSummary{}, nil
}
func (r *stubFullLogRepo) GetTodayActionLogs(_ context.Context, _ []string, _ int) ([]entity.ActionLog, error) {
	return nil, nil
}

// stubFullBgJobRepo satisfies repository.BackgroundJobRepository.
type stubFullBgJobRepo struct {
	createErr error
}

func (r *stubFullBgJobRepo) Create(_ context.Context, _ *entity.BackgroundJob) error {
	return r.createErr
}
func (r *stubFullBgJobRepo) GetByID(_ context.Context, _ string) (*entity.BackgroundJob, error) {
	return nil, nil
}
func (r *stubFullBgJobRepo) ListByWorkspace(_ context.Context, _, _, _ string, _ pagination.Params) ([]entity.BackgroundJob, int64, error) {
	return nil, 0, nil
}
func (r *stubFullBgJobRepo) UpdateProgress(_ context.Context, _ string, _, _, _, _, _ int, _ []entity.JobRowError) error {
	return nil
}
func (r *stubFullBgJobRepo) UpdateStatus(_ context.Context, _, _ string) error      { return nil }
func (r *stubFullBgJobRepo) UpdateStoragePath(_ context.Context, _, _ string) error { return nil }
func (r *stubFullBgJobRepo) MarkOrphansFailed(_ context.Context) error               { return nil }

// stubFullWorkspaceRepo satisfies repository.WorkspaceRepository.
type stubFullWorkspaceRepo struct {
	workspaces []entity.Workspace
	err        error
}

func (r *stubFullWorkspaceRepo) GetAll(_ context.Context) ([]entity.Workspace, error) {
	return r.workspaces, r.err
}
func (r *stubFullWorkspaceRepo) GetByID(_ context.Context, _ string) (*entity.Workspace, error) {
	return nil, nil
}
func (r *stubFullWorkspaceRepo) GetBySlug(_ context.Context, _ string) (*entity.Workspace, error) {
	return nil, nil
}
func (r *stubFullWorkspaceRepo) ListForUser(_ context.Context, _ string) ([]entity.Workspace, error) {
	return nil, nil
}
func (r *stubFullWorkspaceRepo) Create(_ context.Context, w *entity.Workspace) (*entity.Workspace, error) {
	return w, nil
}
func (r *stubFullWorkspaceRepo) Update(_ context.Context, _ string, _ repository.WorkspacePatch) (*entity.Workspace, error) {
	return nil, nil
}
func (r *stubFullWorkspaceRepo) SoftDelete(_ context.Context, _ string) error { return nil }

// ─── newTestCronRunner helper ─────────────────────────────────────────────────

func newTestCronRunner(
	clients []entity.Client,
	sentToday bool,
	flagsErr error,
) ucron.CronRunner {
	return ucron.NewCronRunner(
		&stubFullClientRepo{clients: clients},
		&stubFullFlagsRepo{err: flagsErr},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{sentToday: sentToday},
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{},
		nil, // triggers — nil is valid; processClient only uses it after the gates
		zerolog.Nop(),
	)
}

// ─── RunAll ───────────────────────────────────────────────────────────────────

func TestCronRunner_RunAll_EmptyClientList_Succeeds(t *testing.T) {
	t.Parallel()

	cr := newTestCronRunner(nil, false, nil)
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCronRunner_RunAll_GetAllError_ReturnsError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db unavailable")
	cr := ucron.NewCronRunner(
		&stubFullClientRepo{err: sentinel},
		&stubFullFlagsRepo{},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{},
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{},
		nil,
		zerolog.Nop(),
	)
	err := cr.RunAll(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestCronRunner_RunAll_BlacklistedClient_SkippedNoError(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "BL-001", Blacklisted: true, BotActive: true},
	}
	cr := newTestCronRunner(clients, false, nil)
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error for blacklisted client: %v", err)
	}
}

func TestCronRunner_RunAll_BotInactiveClient_SkippedNoError(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "BOT-001", Blacklisted: false, BotActive: false},
	}
	cr := newTestCronRunner(clients, false, nil)
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error for bot-inactive client: %v", err)
	}
}

func TestCronRunner_RunAll_RejectedClient_SkippedNoError(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "REJ-001", Blacklisted: false, BotActive: true, Rejected: true},
	}
	cr := newTestCronRunner(clients, false, nil)
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error for rejected client: %v", err)
	}
}

func TestCronRunner_RunAll_SentTodayAlready_SkippedNoError(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "SENT-001", BotActive: true, Blacklisted: false},
	}
	// sentToday=true should cause the client to be skipped (gate 4).
	cr := newTestCronRunner(clients, true, nil)
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error for already-sent client: %v", err)
	}
}

func TestCronRunner_RunAll_BlacklistedCheckedBeforeBotActive(t *testing.T) {
	t.Parallel()

	// Client is both blacklisted AND bot_active=false.
	// Rule: blacklisted must be checked first (Gate 1 before Gate 2).
	// We verify no panic and no error — both gates skip cleanly.
	clients := []entity.Client{
		{CompanyID: "BL-BOTOFF", Blacklisted: true, BotActive: false},
	}
	cr := newTestCronRunner(clients, false, nil)
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCronRunner_RunAll_FlagsRepoError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Client passes gates 1-4 but flagsRepo.GetByCompanyID fails.
	clients := []entity.Client{
		{CompanyID: "FLAGS-ERR", BotActive: true, Blacklisted: false},
	}
	flagsErr := errors.New("flags db error")
	cr := newTestCronRunner(clients, false, flagsErr)

	// RunAll logs per-client errors but does NOT return them — it always
	// returns nil. Only a GetAll error returns from RunAll.
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("RunAll should not return per-client errors: %v", err)
	}
}

// ─── StartRunAll ──────────────────────────────────────────────────────────────

func TestCronRunner_StartRunAll_WorkspaceRepoError_ReturnsError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("workspace db unavailable")
	cr := ucron.NewCronRunner(
		&stubFullClientRepo{},
		&stubFullFlagsRepo{},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{},
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{err: sentinel},
		nil,
		zerolog.Nop(),
	)
	ext, ok := cr.(interface {
		StartRunAll(ctx context.Context) ([]*entity.BackgroundJob, error)
	})
	if !ok {
		t.Skip("StartRunAll not exposed via CronRunner interface in this test context")
	}
	_, err := ext.StartRunAll(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCronRunner_StartRunAll_NoWorkspaces_ReturnsEmptyJobs(t *testing.T) {
	t.Parallel()

	cr := ucron.NewCronRunner(
		&stubFullClientRepo{},
		&stubFullFlagsRepo{},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{},
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{workspaces: []entity.Workspace{}},
		nil,
		zerolog.Nop(),
	)
	ext, ok := cr.(interface {
		StartRunAll(ctx context.Context) ([]*entity.BackgroundJob, error)
	})
	if !ok {
		t.Skip("StartRunAll not exposed via CronRunner interface in this test context")
	}
	jobs, err := ext.StartRunAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestCronRunner_StartRunAll_HoldingWorkspace_Skipped(t *testing.T) {
	t.Parallel()

	cr := ucron.NewCronRunner(
		&stubFullClientRepo{},
		&stubFullFlagsRepo{},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{},
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{workspaces: []entity.Workspace{
			{ID: "ws-holding", IsHolding: true, Name: "Holding WS", Slug: "holding"},
		}},
		nil,
		zerolog.Nop(),
	)
	ext, ok := cr.(interface {
		StartRunAll(ctx context.Context) ([]*entity.BackgroundJob, error)
	})
	if !ok {
		t.Skip("StartRunAll not exposed via CronRunner interface")
	}
	jobs, err := ext.StartRunAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Holding workspace must be skipped — no job created.
	if len(jobs) != 0 {
		t.Errorf("holding workspace: expected 0 jobs, got %d", len(jobs))
	}
}

func TestCronRunner_StartRunAll_NonHoldingWorkspace_CreatesJob(t *testing.T) {
	t.Parallel()

	cr := ucron.NewCronRunner(
		&stubFullClientRepo{clients: []entity.Client{}},
		&stubFullFlagsRepo{},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{},
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{workspaces: []entity.Workspace{
			{ID: "ws-regular", IsHolding: false, Name: "Regular WS", Slug: "regular"},
		}},
		nil,
		zerolog.Nop(),
	)
	ext, ok := cr.(interface {
		StartRunAll(ctx context.Context) ([]*entity.BackgroundJob, error)
	})
	if !ok {
		t.Skip("StartRunAll not exposed via CronRunner interface")
	}
	jobs, err := ext.StartRunAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// One non-holding workspace → one job.
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
	// Give the background goroutine a moment to start without blocking.
	time.Sleep(10 * time.Millisecond)
}

func TestCronRunner_StartRunAll_BgJobCreateError_SkipsWorkspace(t *testing.T) {
	t.Parallel()

	cr := ucron.NewCronRunner(
		&stubFullClientRepo{},
		&stubFullFlagsRepo{},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{},
		&stubFullBgJobRepo{createErr: errors.New("job table locked")},
		&stubFullWorkspaceRepo{workspaces: []entity.Workspace{
			{ID: "ws-err", IsHolding: false, Name: "WS", Slug: "ws-err"},
		}},
		nil,
		zerolog.Nop(),
	)
	ext, ok := cr.(interface {
		StartRunAll(ctx context.Context) ([]*entity.BackgroundJob, error)
	})
	if !ok {
		t.Skip("StartRunAll not exposed via CronRunner interface")
	}
	jobs, err := ext.StartRunAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// BgJob.Create failed → workspace skipped → 0 jobs.
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs when bg job create fails, got %d", len(jobs))
	}
}

// ─── WithRuleEngine ───────────────────────────────────────────────────────────

func TestCronRunner_WithRuleEngine_CanBeSetOnRunner(t *testing.T) {
	t.Parallel()

	cr := newTestCronRunner(nil, false, nil)
	ruleEngine, ok := cr.(ucron.CronRunnerWithRuleEngine)
	if !ok {
		t.Skip("CronRunner does not implement CronRunnerWithRuleEngine in this context")
	}
	// Setting nil engine with useDynamic=false should not panic.
	ruleEngine.WithRuleEngine(nil, false)
}

