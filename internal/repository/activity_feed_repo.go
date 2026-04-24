package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// FeedEntry is a unified view over action_log + master_data_mutations +
// activity_log tables. Each row is normalized so FE can render a single
// timeline regardless of origin.
type FeedEntry struct {
	Source       string    `json:"source"` // 'action_log' | 'mutation' | 'activity_log'
	ID           string    `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	ActorEmail   string    `json:"actor_email,omitempty"`
	Action       string    `json:"action"`
	Resource     string    `json:"resource,omitempty"`
	ResourceID   string    `json:"resource_id,omitempty"`
	CompanyID    string    `json:"company_id,omitempty"`
	CompanyName  string    `json:"company_name,omitempty"`
	Summary      string    `json:"summary,omitempty"`
	MutationKind string    `json:"mutation_source,omitempty"` // only set for source=mutation
}

// ActivityFeedRepository returns a newest-first union across all activity
// streams. Kept as a thin wrapper so the handler doesn't know which tables
// to query — FE just calls GET /activity-log/feed and gets the unified view.
type ActivityFeedRepository interface {
	Feed(ctx context.Context, workspaceID string, limit int) ([]FeedEntry, error)
}

type activityFeedRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewActivityFeedRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ActivityFeedRepository {
	return &activityFeedRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *activityFeedRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// Feed runs a UNION ALL across the three known activity sources and orders by
// timestamp DESC. Limit is applied per-source so slow sources don't starve
// the output.
func (r *activityFeedRepo) Feed(ctx context.Context, workspaceID string, limit int) ([]FeedEntry, error) {
	ctx, span := r.tracer.Start(ctx, "activity_feed.repository.Feed")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	perSource := limit // pull `limit` from each, merge, cap at the end

	out := make([]FeedEntry, 0, limit*3)

	// 1) action_log — bot actions + escalation fan-outs.
	rows, err := r.db.QueryContext(ctx, `
        SELECT id::text, company_id, action, COALESCE(template_id,''), timestamp
          FROM action_log
         ORDER BY timestamp DESC
         LIMIT $1`, perSource)
	if err == nil {
		for rows.Next() {
			var e FeedEntry
			var tplID string
			if err := rows.Scan(&e.ID, &e.CompanyID, &e.Action, &tplID, &e.Timestamp); err == nil {
				e.Source = "action_log"
				e.Summary = tplID
				out = append(out, e)
			}
		}
		rows.Close()
	}

	// 2) master_data_mutations — dashboard edits + auto-handoffs + reactivations.
	rows2, err := r.db.QueryContext(ctx, `
        SELECT id::text, COALESCE(actor_email,''), action, COALESCE(source,'dashboard'),
               COALESCE(company_id,''), COALESCE(company_name,''), COALESCE(note,''),
               COALESCE(master_data_id::text,''), timestamp
          FROM master_data_mutations
         WHERE workspace_id::text = $1
         ORDER BY timestamp DESC
         LIMIT $2`, workspaceID, perSource)
	if err == nil {
		for rows2.Next() {
			var e FeedEntry
			if err := rows2.Scan(&e.ID, &e.ActorEmail, &e.Action, &e.MutationKind,
				&e.CompanyID, &e.CompanyName, &e.Summary, &e.ResourceID, &e.Timestamp); err == nil {
				e.Source = "mutation"
				e.Resource = "master_data"
				out = append(out, e)
			}
		}
		rows2.Close()
	}

	// 3) activity_log — user-facing action log (feature 08).
	rows3, err := r.db.QueryContext(ctx, `
        SELECT id::text, COALESCE(actor_email,''), feature, action,
               COALESCE(resource_id,''), created_at
          FROM activity_log
         WHERE workspace_id::text = $1
         ORDER BY created_at DESC
         LIMIT $2`, workspaceID, perSource)
	if err == nil {
		for rows3.Next() {
			var e FeedEntry
			var feature string
			if err := rows3.Scan(&e.ID, &e.ActorEmail, &feature, &e.Action, &e.ResourceID, &e.Timestamp); err == nil {
				e.Source = "activity_log"
				e.Resource = feature
				out = append(out, e)
			}
		}
		rows3.Close()
	}

	// Merge: sort desc by timestamp, cap at limit.
	sortByTimestampDesc(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func sortByTimestampDesc(s []FeedEntry) {
	// Insertion sort — fine for 150-row merges, O(n²) on sorted data → O(n).
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Timestamp.After(s[j-1].Timestamp); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Silence unused-import warnings in case fmt is not referenced.
var _ = fmt.Sprintf
