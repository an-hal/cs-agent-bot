-- Restore NOT NULL constraints. Backfill any rows that landed without these
-- values while the constraint was relaxed: contract_start defaults to today,
-- contract_months to 12 (the typical default for SaaS contracts in this
-- workspace). Adjust the backfill if the rollback is being run after a long
-- period of relaxed inserts.

UPDATE clients SET contract_start = COALESCE(contract_start, activation_date, NOW()::date) WHERE contract_start IS NULL;
UPDATE clients SET contract_months = COALESCE(contract_months, 12) WHERE contract_months IS NULL;

ALTER TABLE clients ALTER COLUMN contract_start  SET NOT NULL;
ALTER TABLE clients ALTER COLUMN contract_months SET NOT NULL;
