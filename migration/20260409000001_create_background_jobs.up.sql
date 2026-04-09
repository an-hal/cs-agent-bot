-- Migration: Create background_jobs table
-- Version: 20260409000001
-- Description: Generic background job tracker for import, export, and future async operations.

CREATE TABLE background_jobs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID        NOT NULL REFERENCES workspaces(id),
    job_type      VARCHAR(20) NOT NULL,                  -- 'import', 'export'
    status        VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'done', 'failed'
    entity_type   VARCHAR(50) NOT NULL,                  -- 'client', etc.
    filename      TEXT,                                   -- original upload name or generated export name
    storage_path  TEXT,                                   -- local/GCS path for output file (export only)
    total_rows    INT         NOT NULL DEFAULT 0,
    processed     INT         NOT NULL DEFAULT 0,
    success       INT         NOT NULL DEFAULT 0,
    failed        INT         NOT NULL DEFAULT 0,
    skipped       INT         NOT NULL DEFAULT 0,
    errors        JSONB,                                  -- [{row, ref_id, reason}]
    metadata      JSONB,                                  -- filters applied (export) or options (import)
    created_by    TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_background_jobs_workspace ON background_jobs(workspace_id);
CREATE INDEX idx_background_jobs_type_status ON background_jobs(job_type, status);
