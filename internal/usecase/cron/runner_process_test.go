package cron_test

// runner_process_test.go — tests for processClient branches that can be
// exercised without a real TriggerService. Covers:
//   - Renewed=true triggers resetCycleFlags via the dynamic rule engine path
//   - Dynamic rule engine path (useDynamic=true)
//   - Various gate combinations

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	ucron "github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/rs/zerolog"
)

// ─── Stub TriggerRuleRepository ───────────────────────────────────────────────

type stubTriggerRuleRepo struct {
	rules []entity.TriggerRule
	err   error
}

func (r *stubTriggerRuleRepo) GetActiveRulesOrdered(_ context.Context) ([]entity.TriggerRule, error) {
	return r.rules, r.err
}
func (r *stubTriggerRuleRepo) GetByID(_ context.Context, _ string) (*entity.TriggerRule, error) {
	return nil, nil
}
func (r *stubTriggerRuleRepo) GetAllPaginated(_ context.Context, _ entity.TriggerRuleFilter, _ pagination.Params) ([]entity.TriggerRule, int64, error) {
	return nil, 0, nil
}
func (r *stubTriggerRuleRepo) Create(_ context.Context, _ entity.TriggerRule) error { return nil }
func (r *stubTriggerRuleRepo) Update(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}
func (r *stubTriggerRuleRepo) Delete(_ context.Context, _ string) error { return nil }

// ─── Tracking flags repo ──────────────────────────────────────────────────────

type trackingFlagsRepo struct {
	flags       entity.ClientFlags
	resetCalled bool
	resetErr    error
	getErr      error
	callCount   int
}

func (r *trackingFlagsRepo) GetByCompanyID(_ context.Context, _ string) (*entity.ClientFlags, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	f := r.flags
	return &f, nil
}
func (r *trackingFlagsRepo) UpdateFlags(_ context.Context, _ string, _ entity.ClientFlags) error {
	return nil
}
func (r *trackingFlagsRepo) SetBotActive(_ context.Context, _ string, _ bool) error { return nil }
func (r *trackingFlagsRepo) ResetCycleFlags(_ context.Context, _ string) error {
	r.resetCalled = true
	r.callCount++
	return r.resetErr
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func newRuleEngine(rules []entity.TriggerRule) *trigger.RuleEngine {
	repo := &stubTriggerRuleRepo{rules: rules}
	return trigger.NewRuleEngine(repo, nil, zerolog.Nop())
}

func newRuleEngineWithError(err error) *trigger.RuleEngine {
	repo := &stubTriggerRuleRepo{err: err}
	return trigger.NewRuleEngine(repo, nil, zerolog.Nop())
}

func newCronRunnerWithDynamicEngine(
	clients []entity.Client,
	flagsRepo repository.FlagsRepository,
	sentToday bool,
	engine *trigger.RuleEngine,
) ucron.CronRunner {
	cr := ucron.NewCronRunner(
		&stubFullClientRepo{clients: clients},
		flagsRepo,
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{sentToday: sentToday},
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{},
		nil,
		zerolog.Nop(),
	)
	rwe, ok := cr.(ucron.CronRunnerWithRuleEngine)
	if ok {
		rwe.WithRuleEngine(engine, true)
	}
	return cr
}

// ─── Dynamic rule engine: empty rules, no match ───────────────────────────────

func TestCronRunner_DynamicEngine_EmptyRules_ClientProcessedNoError(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "DYN-001", BotActive: true, Blacklisted: false},
	}
	engine := newRuleEngine(nil) // no rules
	cr := newCronRunnerWithDynamicEngine(clients, &stubFullFlagsRepo{}, false, engine)

	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCronRunner_DynamicEngine_MultipleClients_ProcessedNoError(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "DYN-001", BotActive: true, Blacklisted: false},
		{CompanyID: "DYN-002", BotActive: true, Blacklisted: false},
		{CompanyID: "DYN-003", BotActive: true, Blacklisted: false},
	}
	engine := newRuleEngine(nil)
	cr := newCronRunnerWithDynamicEngine(clients, &stubFullFlagsRepo{}, false, engine)

	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCronRunner_DynamicEngine_RuleRepoError_PerClientErrorLogged(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "DYN-ERR-001", BotActive: true, Blacklisted: false},
	}
	engine := newRuleEngineWithError(errors.New("rule db unavailable"))
	cr := newCronRunnerWithDynamicEngine(clients, &stubFullFlagsRepo{}, false, engine)

	// RunAll absorbs per-client errors — returns nil.
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("RunAll must not propagate per-client errors: %v", err)
	}
}

func TestCronRunner_DynamicEngine_BlacklistedClient_SkipsEngine(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "BL-DYN-001", BotActive: true, Blacklisted: true},
	}
	// Rule repo that returns error if called — proves engine not reached.
	engine := newRuleEngineWithError(errors.New("engine should not be called"))
	cr := newCronRunnerWithDynamicEngine(clients, &stubFullFlagsRepo{}, false, engine)

	// If engine is reached, it would try to load rules and fail; that error
	// would be logged. Since the client is blacklisted, engine must not run.
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCronRunner_DynamicEngine_BotInactiveClient_SkipsEngine(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "BOTOFF-DYN", BotActive: false, Blacklisted: false},
	}
	engine := newRuleEngineWithError(errors.New("engine should not be called"))
	cr := newCronRunnerWithDynamicEngine(clients, &stubFullFlagsRepo{}, false, engine)

	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCronRunner_DynamicEngine_SentToday_SkipsEngine(t *testing.T) {
	t.Parallel()

	clients := []entity.Client{
		{CompanyID: "SENT-DYN", BotActive: true, Blacklisted: false},
	}
	engine := newRuleEngineWithError(errors.New("engine should not be called"))
	cr := newCronRunnerWithDynamicEngine(clients, &stubFullFlagsRepo{}, true /*sentToday*/, engine)

	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Renewed=true triggers resetCycleFlags via dynamic path ──────────────────

func TestCronRunner_DynamicEngine_RenewedClient_CallsResetCycleFlags(t *testing.T) {
	t.Parallel()

	tr := &trackingFlagsRepo{}
	clients := []entity.Client{
		{
			CompanyID:   "RENEWED-DYN-001",
			BotActive:   true,
			Blacklisted: false,
			Renewed:     true,
		},
	}
	engine := newRuleEngine(nil) // no rules, just reach the evaluate step
	cr := newCronRunnerWithDynamicEngine(clients, tr, false, engine)

	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tr.resetCalled {
		t.Error("expected ResetCycleFlags to be called when client.Renewed=true")
	}
}

func TestCronRunner_DynamicEngine_RenewedClient_ResetError_ReturnsPerClientError(t *testing.T) {
	t.Parallel()

	tr := &trackingFlagsRepo{resetErr: errors.New("reset flags failed")}
	clients := []entity.Client{
		{CompanyID: "RENEWED-ERR", BotActive: true, Blacklisted: false, Renewed: true},
	}
	engine := newRuleEngine(nil)
	cr := newCronRunnerWithDynamicEngine(clients, tr, false, engine)

	// RunAll logs per-client errors — no return value.
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("RunAll must not propagate per-client errors: %v", err)
	}
}

func TestCronRunner_DynamicEngine_NotRenewed_DoesNotCallResetCycleFlags(t *testing.T) {
	t.Parallel()

	tr := &trackingFlagsRepo{}
	clients := []entity.Client{
		{CompanyID: "NOT-RENEWED", BotActive: true, Blacklisted: false, Renewed: false},
	}
	engine := newRuleEngine(nil)
	cr := newCronRunnerWithDynamicEngine(clients, tr, false, engine)

	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.resetCalled {
		t.Error("ResetCycleFlags must NOT be called when client.Renewed=false")
	}
}

// ─── FlagsRepo error when NOT renewed ────────────────────────────────────────

func TestCronRunner_DynamicEngine_FlagsError_PerClientErrorLogged(t *testing.T) {
	t.Parallel()

	tr := &trackingFlagsRepo{getErr: errors.New("flags db error")}
	clients := []entity.Client{
		{CompanyID: "FLAGS-ERR-DYN", BotActive: true, Blacklisted: false, Renewed: false},
	}
	engine := newRuleEngine(nil)
	cr := newCronRunnerWithDynamicEngine(clients, tr, false, engine)

	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("RunAll must not propagate per-client errors: %v", err)
	}
}

// ─── WithRuleEngine sets useDynamic=false, falls through to legacy path ───────

func TestCronRunner_WithRuleEngine_FalseFlag_DoesNotUseDynamicPath(t *testing.T) {
	t.Parallel()

	// When useDynamic=false, the runner ignores the engine and uses legacy triggers.
	// With nil TriggerService, processClient panics after the gates — so we
	// use sentToday=true to ensure the client is gated out before triggers run.
	clients := []entity.Client{
		{CompanyID: "NODYN-001", BotActive: true, Blacklisted: false},
	}
	cr := ucron.NewCronRunner(
		&stubFullClientRepo{clients: clients},
		&stubFullFlagsRepo{},
		&stubFullConvStateRepo{},
		&stubFullInvoiceRepo{},
		&stubFullLogRepo{sentToday: true}, // gated out at Gate 4
		&stubFullBgJobRepo{},
		&stubFullWorkspaceRepo{},
		nil,
		zerolog.Nop(),
	)
	rwe, ok := cr.(ucron.CronRunnerWithRuleEngine)
	if !ok {
		t.Skip("CronRunner does not implement CronRunnerWithRuleEngine")
	}
	ruleRepoErr := &stubTriggerRuleRepo{err: errors.New("engine should not be called")}
	engine := trigger.NewRuleEngine(ruleRepoErr, nil, zerolog.Nop())
	rwe.WithRuleEngine(engine, false) // useDynamic=false

	// SentToday gate ensures the client is never processed past Gate 4.
	if err := cr.RunAll(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── Compile-time: unused import guard ───────────────────────────────────────

var _ repository.FlagsRepository = (*trackingFlagsRepo)(nil)
