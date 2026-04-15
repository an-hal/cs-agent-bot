-- feat/06-workflow-engine: workflow_steps table
CREATE TABLE workflow_steps (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id         UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  step_key            VARCHAR(50)  NOT NULL,
  label               VARCHAR(255) NOT NULL,
  phase               VARCHAR(20)  NOT NULL,
  icon                VARCHAR(10),
  description         TEXT,
  sort_order          INT DEFAULT 0,

  timing              TEXT,
  condition           TEXT,
  stop_if             TEXT,
  sent_flag           VARCHAR(100),
  template_id         VARCHAR(100),

  message_template_id VARCHAR(100),
  email_template_id   VARCHAR(100),

  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, step_key)
);

CREATE INDEX idx_ws_workflow ON workflow_steps(workflow_id);
CREATE INDEX idx_ws_phase    ON workflow_steps(workflow_id, phase);
