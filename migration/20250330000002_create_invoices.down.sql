-- Migration: Drop invoices table
-- Version: 20250330000002
-- Description: Drops the invoices table and its indexes

DROP INDEX IF EXISTS idx_invoices_issue_date;
DROP INDEX IF EXISTS idx_invoices_collection_stage;
DROP INDEX IF EXISTS idx_invoices_due_date;
DROP INDEX IF EXISTS idx_invoices_payment_status;
DROP INDEX IF EXISTS idx_invoices_company_id;

DROP TABLE IF EXISTS invoices;
