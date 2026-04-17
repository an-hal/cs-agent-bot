package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/master_data"
	"github.com/rs/zerolog"
)

// MasterDataHandler implements all /master-data/clients endpoints.
type MasterDataHandler struct {
	uc     master_data.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

// NewMasterDataHandler constructs a MasterDataHandler.
func NewMasterDataHandler(uc master_data.Usecase, logger zerolog.Logger, tr tracer.Tracer) *MasterDataHandler {
	return &MasterDataHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List master data records
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        offset query int false "Offset"
// @Param        limit  query int false "Limit (max 200)"
// @Param        stage  query string false "Comma-separated stages"
// @Param        search query string false "Search company_name/pic_name/company_id"
// @Param        risk_flag query string false "High|Mid|Low|None"
// @Param        bot_active query bool false "Filter by bot_active"
// @Param        payment_status query string false "Exact match"
// @Param        expiry_within query int false "Days to expiry upper bound"
// @Param        sort_by query string false "updated_at|company_name|stage|created_at|contract_end|final_price"
// @Param        sort_dir query string false "asc|desc"
// @Success      200 {object} response.StandardResponseWithMeta{data=[]entity.MasterData}
// @Router       /api/master-data/clients [get]
func (h *MasterDataHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataList")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	q := r.URL.Query()
	pag := pagination.FromRequest(r)
	filter := entity.MasterDataFilter{
		WorkspaceIDs:  []string{wsID},
		Search:        q.Get("search"),
		RiskFlag:      q.Get("risk_flag"),
		PaymentStatus: q.Get("payment_status"),
		SortBy:        q.Get("sort_by"),
		SortDir:       q.Get("sort_dir"),
		Offset:        pag.Offset,
		Limit:         pag.Limit,
	}
	if v := q.Get("stage"); v != "" {
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				filter.Stages = append(filter.Stages, s)
			}
		}
	}
	if v := q.Get("bot_active"); v != "" {
		b := v == "true" || v == "1"
		filter.BotActive = &b
	}
	if v := q.Get("expiry_within"); v != "" {
		n, _ := strconv.Atoi(v)
		filter.ExpiryWithin = n
	}

	rows, total, err := h.uc.List(ctx, wsID, filter)
	if err != nil {
		return err
	}
	if rows == nil {
		rows = []entity.MasterData{}
	}
	meta := pagination.Meta{Total: total, Offset: filter.Offset, Limit: filter.Limit}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Master Data", meta, rows)
}

// Get godoc
// @Summary      Get a single master data record by id
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id path string true "Master data UUID"
// @Success      200 {object} response.StandardResponse{data=entity.MasterData}
// @Router       /api/master-data/clients/{id} [get]
func (h *MasterDataHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataGet")
	defer span.End()
	id := router.GetParam(r, "id")
	out, err := h.uc.Get(ctx, ctxutil.GetWorkspaceID(ctx), id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Master Data record", out)
}

// Create godoc
// @Summary      Create a new master data record
// @Tags         MasterData
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        body body master_data.CreateRequest true "Create request"
// @Success      201 {object} response.StandardResponse{data=entity.MasterData}
// @Router       /api/master-data/clients [post]
func (h *MasterDataHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataCreate")
	defer span.End()
	var req master_data.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Create(ctx, ctxutil.GetWorkspaceID(ctx), callerEmail(r), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Master Data created", out)
}

// Patch godoc
// @Summary      Patch a master data record (partial)
// @Tags         MasterData
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id   path string true "Master data UUID"
// @Param        body body master_data.PatchRequest true "Patch request"
// @Success      200 {object} response.StandardResponse{data=entity.MasterData}
// @Router       /api/master-data/clients/{id} [put]
func (h *MasterDataHandler) Patch(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataPatch")
	defer span.End()
	id := router.GetParam(r, "id")
	var req master_data.PatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, changed, err := h.uc.Patch(ctx, ctxutil.GetWorkspaceID(ctx), id, callerEmail(r), master_data.WriteContextDashboardUser, req)
	if err != nil {
		return err
	}
	resp := map[string]any{"record": out, "changed_fields": changed}
	return response.StandardSuccess(w, r, http.StatusOK, "Master Data updated", resp)
}

// Delete godoc
// @Summary      Request deletion of a master data record (creates approval)
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id path string true "Master data UUID"
// @Success      202 {object} response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/master-data/clients/{id} [delete]
func (h *MasterDataHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataDelete")
	defer span.End()
	id := router.GetParam(r, "id")
	out, err := h.uc.RequestDelete(ctx, ctxutil.GetWorkspaceID(ctx), id, callerEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusAccepted, "Delete approval requested", out)
}

// Transition godoc
// @Summary      Transition stage atomically
// @Tags         MasterData
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        id   path string true "Master data UUID"
// @Param        body body master_data.TransitionRequest true "Transition payload"
// @Success      200 {object} response.StandardResponse{data=master_data.TransitionResult}
// @Router       /api/master-data/clients/{id}/transition [post]
func (h *MasterDataHandler) Transition(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataTransition")
	defer span.End()
	id := router.GetParam(r, "id")
	var req master_data.TransitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	out, err := h.uc.Transition(ctx, ctxutil.GetWorkspaceID(ctx), id, callerEmail(r), master_data.WriteContextDashboardUser, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Stage transitioned", out)
}

// Query godoc
// @Summary      Flexible workflow query (whitelisted ops)
// @Tags         MasterData
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        body body QueryRequest true "Query request"
// @Success      200 {object} response.StandardResponseWithMeta
// @Router       /api/master-data/query [post]
func (h *MasterDataHandler) Query(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataQuery")
	defer span.End()
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.Limit <= 0 {
		req.Limit = 100
	}
	rows, total, err := h.uc.Query(ctx, ctxutil.GetWorkspaceID(ctx), req.Conditions, req.Limit)
	if err != nil {
		return err
	}
	if rows == nil {
		rows = []entity.MasterData{}
	}
	meta := pagination.Meta{Total: total, Limit: req.Limit}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Query results", meta, rows)
}

// QueryRequest is the JSON body for the flexible query endpoint.
type QueryRequest struct {
	Conditions []repository.QueryCondition `json:"conditions"`
	Limit      int                         `json:"limit"`
}

// Stats godoc
// @Summary      Master Data summary cards
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Success      200 {object} response.StandardResponse{data=repository.MasterDataStats}
// @Router       /api/master-data/stats [get]
func (h *MasterDataHandler) Stats(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataStats")
	defer span.End()
	out, err := h.uc.Stats(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Stats", out)
}

// Attention godoc
// @Summary      Attention tab — high-risk / overdue / expiring
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        offset query int false "Offset"
// @Param        limit  query int false "Limit"
// @Param        search query string false "Search company_name/pic_name"
// @Success      200 {object} response.StandardResponseWithMeta
// @Router       /api/master-data/attention [get]
func (h *MasterDataHandler) Attention(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataAttention")
	defer span.End()
	pag := pagination.FromRequest(r)
	rows, total, summary, err := h.uc.Attention(ctx, ctxutil.GetWorkspaceID(ctx), r.URL.Query().Get("search"), pag.Offset, pag.Limit)
	if err != nil {
		return err
	}
	if rows == nil {
		rows = []entity.MasterData{}
	}
	meta := map[string]any{
		"total":   total,
		"offset":  pag.Offset,
		"limit":   pag.Limit,
		"summary": summary,
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Attention", meta, rows)
}

// Mutations godoc
// @Summary      Master Data mutation log
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        since query string false "RFC3339 lower bound"
// @Param        limit query int false "Limit (max 500)"
// @Success      200 {object} response.StandardResponse
// @Router       /api/master-data/mutations [get]
func (h *MasterDataHandler) Mutations(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataMutations")
	defer span.End()
	var since *time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			since = &t
		}
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.uc.ListMutations(ctx, ctxutil.GetWorkspaceID(ctx), since, limit)
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.MasterDataMutation{}
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Mutations", out)
}

// Import godoc
// @Summary      Bulk import (creates approval)
// @Tags         MasterData
// @Security     BearerAuth
// @Accept       multipart/form-data
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Param        file formData file true "Excel file"
// @Param        mode formData string true "add_new|update_existing"
// @Success      202 {object} response.StandardResponse{data=entity.ApprovalRequest}
// @Router       /api/master-data/clients/import [post]
func (h *MasterDataHandler) Import(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataImport")
	defer span.End()
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return apperror.BadRequest("invalid multipart form")
	}
	mode := master_data.ImportMode(r.FormValue("mode"))
	if mode == "" {
		mode = master_data.ImportModeAddNew
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return apperror.BadRequest("file required")
	}
	defer file.Close()

	rows, _, err := master_data.ParseImportRows(file)
	if err != nil {
		return apperror.BadRequest("failed to parse xlsx: " + err.Error())
	}
	preview := make([]map[string]any, 0, 5)
	for i, row := range rows {
		if i >= 5 {
			break
		}
		preview = append(preview, map[string]any{
			"company_id":   row.CompanyID,
			"company_name": row.CompanyName,
		})
	}
	out, err := h.uc.RequestImport(ctx, ctxutil.GetWorkspaceID(ctx), callerEmail(r), header.Filename, mode, len(rows), preview, "")
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusAccepted, "Import approval requested", out)
}

// Export godoc
// @Summary      Export master data as XLSX
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Produce      application/octet-stream
// @Success      200 {file} file
// @Router       /api/master-data/clients/export [get]
func (h *MasterDataHandler) Export(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataExport")
	defer span.End()
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=master-data-export.xlsx")
	if err := h.uc.Export(ctx, ctxutil.GetWorkspaceID(ctx), w); err != nil {
		return err
	}
	return nil
}

// Template godoc
// @Summary      Download import template
// @Tags         MasterData
// @Security     BearerAuth
// @Param        X-Workspace-ID header string true "Workspace ID"
// @Produce      application/octet-stream
// @Success      200 {file} file
// @Router       /api/master-data/clients/template [get]
func (h *MasterDataHandler) Template(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.MasterDataTemplate")
	defer span.End()
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=master-data-import-template.xlsx")
	if err := h.uc.Template(ctx, ctxutil.GetWorkspaceID(ctx), w); err != nil {
		return err
	}
	return nil
}
