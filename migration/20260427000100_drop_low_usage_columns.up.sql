-- Phase 1: Move 8 low-usage `clients` columns into `custom_fields` JSONB,
-- then drop them. Goal: shrink the table to a generic CRM core, push
-- workspace-specific fields to JSONB so they can be configured via
-- /master-data/field-definitions per workspace.
--
-- Columns being moved/dropped:
--   1. first_time_discount_pct
--   2. next_discount_pct_manual
--   3. quotation_link_expires
--   4. bd_prospect_id
--   5. renewal_date
--   6. usage_score_avg_30d
--   7. wa_undeliverable
--   8. churn_reason
--
-- Data preservation: per-row, every non-NULL value is merged into
-- custom_fields under the same snake_case key. NULL values are skipped
-- (custom_fields stays absent — same semantics as a missing column).
--
-- Reversal: down migration recreates the columns, but only churn_reason and
-- bd_prospect_id can be restored from custom_fields losslessly. Numeric and
-- date columns lose precision when round-tripping through JSONB strings, so
-- the down migration restores defaults — manual restore needed if a rollback
-- requires accurate numeric/date values.

UPDATE clients
SET custom_fields = custom_fields ||
    COALESCE(
        (SELECT jsonb_object_agg(key, value)
         FROM (
             VALUES
                 ('first_time_discount_pct', to_jsonb(first_time_discount_pct)),
                 ('next_discount_pct_manual', to_jsonb(next_discount_pct_manual)),
                 ('quotation_link_expires', to_jsonb(quotation_link_expires)),
                 ('bd_prospect_id', to_jsonb(bd_prospect_id)),
                 ('renewal_date', to_jsonb(renewal_date)),
                 ('usage_score_avg_30d', to_jsonb(usage_score_avg_30d)),
                 ('wa_undeliverable', to_jsonb(wa_undeliverable)),
                 ('churn_reason', to_jsonb(churn_reason))
         ) AS kv(key, value)
         WHERE value IS NOT NULL AND value <> 'null'::jsonb),
        '{}'::jsonb
    )
WHERE
    first_time_discount_pct  IS NOT NULL
 OR next_discount_pct_manual IS NOT NULL
 OR quotation_link_expires   IS NOT NULL
 OR bd_prospect_id           IS NOT NULL
 OR renewal_date              IS NOT NULL
 OR usage_score_avg_30d      IS NOT NULL
 OR wa_undeliverable          IS NOT NULL
 OR churn_reason              IS NOT NULL;

ALTER TABLE clients
    DROP COLUMN IF EXISTS first_time_discount_pct,
    DROP COLUMN IF EXISTS next_discount_pct_manual,
    DROP COLUMN IF EXISTS quotation_link_expires,
    DROP COLUMN IF EXISTS bd_prospect_id,
    DROP COLUMN IF EXISTS renewal_date,
    DROP COLUMN IF EXISTS usage_score_avg_30d,
    DROP COLUMN IF EXISTS wa_undeliverable,
    DROP COLUMN IF EXISTS churn_reason;
