-- feat/06-workflow-engine: pipeline_tabs, pipeline_stats, pipeline_columns

CREATE TABLE pipeline_tabs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  tab_key         VARCHAR(50)  NOT NULL,
  label           VARCHAR(100) NOT NULL,
  icon            VARCHAR(10),
  filter          VARCHAR(100) NOT NULL DEFAULT 'all',
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, tab_key)
);

CREATE INDEX idx_pt_workflow ON pipeline_tabs(workflow_id);

-- ────────────────────────────────────────────────────────────────

CREATE TABLE pipeline_stats (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  stat_key        VARCHAR(50)  NOT NULL,
  label           VARCHAR(100) NOT NULL,
  metric          VARCHAR(100) NOT NULL,
  color           VARCHAR(50),
  border          VARCHAR(50),
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, stat_key)
);

CREATE INDEX idx_ps_workflow ON pipeline_stats(workflow_id);

-- ────────────────────────────────────────────────────────────────

CREATE TABLE pipeline_columns (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  column_key      VARCHAR(50)  NOT NULL,
  field           VARCHAR(100) NOT NULL,
  label           VARCHAR(100) NOT NULL,
  width           INT DEFAULT 120,
  visible         BOOLEAN DEFAULT TRUE,
  sort_order      INT DEFAULT 0,

  UNIQUE(workflow_id, column_key)
);

CREATE INDEX idx_pc_workflow ON pipeline_columns(workflow_id);
