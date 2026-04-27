-- Reverse Phase 1: re-add the 8 columns. String/UUID values are restored from
-- custom_fields JSONB; numeric/date values are NOT round-tripped (lossy via
-- JSONB string representation). Manual restore needed for full numeric/date
-- accuracy if rollback requires it.

ALTER TABLE clients ADD COLUMN IF NOT EXISTS first_time_discount_pct  NUMERIC(5,2);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS next_discount_pct_manual NUMERIC(5,2);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS quotation_link_expires   DATE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS bd_prospect_id           VARCHAR(50);
ALTER TABLE clients ADD COLUMN IF NOT EXISTS renewal_date             DATE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS usage_score_avg_30d      SMALLINT;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS wa_undeliverable         BOOLEAN DEFAULT FALSE;
ALTER TABLE clients ADD COLUMN IF NOT EXISTS churn_reason             VARCHAR(200);

-- Best-effort restore from JSONB for the safe types only.
UPDATE clients SET
    bd_prospect_id   = COALESCE(custom_fields->>'bd_prospect_id', bd_prospect_id),
    churn_reason     = COALESCE(custom_fields->>'churn_reason', churn_reason),
    wa_undeliverable = COALESCE((custom_fields->>'wa_undeliverable')::boolean, wa_undeliverable);

UPDATE clients SET custom_fields = custom_fields - 'first_time_discount_pct'
                                                 - 'next_discount_pct_manual'
                                                 - 'quotation_link_expires'
                                                 - 'bd_prospect_id'
                                                 - 'renewal_date'
                                                 - 'usage_score_avg_30d'
                                                 - 'wa_undeliverable'
                                                 - 'churn_reason';
