package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type ClaudeExtractionRepository interface {
	Insert(ctx context.Context, e *entity.ClaudeExtraction) (*entity.ClaudeExtraction, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.ClaudeExtraction, error)
	ListBySource(ctx context.Context, workspaceID, sourceType, sourceID string) ([]entity.ClaudeExtraction, error)
	ListForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ClaudeExtraction, error)
	UpdateResult(ctx context.Context, e *entity.ClaudeExtraction) error
	MarkSuperseded(ctx context.Context, workspaceID, sourceType, sourceID, keepID string) error
}

type claudeExtractionRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewClaudeExtractionRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ClaudeExtractionRepository {
	return &claudeExtractionRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *claudeExtractionRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const ceColumns = `id::text, workspace_id::text, source_type, source_id,
    COALESCE(master_data_id::text, ''), extracted_fields, extraction_prompt, extraction_model,
    bants_budget, bants_authority, bants_need, bants_timing, bants_sentiment,
    bants_score, COALESCE(bants_classification,''), COALESCE(buying_intent,''),
    coaching_notes, status, error_message, prompt_tokens, completion_tokens,
    latency_ms, created_at, updated_at`

func scanCE(s interface{ Scan(dest ...interface{}) error }) (*entity.ClaudeExtraction, error) {
	var e entity.ClaudeExtraction
	var raw []byte
	err := s.Scan(&e.ID, &e.WorkspaceID, &e.SourceType, &e.SourceID,
		&e.MasterDataID, &raw, &e.ExtractionPrompt, &e.ExtractionModel,
		&e.BANTSBudget, &e.BANTSAuthority, &e.BANTSNeed, &e.BANTSTiming, &e.BANTSSentiment,
		&e.BANTSScore, &e.BANTSClassification, &e.BuyingIntent,
		&e.CoachingNotes, &e.Status, &e.ErrorMessage, &e.PromptTokens,
		&e.CompletionTokens, &e.LatencyMS, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &e.ExtractedFields)
	}
	if e.ExtractedFields == nil {
		e.ExtractedFields = map[string]any{}
	}
	return &e, nil
}

func (r *claudeExtractionRepo) Insert(ctx context.Context, e *entity.ClaudeExtraction) (*entity.ClaudeExtraction, error) {
	ctx, span := r.tracer.Start(ctx, "claude_extraction.repository.Insert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, err := json.Marshal(coalesceMap(e.ExtractedFields))
	if err != nil {
		return nil, fmt.Errorf("marshal fields: %w", err)
	}

	// Use raw SQL to handle nullable master_data_id elegantly.
	var masterDataParam any
	if e.MasterDataID != "" {
		masterDataParam = e.MasterDataID
	} else {
		masterDataParam = nil
	}

	query := `
        INSERT INTO claude_extractions
            (workspace_id, source_type, source_id, master_data_id, extracted_fields,
             extraction_prompt, extraction_model, bants_budget, bants_authority,
             bants_need, bants_timing, bants_sentiment, bants_score,
             bants_classification, buying_intent, coaching_notes, status,
             error_message, prompt_tokens, completion_tokens, latency_ms)
        VALUES ($1, $2, $3, $4::uuid, $5::jsonb, $6, $7, $8, $9, $10, $11, $12, $13, NULLIF($14,''), NULLIF($15,''), $16, $17, $18, $19, $20, $21)
        RETURNING ` + ceColumns
	return scanCE(r.db.QueryRowContext(ctx, query,
		e.WorkspaceID, e.SourceType, e.SourceID, masterDataParam, string(raw),
		e.ExtractionPrompt, e.ExtractionModel,
		e.BANTSBudget, e.BANTSAuthority, e.BANTSNeed, e.BANTSTiming, e.BANTSSentiment,
		e.BANTSScore, e.BANTSClassification, e.BuyingIntent, e.CoachingNotes,
		defaultIfBlank(e.Status, entity.ClaudeExtractionStatusPending),
		e.ErrorMessage, e.PromptTokens, e.CompletionTokens, e.LatencyMS,
	))
}

func (r *claudeExtractionRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ClaudeExtraction, error) {
	ctx, span := r.tracer.Start(ctx, "claude_extraction.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Select(ceColumns).From("claude_extractions").
		Where(sq.And{sq.Expr("workspace_id::text = ?", workspaceID), sq.Expr("id::text = ?", id)}).ToSql()
	if err != nil {
		return nil, err
	}
	out, err := scanCE(r.db.QueryRowContext(ctx, q, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *claudeExtractionRepo) ListBySource(ctx context.Context, workspaceID, sourceType, sourceID string) ([]entity.ClaudeExtraction, error) {
	ctx, span := r.tracer.Start(ctx, "claude_extraction.repository.ListBySource")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Select(ceColumns).From("claude_extractions").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Eq{"source_type": sourceType},
			sq.Eq{"source_id": sourceID},
		}).OrderBy("created_at DESC").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.ClaudeExtraction
	for rows.Next() {
		e, err := scanCE(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func (r *claudeExtractionRepo) ListForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ClaudeExtraction, error) {
	ctx, span := r.tracer.Start(ctx, "claude_extraction.repository.ListForMasterData")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q, args, err := database.PSQL.Select(ceColumns).From("claude_extractions").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("master_data_id::text = ?", masterDataID),
		}).OrderBy("created_at DESC").Limit(uint64(limit)).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.ClaudeExtraction
	for rows.Next() {
		e, err := scanCE(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func (r *claudeExtractionRepo) UpdateResult(ctx context.Context, e *entity.ClaudeExtraction) error {
	ctx, span := r.tracer.Start(ctx, "claude_extraction.repository.UpdateResult")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, err := json.Marshal(coalesceMap(e.ExtractedFields))
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
        UPDATE claude_extractions
           SET extracted_fields    = $1::jsonb,
               bants_budget        = $2,
               bants_authority     = $3,
               bants_need          = $4,
               bants_timing        = $5,
               bants_sentiment     = $6,
               bants_score         = $7,
               bants_classification = NULLIF($8,''),
               buying_intent       = NULLIF($9,''),
               coaching_notes      = $10,
               status              = $11,
               error_message       = $12,
               prompt_tokens       = $13,
               completion_tokens   = $14,
               latency_ms          = $15,
               master_data_id      = CASE WHEN $16 = '' THEN master_data_id ELSE $16::uuid END,
               updated_at          = NOW()
         WHERE workspace_id::text = $17 AND id::text = $18`,
		string(raw),
		e.BANTSBudget, e.BANTSAuthority, e.BANTSNeed, e.BANTSTiming, e.BANTSSentiment,
		e.BANTSScore, e.BANTSClassification, e.BuyingIntent,
		e.CoachingNotes, e.Status, e.ErrorMessage,
		e.PromptTokens, e.CompletionTokens, e.LatencyMS,
		e.MasterDataID, e.WorkspaceID, e.ID,
	)
	if err != nil {
		return fmt.Errorf("update extraction: %w", err)
	}
	return nil
}

func (r *claudeExtractionRepo) MarkSuperseded(ctx context.Context, workspaceID, sourceType, sourceID, keepID string) error {
	ctx, span := r.tracer.Start(ctx, "claude_extraction.repository.MarkSuperseded")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `
        UPDATE claude_extractions
           SET status = $1, updated_at = NOW()
         WHERE workspace_id::text = $2
           AND source_type = $3 AND source_id = $4
           AND id::text <> $5
           AND status IN ('pending','running','succeeded')`,
		entity.ClaudeExtractionStatusSuperseded, workspaceID, sourceType, sourceID, keepID,
	)
	if err != nil {
		return fmt.Errorf("mark superseded: %w", err)
	}
	return nil
}
