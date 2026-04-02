-- Migration: Add feature_update_sent to client_flags
-- Version: 20250401000007

ALTER TABLE client_flags ADD COLUMN IF NOT EXISTS feature_update_sent BOOLEAN DEFAULT FALSE;
