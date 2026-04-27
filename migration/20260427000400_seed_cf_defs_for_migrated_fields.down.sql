-- Reverse: remove definitions seeded by 20260427000400.
-- Only deletes defs whose keys match the migrated set. Operator-added defs
-- with the same key (unlikely given timing) survive only because their
-- creation timestamps differ — safer to filter by sort_order range too.
DELETE FROM custom_field_definitions
WHERE field_key IN (
    'first_time_discount_pct', 'next_discount_pct_manual', 'quotation_link_expires',
    'bd_prospect_id', 'renewal_date', 'usage_score_avg_30d', 'wa_undeliverable',
    'churn_reason', 'hc_size', 'plan_type', 'nps_score', 'usage_score',
    'cross_sell_rejected', 'cross_sell_interested', 'cross_sell_resume_date',
    'renewed', 'rejected', 'quotation_link', 'segment'
);
