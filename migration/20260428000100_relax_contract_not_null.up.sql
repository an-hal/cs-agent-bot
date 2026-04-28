-- Phase 5 made contract_end + pic_wa + owner_telegram_id nullable to support
-- non-contract / non-WA business models. It missed contract_start and
-- contract_months — both still NOT NULL — which broke the OneSchema-style
-- wizard: a maker who maps only the columns they have (e.g., a non-contract
-- CRM uses only company_id/name/pic) gets "null value in column
-- contract_start violates not-null constraint" on every row at apply time.
--
-- Drop the NOT NULL on both. The master_data view + repo SELECTs already
-- handle nil contract_start via COALESCE / *time.Time, and activation_date
-- (still NOT NULL) gets COALESCE(contract_start, NOW()::date) at INSERT time
-- so the legacy "client = has contract" model still works for callers that
-- DO provide it.

ALTER TABLE clients ALTER COLUMN contract_start  DROP NOT NULL;
ALTER TABLE clients ALTER COLUMN contract_months DROP NOT NULL;
