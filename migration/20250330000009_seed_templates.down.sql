-- Migration: Seed Templates (Rollback)
-- Version: 20250330000009
-- Description: Removes all seeded templates

-- Drop indexes
DROP INDEX IF EXISTS idx_templates_category_active;
DROP INDEX IF EXISTS idx_templates_id_active;

-- Remove all seeded templates
DELETE FROM templates WHERE template_id LIKE 'TPL-%';
