-- feat/06-workflow-engine: automation_rules table
CREATE TABLE automation_rules (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  rule_code       VARCHAR(100) NOT NULL,
  trigger_id      VARCHAR(100) NOT NULL,
  template_id     VARCHAR(100),

  role            VARCHAR(10)  NOT NULL,
  phase           VARCHAR(20)  NOT NULL,
  phase_label     VARCHAR(100),
  priority        VARCHAR(20),

  timing          TEXT NOT NULL,
  condition       TEXT NOT NULL,
  stop_if         TEXT NOT NULL DEFAULT '-',
  sent_flag       VARCHAR(200),
  channel         VARCHAR(20) NOT NULL DEFAULT 'whatsapp',

  status          VARCHAR(20) NOT NULL DEFAULT 'active',

  -- Legacy trigger_rules FK (nullable — set during backfill migration 000610)
  legacy_trigger_rule_id VARCHAR(50) REFERENCES trigger_rules(rule_id) ON DELETE SET NULL,

  updated_at      TIMESTAMPTZ,
  updated_by      VARCHAR(255),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workspace_id, rule_code)
);

CREATE INDEX idx_ar_workspace   ON automation_rules(workspace_id);
CREATE INDEX idx_ar_status      ON automation_rules(workspace_id, status);
CREATE INDEX idx_ar_role        ON automation_rules(workspace_id, role);
CREATE INDEX idx_ar_trigger     ON automation_rules(trigger_id);
CREATE INDEX idx_ar_phase       ON automation_rules(workspace_id, phase);
