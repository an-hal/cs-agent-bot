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
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	ucDashboard "github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
)

// ─── List ─────────────────────────────────────────────────────────────────────

func TestClientList_ReturnsWithMeta(t *testing.T) {
	mock := &mockUsecase{
		getClientsResult: []entity.Client{{CompanyID: "C01", CompanyName: "PT A"}},
		getClientsTotal:  1,
	}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients?workspace=dealls", nil)
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
	mock := &mockUsecase{getClientsErr: errors.New("db error")}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients", nil)
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
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C01", nil)
	r = withPathParam(r, "company_id", "C01")
	w := callHandler(h.Get, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestClientGet_NotFound(t *testing.T) {
	mock := &mockUsecase{getClientResult: nil}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C99", nil)
	r = withPathParam(r, "company_id", "C99")
	w := callHandler(h.Get, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["errorCode"] != "CLIENT_NOT_FOUND" {
		t.Errorf("expected CLIENT_NOT_FOUND, got %v", body["errorCode"])
	}
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestClientCreate_Success(t *testing.T) {
	mock := &mockUsecase{
		getWSBySlugResult: &entity.Workspace{ID: "ws-1", Slug: "dealls"},
	}
	h := handler.NewClientHandler(mock)

	payload := map[string]string{"company_id": "C01", "company_name": "PT New"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/clients?workspace=dealls", bytes.NewReader(body))
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
	h := handler.NewClientHandler(mock)

	payload := map[string]string{"company_id": "C01"} // missing company_name
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/clients", bytes.NewReader(body))
	w := callHandler(h.Create, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestClientCreate_InvalidBody(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/clients", bytes.NewReader([]byte(`{bad`)))
	w := callHandler(h.Create, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestClientCreate_InvalidWorkspace(t *testing.T) {
	mock := &mockUsecase{getWSBySlugResult: nil}
	h := handler.NewClientHandler(mock)

	payload := map[string]string{"company_id": "C01", "company_name": "PT X"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/clients?workspace=invalid", bytes.NewReader(body))
	w := callHandler(h.Create, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestClientUpdate_Success(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock)

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
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodPut, "/dashboard/clients/C01", bytes.NewReader([]byte(`{bad`)))
	r = withPathParam(r, "company_id", "C01")
	w := callHandler(h.Update, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestClientDelete_Success(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewClientHandler(mock)

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

// ─── GetInvoices ──────────────────────────────────────────────────────────────

func TestClientGetInvoices_ReturnsWithMeta(t *testing.T) {
	mock := &mockUsecase{
		getInvoicesResult: []entity.Invoice{{InvoiceID: "INV-001"}},
		getInvoicesTotal:  1,
	}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C01/invoices", nil)
	r = withPathParam(r, "company_id", "C01")
	w := callHandler(h.GetInvoices, r)

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

func TestClientGetInvoices_RepoError(t *testing.T) {
	mock := &mockUsecase{getInvoicesErr: errors.New("db error")}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C01/invoices", nil)
	r = withPathParam(r, "company_id", "C01")
	err := h.GetInvoices(httptest.NewRecorder(), r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── GetEscalations ──────────────────────────────────────────────────────────

func TestClientGetEscalations_ReturnsWithMeta(t *testing.T) {
	mock := &mockUsecase{
		getEscalationsResult: []entity.Escalation{{EscalationID: "ESC-1"}},
		getEscalationsTotal:  1,
	}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C01/escalations", nil)
	r = withPathParam(r, "company_id", "C01")
	w := callHandler(h.GetEscalations, r)

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

func TestClientGetEscalations_RepoError(t *testing.T) {
	mock := &mockUsecase{getEscalationsErr: errors.New("db error")}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/clients/C01/escalations", nil)
	r = withPathParam(r, "company_id", "C01")
	err := h.GetEscalations(httptest.NewRecorder(), r)
	if err == nil {
		t.Error("expected error")
	}
}

// ─── ListByWorkspaceID ────────────────────────────────────────────────────────

func TestClientListByWorkspaceID_ReturnsWithMeta(t *testing.T) {
	mock := &mockUsecase{
		getClientsByWSIDResult: &ucDashboard.ClientListResult{
			Clients: []entity.Client{{CompanyID: "C01", CompanyName: "PT A"}},
			Meta:    pagination.Meta{Offset: 0, Limit: 10, Total: 1},
		},
	}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/workspaces/ws-1/clients", nil)
	r = withPathParam(r, "workspace_id", "ws-1")
	w := callHandler(h.ListByWorkspaceID, r)

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

func TestClientListByWorkspaceID_EmptyResult(t *testing.T) {
	mock := &mockUsecase{
		getClientsByWSIDResult: &ucDashboard.ClientListResult{
			Clients: nil,
			Meta:    pagination.Meta{Offset: 0, Limit: 10, Total: 0},
		},
	}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/workspaces/ws-1/clients", nil)
	r = withPathParam(r, "workspace_id", "ws-1")
	w := callHandler(h.ListByWorkspaceID, r)

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

func TestClientListByWorkspaceID_RepoError(t *testing.T) {
	mock := &mockUsecase{getClientsByWSIDErr: errors.New("db error")}
	h := handler.NewClientHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/workspaces/ws-1/clients", nil)
	r = withPathParam(r, "workspace_id", "ws-1")
	err := h.ListByWorkspaceID(httptest.NewRecorder(), r)
	if err == nil {
		t.Error("expected error")
	}
}
