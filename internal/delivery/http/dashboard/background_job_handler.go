package dashboard

import (
	"io"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/dashboard"
	"github.com/rs/zerolog"
)

const maxUploadSize = 5 << 20 // 5 MB

// BackgroundJobHandler handles import, export, status, and download endpoints.
type BackgroundJobHandler struct {
	uc     dashboard.DashboardUsecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewBackgroundJobHandler(uc dashboard.DashboardUsecase, logger zerolog.Logger, tr tracer.Tracer) *BackgroundJobHandler {
	return &BackgroundJobHandler{uc: uc, logger: logger, tracer: tr}
}

// ImportClients godoc
// @Summary      Import clients from XLSX
// @Description  Accepts a multipart XLSX file (Template Import sheet), creates a background import job, and returns immediately with the job ID.
// @Tags         Background Jobs
// @Param        X-Workspace-ID  header    string  true  "Workspace ID"
// @Param        file            formData  file    true  "XLSX file (max 5 MB)"
// @Param        update_existing query     bool    false "Update existing clients (default false)"
// @Success      202  {object}  response.StandardResponse{data=entity.BackgroundJob}
// @Failure      400  {object}  response.StandardResponse
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/data-master/clients/import [post]
func (h *BackgroundJobHandler) ImportClients(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ImportClients")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	workspaceID := ctxutil.GetWorkspaceID(ctx)
	actor := actorFromCtx(r)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		return apperror.BadRequest("File too large or invalid multipart form (max 5 MB)")
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return apperror.BadRequest("Missing file field in form")
	}
	defer file.Close()

	updateExisting := r.URL.Query().Get("update_existing") == "true"

	logger.Info().Str("workspace_id", workspaceID).Str("filename", header.Filename).Bool("update_existing", updateExisting).Msg("Incoming import clients request")

	job, err := h.uc.StartImportClients(ctx, workspaceID, actor, header.Filename, file, updateExisting)
	if err != nil {
		return err
	}

	logger.Info().Str("job_id", job.ID).Msg("Import job started")

	return response.StandardSuccess(w, r, http.StatusAccepted, "Import started", job)
}

// ExportClients godoc
// @Summary      Export clients to XLSX
// @Description  Starts a background export job for the workspace. Poll /jobs/{job_id} for status, then download via /jobs/{job_id}/download.
// @Tags         Background Jobs
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        search          query   string  false  "Search term"
// @Param        segment         query   string  false  "Segment filter"
// @Param        payment_status  query   string  false  "Payment status filter"
// @Param        sequence_cs     query   string  false  "Sequence CS filter"
// @Param        plan_type       query   string  false  "Plan type filter"
// @Success      202  {object}  response.StandardResponse{data=entity.BackgroundJob}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/data-master/clients/export [post]
func (h *BackgroundJobHandler) ExportClients(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ExportClients")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	workspaceID := ctxutil.GetWorkspaceID(ctx)
	actor := actorFromCtx(r)

	q := r.URL.Query()
	filter := entity.ClientFilter{
		WorkspaceIDs:  []string{workspaceID},
		Search:        q.Get("search"),
		Segment:       q.Get("segment"),
		PaymentStatus: q.Get("payment_status"),
		SequenceCS:    q.Get("sequence_cs"),
		PlanType:      q.Get("plan_type"),
	}

	logger.Info().Str("workspace_id", workspaceID).Msg("Incoming export clients request")

	job, err := h.uc.StartExportClients(ctx, workspaceID, actor, filter)
	if err != nil {
		return err
	}

	logger.Info().Str("job_id", job.ID).Msg("Export job started")

	return response.StandardSuccess(w, r, http.StatusAccepted, "Export started", job)
}

// GetJob godoc
// @Summary      Get background job status
// @Description  Returns the current status and progress of any background job (import or export).
// @Tags         Background Jobs
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        job_id          path    string  true  "Job ID"
// @Success      200  {object}  response.StandardResponse{data=entity.BackgroundJob}
// @Failure      403  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/data-master/jobs/{job_id} [get]
func (h *BackgroundJobHandler) GetJob(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.GetJob")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	jobID := router.GetParam(r, "job_id")
	workspaceID := ctxutil.GetWorkspaceID(ctx)

	logger.Info().Str("job_id", jobID).Msg("Incoming get job request")

	job, err := h.uc.GetBackgroundJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return apperror.NotFound("job", "Background job not found")
	}
	if job.WorkspaceID != workspaceID {
		return apperror.Forbidden("Access denied")
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Background job", job)
}

// ListJobs godoc
// @Summary      List background jobs
// @Description  Returns paginated background jobs for the workspace, optionally filtered by job_type and entity_type.
// @Tags         Background Jobs
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        job_type        query   string  false  "Filter by job type: import|export"
// @Param        entity_type     query   string  false  "Filter by entity type: client"
// @Param        offset          query   int     false  "Pagination offset"
// @Param        limit           query   int     false  "Page size"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.BackgroundJob}
// @Failure      500  {object}  response.StandardResponse
// @Router       /api/data-master/jobs [get]
func (h *BackgroundJobHandler) ListJobs(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.ListJobs")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	workspaceID := ctxutil.GetWorkspaceID(ctx)
	params := pagination.FromRequest(r)
	q := r.URL.Query()

	logger.Info().Str("workspace_id", workspaceID).Msg("Incoming list jobs request")

	jobs, total, err := h.uc.ListBackgroundJobs(ctx, workspaceID, q.Get("job_type"), q.Get("entity_type"), params)
	if err != nil {
		return err
	}

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Background jobs", pagination.NewMeta(params, total), jobs)
}

// DownloadJobFile godoc
// @Summary      Download export file
// @Description  Streams the XLSX file for a completed export job. Returns 409 if the job is not yet done.
// @Tags         Background Jobs
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        job_id          path    string  true  "Job ID"
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Success      200  {file}    binary
// @Failure      403  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Failure      409  {object}  response.StandardResponse
// @Router       /api/data-master/jobs/{job_id}/download [get]
func (h *BackgroundJobHandler) DownloadJobFile(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.DownloadJobFile")
	defer span.End()

	logger := ctxutil.LoggerWithRequestID(ctx, h.logger)
	jobID := router.GetParam(r, "job_id")
	workspaceID := ctxutil.GetWorkspaceID(ctx)

	logger.Info().Str("job_id", jobID).Msg("Incoming download job file request")

	filename, rc, err := h.uc.DownloadJobFile(ctx, jobID, workspaceID)
	if err != nil {
		return err
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, rc); err != nil {
		logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to stream export file")
	}
	return nil
}
