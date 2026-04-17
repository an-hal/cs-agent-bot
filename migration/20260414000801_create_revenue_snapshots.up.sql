CREATE TABLE IF NOT EXISTS revenue_snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    year            INT NOT NULL,
    month           INT NOT NULL CHECK (month BETWEEN 1 AND 12),
    revenue_actual  BIGINT NOT NULL DEFAULT 0,
    deals_won       INT NOT NULL DEFAULT 0,
    deals_lost      INT NOT NULL DEFAULT 0,
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, year, month)
);

CREATE INDEX idx_revenue_snapshots_workspace ON revenue_snapshots (workspace_id);
CREATE INDEX idx_revenue_snapshots_period ON revenue_snapshots (year, month);
