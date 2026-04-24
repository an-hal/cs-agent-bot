package pdp

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

// sqlEnforcer is a concrete PolicyEnforcer that runs a parameterized DELETE
// (or UPDATE for anonymize) against a whitelisted set of tables. `archive`
// falls back to delete because this service has no warm-cold tiering — the
// same rows are cold-queued-elsewhere-by-someone-else convention.
//
// Whitelist is deliberate — we never interpolate data_class into SQL
// directly; only pre-approved mappings are executed.
type sqlEnforcer struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewSQLEnforcer returns a production PolicyEnforcer. Safe to pass into
// pdp.New() when the real DB is available.
func NewSQLEnforcer(db *sql.DB, logger zerolog.Logger) PolicyEnforcer {
	return &sqlEnforcer{db: db, logger: logger}
}

// retentionTarget describes how each data_class is scrubbed.
type retentionTarget struct {
	table     string
	tsColumn  string            // column used to compare against cutoff
	wsColumn  string            // optional workspace filter column (empty = none)
	anonymize map[string]string // columns to set to literal for anonymize action
}

var retentionTargets = map[string]retentionTarget{
	"action_log": {
		table:    "action_log",
		tsColumn: "created_at",
		wsColumn: "workspace_id",
	},
	"master_data_mutations": {
		table:    "master_data_mutations",
		tsColumn: "timestamp",
		wsColumn: "workspace_id",
	},
	"fireflies_transcripts": {
		table:    "fireflies_transcripts",
		tsColumn: "created_at",
		wsColumn: "workspace_id",
		anonymize: map[string]string{
			"transcript_text": "''",
			"participants":    "'[]'::jsonb",
			"raw_payload":     "'{}'::jsonb",
		},
	},
	"claude_extractions": {
		table:    "claude_extractions",
		tsColumn: "created_at",
		wsColumn: "workspace_id",
	},
	"notifications": {
		table:    "notifications",
		tsColumn: "created_at",
		wsColumn: "workspace_id",
	},
	"manual_action_queue": {
		table:    "manual_action_queue",
		tsColumn: "created_at",
		wsColumn: "workspace_id",
	},
	"rejection_analysis_log": {
		table:    "rejection_analysis_log",
		tsColumn: "detected_at",
		wsColumn: "workspace_id",
	},
	"audit_logs_workspace_access": {
		table:    "audit_logs_workspace_access",
		tsColumn: "created_at",
		wsColumn: "workspace_id",
	},
	"reactivation_events": {
		table:    "reactivation_events",
		tsColumn: "fired_at",
		wsColumn: "workspace_id",
	},
}

func (e *sqlEnforcer) Enforce(ctx context.Context, p entity.PDPRetentionPolicy) (int, error) {
	target, ok := retentionTargets[p.DataClass]
	if !ok {
		return 0, fmt.Errorf("pdp enforcer: unknown data_class %q — add to retentionTargets whitelist first", p.DataClass)
	}
	if p.RetentionDays <= 0 {
		return 0, nil
	}

	switch p.Action {
	case entity.PDPRetentionActionDelete, entity.PDPRetentionActionArchive, "":
		return e.delete(ctx, target, p)
	case entity.PDPRetentionActionAnonymize:
		if len(target.anonymize) == 0 {
			// No anonymize columns configured — skip safely (admin can change to delete).
			return 0, nil
		}
		return e.anonymize(ctx, target, p)
	}
	return 0, fmt.Errorf("pdp enforcer: unknown action %q", p.Action)
}

func (e *sqlEnforcer) delete(ctx context.Context, t retentionTarget, p entity.PDPRetentionPolicy) (int, error) {
	q := "DELETE FROM " + t.table + " WHERE " + t.tsColumn + " < NOW() - $1::interval"
	args := []any{fmt.Sprintf("%d days", p.RetentionDays)}
	if t.wsColumn != "" {
		q += " AND " + t.wsColumn + "::text = $2"
		args = append(args, p.WorkspaceID)
	}
	res, err := e.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("pdp delete %s: %w", t.table, err)
	}
	n, _ := res.RowsAffected()
	e.logger.Info().
		Str("data_class", p.DataClass).
		Str("table", t.table).
		Int64("rows", n).
		Msg("PDP retention enforced (delete)")
	return int(n), nil
}

func (e *sqlEnforcer) anonymize(ctx context.Context, t retentionTarget, p entity.PDPRetentionPolicy) (int, error) {
	// Build SET clause from the target's anonymize map.
	first := true
	q := "UPDATE " + t.table + " SET "
	for col, literal := range t.anonymize {
		if !first {
			q += ", "
		}
		q += col + " = " + literal
		first = false
	}
	q += " WHERE " + t.tsColumn + " < NOW() - $1::interval"
	args := []any{fmt.Sprintf("%d days", p.RetentionDays)}
	if t.wsColumn != "" {
		q += " AND " + t.wsColumn + "::text = $2"
		args = append(args, p.WorkspaceID)
	}
	res, err := e.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("pdp anonymize %s: %w", t.table, err)
	}
	n, _ := res.RowsAffected()
	e.logger.Info().
		Str("data_class", p.DataClass).
		Str("table", t.table).
		Int64("rows", n).
		Msg("PDP retention enforced (anonymize)")
	return int(n), nil
}

// sqlErasureExecutor scrubs all rows owned by the subject email across the
// tables listed in the erasure request's scope. Uses the same whitelist as
// the retention enforcer so we never hit untracked tables.
type sqlErasureExecutor struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewSQLErasureExecutor returns a production ErasureExecutor.
func NewSQLErasureExecutor(db *sql.DB, logger zerolog.Logger) ErasureExecutor {
	return &sqlErasureExecutor{db: db, logger: logger}
}

// subjectEmailColumns maps data_class → column name that stores the subject's
// email. When a scope entry is not present here, the executor skips it and
// records a "skipped" note in the summary.
var subjectEmailColumns = map[string]string{
	"fireflies_transcripts":       "host_email",
	"manual_action_queue":         "assigned_to_user",
	"notifications":               "recipient_email",
	"audit_logs_workspace_access": "actor_email",
	"master_data_mutations":       "actor_email",
	"action_log":                  "actor_email",
}

func (e *sqlErasureExecutor) Execute(ctx context.Context, req entity.PDPErasureRequest) (map[string]any, error) {
	summary := map[string]any{
		"scope":          req.Scope,
		"subject":        req.SubjectEmail,
		"scrubbed":       map[string]int{},
		"skipped_tables": []string{},
	}
	counts := map[string]int{}
	var skipped []string

	for _, dataClass := range req.Scope {
		col, ok := subjectEmailColumns[dataClass]
		if !ok {
			skipped = append(skipped, dataClass)
			continue
		}
		// DELETE rows for this subject. Use parameterized workspace + email.
		q := fmt.Sprintf("DELETE FROM %s WHERE workspace_id::text = $1 AND %s = $2", dataClass, col)
		res, err := e.db.ExecContext(ctx, q, req.WorkspaceID, req.SubjectEmail)
		if err != nil {
			e.logger.Warn().Err(err).
				Str("data_class", dataClass).
				Msg("erasure scrub failed — continuing")
			counts[dataClass] = -1
			continue
		}
		n, _ := res.RowsAffected()
		counts[dataClass] = int(n)
	}
	summary["scrubbed"] = counts
	summary["skipped_tables"] = skipped
	return summary, nil
}
