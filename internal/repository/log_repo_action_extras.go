package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/lib/pq"
)

const actionLogSelectColumns = `timestamp, COALESCE(company_id,''), COALESCE(company_name,''),
    COALESCE(trigger_type,''), COALESCE(template_id,''), COALESCE(channel,''),
    COALESCE(message_sent,false), COALESCE(response_received,false),
    COALESCE(response_classification,''), COALESCE(next_action_triggered,''),
    COALESCE(log_notes,''), reply_timestamp, COALESCE(reply_text,''),
    COALESCE(ae_notified,false), COALESCE(workspace_id::text,'')`

func scanActionLogRow(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.ActionLog, error) {
	var a entity.ActionLog
	err := scanner.Scan(&a.Timestamp, &a.CompanyID, &a.CompanyName,
		&a.TriggerType, &a.TemplateID, &a.Channel,
		&a.MessageSent, &a.ResponseReceived, &a.ResponseClassification,
		&a.NextActionTriggered, &a.LogNotes, &a.ReplyTimestamp, &a.ReplyText,
		&a.AENotified, &a.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetRecentActionLogs returns the last N bot action log rows for the given
// workspace set. Newest first. Limit defaults to 50, capped at 200.
func (r *logRepo) GetRecentActionLogs(ctx context.Context, workspaceIDs []string, limit int) ([]entity.ActionLog, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.GetRecentActionLogs")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT ` + actionLogSelectColumns + `
         FROM action_log
        WHERE workspace_id::text = ANY($1)
        ORDER BY timestamp DESC
        LIMIT $2`
	rows, err := r.db.QueryContext(ctx, q, pq.Array(workspaceIDs), limit)
	if err != nil {
		return nil, fmt.Errorf("recent action logs: %w", err)
	}
	defer rows.Close()
	out := make([]entity.ActionLog, 0, limit)
	for rows.Next() {
		a, err := scanActionLogRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// GetTodayActionLogs returns bot action logs from the current UTC day.
// Convenience wrapper around GetRecentActionLogs with a `since=today midnight`
// filter that avoids FE having to compute the boundary.
func (r *logRepo) GetTodayActionLogs(ctx context.Context, workspaceIDs []string, limit int) ([]entity.ActionLog, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.GetTodayActionLogs")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 500 {
		limit = 200
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	q := `SELECT ` + actionLogSelectColumns + `
         FROM action_log
        WHERE workspace_id::text = ANY($1)
          AND timestamp >= $2
        ORDER BY timestamp DESC
        LIMIT $3`
	rows, err := r.db.QueryContext(ctx, q, pq.Array(workspaceIDs), today, limit)
	if err != nil {
		return nil, fmt.Errorf("today action logs: %w", err)
	}
	defer rows.Close()
	out := make([]entity.ActionLog, 0, limit)
	for rows.Next() {
		a, err := scanActionLogRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// GetActionLogSummary returns aggregated counts for the dashboard bot-activity
// sidebar, scoped to rows at or after `since`.
func (r *logRepo) GetActionLogSummary(ctx context.Context, workspaceIDs []string, since time.Time) (*entity.ActionLogSummary, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.GetActionLogSummary")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	sum := &entity.ActionLogSummary{}
	if since.IsZero() {
		// Default: last 24h when caller forgets to pass one.
		since = time.Now().UTC().Add(-24 * time.Hour)
	}
	// Single round-trip with FILTER for each conditional sub-count.
	err := r.db.QueryRowContext(ctx, `
        SELECT
          COUNT(*) AS total,
          COUNT(*) FILTER (WHERE message_sent = true) AS messages_sent,
          COUNT(*) FILTER (WHERE response_received = true) AS replies_received,
          COUNT(*) FILTER (WHERE next_action_triggered = 'escalate') AS escalations_fired,
          COUNT(*) FILTER (WHERE ae_notified = true) AS ae_notifications
        FROM action_log
        WHERE workspace_id::text = ANY($1)
          AND timestamp >= $2`,
		pq.Array(workspaceIDs), since,
	).Scan(&sum.Total, &sum.MessagesSent, &sum.RepliesReceived,
		&sum.EscalationsFired, &sum.AENotifications)
	if err != nil {
		return nil, fmt.Errorf("action log summary: %w", err)
	}
	if sum.MessagesSent > 0 {
		sum.ReplyRatePct = float64(sum.RepliesReceived) / float64(sum.MessagesSent) * 100.0
	}
	return sum, nil
}
