package dashboard_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/dashboard"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
)

// --- mock collection.Usecase --------------------------------------------------

type stubCollectionUC struct {
	listCalled       bool
	listResult       []entity.Collection
	getResult        *entity.Collection
	requestCreateRes *entity.ApprovalRequest
	requestCreateErr error
	createRecordRes  *entity.CollectionRecord
	createRecordErr  error
	distinctRes      *entity.DistinctResult
	applyRes         *entity.ApprovalRequest
	applyErr         error
}

func (s *stubCollectionUC) ListCollections(ctx context.Context, workspaceID string) ([]entity.Collection, error) {
	s.listCalled = true
	return s.listResult, nil
}
func (s *stubCollectionUC) GetCollection(ctx context.Context, workspaceID, id string) (*entity.Collection, error) {
	if s.getResult == nil {
		return nil, errors.New("not found")
	}
	return s.getResult, nil
}
func (s *stubCollectionUC) RequestCreateCollection(ctx context.Context, workspaceID, actor string, req collection.CreateCollectionRequest) (*entity.ApprovalRequest, error) {
	return s.requestCreateRes, s.requestCreateErr
}
func (s *stubCollectionUC) RequestDeleteCollection(ctx context.Context, workspaceID, actor, id string) (*entity.ApprovalRequest, error) {
	return &entity.ApprovalRequest{ID: "ar-del", Status: entity.ApprovalStatusPending}, nil
}
func (s *stubCollectionUC) RequestAddField(ctx context.Context, workspaceID, actor, collectionID string, req collection.FieldInput) (*entity.ApprovalRequest, error) {
	return &entity.ApprovalRequest{ID: "ar-add-field", Status: entity.ApprovalStatusPending}, nil
}
func (s *stubCollectionUC) RequestDeleteField(ctx context.Context, workspaceID, actor, collectionID, fieldID string) (*entity.ApprovalRequest, error) {
	return &entity.ApprovalRequest{ID: "ar-del-field", Status: entity.ApprovalStatusPending}, nil
}
func (s *stubCollectionUC) UpdateCollectionMeta(ctx context.Context, workspaceID, actor, id string, req collection.UpdateCollectionMetaRequest) (*entity.Collection, error) {
	return &entity.Collection{ID: id, Name: req.Name}, nil
}
func (s *stubCollectionUC) UpdateFieldMeta(ctx context.Context, workspaceID, actor, collectionID, fieldID string, req collection.UpdateFieldMetaRequest) (*entity.CollectionField, error) {
	return &entity.CollectionField{ID: fieldID, Label: req.Label}, nil
}
func (s *stubCollectionUC) ApplyCollectionSchemaChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.ApprovalRequest, error) {
	return s.applyRes, s.applyErr
}
func (s *stubCollectionUC) ListRecords(ctx context.Context, workspaceID, collectionID string, req collection.ListRecordsRequest) ([]entity.CollectionRecord, int, error) {
	return []entity.CollectionRecord{{ID: "r1"}}, 1, nil
}
func (s *stubCollectionUC) DistinctValues(ctx context.Context, workspaceID, collectionID, fieldKey string, limit int, filter string) (*entity.DistinctResult, error) {
	return s.distinctRes, nil
}
func (s *stubCollectionUC) CreateRecord(ctx context.Context, workspaceID, actor, collectionID string, data map[string]any) (*entity.CollectionRecord, error) {
	return s.createRecordRes, s.createRecordErr
}
func (s *stubCollectionUC) UpdateRecord(ctx context.Context, workspaceID, actor, collectionID, recordID string, data map[string]any) (*entity.CollectionRecord, error) {
	return &entity.CollectionRecord{ID: recordID, Data: data}, nil
}
func (s *stubCollectionUC) DeleteRecord(ctx context.Context, workspaceID, actor, collectionID, recordID string) error {
	return nil
}
func (s *stubCollectionUC) BulkRecords(ctx context.Context, workspaceID, actor, collectionID string, req collection.BulkRecordsRequest) (*collection.BulkRecordsResult, error) {
	return &collection.BulkRecordsResult{Affected: len(req.IDs)}, nil
}

// --- helpers ------------------------------------------------------------------

func newAuthedRequest(method, path string, body []byte) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r = r.WithContext(ctxutil.SetWorkspaceID(r.Context(), "ws-test"))
	return withJWTUser(r, "tester@x.com")
}

// --- collection CRUD ---------------------------------------------------------

func TestCollectionHandler_List(t *testing.T) {
	uc := &stubCollectionUC{listResult: []entity.Collection{{ID: "c1", Slug: "s", Name: "N"}}}
	h := dashboard.NewCollectionHandler(uc, testLogger, testTr)
	r := newAuthedRequest(http.MethodGet, "/api/collections", nil)

	w := callHandler(h.List, r)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if !uc.listCalled {
		t.Fatal("expected ListCollections to be invoked")
	}
}

func TestCollectionHandler_Create_Returns202(t *testing.T) {
	uc := &stubCollectionUC{requestCreateRes: &entity.ApprovalRequest{ID: "ar-1", Status: entity.ApprovalStatusPending}}
	h := dashboard.NewCollectionHandler(uc, testLogger, testTr)
	body, _ := json.Marshal(collection.CreateCollectionRequest{
		Slug: "e", Name: "E", Fields: []collection.FieldInput{{Key: "t", Label: "T", Type: entity.ColFieldText}},
	})
	r := newAuthedRequest(http.MethodPost, "/api/collections", body)
	w := callHandler(h.Create, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202 (body=%s)", w.Code, w.Body.String())
	}
}

func TestCollectionHandler_Create_InvalidJSON(t *testing.T) {
	uc := &stubCollectionUC{}
	h := dashboard.NewCollectionHandler(uc, testLogger, testTr)
	r := newAuthedRequest(http.MethodPost, "/api/collections", []byte("{not json"))
	w := callHandler(h.Create, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 (body=%s)", w.Code, w.Body.String())
	}
}

func TestCollectionHandler_Delete_Returns202(t *testing.T) {
	uc := &stubCollectionUC{}
	h := dashboard.NewCollectionHandler(uc, testLogger, testTr)
	r := newAuthedRequest(http.MethodDelete, "/api/collections/c1", nil)
	r = withPathParam(r, "id", "c1")
	w := callHandler(h.Delete, r)
	if w.Code != http.StatusAccepted {
		t.Fatalf("got %d, want 202 (body=%s)", w.Code, w.Body.String())
	}
}

func TestCollectionHandler_ApplyApproval(t *testing.T) {
	uc := &stubCollectionUC{applyRes: &entity.ApprovalRequest{ID: "ar-1", Status: entity.ApprovalStatusApproved}}
	h := dashboard.NewCollectionHandler(uc, testLogger, testTr)
	r := newAuthedRequest(http.MethodPost, "/api/collections/approvals/ar-1/approve", nil)
	r = withPathParam(r, "approval_id", "ar-1")
	w := callHandler(h.ApplyApproval, r)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

// --- record CRUD -------------------------------------------------------------

func TestCollectionRecordHandler_ListReturnsShape(t *testing.T) {
	uc := &stubCollectionUC{}
	h := dashboard.NewCollectionRecordHandler(uc, testLogger, testTr)
	r := newAuthedRequest(http.MethodGet, "/api/collections/c1/records?limit=10&offset=0", nil)
	r = withPathParam(r, "id", "c1")
	w := callHandler(h.List, r)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %T", body["data"])
	}
	if _, ok := data["meta"]; !ok {
		t.Fatalf("expected meta in response, got %+v", data)
	}
}

func TestCollectionRecordHandler_Distinct_RequiresField(t *testing.T) {
	uc := &stubCollectionUC{}
	h := dashboard.NewCollectionRecordHandler(uc, testLogger, testTr)
	r := newAuthedRequest(http.MethodGet, "/api/collections/c1/records/distinct", nil)
	r = withPathParam(r, "id", "c1")
	w := callHandler(h.Distinct, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400 (body=%s)", w.Code, w.Body.String())
	}
}

func TestCollectionRecordHandler_Create(t *testing.T) {
	uc := &stubCollectionUC{createRecordRes: &entity.CollectionRecord{ID: "r-1", Data: map[string]any{"title": "x"}}}
	h := dashboard.NewCollectionRecordHandler(uc, testLogger, testTr)
	body, _ := json.Marshal(map[string]any{"data": map[string]any{"title": "x"}})
	r := newAuthedRequest(http.MethodPost, "/api/collections/c1/records", body)
	r = withPathParam(r, "id", "c1")
	w := callHandler(h.Create, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201 (body=%s)", w.Code, w.Body.String())
	}
}

func TestCollectionRecordHandler_Create_MissingData(t *testing.T) {
	uc := &stubCollectionUC{}
	h := dashboard.NewCollectionRecordHandler(uc, testLogger, testTr)
	body, _ := json.Marshal(map[string]any{})
	r := newAuthedRequest(http.MethodPost, "/api/collections/c1/records", body)
	r = withPathParam(r, "id", "c1")
	w := callHandler(h.Create, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", w.Code)
	}
}
