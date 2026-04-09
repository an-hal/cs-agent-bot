package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type FlagsRepository interface {
	GetByCompanyID(ctx context.Context, companyID string) (*entity.ClientFlags, error)
	UpdateFlags(ctx context.Context, companyID string, flags entity.ClientFlags) error
	SetBotActive(ctx context.Context, companyID string, active bool) error
	ResetCycleFlags(ctx context.Context, companyID string) error
}

type flagsRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewFlagsRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) FlagsRepository {
	return &flagsRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *flagsRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// flagsColumns lists all client_flags columns in scan order.
var flagsColumns = []string{
	"company_id",
	"ren60_sent", "ren45_sent", "ren30_sent", "ren15_sent", "ren0_sent",
	"checkin_a1_form_sent", "checkin_a1_call_sent", "checkin_a2_form_sent", "checkin_a2_call_sent",
	"checkin_b1_form_sent", "checkin_b1_call_sent", "checkin_b2_form_sent", "checkin_b2_call_sent",
	"checkin_replied",
	"nps1_sent", "nps2_sent", "nps3_sent", "nps_replied", "referral_sent_this_cycle",
	"quotation_acknowledged",
	"low_usage_msg_sent", "low_nps_msg_sent",
	"cs_h7", "cs_h14", "cs_h21", "cs_h30", "cs_h45", "cs_h60", "cs_h75", "cs_h90",
	"cs_lt1", "cs_lt2", "cs_lt3",
	"feature_update_sent",
	"workspace_id",
}

// scanFlags scans a single row into a ClientFlags struct using the flagsColumns order.
func scanFlags(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.ClientFlags, error) {
	var f entity.ClientFlags
	err := scanner.Scan(
		&f.CompanyID,
		&f.Ren60Sent, &f.Ren45Sent, &f.Ren30Sent, &f.Ren15Sent, &f.Ren0Sent,
		&f.CheckinA1FormSent, &f.CheckinA1CallSent, &f.CheckinA2FormSent, &f.CheckinA2CallSent,
		&f.CheckinB1FormSent, &f.CheckinB1CallSent, &f.CheckinB2FormSent, &f.CheckinB2CallSent,
		&f.CheckinReplied,
		&f.NPS1Sent, &f.NPS2Sent, &f.NPS3Sent, &f.NPSReplied, &f.ReferralSentThisCycle,
		&f.QuotationAcknowledged,
		&f.LowUsageMsgSent, &f.LowNPSMsgSent,
		&f.CSH7, &f.CSH14, &f.CSH21, &f.CSH30, &f.CSH45, &f.CSH60, &f.CSH75, &f.CSH90,
		&f.CSLT1, &f.CSLT2, &f.CSLT3,
		&f.FeatureUpdateSent,
		&f.WorkspaceID,
	)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// flagValues returns the column values for a ClientFlags in the same order as flagsColumns.
func flagValues(f entity.ClientFlags) []interface{} {
	return []interface{}{
		f.CompanyID,
		f.Ren60Sent, f.Ren45Sent, f.Ren30Sent, f.Ren15Sent, f.Ren0Sent,
		f.CheckinA1FormSent, f.CheckinA1CallSent, f.CheckinA2FormSent, f.CheckinA2CallSent,
		f.CheckinB1FormSent, f.CheckinB1CallSent, f.CheckinB2FormSent, f.CheckinB2CallSent,
		f.CheckinReplied,
		f.NPS1Sent, f.NPS2Sent, f.NPS3Sent, f.NPSReplied, f.ReferralSentThisCycle,
		f.QuotationAcknowledged,
		f.LowUsageMsgSent, f.LowNPSMsgSent,
		f.CSH7, f.CSH14, f.CSH21, f.CSH30, f.CSH45, f.CSH60, f.CSH75, f.CSH90,
		f.CSLT1, f.CSLT2, f.CSLT3,
		f.FeatureUpdateSent,
		f.WorkspaceID,
	}
}

func (r *flagsRepo) GetByCompanyID(ctx context.Context, companyID string) (*entity.ClientFlags, error) {
	ctx, span := r.tracer.Start(ctx, "flags.repository.GetByCompanyID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(flagsColumns...).
		From("client_flags").
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	f, err := scanFlags(r.DB.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return &entity.ClientFlags{CompanyID: companyID}, nil
		}
		return nil, fmt.Errorf("query flags: %w", err)
	}

	return f, nil
}

func (r *flagsRepo) UpdateFlags(ctx context.Context, companyID string, flags entity.ClientFlags) error {
	ctx, span := r.tracer.Start(ctx, "flags.repository.UpdateFlags")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Build ON CONFLICT SET clause using EXCLUDED.* for all columns except company_id.
	var setParts []string
	for _, col := range flagsColumns {
		if col == "company_id" {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
	}
	upsertSuffix := fmt.Sprintf(
		"ON CONFLICT (company_id) DO UPDATE SET %s",
		strings.Join(setParts, ", "),
	)

	query, args, err := database.PSQL.
		Insert("client_flags").
		Columns(flagsColumns...).
		Values(flagValues(flags)...).
		Suffix(upsertSuffix).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert flags: %w", err)
	}

	return nil
}

func (r *flagsRepo) SetBotActive(ctx context.Context, companyID string, active bool) error {
	ctx, span := r.tracer.Start(ctx, "flags.repository.SetBotActive")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	activeStr := "false"
	if active {
		activeStr = "true"
	}
	upsertSuffix := "ON CONFLICT (company_id) DO UPDATE SET bot_active = " + activeStr + ", updated_at = NOW()"

	query, args, err := database.PSQL.
		Insert("conversation_states").
		Columns("company_id", "bot_active", "reason_bot_paused").
		Values(companyID, active, "").
		Suffix(upsertSuffix).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert bot_active: %w", err)
	}

	return nil
}

// cycleFlagColumns lists the flags that are reset each renewal cycle.
// Cross-sell flags (CSH*, CSLT*, FeatureUpdateSent) are intentionally excluded.
var cycleFlagColumns = map[string]interface{}{
	"ren60_sent": false, "ren45_sent": false, "ren30_sent": false, "ren15_sent": false, "ren0_sent": false,
	"checkin_a1_form_sent": false, "checkin_a1_call_sent": false,
	"checkin_a2_form_sent": false, "checkin_a2_call_sent": false,
	"checkin_b1_form_sent": false, "checkin_b1_call_sent": false,
	"checkin_b2_form_sent": false, "checkin_b2_call_sent": false,
	"checkin_replied": false,
	"nps1_sent":       false, "nps2_sent": false, "nps3_sent": false, "nps_replied": false,
	"referral_sent_this_cycle": false,
	"low_usage_msg_sent":       false,
	"low_nps_msg_sent":         false,
}

func (r *flagsRepo) ResetCycleFlags(ctx context.Context, companyID string) error {
	ctx, span := r.tracer.Start(ctx, "flags.repository.ResetCycleFlags")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("client_flags").
		SetMap(cycleFlagColumns).
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("reset cycle flags: %w", err)
	}

	return nil
}
