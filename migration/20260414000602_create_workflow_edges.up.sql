-- feat/06-workflow-engine: workflow_edges table
CREATE TABLE workflow_edges (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  edge_id         VARCHAR(100) NOT NULL,
  source_node_id  VARCHAR(100) NOT NULL,
  target_node_id  VARCHAR(100) NOT NULL,
  source_handle   VARCHAR(50),
  target_handle   VARCHAR(50),

  label           VARCHAR(255),
  animated        BOOLEAN DEFAULT FALSE,
  style           JSONB,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, edge_id)
);

CREATE INDEX idx_we_workflow ON workflow_edges(workflow_id);
CREATE INDEX idx_we_source   ON workflow_edges(workflow_id, source_node_id);
CREATE INDEX idx_we_target   ON workflow_edges(workflow_id, target_node_id);
