-- Feature 04: extend approval_requests_min scaffold with fields used by the
-- team approvals flow. Additive only — feat/03 still works as before.

ALTER TABLE approval_requests
    ADD COLUMN IF NOT EXISTS resource_id UUID;

CREATE INDEX IF NOT EXISTS idx_ar_resource ON approval_requests(resource_id);
