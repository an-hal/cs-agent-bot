package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// InvoiceSequenceRepository provides atomic per-workspace per-year sequential IDs.
type InvoiceSequenceRepository interface {
	// NextSeq atomically increments and returns the next sequence number for a
	// workspace+year pair. The caller formats it as: INV-{WS}-{YEAR}-{SEQ:03d}.
	NextSeq(ctx context.Context, tx *sql.Tx, workspaceID string, year int) (int, error)
}

type invoiceSequenceRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewInvoiceSequenceRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) InvoiceSequenceRepository {
	return &invoiceSequenceRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *invoiceSequenceRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// NextSeq atomically bumps the sequence counter and returns the new value.
// Uses INSERT ... ON CONFLICT DO UPDATE to guarantee no gaps under concurrent writes.
func (r *invoiceSequenceRepo) NextSeq(ctx context.Context, tx *sql.Tx, workspaceID string, year int) (int, error) {
	ctx, span := r.tracer.Start(ctx, "invoice_sequence.repository.NextSeq")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	const query = `
INSERT INTO invoice_sequences (workspace_id, year, last_seq)
VALUES ($1, $2, 1)
ON CONFLICT (workspace_id, year)
DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
RETURNING last_seq`

	var seq int
	var err error
	if tx != nil {
		err = tx.QueryRowContext(ctx, query, workspaceID, year).Scan(&seq)
	} else {
		err = r.DB.QueryRowContext(ctx, query, workspaceID, year).Scan(&seq)
	}
	if err != nil {
		return 0, fmt.Errorf("next invoice seq (ws=%s year=%d): %w", workspaceID, year, err)
	}
	return seq, nil
}
