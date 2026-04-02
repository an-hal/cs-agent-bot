-- Migration: Add feature_update_sent to client_flags (Rollback)
-- Version: 20250401000007

ALTER TABLE client_flags DROP COLUMN IF EXISTS feature_update_sent;
