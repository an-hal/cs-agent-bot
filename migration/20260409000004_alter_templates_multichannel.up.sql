-- Migration: Alter templates table for multi-channel support
-- Version: 20260409000004
-- Description: Adds channel column and email-specific fields. Renames template_content to wa_content.
-- Converts all template variables from [Bracket] to {Bracket} syntax.

-- Add new columns
ALTER TABLE templates ADD COLUMN IF NOT EXISTS channel VARCHAR(10) NOT NULL DEFAULT 'wa';
ALTER TABLE templates ADD COLUMN IF NOT EXISTS email_subject VARCHAR(255);
ALTER TABLE templates ADD COLUMN IF NOT EXISTS email_body_html TEXT;
ALTER TABLE templates ADD COLUMN IF NOT EXISTS email_body_text TEXT;

-- Rename template_content to wa_content for clarity
ALTER TABLE templates RENAME COLUMN template_content TO wa_content;

-- Convert all existing templates from [Bracket] to {bracket} syntax (lowercase)
UPDATE templates SET wa_content = REPLACE(REPLACE(wa_content, '[', '{'), ']', '}');

-- Lowercase all variable names inside {brackets}
UPDATE templates SET wa_content = REPLACE(wa_content, '{Company_Name}', '{company_name}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Company_ID}', '{company_id}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{PIC_Name}', '{pic_name}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Owner_Name}', '{owner_name}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Owner_WA}', '{owner_wa}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Due_Date}', '{due_date}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Invoice_ID}', '{invoice_id}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Benefit_Referral}', '{benefit_referral}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Esc_ID}', '{esc_id}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Priority}', '{priority}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Reason}', '{reason}');
UPDATE templates SET wa_content = REPLACE(wa_content, '{Status}', '{status}');

-- Update composite unique constraint: (template_id, channel)
-- template_id is already PK, so we add a unique index for the pair
CREATE UNIQUE INDEX IF NOT EXISTS idx_templates_id_channel ON templates(template_id, channel);

-- Index for channel filtering
CREATE INDEX IF NOT EXISTS idx_templates_channel ON templates(channel);
