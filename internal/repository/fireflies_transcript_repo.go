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

type FirefliesTranscriptRepository interface {
	Insert(ctx context.Context, t *entity.FirefliesTranscript) (*entity.FirefliesTranscript, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.FirefliesTranscript, error)
	GetByFirefliesID(ctx context.Context, workspaceID, firefliesID string) (*entity.FirefliesTranscript, error)
	List(ctx context.Context, workspaceID, status string, limit, offset int) ([]entity.FirefliesTranscript, int64, error)
	UpdateExtraction(ctx context.Context, workspaceID, id, status, errMsg, masterDataID string) error
}

type firefliesTranscriptRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewFirefliesTranscriptRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) FirefliesTranscriptRepository {
	return &firefliesTranscriptRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *firefliesTranscriptRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const ffColumns = `id::text, workspace_id::text, fireflies_id, meeting_title,
    meeting_date, duration_seconds, host_email, participants,
    transcript_text, raw_payload, extraction_status, extraction_error,
    extracted_at, COALESCE(master_data_id::text, ''), created_at, updated_at`

func scanFF(s interface{ Scan(dest ...interface{}) error }) (*entity.FirefliesTranscript, error) {
	var f entity.FirefliesTranscript
	var pRaw, payRaw []byte
	err := s.Scan(&f.ID, &f.WorkspaceID, &f.FirefliesID, &f.MeetingTitle,
		&f.MeetingDate, &f.DurationSeconds, &f.HostEmail, &pRaw,
		&f.TranscriptText, &payRaw, &f.ExtractionStatus, &f.ExtractionError,
		&f.ExtractedAt, &f.MasterDataID, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(pRaw) > 0 {
		_ = json.Unmarshal(pRaw, &f.Participants)
	}
	if len(payRaw) > 0 {
		_ = json.Unmarshal(payRaw, &f.RawPayload)
	}
	if f.Participants == nil {
		f.Participants = []string{}
	}
	if f.RawPayload == nil {
		f.RawPayload = map[string]any{}
	}
	return &f, nil
}

func (r *firefliesTranscriptRepo) Insert(ctx context.Context, t *entity.FirefliesTranscript) (*entity.FirefliesTranscript, error) {
	ctx, span := r.tracer.Start(ctx, "fireflies.repository.Insert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if t.Participants == nil {
		t.Participants = []string{}
	}
	pRaw, _ := json.Marshal(t.Participants)
	payRaw, err := json.Marshal(coalesceMap(t.RawPayload))
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	query, args, err := database.PSQL.
		Insert("fireflies_transcripts").
		Columns("workspace_id", "fireflies_id", "meeting_title", "meeting_date",
			"duration_seconds", "host_email", "participants", "transcript_text", "raw_payload").
		Values(t.WorkspaceID, t.FirefliesID, t.MeetingTitle, t.MeetingDate,
			t.DurationSeconds, t.HostEmail, string(pRaw), t.TranscriptText, string(payRaw)).
		Suffix("RETURNING " + ffColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	return scanFF(r.db.QueryRowContext(ctx, query, args...))
}

func (r *firefliesTranscriptRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.FirefliesTranscript, error) {
	ctx, span := r.tracer.Start(ctx, "fireflies.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Select(ffColumns).From("fireflies_transcripts").
		Where(sq.And{sq.Expr("workspace_id::text = ?", workspaceID), sq.Expr("id::text = ?", id)}).ToSql()
	if err != nil {
		return nil, err
	}
	out, err := scanFF(r.db.QueryRowContext(ctx, q, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *firefliesTranscriptRepo) GetByFirefliesID(ctx context.Context, workspaceID, firefliesID string) (*entity.FirefliesTranscript, error) {
	ctx, span := r.tracer.Start(ctx, "fireflies.repository.GetByFirefliesID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args, err := database.PSQL.Select(ffColumns).From("fireflies_transcripts").
		Where(sq.And{sq.Expr("workspace_id::text = ?", workspaceID), sq.Eq{"fireflies_id": firefliesID}}).ToSql()
	if err != nil {
		return nil, err
	}
	out, err := scanFF(r.db.QueryRowContext(ctx, q, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *firefliesTranscriptRepo) List(ctx context.Context, workspaceID, status string, limit, offset int) ([]entity.FirefliesTranscript, int64, error) {
	ctx, span := r.tracer.Start(ctx, "fireflies.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{sq.Expr("workspace_id::text = ?", workspaceID)}
	if status != "" {
		conds = append(conds, sq.Eq{"extraction_status": status})
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q, args, err := database.PSQL.Select(ffColumns).From("fireflies_transcripts").
		Where(conds).OrderBy("created_at DESC").
		Limit(uint64(limit)).Offset(uint64(offset)).ToSql()
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []entity.FirefliesTranscript
	for rows.Next() {
		f, err := scanFF(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	cQ, cArgs, err := database.PSQL.Select("COUNT(*)").From("fireflies_transcripts").Where(conds).ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, cQ, cArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *firefliesTranscriptRepo) UpdateExtraction(ctx context.Context, workspaceID, id, status, errMsg, masterDataID string) error {
	ctx, span := r.tracer.Start(ctx, "fireflies.repository.UpdateExtraction")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `
        UPDATE fireflies_transcripts
           SET extraction_status = $1,
               extraction_error  = $2,
               extracted_at      = CASE WHEN $1 IN ('succeeded','failed') THEN NOW() ELSE extracted_at END,
               master_data_id    = CASE WHEN $3 = '' THEN master_data_id ELSE $3::uuid END,
               updated_at        = NOW()
         WHERE workspace_id::text = $4 AND id::text = $5`,
		status, errMsg, masterDataID, workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("update extraction: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("fireflies transcript not found")
	}
	return nil
}
