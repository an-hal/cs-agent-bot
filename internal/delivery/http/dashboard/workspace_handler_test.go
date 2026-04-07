package dashboard_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	handler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

func TestWorkspaceList_ReturnsWithMeta(t *testing.T) {
	mock := &mockUsecase{
		getWorkspacesResult: []entity.Workspace{
			{Slug: "dealls", Name: "Dealls"},
			{Slug: "kantorku", Name: "KantorKu"},
		},
	}
	h := handler.NewWorkspaceHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/workspaces", nil)
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["status"] != "success" {
		t.Errorf("expected status=success, got %v", body["status"])
	}
	meta, ok := body["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("expected meta in response")
	}
	if meta["total"] != float64(2) {
		t.Errorf("expected total=2, got %v", meta["total"])
	}
}

func TestWorkspaceList_EmptySlice(t *testing.T) {
	mock := &mockUsecase{getWorkspacesResult: nil}
	h := handler.NewWorkspaceHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/workspaces", nil)
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

func TestWorkspaceList_RepoError(t *testing.T) {
	mock := &mockUsecase{getWorkspacesErr: errors.New("db error")}
	h := handler.NewWorkspaceHandler(mock, testLogger, testTr)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/workspaces", nil)
	w := httptest.NewRecorder()
	err := h.List(w, r)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
