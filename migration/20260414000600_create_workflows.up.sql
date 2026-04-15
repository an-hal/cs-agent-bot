-- feat/06-workflow-engine: workflows table
CREATE TABLE workflows (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),

  name            VARCHAR(255) NOT NULL,
  icon            VARCHAR(10),
  slug            VARCHAR(100),
  description     TEXT,
  status          VARCHAR(20) NOT NULL DEFAULT 'active',

  -- Stage filter stored as text array
  stage_filter    TEXT[] NOT NULL DEFAULT '{}',

  created_by      VARCHAR(255),
  updated_by      VARCHAR(255),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workspace_id, slug)
);

CREATE INDEX idx_wf_workspace ON workflows(workspace_id);
CREATE INDEX idx_wf_status    ON workflows(workspace_id, status);
