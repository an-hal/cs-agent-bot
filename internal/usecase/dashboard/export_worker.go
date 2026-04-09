package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/xlsxexport"
	"github.com/google/uuid"
)

const exportBatchSize = 500

// StartExportClients creates a background_job record and launches the export in a goroutine.
func (u *dashboardUsecase) StartExportClients(ctx context.Context, workspaceID, actor string, filter entity.ClientFilter) (*entity.BackgroundJob, error) {
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	filename := fmt.Sprintf("clients_export_%s.xlsx", time.Now().Format("2006-01-02"))

	job := &entity.BackgroundJob{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		JobType:     entity.JobTypeExport,
		Status:      entity.JobStatusPending,
		EntityType:  entity.JobEntityClient,
		Filename:    filename,
		CreatedBy:   actor,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := u.bgJobRepo.Create(ctx, job); err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to create export job")
	}

	if err := u.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID: workspaceID,
		Category:    entity.ActivityCategoryData,
		ActorType:   entity.ActivityActorHuman,
		Actor:       actor,
		Action:      "start_export",
		Target:      filename,
		RefID:       job.ID,
		Detail:      fmt.Sprintf("job_id=%s", job.ID),
	}); err != nil {
		logger.Warn().Err(err).Msg("Failed to record start_export activity")
	}

	go u.runExport(job.ID, workspaceID, actor, filename, filter)

	return job, nil
}

// runExport fetches all clients in batches and writes them to an XLSX file.
func (u *dashboardUsecase) runExport(jobID, workspaceID, actor, filename string, filter entity.ClientFilter) {
	ctx := context.Background()
	logger := u.logger.With().Str("job_id", jobID).Logger()

	if err := u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusProcessing); err != nil {
		logger.Error().Err(err).Msg("export: failed to set processing status")
		return
	}

	// Resolve holding workspace IDs.
	wsIDs, err := u.resolveWorkspaceIDs(ctx, workspaceID)
	if err != nil {
		logger.Error().Err(err).Msg("export: failed to resolve workspace IDs")
		_ = u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
		return
	}
	filter.WorkspaceIDs = wsIDs

	total, err := u.clientRepo.CountByFilter(ctx, filter)
	if err != nil {
		logger.Error().Err(err).Msg("export: failed to count clients")
		_ = u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
		return
	}

	_ = u.bgJobRepo.UpdateProgress(ctx, jobID, int(total), 0, 0, 0, 0, nil)

	var allClients []entity.Client
	offset := 0
	for {
		batch, err := u.clientRepo.FetchByFilter(ctx, filter, pagination.Params{Offset: offset, Limit: exportBatchSize})
		if err != nil {
			logger.Error().Err(err).Int("offset", offset).Msg("export: fetch batch failed")
			_ = u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
			return
		}
		allClients = append(allClients, batch...)
		offset += len(batch)
		_ = u.bgJobRepo.UpdateProgress(ctx, jobID, int(total), offset, offset, 0, 0, nil)

		if len(batch) < exportBatchSize {
			break
		}
	}

	var buf bytes.Buffer
	if err := xlsxexport.WriteClientSheet(&buf, allClients); err != nil {
		logger.Error().Err(err).Msg("export: failed to write xlsx")
		_ = u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
		return
	}

	storagePath, err := u.fileStore.Write(jobID, filename, &buf)
	if err != nil {
		logger.Error().Err(err).Msg("export: failed to store file")
		_ = u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
		return
	}

	if err := u.bgJobRepo.UpdateStoragePath(ctx, jobID, storagePath); err != nil {
		logger.Error().Err(err).Msg("export: failed to update storage path")
	}

	_ = u.bgJobRepo.UpdateProgress(ctx, jobID, int(total), int(total), int(total), 0, 0, nil)
	_ = u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusDone)


	_ = u.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID: workspaceID,
		Category:    entity.ActivityCategoryData,
		ActorType:   entity.ActivityActorHuman,
		Actor:       actor,
		Action:      "complete_export",
		Target:      filename,
		RefID:       jobID,
		Detail:      fmt.Sprintf("job_id=%s rows=%d", jobID, total),
	})

	logger.Info().Int64("total_rows", total).Str("file", storagePath).Msg("export: completed")
}

// GetBackgroundJob returns a single job by ID.
func (u *dashboardUsecase) GetBackgroundJob(ctx context.Context, jobID string) (*entity.BackgroundJob, error) {
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	job, err := u.bgJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to get background job")
	}
	return job, nil
}

// ListBackgroundJobs returns paginated jobs for a workspace, optionally filtered by type/entity.
func (u *dashboardUsecase) ListBackgroundJobs(ctx context.Context, workspaceID, jobType, entityType string, p pagination.Params) ([]entity.BackgroundJob, int64, error) {
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	jobs, total, err := u.bgJobRepo.ListByWorkspace(ctx, workspaceID, jobType, entityType, p)
	if err != nil {
		return nil, 0, apperror.WrapInternal(logger, err, "Failed to list background jobs")
	}
	if jobs == nil {
		jobs = []entity.BackgroundJob{}
	}
	return jobs, total, nil
}

// DownloadJobFile returns the filename and a ReadCloser for the export file.
// Returns apperror.Conflict if the job is not yet done.
// Returns apperror.Forbidden if the job does not belong to workspaceID.
func (u *dashboardUsecase) DownloadJobFile(ctx context.Context, jobID, workspaceID string) (string, io.ReadCloser, error) {
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	job, err := u.bgJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return "", nil, apperror.WrapInternal(logger, err, "Failed to get job")
	}
	if job == nil {
		return "", nil, apperror.NotFound("job", "Background job not found")
	}
	if job.WorkspaceID != workspaceID {
		return "", nil, apperror.Forbidden("Access denied")
	}
	if job.Status != entity.JobStatusDone {
		return "", nil, apperror.Conflict("Export is not ready yet, current status: " + job.Status)
	}
	if job.StoragePath == "" {
		return "", nil, apperror.InternalError(fmt.Errorf("storage path empty for job %s", jobID))
	}

	rc, err := u.fileStore.Read(job.StoragePath)
	if err != nil {
		return "", nil, apperror.WrapInternal(logger, err, "Failed to read export file")
	}
	return job.Filename, rc, nil
}

// updateJobProgress is a convenience wrapper that also sets total_rows on first call.
func (u *dashboardUsecase) updateExportProgress(ctx context.Context, jobID string, total, processed, success, failed, skipped int, errs []entity.JobRowError) error {
	return u.bgJobRepo.UpdateProgress(ctx, jobID, total, processed, success, failed, skipped, errs)
}
