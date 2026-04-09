package dashboard_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	ucDashboard "github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
)

// ─── List ─────────────────────────────────────────────────────────────────────

func withWorkspaceID(r *http.Request, wsID string) *http.Request {
	ctx := ctxutil.SetWorkspaceID(r.Context(), wsID)
	return r.WithContext(ctx)
}

func TestClientList_ReturnsWithMeta(t *testing.T) {
	mock := &mockUsecase{
		getClientsByWSIDResult: &ucDashboard.ClientListResult{
			Clients: []entity.Client{{CompanyID: "C01", CompanyName: "PT A"}},
			Meta:    pagination.Meta{Offset: 0, Limit: 10, Total: 1},
		},
	}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients", nil)
	r = withWorkspaceID(r, "ws-1")
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := decodeBody(t, w)
	meta, ok := body["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta in response")
	}
	if meta["total"] != float64(1) {
		t.Errorf("expected total=1, got %v", meta["total"])
	}
}

func TestClientList_RepoError(t *testing.T) {
	mock := &mockUsecase{getClientsByWSIDErr: errors.New("db error")}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients", nil)
	r = withWorkspaceID(r, "ws-1")
	err := h.List(httptest.NewRecorder(), r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestClientGet_Found(t *testing.T) {
	mock := &mockUsecase{
		getClientResult: &entity.Client{CompanyID: "C01", CompanyName: "PT A"},
	}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C01", nil)
	r = withPathParam(r, "company_id", "C01")
	w := callHandler(h.Get, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestClientGet_NotFound(t *testing.T) {
	mock := &mockUsecase{getClientResult: nil}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C99", nil)
	r = withPathParam(r, "company_id", "C99")
	w := callHandler(h.Get, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["errorCode"] != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %v", body["errorCode"])
	}
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestClientCreate_Success(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	payload := map[string]string{"company_id": "C01", "company_name": "PT New"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/clients", bytes.NewReader(body))
	r = withWorkspaceID(r, "ws-1")
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Create, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d — body: %s", w.Code, w.Body)
	}
	if mock.recordEntry.Action != "add_client" {
		t.Errorf("expected activity action=add_client, got %q", mock.recordEntry.Action)
	}
}

func TestClientCreate_MissingFields(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	payload := map[string]string{"company_id": "C01"} // missing company_name
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/clients", bytes.NewReader(body))
	r = withWorkspaceID(r, "ws-1")
	w := callHandler(h.Create, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestClientCreate_InvalidBody(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/clients", bytes.NewReader([]byte(`{bad`)))
	r = withWorkspaceID(r, "ws-1")
	w := callHandler(h.Create, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestClientUpdate_Success(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	payload := map[string]interface{}{"notes": "updated"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPut, "/dashboard/clients/C01", bytes.NewReader(body))
	r = withPathParam(r, "company_id", "C01")
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Update, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mock.recordEntry.Action != "edit_client" {
		t.Errorf("expected activity action=edit_client, got %q", mock.recordEntry.Action)
	}
}

func TestClientUpdate_InvalidBody(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodPut, "/dashboard/clients/C01", bytes.NewReader([]byte(`{bad`)))
	r = withPathParam(r, "company_id", "C01")
	w := callHandler(h.Update, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestClientDelete_Success(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodDelete, "/dashboard/clients/C01", nil)
	r = withPathParam(r, "company_id", "C01")
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Delete, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mock.recordEntry.Action != "delete_client" {
		t.Errorf("expected activity action=delete_client, got %q", mock.recordEntry.Action)
	}
}

func TestClientList_EmptyResult(t *testing.T) {
	mock := &mockUsecase{
		getClientsByWSIDResult: &ucDashboard.ClientListResult{
			Clients: nil,
			Meta:    pagination.Meta{Offset: 0, Limit: 10, Total: 0},
		},
	}
	h := handler.NewClientHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/data-master/clients", nil)
	r = withWorkspaceID(r, "ws-1")
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := decodeBody(t, w)
	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatal("expected data as array")
	}
	if len(data) != 0 {
		t.Errorf("expected empty array, got %d items", len(data))
	}
}
