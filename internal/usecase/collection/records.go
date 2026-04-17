package collection

import (
	"context"
	"sort"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// fieldErrsToSlice converts the map produced by ValidateRecordData into the
// []FieldError slice apperror expects. Keys sorted for deterministic output.
func fieldErrsToSlice(m map[string]string) []apperror.FieldError {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]apperror.FieldError, 0, len(keys))
	for _, k := range keys {
		out = append(out, apperror.FieldError{Field: k, Message: m[k]})
	}
	return out
}

// ListRecords returns records matching the DSL filter/sort, with pagination.
func (u *usecase) ListRecords(ctx context.Context, workspaceID, collectionID string, req ListRecordsRequest) ([]entity.CollectionRecord, int, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, 0, err
	}
	fields, err := u.loadFields(ctx, c.ID)
	if err != nil {
		return nil, 0, err
	}

	filter, err := parseFilterDSL(req.Filter, fields, 1)
	if err != nil {
		return nil, 0, err
	}
	sortKey, sortType, desc, err := parseSort(req.Sort, fields)
	if err != nil {
		return nil, 0, err
	}

	return u.recordRepo.List(ctx, entity.CollectionRecordListOptions{
		CollectionID: c.ID,
		Limit:        req.Limit,
		Offset:       req.Offset,
		SortKey:      sortKey,
		SortType:     sortType,
		SortDesc:     desc,
		Search:       req.Search,
		FilterSQL:    filter.sql,
		FilterArgs:   filter.args,
	})
}

// DistinctValues returns the set of distinct non-empty values on a field.
func (u *usecase) DistinctValues(ctx context.Context, workspaceID, collectionID, fieldKey string, limit int, filter string) (*entity.DistinctResult, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, err
	}
	fields, err := u.loadFields(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	var targetType string
	found := false
	for _, f := range fields {
		if f.Key == fieldKey {
			targetType = f.Type
			found = true
			break
		}
	}
	if !found {
		return nil, apperror.ValidationError("unknown field key: " + fieldKey)
	}

	pf, err := parseFilterDSL(filter, fields, 1)
	if err != nil {
		return nil, err
	}

	values, truncated, err := u.recordRepo.Distinct(ctx, entity.DistinctOptions{
		CollectionID: c.ID,
		FieldKey:     fieldKey,
		FieldType:    targetType,
		Limit:        limit,
		FilterSQL:    pf.sql,
		FilterArgs:   pf.args,
	})
	if err != nil {
		return nil, err
	}
	return &entity.DistinctResult{
		Field:     fieldKey,
		Values:    values,
		Truncated: truncated,
	}, nil
}

// CreateRecord validates data against schema and inserts a new record.
func (u *usecase) CreateRecord(ctx context.Context, workspaceID, actor, collectionID string, data map[string]any) (*entity.CollectionRecord, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, err
	}
	fields, err := u.loadFields(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	// Hard limit: max 10k active records per collection.
	count, err := u.recordRepo.CountActiveByCollection(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	if count >= entity.MaxRecordsPerCollection {
		return nil, apperror.ValidationError("collection has reached max records (10000)")
	}

	fieldErrs, err := ValidateRecordData(ctx, fields, data, true, u.linkClientCheck(workspaceID))
	if err != nil {
		return nil, apperror.ValidationErrorWithFields("validation failed", fieldErrsToSlice(fieldErrs))
	}

	rec := &entity.CollectionRecord{
		CollectionID: c.ID,
		Data:         data,
		CreatedBy:    actor,
	}
	out, err := u.recordRepo.Create(ctx, rec)
	if err != nil {
		return nil, err
	}
	u.audit(ctx, workspaceID, actor, "collection_record.create", c.ID, c.Name, out.ID)
	return out, nil
}

// UpdateRecord PATCH-merges data into an existing record after validation.
func (u *usecase) UpdateRecord(ctx context.Context, workspaceID, actor, collectionID, recordID string, data map[string]any) (*entity.CollectionRecord, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, err
	}
	existing, err := u.recordRepo.Get(ctx, c.ID, recordID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.NotFound("collection_record", recordID)
	}
	fields, err := u.loadFields(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	// Validate the merged result so required-field constraints still hold.
	merged := map[string]any{}
	for k, v := range existing.Data {
		merged[k] = v
	}
	for k, v := range data {
		merged[k] = v
	}

	fieldErrs, err := ValidateRecordData(ctx, fields, merged, true, u.linkClientCheck(workspaceID))
	if err != nil {
		return nil, apperror.ValidationErrorWithFields("validation failed", fieldErrsToSlice(fieldErrs))
	}

	out, err := u.recordRepo.Update(ctx, c.ID, recordID, data)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("collection_record", recordID)
	}
	u.audit(ctx, workspaceID, actor, "collection_record.update", c.ID, c.Name, recordID)
	return out, nil
}

// DeleteRecord soft-deletes a single record.
func (u *usecase) DeleteRecord(ctx context.Context, workspaceID, actor, collectionID, recordID string) error {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return err
	}
	if err := u.recordRepo.SoftDelete(ctx, c.ID, recordID); err != nil {
		return err
	}
	u.audit(ctx, workspaceID, actor, "collection_record.delete", c.ID, c.Name, recordID)
	return nil
}

// BulkRecords applies a delete-many or update-many op.
func (u *usecase) BulkRecords(ctx context.Context, workspaceID, actor, collectionID string, req BulkRecordsRequest) (*BulkRecordsResult, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, apperror.ValidationError("ids required")
	}

	switch req.Op {
	case "delete":
		n, err := u.recordRepo.BulkSoftDelete(ctx, c.ID, req.IDs)
		if err != nil {
			return nil, err
		}
		u.audit(ctx, workspaceID, actor, "collection_record.bulk_delete", c.ID, c.Name, "")
		return &BulkRecordsResult{Affected: n}, nil

	case "update":
		if len(req.Data) == 0 {
			return nil, apperror.ValidationError("data required for update op")
		}
		fields, err := u.loadFields(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		// Validate the patch in isolation — required-field shape is respected
		// because unchanged columns keep their prior values.
		fieldErrs, err := ValidateRecordData(ctx, fields, req.Data, true, u.linkClientCheck(workspaceID))
		if err != nil && fieldErrs != nil {
			// Strip "required" errors for keys not present in the patch — bulk
			// updates are PATCH-like: only keys in the patch are touched.
			for k, v := range fieldErrs {
				if v == "required" {
					if _, present := req.Data[k]; !present {
						delete(fieldErrs, k)
					}
				}
			}
			if len(fieldErrs) > 0 {
				return nil, apperror.ValidationErrorWithFields("validation failed", fieldErrsToSlice(fieldErrs))
			}
		}
		n, err := u.recordRepo.BulkUpdate(ctx, c.ID, req.IDs, req.Data)
		if err != nil {
			return nil, err
		}
		u.audit(ctx, workspaceID, actor, "collection_record.bulk_update", c.ID, c.Name, "")
		return &BulkRecordsResult{Affected: n}, nil
	}
	return nil, apperror.ValidationError("unsupported bulk op: " + req.Op)
}

// linkClientCheck returns a validation callback that verifies a client id exists
// in the current workspace. nil-safe: if masterDataRepo is not wired, the check
// is skipped (useful for unit tests that don't touch master_data).
func (u *usecase) linkClientCheck(workspaceID string) func(ctx context.Context, id string) (bool, error) {
	if u.masterDataRepo == nil {
		return nil
	}
	return func(ctx context.Context, id string) (bool, error) {
		md, err := u.masterDataRepo.GetByID(ctx, workspaceID, id)
		if err != nil {
			return false, err
		}
		return md != nil, nil
	}
}
