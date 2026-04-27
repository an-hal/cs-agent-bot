-- Reverse Phase 2: re-add 4 columns. Best-effort restore from custom_fields
-- (string types restore cleanly, numeric types via JSONB string parse).

ALTER TABLE clients ADD COLUMN IF NOT EXISTS hc_size     VARCHAR(20);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS plan_type   VARCHAR(50);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS nps_score   SMALLINT;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS usage_score SMALLINT;

UPDATE clients SET
    hc_size     = COALESCE(custom_fields->>'hc_size', hc_size),
    plan_type   = COALESCE(custom_fields->>'plan_type', plan_type),
    nps_score   = COALESCE((custom_fields->>'nps_score')::smallint, nps_score),
    usage_score = COALESCE((custom_fields->>'usage_score')::smallint, usage_score);
