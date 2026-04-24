package collection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// ---------------------------------------------------------------------------
// Fake repositories. Keep behaviour simple and observable from tests.
// ---------------------------------------------------------------------------

type fakeCollectionRepo struct {
	items       map[string]*entity.Collection
	countByWS   int
	createErr   error
	softDeleted map[string]bool
}

func newFakeCollectionRepo() *fakeCollectionRepo {
	return &fakeCollectionRepo{
		items:       map[string]*entity.Collection{},
		softDeleted: map[string]bool{},
	}
}

func (f *fakeCollectionRepo) List(ctx context.Context, workspaceID string) ([]entity.Collection, error) {
	out := []entity.Collection{}
	for _, c := range f.items {
		if c.WorkspaceID == workspaceID && !f.softDeleted[c.ID] {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (f *fakeCollectionRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.Collection, error) {
	c, ok := f.items[id]
	if !ok || c.WorkspaceID != workspaceID || f.softDeleted[id] {
		return nil, nil
	}
	copy := *c
	return &copy, nil
}

func (f *fakeCollectionRepo) GetBySlug(ctx context.Context, workspaceID, slug string) (*entity.Collection, error) {
	for _, c := range f.items {
		if c.WorkspaceID == workspaceID && c.Slug == slug && !f.softDeleted[c.ID] {
			copy := *c
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeCollectionRepo) Create(ctx context.Context, c *entity.Collection) (*entity.Collection, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	id := c.ID
	if id == "" {
		id = "col-" + c.Slug
	}
	c.ID = id
	c.CreatedAt = time.Now()
	c.UpdatedAt = c.CreatedAt
	f.items[id] = c
	return c, nil
}

func (f *fakeCollectionRepo) UpdateMeta(ctx context.Context, workspaceID, id, name, description, icon string, permissions map[string]any) (*entity.Collection, error) {
	c, ok := f.items[id]
	if !ok || c.WorkspaceID != workspaceID || f.softDeleted[id] {
		return nil, nil
	}
	c.Name = name
	c.Description = description
	c.Icon = icon
	c.Permissions = permissions
	return c, nil
}

func (f *fakeCollectionRepo) SoftDelete(ctx context.Context, workspaceID, id string) error {
	c, ok := f.items[id]
	if !ok || c.WorkspaceID != workspaceID {
		return errors.New("not found")
	}
	f.softDeleted[id] = true
	return nil
}

func (f *fakeCollectionRepo) CountActiveByWorkspace(ctx context.Context, workspaceID string) (int, error) {
	if f.countByWS > 0 {
		return f.countByWS, nil
	}
	n := 0
	for _, c := range f.items {
		if c.WorkspaceID == workspaceID && !f.softDeleted[c.ID] {
			n++
		}
	}
	return n, nil
}

type fakeFieldRepo struct {
	items     map[string]*entity.CollectionField
	byColl    map[string][]*entity.CollectionField
	forcedCount int
	strippedKey string
}

func newFakeFieldRepo() *fakeFieldRepo {
	return &fakeFieldRepo{
		items:  map[string]*entity.CollectionField{},
		byColl: map[string][]*entity.CollectionField{},
	}
}

func (f *fakeFieldRepo) ListByCollection(ctx context.Context, collectionID string) ([]entity.CollectionField, error) {
	out := []entity.CollectionField{}
	for _, fd := range f.byColl[collectionID] {
		out = append(out, *fd)
	}
	return out, nil
}

func (f *fakeFieldRepo) GetByID(ctx context.Context, collectionID, id string) (*entity.CollectionField, error) {
	fd, ok := f.items[id]
	if !ok || fd.CollectionID != collectionID {
		return nil, nil
	}
	copy := *fd
	return &copy, nil
}

func (f *fakeFieldRepo) GetByKey(ctx context.Context, collectionID, key string) (*entity.CollectionField, error) {
	for _, fd := range f.byColl[collectionID] {
		if fd.Key == key {
			copy := *fd
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeFieldRepo) Create(ctx context.Context, fd *entity.CollectionField) (*entity.CollectionField, error) {
	if fd.ID == "" {
		fd.ID = "f-" + fd.CollectionID + "-" + fd.Key
	}
	f.items[fd.ID] = fd
	f.byColl[fd.CollectionID] = append(f.byColl[fd.CollectionID], fd)
	return fd, nil
}

func (f *fakeFieldRepo) UpdateMeta(ctx context.Context, collectionID, id, label string, required bool, order int, options map[string]any) (*entity.CollectionField, error) {
	fd, ok := f.items[id]
	if !ok || fd.CollectionID != collectionID {
		return nil, nil
	}
	fd.Label = label
	fd.Required = required
	fd.Order = order
	fd.Options = options
	return fd, nil
}

func (f *fakeFieldRepo) Delete(ctx context.Context, collectionID, id string) error {
	fd, ok := f.items[id]
	if !ok || fd.CollectionID != collectionID {
		return errors.New("not found")
	}
	delete(f.items, id)
	list := f.byColl[collectionID]
	filtered := list[:0]
	for _, existing := range list {
		if existing.ID != id {
			filtered = append(filtered, existing)
		}
	}
	f.byColl[collectionID] = filtered
	return nil
}

func (f *fakeFieldRepo) CountByCollection(ctx context.Context, collectionID string) (int, error) {
	if f.forcedCount > 0 {
		return f.forcedCount, nil
	}
	return len(f.byColl[collectionID]), nil
}

func (f *fakeFieldRepo) StripKeyFromRecords(ctx context.Context, collectionID, key string) error {
	f.strippedKey = key
	return nil
}

type fakeRecordRepo struct {
	count      int
	created    []entity.CollectionRecord
	distinct   []string
	listResult []entity.CollectionRecord
	listTotal  int
}

func (f *fakeRecordRepo) List(ctx context.Context, opts entity.CollectionRecordListOptions) ([]entity.CollectionRecord, int, error) {
	return f.listResult, f.listTotal, nil
}

func (f *fakeRecordRepo) Get(ctx context.Context, collectionID, id string) (*entity.CollectionRecord, error) {
	for i := range f.created {
		if f.created[i].ID == id {
			c := f.created[i]
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeRecordRepo) CountActiveByCollection(ctx context.Context, collectionID string) (int, error) {
	return f.count, nil
}

func (f *fakeRecordRepo) Create(ctx context.Context, rec *entity.CollectionRecord) (*entity.CollectionRecord, error) {
	if rec.ID == "" {
		rec.ID = "rec-new"
	}
	rec.CreatedAt = time.Now()
	rec.UpdatedAt = rec.CreatedAt
	f.created = append(f.created, *rec)
	f.count++
	return rec, nil
}

func (f *fakeRecordRepo) Update(ctx context.Context, collectionID, id string, data map[string]any) (*entity.CollectionRecord, error) {
	for i := range f.created {
		if f.created[i].ID == id {
			for k, v := range data {
				f.created[i].Data[k] = v
			}
			f.created[i].UpdatedAt = time.Now()
			c := f.created[i]
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeRecordRepo) SoftDelete(ctx context.Context, collectionID, id string) error {
	for i := range f.created {
		if f.created[i].ID == id {
			now := time.Now()
			f.created[i].DeletedAt = &now
			return nil
		}
	}
	return errors.New("not found")
}

func (f *fakeRecordRepo) BulkSoftDelete(ctx context.Context, collectionID string, ids []string) (int, error) {
	return len(ids), nil
}

func (f *fakeRecordRepo) BulkUpdate(ctx context.Context, collectionID string, ids []string, patch map[string]any) (int, error) {
	return len(ids), nil
}

func (f *fakeRecordRepo) Distinct(ctx context.Context, opts entity.DistinctOptions) ([]string, bool, error) {
	return f.distinct, false, nil
}

type fakeApprovalRepo struct {
	items   map[string]*entity.ApprovalRequest
	updates []string
}

func newFakeApprovalRepo() *fakeApprovalRepo {
	return &fakeApprovalRepo{items: map[string]*entity.ApprovalRequest{}}
}

func (f *fakeApprovalRepo) Create(ctx context.Context, a *entity.ApprovalRequest) (*entity.ApprovalRequest, error) {
	if a.ID == "" {
		a.ID = "ar-" + a.Description
	}
	f.items[a.ID] = a
	return a, nil
}

func (f *fakeApprovalRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ApprovalRequest, error) {
	a, ok := f.items[id]
	if !ok || a.WorkspaceID != workspaceID {
		return nil, nil
	}
	copy := *a
	return &copy, nil
}

func (f *fakeApprovalRepo) UpdateStatus(ctx context.Context, workspaceID, id, newStatus, checkerEmail, reason string) error {
	a, ok := f.items[id]
	if !ok {
		return errors.New("not found")
	}
	a.Status = newStatus
	a.CheckerEmail = checkerEmail
	f.updates = append(f.updates, id+":"+newStatus)
	return nil
}

type fakeLogRepo struct{ entries int }

func (f *fakeLogRepo) AppendLog(ctx context.Context, entry entity.ActionLog) error  { return nil }
func (f *fakeLogRepo) SentTodayAlready(ctx context.Context, companyID string) (bool, error) {
	return false, nil
}
func (f *fakeLogRepo) MessageIDExists(ctx context.Context, messageID string) (bool, error) {
	return false, nil
}
func (f *fakeLogRepo) AppendActivity(ctx context.Context, entry entity.ActivityLog) error {
	f.entries++
	return nil
}
func (f *fakeLogRepo) GetActivities(ctx context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	return nil, 0, nil
}
func (f *fakeLogRepo) GetActivityStats(ctx context.Context, workspaceIDs []string) (entity.ActivityStats, error) {
	return entity.ActivityStats{}, nil
}
func (f *fakeLogRepo) GetRecentActivities(ctx context.Context, workspaceIDs []string, since time.Time, limit int) ([]entity.ActivityLog, error) {
	return nil, nil
}
func (f *fakeLogRepo) GetCompanySummary(ctx context.Context, workspaceIDs []string, companyID string) (*entity.CompanySummary, error) {
	return nil, nil
}
func (f *fakeLogRepo) GetRecentActionLogs(_ context.Context, _ []string, _ int) ([]entity.ActionLog, error) {
	return nil, nil
}
func (f *fakeLogRepo) GetActionLogSummary(_ context.Context, _ []string, _ time.Time) (*entity.ActionLogSummary, error) {
	return &entity.ActionLogSummary{}, nil
}
func (f *fakeLogRepo) GetTodayActionLogs(_ context.Context, _ []string, _ int) ([]entity.ActionLog, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Builder helpers.
// ---------------------------------------------------------------------------

func buildUsecase(t *testing.T) (*usecase, *fakeCollectionRepo, *fakeFieldRepo, *fakeRecordRepo, *fakeApprovalRepo, *fakeLogRepo) {
	t.Helper()
	cRepo := newFakeCollectionRepo()
	fRepo := newFakeFieldRepo()
	rRepo := &fakeRecordRepo{}
	aRepo := newFakeApprovalRepo()
	lRepo := &fakeLogRepo{}
	var mdRepo repository.MasterDataRepository // nil skips link_client checks
	u := New(cRepo, fRepo, rRepo, aRepo, lRepo, mdRepo, tracer.NewNoopTracer(), zerolog.Nop()).(*usecase)
	return u, cRepo, fRepo, rRepo, aRepo, lRepo
}

// ---------------------------------------------------------------------------
// Tests.
// ---------------------------------------------------------------------------

func TestRequestCreateCollection_ValidationRules(t *testing.T) {
	ctx := context.Background()
	u, _, _, _, _, _ := buildUsecase(t)

	base := CreateCollectionRequest{
		Slug: "events",
		Name: "Events",
		Fields: []FieldInput{
			{Key: "title", Label: "Title", Type: entity.ColFieldText, Required: true},
		},
	}

	tests := []struct {
		name      string
		mutate    func(r *CreateCollectionRequest)
		wantErr   string
	}{
		{name: "happy", mutate: func(r *CreateCollectionRequest) {}},
		{name: "bad slug", mutate: func(r *CreateCollectionRequest) { r.Slug = "A" }, wantErr: "slug"},
		{name: "empty name", mutate: func(r *CreateCollectionRequest) { r.Name = "" }, wantErr: "name"},
		{name: "no fields", mutate: func(r *CreateCollectionRequest) { r.Fields = nil }, wantErr: "at least one field"},
		{name: "duplicate field key", mutate: func(r *CreateCollectionRequest) {
			r.Fields = append(r.Fields, FieldInput{Key: "title", Label: "T2", Type: entity.ColFieldText})
		}, wantErr: "duplicate"},
		{name: "invalid field type", mutate: func(r *CreateCollectionRequest) {
			r.Fields[0].Type = "banana"
		}, wantErr: "invalid field type"},
		{name: "enum without choices", mutate: func(r *CreateCollectionRequest) {
			r.Fields[0].Type = entity.ColFieldEnum
			r.Fields[0].Options = map[string]any{}
		}, wantErr: "choices"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.Fields = append([]FieldInput(nil), base.Fields...)
			tc.mutate(&req)
			_, err := u.RequestCreateCollection(ctx, "ws-1", "alice@x.com", req)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestRequestCreateCollection_HardLimit(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.countByWS = entity.MaxCollectionsPerWorkspace

	_, err := u.RequestCreateCollection(ctx, "ws-1", "alice@x.com", CreateCollectionRequest{
		Slug:   "events",
		Name:   "Events",
		Fields: []FieldInput{{Key: "t", Label: "T", Type: entity.ColFieldText}},
	})
	if err == nil || !strings.Contains(err.Error(), "max collections") {
		t.Fatalf("expected hard-limit error, got %v", err)
	}
}

func TestRequestAddField_HardLimit(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)
	// Seed a collection.
	c := &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Slug: "e", Name: "E"}
	cRepo.items["c1"] = c
	fRepo.forcedCount = entity.MaxFieldsPerCollection

	_, err := u.RequestAddField(ctx, "ws-1", "alice@x.com", "c1", FieldInput{
		Key: "extra", Label: "X", Type: entity.ColFieldText,
	})
	if err == nil || !strings.Contains(err.Error(), "max fields") {
		t.Fatalf("expected hard-limit error, got %v", err)
	}
}

func TestRecordCreate_HardLimit(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, rRepo, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Slug: "e", Name: "E"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "title", Type: entity.ColFieldText, Required: true},
	}
	rRepo.count = entity.MaxRecordsPerCollection

	_, err := u.CreateRecord(ctx, "ws-1", "alice@x.com", "c1", map[string]any{"title": "x"})
	if err == nil || !strings.Contains(err.Error(), "max records") {
		t.Fatalf("expected hard-limit error, got %v", err)
	}
}

func TestRecordCreate_WorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-A", Slug: "e", Name: "E"}

	_, err := u.CreateRecord(ctx, "ws-B", "alice@x.com", "c1", map[string]any{"title": "x"})
	if err == nil {
		t.Fatalf("expected cross-workspace access to be rejected")
	}
}

func TestRecordCreate_AuditLogged(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, lRepo := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Slug: "e", Name: "E"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "title", Type: entity.ColFieldText, Required: true},
	}

	_, err := u.CreateRecord(ctx, "ws-1", "alice@x.com", "c1", map[string]any{"title": "Hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lRepo.entries == 0 {
		t.Fatal("expected an activity log entry to be appended")
	}
}

func TestApplyCollectionSchemaChange_CreateCollection(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, aRepo, _ := buildUsecase(t)

	ar, err := u.RequestCreateCollection(ctx, "ws-1", "maker@x.com", CreateCollectionRequest{
		Slug: "events",
		Name: "Events",
		Fields: []FieldInput{
			{Key: "title", Label: "T", Type: entity.ColFieldText, Required: true},
			{Key: "count", Label: "N", Type: entity.ColFieldNumber},
		},
	})
	if err != nil {
		t.Fatalf("request create: %v", err)
	}
	if ar.Status != entity.ApprovalStatusPending {
		t.Fatalf("approval should be pending, got %q", ar.Status)
	}

	// Maker cannot approve own.
	if _, err := u.ApplyCollectionSchemaChange(ctx, "ws-1", ar.ID, "maker@x.com"); err == nil {
		t.Fatal("maker should not be able to approve own request")
	}

	// Checker applies.
	out, err := u.ApplyCollectionSchemaChange(ctx, "ws-1", ar.ID, "checker@x.com")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out.Status != entity.ApprovalStatusApproved {
		t.Fatalf("status after apply = %q, want approved", out.Status)
	}
	// One collection + 2 fields should now exist.
	if len(cRepo.items) != 1 {
		t.Fatalf("expected 1 collection, got %d", len(cRepo.items))
	}
	var cid string
	for id := range cRepo.items {
		cid = id
	}
	if len(fRepo.byColl[cid]) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fRepo.byColl[cid]))
	}
	// Approval repo saw the update.
	if len(aRepo.updates) != 1 {
		t.Fatalf("expected 1 approval update, got %d", len(aRepo.updates))
	}
}

func TestApplyCollectionSchemaChange_DeleteField(t *testing.T) {
	ctx := context.Background()
	u, cRepo, fRepo, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1", Slug: "e", Name: "E"}
	fRepo.byColl["c1"] = []*entity.CollectionField{
		{ID: "f1", CollectionID: "c1", Key: "category", Type: entity.ColFieldText},
	}
	fRepo.items["f1"] = fRepo.byColl["c1"][0]

	ar, err := u.RequestDeleteField(ctx, "ws-1", "maker@x.com", "c1", "f1")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if _, err := u.ApplyCollectionSchemaChange(ctx, "ws-1", ar.ID, "checker@x.com"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(fRepo.byColl["c1"]) != 0 {
		t.Fatalf("field should be deleted, still have %d", len(fRepo.byColl["c1"]))
	}
	if fRepo.strippedKey != "category" {
		t.Fatalf("expected StripKeyFromRecords('category'), got %q", fRepo.strippedKey)
	}
}

func TestDistinctValues_UnknownField(t *testing.T) {
	ctx := context.Background()
	u, cRepo, _, _, _, _ := buildUsecase(t)
	cRepo.items["c1"] = &entity.Collection{ID: "c1", WorkspaceID: "ws-1"}

	_, err := u.DistinctValues(ctx, "ws-1", "c1", "ghost", 100, "")
	if err == nil {
		t.Fatal("expected unknown field error")
	}
}
