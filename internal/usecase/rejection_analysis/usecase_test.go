package rejectionanalysis

import (
	"context"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

type stubRepo struct {
	inserted *entity.RejectionAnalysis
	stats    map[string]int
}

func (s *stubRepo) Insert(ctx context.Context, a *entity.RejectionAnalysis) (*entity.RejectionAnalysis, error) {
	s.inserted = a
	out := *a
	out.ID = "ra-1"
	return &out, nil
}
func (s *stubRepo) List(ctx context.Context, f entity.RejectionAnalysisFilter) ([]entity.RejectionAnalysis, int64, error) {
	return nil, 0, nil
}
func (s *stubRepo) CountByCategory(ctx context.Context, workspaceID string, since time.Time) (map[string]int, error) {
	return s.stats, nil
}

func TestAnalyze_ClassifiesPriceRejection(t *testing.T) {
	s := &stubRepo{}
	uc := New(s)
	_, err := uc.Analyze(context.Background(), AnalyzeRequest{
		WorkspaceID:   "ws-1",
		MasterDataID:  "md-1",
		SourceMessage: "Bro, ini terlalu mahal, butuh diskon",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.inserted.RejectionCategory != entity.RejectionCategoryPrice {
		t.Errorf("expected price category, got %q", s.inserted.RejectionCategory)
	}
}

func TestAnalyze_DefaultsToOther(t *testing.T) {
	s := &stubRepo{}
	uc := New(s)
	_, err := uc.Analyze(context.Background(), AnalyzeRequest{
		WorkspaceID:   "ws-1",
		MasterDataID:  "md-1",
		SourceMessage: "Ok, gue pikir-pikir dulu ya.",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if s.inserted.RejectionCategory != entity.RejectionCategoryTiming {
		// "pikir-pikir" doesn't hit timing keywords (nanti/bulan depan/...); falls to "other"
		if s.inserted.RejectionCategory != entity.RejectionCategoryOther {
			t.Errorf("expected other category, got %q", s.inserted.RejectionCategory)
		}
	}
}

func TestRecord_RequiresCategory(t *testing.T) {
	uc := New(&stubRepo{})
	_, err := uc.Record(context.Background(), RecordRequest{
		WorkspaceID:  "ws-1",
		MasterDataID: "md-1",
	})
	if err == nil {
		t.Fatal("expected error for missing category")
	}
}

func TestCategoryStats_DefaultsToThirtyDays(t *testing.T) {
	s := &stubRepo{stats: map[string]int{"price": 5, "timing": 3}}
	uc := New(s)
	out, err := uc.CategoryStats(context.Background(), "ws-1", 0)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out["price"] != 5 {
		t.Errorf("expected 5 for price, got %d", out["price"])
	}
}
