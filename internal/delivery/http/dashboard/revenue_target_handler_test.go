package dashboard_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ─── mockRevenueTargetRepo ───────────────────────────────────────────────────

type mockRevenueTargetRepo struct {
	targets   []entity.RevenueTarget
	listErr   error
	upsertErr error
	lastUpsert entity.RevenueTarget
}

func (m *mockRevenueTargetRepo) List(_ context.Context, _ string) ([]entity.RevenueTarget, error) {
	return m.targets, m.listErr
}
func (m *mockRevenueTargetRepo) Upsert(_ context.Context, t entity.RevenueTarget) error {
	m.lastUpsert = t
	return m.upsertErr
}

// ─── List ────────────────────────────────────────────────────────────────────

func TestRevenueTargetList_ReturnsOK(t *testing.T) {
	mock := &mockRevenueTargetRepo{
		targets: []entity.RevenueTarget{
			{Year: 2026, Month: 4, TargetAmount: 5_000_000_000},
		},
	}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/revenue-targets", nil)
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body)
	}
}

func TestRevenueTargetList_Error(t *testing.T) {
	mock := &mockRevenueTargetRepo{listErr: errors.New("fail")}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/revenue-targets", nil)
	w := httptest.NewRecorder()
	err := h.List(w, r)
	if err == nil {
		t.Error("expected error")
	}
}

func TestRevenueTargetList_EmptyReturnsEmptyArray(t *testing.T) {
	mock := &mockRevenueTargetRepo{targets: []entity.RevenueTarget{}}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/revenue-targets", nil)
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ─── Upsert ──────────────────────────────────────────────────────────────────

func TestRevenueTargetUpsert_ValidRequest(t *testing.T) {
	mock := &mockRevenueTargetRepo{}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	body, _ := json.Marshal(map[string]interface{}{
		"year":          2026,
		"month":         4,
		"target_amount": 5000000000,
	})
	r := httptest.NewRequest(http.MethodPut, "/revenue-targets", bytes.NewReader(body))
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Upsert, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", w.Code, w.Body)
	}
	if mock.lastUpsert.Year != 2026 {
		t.Errorf("year = %d, want 2026", mock.lastUpsert.Year)
	}
	if mock.lastUpsert.Month != 4 {
		t.Errorf("month = %d, want 4", mock.lastUpsert.Month)
	}
	if mock.lastUpsert.TargetAmount != 5_000_000_000 {
		t.Errorf("target = %d, want 5000000000", mock.lastUpsert.TargetAmount)
	}
	if mock.lastUpsert.CreatedBy != "admin@dealls.com" {
		t.Errorf("created_by = %q, want admin@dealls.com", mock.lastUpsert.CreatedBy)
	}
}

func TestRevenueTargetUpsert_InvalidBody(t *testing.T) {
	mock := &mockRevenueTargetRepo{}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodPut, "/revenue-targets", bytes.NewReader([]byte("{")))
	w := callHandler(h.Upsert, r)

	if w.Code == http.StatusOK {
		t.Error("expected error for invalid body")
	}
}

func TestRevenueTargetUpsert_InvalidYear(t *testing.T) {
	mock := &mockRevenueTargetRepo{}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	body, _ := json.Marshal(map[string]interface{}{"year": 1999, "month": 4, "target_amount": 100})
	r := httptest.NewRequest(http.MethodPut, "/revenue-targets", bytes.NewReader(body))
	w := callHandler(h.Upsert, r)

	if w.Code == http.StatusOK {
		t.Error("expected error for invalid year < 2020")
	}
}

func TestRevenueTargetUpsert_InvalidMonth(t *testing.T) {
	mock := &mockRevenueTargetRepo{}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	body, _ := json.Marshal(map[string]interface{}{"year": 2026, "month": 13, "target_amount": 100})
	r := httptest.NewRequest(http.MethodPut, "/revenue-targets", bytes.NewReader(body))
	w := callHandler(h.Upsert, r)

	if w.Code == http.StatusOK {
		t.Error("expected error for invalid month > 12")
	}
}

func TestRevenueTargetUpsert_NegativeAmount(t *testing.T) {
	mock := &mockRevenueTargetRepo{}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	body, _ := json.Marshal(map[string]interface{}{"year": 2026, "month": 4, "target_amount": -100})
	r := httptest.NewRequest(http.MethodPut, "/revenue-targets", bytes.NewReader(body))
	w := callHandler(h.Upsert, r)

	if w.Code == http.StatusOK {
		t.Error("expected error for negative amount")
	}
}

func TestRevenueTargetUpsert_RepoError(t *testing.T) {
	mock := &mockRevenueTargetRepo{upsertErr: errors.New("fail")}
	h := handler.NewRevenueTargetHandler(mock, testLogger, testTr)

	body, _ := json.Marshal(map[string]interface{}{"year": 2026, "month": 4, "target_amount": 100})
	r := httptest.NewRequest(http.MethodPut, "/revenue-targets", bytes.NewReader(body))
	w := httptest.NewRecorder()
	err := h.Upsert(w, r)
	if err == nil {
		t.Error("expected error")
	}
}
