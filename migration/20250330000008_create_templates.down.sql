-- Migration: Drop templates table
-- Version: 20250330000008
-- Description: Drops the templates table and its indexes

DROP INDEX IF EXISTS idx_templates_name;
DROP INDEX IF EXISTS idx_templates_language;
DROP INDEX IF EXISTS idx_templates_active;
DROP INDEX IF EXISTS idx_templates_category;

DROP TABLE IF EXISTS templates;
