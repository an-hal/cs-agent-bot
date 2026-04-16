package master_data

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// ──────────────────────────── stub repos ────────────────────────────

type stubMDRepo struct {
	listOut    []entity.MasterData
	listTotal  int64
	getOut     *entity.MasterData
	createOut  *entity.MasterData
	patchOut   *entity.MasterData
	statsOut   *repository.MasterDataStats
	queryOut   []entity.MasterData
	transPrev  *entity.MasterData
	transCurr  *entity.MasterData
	deleteErr  error
	patchCalls int
	deleted    int
}

func (s *stubMDRepo) List(ctx context.Context, f entity.MasterDataFilter) ([]entity.MasterData, int64, error) {
	return s.listOut, s.listTotal, nil
}
func (s *stubMDRepo) GetByID(ctx context.Context, ws, id string) (*entity.MasterData, error) {
	return s.getOut, nil
}
func (s *stubMDRepo) Create(ctx context.Context, ws string, m *entity.MasterData) (*entity.MasterData, error) {
	if s.createOut != nil {
		return s.createOut, nil
	}
	out := *m
	out.ID = "md-new"
	out.WorkspaceID = ws
	return &out, nil
}
func (s *stubMDRepo) Patch(ctx context.Context, ws, id string, p repository.MasterDataPatch) (*entity.MasterData, error) {
	s.patchCalls++
	if s.patchOut != nil {
		return s.patchOut, nil
	}
	return s.getOut, nil
}
func (s *stubMDRepo) HardDelete(ctx context.Context, ws, id string) error {
	s.deleted++
	return s.deleteErr
}
func (s *stubMDRepo) Stats(ctx context.Context, ws string) (*repository.MasterDataStats, error) {
	if s.statsOut != nil {
		return s.statsOut, nil
	}
	return &repository.MasterDataStats{ByStage: map[string]int64{}}, nil
}
func (s *stubMDRepo) Attention(ctx context.Context, ws, search string, offset, limit int) ([]entity.MasterData, int64, error) {
	return s.listOut, s.listTotal, nil
}
func (s *stubMDRepo) Query(ctx context.Context, ws string, conds []repository.QueryCondition, limit int) ([]entity.MasterData, int64, error) {
	return s.queryOut, int64(len(s.queryOut)), nil
}
func (s *stubMDRepo) Transition(ctx context.Context, ws, id, newStage string, _ repository.MasterDataPatch, _ map[string]any) (*entity.MasterData, *entity.MasterData, error) {
	if s.transPrev == nil {
		return nil, nil, errors.New("not found")
	}
	return s.transPrev, s.transCurr, nil
}

type stubCFDRepo struct {
	defs []entity.CustomFieldDefinition
}

func (s *stubCFDRepo) List(ctx context.Context, ws string) ([]entity.CustomFieldDefinition, error) {
	return s.defs, nil
}
func (s *stubCFDRepo) GetByID(ctx context.Context, ws, id string) (*entity.CustomFieldDefinition, error) {
	for i := range s.defs {
		if s.defs[i].ID == id {
			return &s.defs[i], nil
		}
	}
	return nil, nil
}
func (s *stubCFDRepo) Create(ctx context.Context, ws string, d *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error) {
	d.ID = "cfd-new"
	return d, nil
}
func (s *stubCFDRepo) Update(ctx context.Context, ws, id string, d *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error) {
	return d, nil
}
func (s *stubCFDRepo) Delete(ctx context.Context, ws, id string) error { return nil }
func (s *stubCFDRepo) Reorder(ctx context.Context, ws string, items []repository.ReorderItem) error {
	return nil
}

type stubMutRepo struct{ appended int }

func (s *stubMutRepo) Append(ctx context.Context, m *entity.MasterDataMutation) error {
	s.appended++
	return nil
}
func (s *stubMutRepo) List(ctx context.Context, ws string, since *time.Time, limit int) ([]entity.MasterDataMutation, error) {
	return nil, nil
}

type stubAR struct {
	created  *entity.ApprovalRequest
	getOut   *entity.ApprovalRequest
	updated  string
	updateAR string
}

func (s *stubAR) Create(ctx context.Context, a *entity.ApprovalRequest) (*entity.ApprovalRequest, error) {
	a.ID = "ar-1"
	a.Status = entity.ApprovalStatusPending
	s.created = a
	return a, nil
}
func (s *stubAR) GetByID(ctx context.Context, ws, id string) (*entity.ApprovalRequest, error) {
	return s.getOut, nil
}
func (s *stubAR) UpdateStatus(ctx context.Context, ws, id, status, checker, reason string) error {
	s.updated = status
	s.updateAR = id
	return nil
}

func newUC(md *stubMDRepo, cfd *stubCFDRepo, mr *stubMutRepo, ar *stubAR) Usecase {
	return New(md, cfd, mr, ar)
}

// ──────────────────────────── ParseFilter ────────────────────────────

func TestParseFilter(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		assert func(t *testing.T, f entity.MasterDataFilter)
	}{
		{"empty returns zero", "", func(t *testing.T, f entity.MasterDataFilter) {
			if len(f.Stages) != 0 || f.RiskFlag != "" {
				t.Fatalf("expected zero filter")
			}
		}},
		{"bot_active sets pointer", "bot_active", func(t *testing.T, f entity.MasterDataFilter) {
			if f.BotActive == nil || !*f.BotActive {
				t.Fatalf("BotActive not true")
			}
		}},
		{"stage list", "stage:LEAD,PROSPECT", func(t *testing.T, f entity.MasterDataFilter) {
			if len(f.Stages) != 2 || f.Stages[0] != "LEAD" {
				t.Fatalf("stages: %v", f.Stages)
			}
		}},
		{"payment", "payment:Overdue", func(t *testing.T, f entity.MasterDataFilter) {
			if f.PaymentStatus != "Overdue" {
				t.Fatalf("payment: %s", f.PaymentStatus)
			}
		}},
		{"expiry days", "expiry:30", func(t *testing.T, f entity.MasterDataFilter) {
			if f.ExpiryWithin != 30 {
				t.Fatalf("expiry: %d", f.ExpiryWithin)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, ParseFilter(tc.input))
		})
	}
}

// ─────────────────────── Patch / write context ───────────────────────

func TestPatchRejectsBotProtectedFields(t *testing.T) {
	uc := newUC(
		&stubMDRepo{getOut: &entity.MasterData{ID: "x", BotActive: true, Stage: "LEAD"}},
		&stubCFDRepo{},
		&stubMutRepo{},
		&stubAR{},
	)
	paid := "Paid"
	_, _, err := uc.Patch(context.Background(), "ws", "x", "ae@example.com", WriteContextBot, PatchRequest{PaymentStatus: &paid})
	if err == nil || !strings.Contains(err.Error(), "payment_status") {
		t.Fatalf("expected forbidden error for bot, got %v", err)
	}

	renewed := true
	_, _, err = uc.Patch(context.Background(), "ws", "x", "ae@example.com", WriteContextBot, PatchRequest{Renewed: &renewed})
	if err == nil || !strings.Contains(err.Error(), "renewed") {
		t.Fatalf("expected forbidden for renewed, got %v", err)
	}
}

func TestPatchAllowsDashboardUserOnProtectedFields(t *testing.T) {
	md := &stubMDRepo{
		getOut:   &entity.MasterData{ID: "x", BotActive: true, Stage: "LEAD"},
		patchOut: &entity.MasterData{ID: "x", BotActive: true, Stage: "LEAD", PaymentStatus: "Paid"},
	}
	uc := newUC(md, &stubCFDRepo{}, &stubMutRepo{}, &stubAR{})
	paid := "Paid"
	out, _, err := uc.Patch(context.Background(), "ws", "x", "ae@example.com", WriteContextDashboardUser, PatchRequest{PaymentStatus: &paid})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.PaymentStatus != "Paid" {
		t.Fatalf("expected Paid, got %s", out.PaymentStatus)
	}
}

// ─────────────────────── Query whitelist ───────────────────────

func TestQueryRejectsBadOp(t *testing.T) {
	uc := newUC(&stubMDRepo{}, &stubCFDRepo{}, &stubMutRepo{}, &stubAR{})
	_, _, err := uc.Query(context.Background(), "ws", []repository.QueryCondition{
		{Field: "stage", Op: "DROP TABLE", Value: "x"},
	}, 100)
	if err == nil {
		t.Fatalf("expected error for invalid op")
	}
}

// ─────────────────────── Validation ───────────────────────

func TestValidateCustomFields(t *testing.T) {
	min := float64(0)
	max := float64(10)
	defs := []entity.CustomFieldDefinition{
		{FieldKey: "hc_size", FieldType: entity.FieldTypeNumber, IsRequired: true, MinValue: &min, MaxValue: &max},
		{FieldKey: "industry", FieldType: entity.FieldTypeText},
	}
	t.Run("missing required", func(t *testing.T) {
		err := ValidateCustomFields(defs, map[string]any{}, true)
		if err == nil {
			t.Fatalf("expected required error")
		}
	})
	t.Run("type mismatch", func(t *testing.T) {
		err := ValidateCustomFields(defs, map[string]any{"hc_size": "abc"}, true)
		if err == nil {
			t.Fatalf("expected type error")
		}
	})
	t.Run("below min", func(t *testing.T) {
		err := ValidateCustomFields(defs, map[string]any{"hc_size": float64(-1)}, true)
		if err == nil {
			t.Fatalf("expected min error")
		}
	})
	t.Run("ok", func(t *testing.T) {
		err := ValidateCustomFields(defs, map[string]any{"hc_size": float64(5)}, true)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
	})
}

// ─────────────────────── Create + Mutation log ───────────────────────

func TestCreateLogsMutation(t *testing.T) {
	mut := &stubMutRepo{}
	uc := newUC(&stubMDRepo{}, &stubCFDRepo{}, mut, &stubAR{})
	_, err := uc.Create(context.Background(), "ws", "ae@example.com", CreateRequest{
		CompanyID:   "DE-1",
		CompanyName: "PT One",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if mut.appended != 1 {
		t.Fatalf("expected 1 mutation, got %d", mut.appended)
	}
}

// ─────────────────────── Delete approval gate ───────────────────────

func TestDeleteRequiresApproval(t *testing.T) {
	md := &stubMDRepo{getOut: &entity.MasterData{ID: "x", CompanyID: "DE-1", CompanyName: "One"}}
	ar := &stubAR{}
	uc := newUC(md, &stubCFDRepo{}, &stubMutRepo{}, ar)
	out, err := uc.RequestDelete(context.Background(), "ws", "x", "ae@example.com")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.ID != "ar-1" || ar.created == nil {
		t.Fatalf("approval not created")
	}
	if md.deleted != 0 {
		t.Fatalf("delete should not run before approval")
	}
}

func TestApplyApprovedDelete(t *testing.T) {
	md := &stubMDRepo{}
	ar := &stubAR{
		getOut: &entity.ApprovalRequest{
			ID: "ar-1", RequestType: entity.ApprovalTypeDeleteClient,
			Status: entity.ApprovalStatusPending, Payload: map[string]any{"client_id": "x"},
		},
	}
	uc := newUC(md, &stubCFDRepo{}, &stubMutRepo{}, ar)
	if err := uc.ApplyApprovedDelete(context.Background(), "ws", "ar-1", "checker@example.com"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if md.deleted != 1 {
		t.Fatalf("expected delete called")
	}
	if ar.updated != entity.ApprovalStatusApproved {
		t.Fatalf("expected approval marked approved")
	}
}

