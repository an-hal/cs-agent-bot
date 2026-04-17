package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/collection"
	"github.com/rs/zerolog"
)

// CollectionRecordHandler implements record-level endpoints for a collection.
type CollectionRecordHandler struct {
	uc     collection.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewCollectionRecordHandler constructs a CollectionRecordHandler.
func NewCollectionRecordHandler(uc collection.Usecase, logger zerolog.Logger, tr tracer.Tracer) *CollectionRecordHandler {
	return &CollectionRecordHandler{uc: uc, logger: logger, tracer: tr}
}

// listRecordsResponse mirrors the shape required by spec §03-api-endpoints.
type listRecordsResponse struct {
	Data []any        `json:"data"`
	Meta recordsMeta  `json:"meta"`
}

type recordsMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// List godoc
// @Summary      List records with filter/sort/pagination
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id      path   string  true   "Collection UUID"
// @Param        limit   query  int     false  "page size (default 50, max 500)"
// @Param        offset  query  int     false  "offset (default 0)"
// @Param        sort    query  string  false  "e.g. created_at:desc, data.title:asc"
// @Param        filter  query  string  false  "filter DSL, e.g. data.category in [\"A\"]"
// @Param        search  query  string  false  "full-text search"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/collections/{id}/records [get]
func (h *CollectionRecordHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.record.handler.List")
	defer span.End()
	id := router.GetParam(r, "id")

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	recs, total, err := h.uc.ListRecords(ctx, ctxutil.GetWorkspaceID(ctx), id, collection.ListRecordsRequest{
		Limit:  limit,
		Offset: offset,
		Sort:   q.Get("sort"),
		Filter: q.Get("filter"),
		Search: q.Get("search"),
	})
	if err != nil {
		return err
	}
	data := make([]any, 0, len(recs))
	for _, rec := range recs {
		data = append(data, rec)
	}
	actualLimit := limit
	if actualLimit <= 0 {
		actualLimit = 50
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Records retrieved", listRecordsResponse{
		Data: data,
		Meta: recordsMeta{Total: total, Limit: actualLimit, Offset: offset},
	})
}

// Distinct godoc
// @Summary      Distinct values for a field
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id      path   string  true   "Collection UUID"
// @Param        field   query  string  true   "field key"
// @Param        limit   query  int     false  "max values to return (default 100, hard cap 500)"
// @Param        filter  query  string  false  "filter DSL"
// @Success      200  {object}  response.StandardResponse{data=entity.DistinctResult}
// @Router       /api/collections/{id}/records/distinct [get]
func (h *CollectionRecordHandler) Distinct(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.record.handler.Distinct")
	defer span.End()
	id := router.GetParam(r, "id")
	q := r.URL.Query()
	field := q.Get("field")
	if field == "" {
		return apperror.BadRequest("field query parameter required")
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	res, err := h.uc.DistinctValues(ctx, ctxutil.GetWorkspaceID(ctx), id, field, limit, q.Get("filter"))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Distinct values retrieved", res)
}

// Create godoc
// @Summary      Create record
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id    path  string  true  "Collection UUID"
// @Param        body  body  object  true  "Record data"
// @Success      201  {object}  response.StandardResponse{data=entity.CollectionRecord}
// @Router       /api/collections/{id}/records [post]
func (h *CollectionRecordHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.record.handler.Create")
	defer span.End()
	id := router.GetParam(r, "id")
	var req struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.Data == nil {
		return apperror.BadRequest("data field required")
	}
	out, err := h.uc.CreateRecord(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, req.Data)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Record created", out)
}

// Update godoc
// @Summary      Update record (PATCH merge)
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id         path  string  true  "Collection UUID"
// @Param        record_id  path  string  true  "Record UUID"
// @Param        body  body  object  true  "Partial record data"
// @Success      200  {object}  response.StandardResponse{data=entity.CollectionRecord}
// @Router       /api/collections/{id}/records/{record_id} [patch]
func (h *CollectionRecordHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.record.handler.Update")
	defer span.End()
	id := router.GetParam(r, "id")
	recordID := router.GetParam(r, "record_id")
	var req struct {
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if len(req.Data) == 0 {
		return apperror.BadRequest("data field required")
	}
	out, err := h.uc.UpdateRecord(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, recordID, req.Data)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Record updated", out)
}

// Delete godoc
// @Summary      Soft-delete record
// @Tags         Collections
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id         path  string  true  "Collection UUID"
// @Param        record_id  path  string  true  "Record UUID"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/collections/{id}/records/{record_id} [delete]
func (h *CollectionRecordHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.record.handler.Delete")
	defer span.End()
	id := router.GetParam(r, "id")
	recordID := router.GetParam(r, "record_id")
	if err := h.uc.DeleteRecord(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, recordID); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Record deleted", nil)
}

// Bulk godoc
// @Summary      Bulk records op (delete or update)
// @Tags         Collections
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id    path  string  true  "Collection UUID"
// @Param        body  body  collection.BulkRecordsRequest  true  "Bulk payload"
// @Success      200  {object}  response.StandardResponse{data=collection.BulkRecordsResult}
// @Router       /api/collections/{id}/records/bulk [post]
func (h *CollectionRecordHandler) Bulk(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "collection.record.handler.Bulk")
	defer span.End()
	id := router.GetParam(r, "id")
	var req collection.BulkRecordsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	res, err := h.uc.BulkRecords(ctx, ctxutil.GetWorkspaceID(ctx), actorEmail(r), id, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Bulk op complete", res)
}
