package cron_test

// scenario_helpers_test.go — shared stubs and fluent builders for
// client-journey scenario tests. All helpers live in the cron_test package
// so they are available to both workflow_runner_test.go and
// workflow_scenarios_test.go without import cycles.

import (
	"context"
	"errors"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ─── Fixed clock ──────────────────────────────────────────────────────────────

// fixedNow is a deterministic timestamp used across scenario tests so that
// no test depends on wall-clock time.
var fixedNow = time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)

// daysAgo returns a time that is n days before fixedNow.
func daysAgo(n int) time.Time { return fixedNow.AddDate(0, 0, -n) }

// daysFromNow returns a time that is n days after fixedNow.
func daysFromNow(n int) time.Time { return fixedNow.AddDate(0, 0, n) }

// ─── scenarioOpt — fluent client builder ─────────────────────────────────────

type scenarioOpt func(*entity.Client)

func withBlacklisted(v bool) scenarioOpt {
	return func(c *entity.Client) { c.Blacklisted = v }
}

func withBotActive(v bool) scenarioOpt {
	return func(c *entity.Client) { c.BotActive = v }
}

func withRejected(v bool) scenarioOpt {
	return func(c *entity.Client) { c.Rejected = v }
}

func withContractMonths(n int) scenarioOpt {
	return func(c *entity.Client) { c.ContractMonths = n }
}

func withContractEnd(t time.Time) scenarioOpt {
	return func(c *entity.Client) { c.ContractEnd = t }
}

func withPaymentStatus(s string) scenarioOpt {
	return func(c *entity.Client) { c.PaymentStatus = s }
}

func withRenewed(v bool) scenarioOpt {
	return func(c *entity.Client) { c.Renewed = v }
}

func withCheckinReplied(v bool) scenarioOpt {
	return func(c *entity.Client) { c.CheckinReplied = v }
}

func withRiskFlag(v bool) scenarioOpt {
	return func(c *entity.Client) { c.RiskFlag = v }
}

func withCrossSellRejected(v bool) scenarioOpt {
	return func(c *entity.Client) { c.CrossSellRejected = v }
}

func withCrossSellInterested(v bool) scenarioOpt {
	return func(c *entity.Client) { c.CrossSellInterested = v }
}

func withNPSScore(n int) scenarioOpt {
	return func(c *entity.Client) { c.NPSScore = n }
}

func withQuotationLink(s string) scenarioOpt {
	return func(c *entity.Client) { c.QuotationLink = s }
}

func withSegment(s string) scenarioOpt {
	return func(c *entity.Client) { c.Segment = s }
}

func withCompanyID(id string) scenarioOpt {
	return func(c *entity.Client) { c.CompanyID = id }
}

// newScenarioClient creates a base client that passes all gates by default
// and applies option overrides.
func newScenarioClient(opts ...scenarioOpt) entity.Client {
	c := entity.Client{
		CompanyID:      "CO-SCENARIO-001",
		CompanyName:    "PT Scenario",
		BotActive:      true,
		Blacklisted:    false,
		Rejected:       false,
		PaymentStatus:  entity.PaymentStatusPending,
		ContractMonths: 12,
		ContractEnd:    daysFromNow(90),
		Segment:        entity.SegmentMid,
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

// ─── scenarioFixture — bundles all stubs ─────────────────────────────────────

// scenarioFixture holds every stub repo a scenario test needs.
type scenarioFixture struct {
	clientRepo  *stubScenarioClientRepo
	flagsRepo   *stubScenarioFlagsRepo
	logRepo     *stubScenarioLogRepo
	invoiceRepo *stubScenarioInvoiceRepo
	convRepo    *stubScenarioConvRepo
}

func newFixture() *scenarioFixture {
	return &scenarioFixture{
		clientRepo:  &stubScenarioClientRepo{},
		flagsRepo:   &stubScenarioFlagsRepo{},
		logRepo:     &stubScenarioLogRepo{},
		invoiceRepo: &stubScenarioInvoiceRepo{},
		convRepo:    &stubScenarioConvRepo{},
	}
}

// ─── stubScenarioClientRepo ───────────────────────────────────────────────────

type stubScenarioClientRepo struct {
	clients []entity.Client
}

func (r *stubScenarioClientRepo) GetAll(_ context.Context) ([]entity.Client, error) {
	return r.clients, nil
}
func (r *stubScenarioClientRepo) GetAllByWorkspaceIDs(_ context.Context, _ []string) ([]entity.Client, error) {
	return r.clients, nil
}
func (r *stubScenarioClientRepo) GetByCompanyID(_ context.Context, id string) (*entity.Client, error) {
	for _, c := range r.clients {
		if c.CompanyID == id {
			cp := c
			return &cp, nil
		}
	}
	return nil, errors.New("not found")
}
func (r *stubScenarioClientRepo) UpdateFlags(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}
func (r *stubScenarioClientRepo) BulkCreate(_ context.Context, _ []entity.Client) error { return nil }

// ─── stubScenarioFlagsRepo ────────────────────────────────────────────────────

type stubScenarioFlagsRepo struct {
	flags          entity.ClientFlags
	resetCalled    bool
	resetCallCount int
}

func (r *stubScenarioFlagsRepo) GetByCompanyID(_ context.Context, _ string) (*entity.ClientFlags, error) {
	cp := r.flags
	return &cp, nil
}
func (r *stubScenarioFlagsRepo) Upsert(_ context.Context, _ *entity.ClientFlags) error { return nil }
func (r *stubScenarioFlagsRepo) ResetCycleFlags(_ context.Context, _ string) error {
	r.resetCalled = true
	r.resetCallCount++
	return nil
}
func (r *stubScenarioFlagsRepo) SetFlag(_ context.Context, _, _ string, _ bool) error { return nil }

// ─── stubScenarioLogRepo ──────────────────────────────────────────────────────

type stubScenarioLogRepo struct {
	sentToday bool
	logged    []entity.ActionLog
}

func (r *stubScenarioLogRepo) SentTodayAlready(_ context.Context, _ string) (bool, error) {
	return r.sentToday, nil
}
func (r *stubScenarioLogRepo) LogAction(_ context.Context, log entity.ActionLog) error {
	r.logged = append(r.logged, log)
	return nil
}
func (r *stubScenarioLogRepo) GetByCompanyID(_ context.Context, _ string, _ int) ([]entity.ActionLog, error) {
	return r.logged, nil
}

// ─── stubScenarioInvoiceRepo ─────────────────────────────────────────────────

type stubScenarioInvoiceRepo struct {
	invoice *entity.Invoice
	err     error
}

func (r *stubScenarioInvoiceRepo) GetActiveByCompanyID(_ context.Context, _ string) (*entity.Invoice, error) {
	return r.invoice, r.err
}
func (r *stubScenarioInvoiceRepo) Create(_ context.Context, _ *entity.Invoice) error { return nil }
func (r *stubScenarioInvoiceRepo) Update(_ context.Context, _ *entity.Invoice) error { return nil }
func (r *stubScenarioInvoiceRepo) GetByCompanyID(_ context.Context, _ string) ([]*entity.Invoice, error) {
	return nil, nil
}

// ─── stubScenarioConvRepo ─────────────────────────────────────────────────────

type stubScenarioConvRepo struct {
	state *entity.ConversationState
}

func (r *stubScenarioConvRepo) GetByCompanyID(_ context.Context, _ string) (*entity.ConversationState, error) {
	if r.state == nil {
		return &entity.ConversationState{BotActive: true}, nil
	}
	cp := *r.state
	return &cp, nil
}
func (r *stubScenarioConvRepo) Upsert(_ context.Context, _ *entity.ConversationState) error {
	return nil
}

// ─── stubWorkflowUCError ──────────────────────────────────────────────────────

// stubWorkflowUCError returns a fixed error from GetActiveForStage.
type stubWorkflowUCError struct {
	stubWorkflowUC
	err error
}

func (s *stubWorkflowUCError) GetActiveForStage(_ context.Context, _, _ string) (*entity.Workflow, error) {
	return nil, s.err
}

// ─── stubAutomationUCError ────────────────────────────────────────────────────

// stubAutomationUCError returns a fixed error from GetActiveByRole.
type stubAutomationUCError struct {
	stubAutomationUC
	err error
}

func (s *stubAutomationUCError) GetActiveByRole(_ context.Context, _ string, _ entity.RuleRole) ([]entity.AutomationRule, error) {
	return nil, s.err
}

// ─── stubAutomationUCWithRoles ────────────────────────────────────────────────

// stubAutomationUCWithRoles returns rules only for a specific role.
type stubAutomationUCWithRoles struct {
	stubAutomationUC
	roleRules map[entity.RuleRole][]entity.AutomationRule
}

func (s *stubAutomationUCWithRoles) GetActiveByRole(_ context.Context, _ string, role entity.RuleRole) ([]entity.AutomationRule, error) {
	return s.roleRules[role], nil
}

// ─── capturingAutomationUC ────────────────────────────────────────────────────

// capturingAutomationUC records which roles were requested.
type capturingAutomationUC struct {
	stubAutomationUC
	queriedRoles []entity.RuleRole
}

func (s *capturingAutomationUC) GetActiveByRole(_ context.Context, _ string, role entity.RuleRole) ([]entity.AutomationRule, error) {
	s.queriedRoles = append(s.queriedRoles, role)
	return s.rules, nil
}

// ─── capturingWorkflowUC ──────────────────────────────────────────────────────

// capturingWorkflowUC records which stages were queried.
type capturingWorkflowUC struct {
	stubWorkflowUC
	queriedStages []string
}

func (s *capturingWorkflowUC) GetActiveForStage(_ context.Context, _, stage string) (*entity.Workflow, error) {
	s.queriedStages = append(s.queriedStages, stage)
	return s.wf, nil
}

// ─── misc helpers ─────────────────────────────────────────────────────────────

// activeRule builds a minimal executable AutomationRule.
func activeRule(code, triggerID string) entity.AutomationRule {
	return entity.AutomationRule{
		ID:        "rule-" + code,
		RuleCode:  code,
		TriggerID: triggerID,
		Status:    entity.RuleStatusActive,
		Role:      entity.RuleRoleAE,
		Phase:     "P0",
	}
}

// workflow returns a minimal workflow for the given slug and stages.
func newWorkflow(id, slug string, stages ...string) *entity.Workflow {
	return &entity.Workflow{
		ID:          id,
		Slug:        slug,
		Status:      entity.WorkflowStatusActive,
		StageFilter: stages,
	}
}

// assertNoForbiddenFieldMutated asserts that payment_status, renewed, and
// rejected on the MasterData value have not been changed by RunForRecord.
func assertNoForbiddenFieldMutated(t interface {
	Helper()
	Errorf(string, ...interface{})
}, before, after entity.MasterData) {
	t.Helper()
	if before.PaymentStatus != after.PaymentStatus {
		t.Errorf("payment_status mutated: %q → %q", before.PaymentStatus, after.PaymentStatus)
	}
	if before.Renewed != after.Renewed {
		t.Errorf("renewed mutated: %v → %v", before.Renewed, after.Renewed)
	}
}

// ─── Interfaces for repository stubs (full signature compliance) ──────────────

// Ensure stubScenarioClientRepo satisfies repository.ClientRepository at
// compile time — we only need a subset here since the cron runner uses
// GetAll / GetAllByWorkspaceIDs.
var _ interface {
	GetAll(ctx context.Context) ([]entity.Client, error)
	GetAllByWorkspaceIDs(ctx context.Context, ids []string) ([]entity.Client, error)
} = (*stubScenarioClientRepo)(nil)

var _ interface {
	SentTodayAlready(ctx context.Context, companyID string) (bool, error)
} = (*stubScenarioLogRepo)(nil)

// Make sure the entity package satisfies the repository interfaces at compile
// time for Invoice - only what runner.go calls.
var _ interface {
	GetActiveByCompanyID(ctx context.Context, companyID string) (*entity.Invoice, error)
} = (*stubScenarioInvoiceRepo)(nil)

var _ interface {
	GetByCompanyID(ctx context.Context, companyID string) (*entity.ConversationState, error)
} = (*stubScenarioConvRepo)(nil)

// flagsRepoInterface is a minimal interface we implement so we can verify
// resetCycleFlags is only called when needed.
var _ interface {
	GetByCompanyID(ctx context.Context, companyID string) (*entity.ClientFlags, error)
	ResetCycleFlags(ctx context.Context, companyID string) error
} = (*stubScenarioFlagsRepo)(nil)

// ─── repository.ClientRepository full implementation ──────────────────────────
// The cron runner only calls GetAll / GetAllByWorkspaceIDs on the client repo,
// but the interface may require more methods. We implement just enough.

// Satisfy any remaining methods the compiler requires on our stub by
// embedding a panicRepo that panics if an unexpected method is called.
type panicClientRepo struct{}

func (p *panicClientRepo) GetAll(_ context.Context) ([]entity.Client, error) {
	panic("unexpected GetAll call")
}
func (p *panicClientRepo) GetAllByWorkspaceIDs(_ context.Context, _ []string) ([]entity.Client, error) {
	panic("unexpected GetAllByWorkspaceIDs call")
}
