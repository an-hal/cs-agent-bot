-- Migration: Add resource_type column to activity_log
-- Version: 20260410000001
-- Description: Allows per-module activity feed filtering (client, invoice, template, trigger_rule, bot)

ALTER TABLE activity_log ADD COLUMN resource_type VARCHAR(50);

CREATE INDEX idx_al_resource_type ON activity_log (workspace_id, resource_type, occurred_at DESC);
