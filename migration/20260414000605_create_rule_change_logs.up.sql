-- feat/06-workflow-engine: rule_change_logs table (INSERT-only audit trail)
CREATE TABLE rule_change_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_id         UUID NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  field           VARCHAR(50)  NOT NULL,
  old_value       TEXT,
  new_value       TEXT NOT NULL DEFAULT '',

  edited_by       VARCHAR(255) NOT NULL,
  edited_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rcl_rule       ON rule_change_logs(rule_id);
CREATE INDEX idx_rcl_workspace  ON rule_change_logs(workspace_id);
CREATE INDEX idx_rcl_edited_at  ON rule_change_logs(workspace_id, edited_at DESC);

-- INSERT-only: revoke UPDATE and DELETE from all non-superuser roles
REVOKE UPDATE, DELETE ON rule_change_logs FROM PUBLIC;
