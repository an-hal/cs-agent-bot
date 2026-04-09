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
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/xlsximport"
	"github.com/google/uuid"
)

// StartImportClients creates a background_job record and launches the import in a goroutine.
// Returns immediately with the job metadata; processing happens asynchronously.
func (u *dashboardUsecase) StartImportClients(ctx context.Context, workspaceID, actor, filename string, file io.Reader, updateExisting bool) (*entity.BackgroundJob, error) {
	logger := ctxutil.LoggerWithRequestID(ctx, u.logger)

	// Buffer the file in memory so it outlives the request context.
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to read import file")
	}

	job := &entity.BackgroundJob{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		JobType:     entity.JobTypeImport,
		Status:      entity.JobStatusPending,
		EntityType:  entity.JobEntityClient,
		Filename:    filename,
		CreatedBy:   actor,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata: map[string]any{
			"update_existing": updateExisting,
		},
	}

	if err := u.bgJobRepo.Create(ctx, job); err != nil {
		return nil, apperror.WrapInternal(logger, err, "Failed to create import job")
	}

	if err := u.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID: workspaceID,
		Category:    entity.ActivityCategoryData,
		ActorType:   entity.ActivityActorHuman,
		Actor:       actor,
		Action:      "start_import",
		Target:      filename,
		RefID:       job.ID,
		Detail:      fmt.Sprintf("job_id=%s filename=%s", job.ID, filename),
	}); err != nil {
		logger.Warn().Err(err).Msg("Failed to record start_import activity")
	}

	go u.runImport(job.ID, workspaceID, actor, data, updateExisting)

	return job, nil
}

// runImport is the background goroutine that processes the XLSX file row by row.
func (u *dashboardUsecase) runImport(jobID, workspaceID, actor string, data []byte, updateExisting bool) {
	ctx := context.Background()
	logger := u.logger.With().Str("job_id", jobID).Logger()

	if err := u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusProcessing); err != nil {
		logger.Error().Err(err).Msg("import: failed to set processing status")
		return
	}

	rows, parseErrs, err := xlsximport.ParseClientSheet(bytes.NewReader(data))
	if err != nil {
		logger.Error().Err(err).Msg("import: parse error")
		_ = u.bgJobRepo.UpdateStatus(ctx, jobID, entity.JobStatusFailed)
		return
	}

	totalRows := len(rows) + len(parseErrs)
	if err := u.updateJobProgress(ctx, jobID, totalRows, 0, 0, 0, len(parseErrs), toJobRowErrors(parseErrs)); err != nil {
		logger.Warn().Err(err).Msg("import: failed to persist initial row count")
	}

	var jobErrs []entity.JobRowError
	jobErrs = append(jobErrs, toJobRowErrors(parseErrs)...)

	success, failed, skipped := 0, len(parseErrs), 0

	for i, row := range rows {
		processed := i + 1

		existing, err := u.clientRepo.GetByCompanyID(ctx, row.CompanyID)
		if err != nil {
			jobErrs = append(jobErrs, entity.JobRowError{Row: i + 2, RefID: row.CompanyID, Reason: "db error: " + err.Error()})
			failed++
			continue
		}

		if existing != nil {
			if !updateExisting {
				skipped++
			} else {
				if err := u.clientRepo.UpdateClientFields(ctx, row.CompanyID, importRowToUpdateFields(row)); err != nil {
					jobErrs = append(jobErrs, entity.JobRowError{Row: i + 2, RefID: row.CompanyID, Reason: "update failed: " + err.Error()})
					failed++
				} else {
					success++
					u.recordImportRowActivity(ctx, workspaceID, actor, jobID, row.CompanyID, row.CompanyName, "update")
				}
			}
		} else {
			client := importRowToClient(row, workspaceID)
			if err := u.clientRepo.CreateClient(ctx, client); err != nil {
				jobErrs = append(jobErrs, entity.JobRowError{Row: i + 2, RefID: row.CompanyID, Reason: "create failed: " + err.Error()})
				failed++
			} else {
				success++
				u.recordImportRowActivity(ctx, workspaceID, actor, jobID, row.CompanyID, row.CompanyName, "create")
			}
		}

		// Persist progress every 10 rows.
		if processed%10 == 0 {
			_ = u.updateJobProgress(ctx, jobID, totalRows, processed, success, failed, skipped, jobErrs)
		}
	}

	finalStatus := entity.JobStatusDone
	if failed > 0 && success == 0 {
		finalStatus = entity.JobStatusFailed
	}

	if err := u.updateJobProgress(ctx, jobID, totalRows, len(rows), success, failed, skipped, jobErrs); err != nil {
		logger.Warn().Err(err).Msg("import: failed to persist final progress")
	}
	if err := u.bgJobRepo.UpdateStatus(ctx, jobID, finalStatus); err != nil {
		logger.Error().Err(err).Msg("import: failed to set final status")
	}

	logger.Info().
		Str("status", finalStatus).
		Int("success", success).
		Int("failed", failed).
		Int("skipped", skipped).
		Msg("import: completed")
}

func (u *dashboardUsecase) updateJobProgress(ctx context.Context, jobID string, total, processed, success, failed, skipped int, errs []entity.JobRowError) error {
	return u.bgJobRepo.UpdateProgress(ctx, jobID, total, processed, success, failed, skipped, errs)
}

func (u *dashboardUsecase) recordImportRowActivity(ctx context.Context, workspaceID, actor, jobID, companyID, companyName, action string) {
	_ = u.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID: workspaceID,
		Category:    entity.ActivityCategoryData,
		ActorType:   entity.ActivityActorHuman,
		Actor:       actor,
		Action:      "import_client_" + action,
		Target:      companyName,
		RefID:       companyID,
		Detail:      fmt.Sprintf("job_id=%s", jobID),
	})
}

func toJobRowErrors(parseErrs []xlsximport.ParseError) []entity.JobRowError {
	out := make([]entity.JobRowError, len(parseErrs))
	for i, e := range parseErrs {
		out[i] = entity.JobRowError{Row: e.Row, RefID: e.RefID, Reason: e.Reason}
	}
	return out
}

func importRowToClient(row xlsximport.ClientImportRow, workspaceID string) entity.Client {
	c := entity.Client{
		CompanyID:           row.CompanyID,
		CompanyName:         row.CompanyName,
		PICName:             row.PICName,
		PICRole:             row.PICRole,
		PICWA:               row.PICWA,
		PICEmail:            row.PICEmail,
		OwnerName:           row.OwnerName,
		OwnerTelegramID:     row.OwnerTelegramID,
		ContractStart:       row.ContractStart,
		ContractEnd:         row.ContractEnd,
		ContractMonths:      row.ContractMonths,
		HCSize:              row.HCSize,
		PlanType:            row.PlanType,
		PaymentTerms:        row.PaymentTerms,
		FinalPrice:          row.FinalPrice,
		PaymentStatus:       row.PaymentStatus,
		QuotationLink:       row.QuotationLink,
		NPSScore:            row.NPSScore,
		UsageScore:          row.UsageScore,
		SequenceCS:          row.SequenceCS,
		Renewed:             row.Renewed,
		BotActive:           row.BotActive,
		Blacklisted:         row.Blacklisted,
		CheckinReplied:      row.CheckinReplied,
		CrossSellInterested: row.CrossSellInterested,
		CrossSellRejected:   row.CrossSellRejected,
		Segment:             row.Segment,
		Notes:               row.Notes,
		LastPaymentDate:     row.LastPaymentDate,
		LastInteractionDate: row.LastInteractionDate,
		CrossSellResumeDate: row.CrossSellResumeDate,
		WorkspaceID:         workspaceID,
		ActivationDate:      row.ContractStart, // default activation to contract start
		CreatedAt:           time.Now(),
	}

	if row.OwnerWA != "" {
		c.OwnerWA = &row.OwnerWA
	}

	if c.SequenceCS == "" {
		c.SequenceCS = "ACTIVE"
	}
	if c.PaymentStatus == "" {
		c.PaymentStatus = "Paid"
	}
	return c
}

func importRowToUpdateFields(row xlsximport.ClientImportRow) map[string]interface{} {
	fields := map[string]interface{}{
		"company_name":         row.CompanyName,
		"pic_name":             row.PICName,
		"pic_role":             row.PICRole,
		"pic_wa":               row.PICWA,
		"pic_email":            row.PICEmail,
		"owner_name":           row.OwnerName,
		"owner_telegram_id":    row.OwnerTelegramID,
		"contract_start":       row.ContractStart,
		"contract_end":         row.ContractEnd,
		"contract_months":      row.ContractMonths,
		"hc_size":              row.HCSize,
		"plan_type":            row.PlanType,
		"payment_terms":        row.PaymentTerms,
		"final_price":          row.FinalPrice,
		"payment_status":       row.PaymentStatus,
		"quotation_link":       row.QuotationLink,
		"nps_score":            row.NPSScore,
		"usage_score":          row.UsageScore,
		"sequence_cs":          row.SequenceCS,
		"renewed":              row.Renewed,
		"bot_active":           row.BotActive,
		"blacklisted":          row.Blacklisted,
		"checkin_replied":      row.CheckinReplied,
		"cross_sell_interested": row.CrossSellInterested,
		"cross_sell_rejected":  row.CrossSellRejected,
		"segment":              row.Segment,
		"notes":                row.Notes,
	}
	if row.OwnerWA != "" {
		fields["owner_wa"] = row.OwnerWA
	}
	if row.LastPaymentDate != nil {
		fields["last_payment_date"] = row.LastPaymentDate
	}
	if row.LastInteractionDate != nil {
		fields["last_interaction_date"] = row.LastInteractionDate
	}
	if row.CrossSellResumeDate != nil {
		fields["cross_sell_resume_date"] = row.CrossSellResumeDate
	}
	return fields
}
