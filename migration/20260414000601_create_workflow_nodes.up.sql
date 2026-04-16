-- feat/06-workflow-engine: workflow_nodes table
CREATE TABLE workflow_nodes (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id     UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

  node_id         VARCHAR(100) NOT NULL,
  node_type       VARCHAR(20)  NOT NULL DEFAULT 'workflow',

  position_x      FLOAT NOT NULL DEFAULT 0,
  position_y      FLOAT NOT NULL DEFAULT 0,
  width           FLOAT,
  height          FLOAT,

  data            JSONB NOT NULL DEFAULT '{}',

  draggable       BOOLEAN DEFAULT TRUE,
  selectable      BOOLEAN DEFAULT TRUE,
  connectable     BOOLEAN DEFAULT TRUE,
  z_index         INT DEFAULT 0,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE(workflow_id, node_id)
);

CREATE INDEX idx_wn_workflow        ON workflow_nodes(workflow_id);
CREATE INDEX idx_wn_data_trigger    ON workflow_nodes USING GIN(data);
