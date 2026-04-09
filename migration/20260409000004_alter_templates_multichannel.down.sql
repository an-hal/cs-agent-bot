-- Revert {Bracket} back to [Bracket]
UPDATE templates SET wa_content = REPLACE(REPLACE(wa_content, '{', '['), '}', ']');

-- Rename back
ALTER TABLE templates RENAME COLUMN wa_content TO template_content;

-- Drop new columns
ALTER TABLE templates DROP COLUMN IF EXISTS email_body_text;
ALTER TABLE templates DROP COLUMN IF EXISTS email_body_html;
ALTER TABLE templates DROP COLUMN IF EXISTS email_subject;
ALTER TABLE templates DROP COLUMN IF EXISTS channel;

-- Drop indexes
DROP INDEX IF EXISTS idx_templates_id_channel;
DROP INDEX IF EXISTS idx_templates_channel;
