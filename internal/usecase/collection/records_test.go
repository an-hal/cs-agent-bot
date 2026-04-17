package collection

import (
	"context"
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

func TestListCollections_And_GetCollection(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)

	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Slug: "s", Name: "A"}
	cRepo.items["c2"] = &entity.Collection{ID: "c2", WorkspaceID: "ws-1", Slug: "t", Name: "B"}
	cRepo.items["c3"] = &entity.Collection{ID: "c3", WorkspaceID: "ws-2", Slug: "u", Name: "Other WS"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "title", Type: entity.ColFieldText},
	}

	list, err := u.ListCollections(ctx, "ws-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 ws-1 collections, got %d", len(list))
	}

	c, err := u.GetCollection(ctx, "ws-1", "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if c.FieldCount != 1 || len(c.Fields) != 1 {
		t.Fatalf("expected 1 field attached, got %d (len=%d)", c.FieldCount, len(c.Fields))
	}

	if _, err := u.GetCollection(ctx, "ws-1", "missing"); err == nil {
		t.Fatal("expected not-found error")
	}

	if _, err := u.ListCollections(ctx, ""); err == nil {
		t.Fatal("expected validation error for empty workspace")
	}
}

func TestListRecords_RoundTrip(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, rRepo, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "category", Type: entity.ColFieldText},
	}
	rRepo.listResult = []entity.CollectionRecord{{ID: "r1", Data: map[string]any{"category": "A"}}}
	rRepo.listTotal = 1

	got, total, err := u.ListRecords(ctx, "ws-1", "c1", ListRecordsRequest{
		Filter: `data.category in ["A"]`,
		Sort:   `data.category:asc`,
	})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("got total=%d len=%d, want 1/1", total, len(got))
	}
}

func TestListRecords_BadFilterBubbles(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "category", Type: entity.ColFieldText},
	}

	_, _, err := u.ListRecords(ctx, "ws-1", "c1", ListRecordsRequest{Filter: "bogus"})
	if err == nil {
		t.Fatal("expected error from bad filter")
	}
}

func TestUpdateRecord_Merges(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, rRepo, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "title", Type: entity.ColFieldText, Required: true},
		{ID: "f2", CollectionID: "c1", Key: "note", Type: entity.ColFieldText},
	}
	rRepo.created = append(rRepo.created, entity.CollectionRecord{
		ID: "r1", CollectionID: "c1", Data: map[string]any{"title": "hello"},
	})

	out, err := u.UpdateRecord(ctx, "ws-1", "alice@x.com", "c1", "r1", map[string]any{"note": "yo"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if out.Data["title"] != "hello" || out.Data["note"] != "yo" {
		t.Fatalf("merge wrong: %+v", out.Data)
	}
}

func TestUpdateRecord_NotFound(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}

	_, err := u.UpdateRecord(ctx, "ws-1", "alice@x.com", "c1", "ghost", map[string]any{"title": "x"})
	if err == nil {
		t.Fatal("expected not-found")
	}
}

func TestDeleteRecord(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, rRepo, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	rRepo.created = append(rRepo.created, entity.CollectionRecord{ID: "r1", CollectionID: "c1"})

	if err := u.DeleteRecord(ctx, "ws-1", "alice@x.com", "c1", "r1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestBulkRecords_Delete(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	res, err := u.BulkRecords(ctx, "ws-1", "a@x.com", "c1", BulkRecordsRequest{Op: "delete", IDs: []string{"r1", "r2"}})
	if err != nil {
		t.Fatalf("bulk delete: %v", err)
	}
	if res.Affected != 2 {
		t.Fatalf("expected affected=2, got %d", res.Affected)
	}
}

func TestBulkRecords_Update_StripsRequiredErrorsForPatch(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "title", Type: entity.ColFieldText, Required: true},
		{ID: "f2", CollectionID: "c1", Key: "note", Type: entity.ColFieldText},
	}
	// Patch only touches `note`, so the `title` required-missing error must be dropped.
	res, err := u.BulkRecords(ctx, "ws-1", "a@x.com", "c1", BulkRecordsRequest{
		Op:   "update",
		IDs:  []string{"r1"},
		Data: map[string]any{"note": "ok"},
	})
	if err != nil {
		t.Fatalf("bulk update: %v", err)
	}
	if res.Affected != 1 {
		t.Fatalf("expected affected=1, got %d", res.Affected)
	}
}

func TestBulkRecords_UnsupportedOp(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}

	_, err := u.BulkRecords(ctx, "ws-1", "a@x.com", "c1", BulkRecordsRequest{Op: "ghost", IDs: []string{"r1"}})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported op error, got %v", err)
	}
}

func TestDistinctValues_Success(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, rRepo, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "category", Type: entity.ColFieldText},
	}
	rRepo.distinct = []string{"A", "B"}

	out, err := u.DistinctValues(ctx, "ws-1", "c1", "category", 100, "")
	if err != nil {
		t.Fatalf("distinct: %v", err)
	}
	if len(out.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(out.Values))
	}
}

func TestUpdateCollectionMeta_Succeeds(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Name: "Old"}

	out, err := u.UpdateCollectionMeta(ctx, "ws-1", "a@x.com", "c1", UpdateCollectionMetaRequest{Name: "New"})
	if err != nil {
		t.Fatalf("update meta: %v", err)
	}
	if out.Name != "New" {
		t.Fatalf("expected name=New, got %q", out.Name)
	}
}

func TestUpdateFieldMeta_NoType_Change(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	fld := &entity.CollectionField{ID: "f1", CollectionID: "c1", Key: "title", Label: "Old", Type: entity.ColFieldText}
	fRepo.items["f1"] = fld
	fRepo.byColl["c1"] = []*entity.CollectionField{fld}

	out, err := u.UpdateFieldMeta(ctx, "ws-1", "a@x.com", "c1", "f1", UpdateFieldMetaRequest{Label: "New"})
	if err != nil {
		t.Fatalf("update field meta: %v", err)
	}
	if out.Label != "New" {
		t.Fatalf("expected label=New, got %q", out.Label)
	}
}

func TestRequestDeleteCollection(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Slug: "e", Name: "Events"}

	ar, err := u.RequestDeleteCollection(ctx, "ws-1", "alice@x.com", "c1")
	if err != nil {
		t.Fatalf("request delete: %v", err)
	}
	if ar.Status != entity.ApprovalStatusPending {
		t.Fatalf("expected pending, got %q", ar.Status)
	}
}

func TestApplyCollectionSchemaChange_DeleteCollection(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Slug: "e", Name: "Events"}

	ar, err := u.RequestDeleteCollection(ctx, "ws-1", "maker@x.com", "c1")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if _, err := u.ApplyCollectionSchemaChange(ctx, "ws-1", ar.ID, "checker@x.com"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !cRepo.softDeleted["c1"] {
		t.Fatal("expected c1 to be soft-deleted")
	}
}

func TestApplyCollectionSchemaChange_AddField(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}

	ar, err := u.RequestAddField(ctx, "ws-1", "maker@x.com", "c1", FieldInput{
		Key: "cat", Label: "Cat", Type: entity.ColFieldText, Order: 1,
	})
	if err != nil {
		t.Fatalf("request add field: %v", err)
	}
	if _, err := u.ApplyCollectionSchemaChange(ctx, "ws-1", ar.ID, "checker@x.com"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(fRepo.byColl["c1"]) != 1 {
		t.Fatalf("expected 1 field after approval, got %d", len(fRepo.byColl["c1"]))
	}
}

func TestApplyCollectionSchemaChange_RejectsWrongType(t *testing.T) {
	ctx := context.Background()
	u, _, _, _, aRepo, _ := buildUsecase(t)
	aRepo.items["ar-1"] = &entity.ApprovalRequest{
		ID:          "ar-1",
		WorkspaceID: "ws-1",
		RequestType: "something_else",
		Status:      entity.ApprovalStatusPending,
	}
	_, err := u.ApplyCollectionSchemaChange(ctx, "ws-1", "ar-1", "checker@x.com")
	if err == nil || !strings.Contains(err.Error(), "collection_schema_change") {
		t.Fatalf("expected wrong-type error, got %v", err)
	}
}

func TestApplyCollectionSchemaChange_RejectsNotPending(t *testing.T) {
	ctx := context.Background()
	u, _, _, _, aRepo, _ := buildUsecase(t)
	aRepo.items["ar-1"] = &entity.ApprovalRequest{
		ID:          "ar-1",
		WorkspaceID: "ws-1",
		RequestType: entity.CollectionSchemaChangeType,
		Status:      entity.ApprovalStatusApproved,
	}
	_, err := u.ApplyCollectionSchemaChange(ctx, "ws-1", "ar-1", "checker@x.com")
	if err == nil || !strings.Contains(err.Error(), "not pending") {
		t.Fatalf("expected not-pending error, got %v", err)
	}
}

func TestRequestAddField_DuplicateKey(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "title", Type: entity.ColFieldText},
	}
	_, err := u.RequestAddField(ctx, "ws-1", "a@x.com", "c1", FieldInput{
		Key: "title", Label: "Dup", Type: entity.ColFieldText,
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}
