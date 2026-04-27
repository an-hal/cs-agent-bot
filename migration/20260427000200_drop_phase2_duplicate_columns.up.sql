-- Phase 2: drop 4 columns that are already mirrored in custom_field_definitions.
-- Source-of-truth shifts entirely to clients.custom_fields JSONB.
--
-- Columns dropped:
--   1. hc_size       (CF key: hc_size or headcount, depending on workspace)
--   2. plan_type     (CF key: plan_type)
--   3. nps_score     (CF key: nps_score)
--   4. usage_score   (CF key: usage_score)
--
-- Data preservation: per-row, every non-NULL column value is merged into
-- custom_fields under the same snake_case key. If the CF key already exists
-- (e.g. populated by API), the column value is skipped (custom_fields wins).

UPDATE clients
SET custom_fields = COALESCE(
        (SELECT jsonb_object_agg(key, value)
         FROM (
             VALUES
                 ('hc_size', to_jsonb(hc_size)),
                 ('plan_type', to_jsonb(plan_type)),
                 ('nps_score', to_jsonb(nps_score)),
                 ('usage_score', to_jsonb(usage_score))
         ) AS kv(key, value)
         WHERE value IS NOT NULL AND value <> 'null'::jsonb
           AND NOT (custom_fields ? key)),
        '{}'::jsonb
    ) || custom_fields
WHERE
    hc_size     IS NOT NULL
 OR plan_type   IS NOT NULL
 OR nps_score   IS NOT NULL
 OR usage_score IS NOT NULL;

ALTER TABLE clients
    DROP COLUMN IF EXISTS hc_size,
    DROP COLUMN IF EXISTS plan_type,
    DROP COLUMN IF EXISTS nps_score,
    DROP COLUMN IF EXISTS usage_score;
