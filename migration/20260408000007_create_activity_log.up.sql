-- Migration: Create activity_log table
-- Version: 20260408000007
-- Description: Unified audit trail for all app activities (bot, data, team)

CREATE TABLE activity_log (
  id           BIGSERIAL    PRIMARY KEY,
  workspace_id VARCHAR(50),
  category     VARCHAR(10)  NOT NULL,   -- bot | data | team
  actor_type   VARCHAR(10)  NOT NULL,   -- bot | human
  actor        VARCHAR(100),            -- email or 'bot'
  action       VARCHAR(100) NOT NULL,
  target       VARCHAR(200),
  detail       TEXT,
  ref_id       VARCHAR(50),             -- company_id, member email, etc.
  status       VARCHAR(30),             -- delivered|escalated|failed (bot only)
  occurred_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_al_workspace_id ON activity_log(workspace_id);
CREATE INDEX idx_al_category     ON activity_log(category);
CREATE INDEX idx_al_occurred_at  ON activity_log(occurred_at);
CREATE INDEX idx_al_actor        ON activity_log(actor);
