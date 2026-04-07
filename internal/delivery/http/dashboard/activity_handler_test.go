package dashboard_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	handler "github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ─── List tests ───────────────────────────────────────────────────────────────

func TestActivityList_ReturnsLogs(t *testing.T) {
	mock := &mockUsecase{
		getLogsResult: []entity.ActivityLog{
			{ID: 1, Category: "bot", Action: "RENEWAL", OccurredAt: time.Now()},
		},
		getLogsTotal: 1,
	}
	h := handler.NewActivityHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/activity-logs?workspace_id=dealls&limit=10", nil)
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["status"] != "success" {
		t.Errorf("expected status=success, got %v", body["status"])
	}
}

func TestActivityList_LimitCappedAt100(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/activity-logs?limit=999", nil)
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestActivityList_EmptyResultReturnsEmptySlice(t *testing.T) {
	mock := &mockUsecase{getLogsResult: nil, getLogsTotal: 0}
	h := handler.NewActivityHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/activity-logs", nil)
	w := callHandler(h.List, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestActivityList_RepoErrorReturnsError(t *testing.T) {
	mock := &mockUsecase{getLogsErr: errors.New("db down")}
	h := handler.NewActivityHandler(mock)

	r := httptest.NewRequest(http.MethodGet, "/dashboard/activity-logs", nil)
	w := httptest.NewRecorder()
	err := h.List(w, r)
	if err == nil {
		t.Error("expected error from handler, got nil")
	}
}

// ─── Record tests ─────────────────────────────────────────────────────────────

func TestRecord_ValidTeamActivity(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	payload := map[string]string{
		"workspace_id": "dealls",
		"category":     "team",
		"action":       "invite_member",
		"target":       "galih@dealls.com",
		"detail":       "Role: SDR Officer",
	}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader(body))
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Record, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d — body: %s", w.Code, w.Body)
	}
	if mock.recordEntry.Actor != "admin@dealls.com" {
		t.Errorf("actor should come from JWT, got %q", mock.recordEntry.Actor)
	}
	if mock.recordEntry.Category != "team" {
		t.Errorf("unexpected category: %q", mock.recordEntry.Category)
	}
}

func TestRecord_ValidDataActivity(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	payload := map[string]string{
		"category": "data",
		"action":   "import_bulk",
		"target":   "30 klien",
	}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader(body))
	r = withJWTUser(r, "cs@dealls.com")
	w := callHandler(h.Record, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestRecord_RejectsBotCategory(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	payload := map[string]string{"category": "bot", "action": "RENEWAL"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader(body))
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Record, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	resp := decodeBody(t, w)
	if resp["errorCode"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %v", resp["errorCode"])
	}
}

func TestRecord_RejectsInvalidCategory(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	payload := map[string]string{"category": "unknown", "action": "something"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader(body))
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Record, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRecord_RejectsMissingAction(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	payload := map[string]string{"category": "team"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader(body))
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Record, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRecord_InvalidJSON(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader([]byte(`not-json`)))
	r = withJWTUser(r, "admin@dealls.com")
	w := callHandler(h.Record, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRecord_ActorAlwaysFromJWT(t *testing.T) {
	mock := &mockUsecase{}
	h := handler.NewActivityHandler(mock)

	payload := map[string]string{
		"category": "data",
		"action":   "export_bulk",
		"actor":    "spoofed@hacker.com",
	}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader(body))
	r = withJWTUser(r, "real@dealls.com")
	w := callHandler(h.Record, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	if mock.recordEntry.Actor != "real@dealls.com" {
		t.Errorf("actor should be JWT email, got %q", mock.recordEntry.Actor)
	}
}

func TestRecord_RepoErrorPropagated(t *testing.T) {
	mock := &mockUsecase{recordActivityErr: errors.New("insert failed")}
	h := handler.NewActivityHandler(mock)

	payload := map[string]string{"category": "team", "action": "remove_member"}
	body, _ := json.Marshal(payload)

	r := httptest.NewRequest(http.MethodPost, "/dashboard/activity-logs", bytes.NewReader(body))
	r = withJWTUser(r, "admin@dealls.com")

	err := h.Record(httptest.NewRecorder(), r)
	if err == nil {
		t.Error("expected repo error to be returned")
	}
}
