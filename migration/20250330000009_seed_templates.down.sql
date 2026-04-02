-- Migration: Seed Templates (Rollback)
-- Version: 20250330000009
-- Description: Removes all seeded templates

-- Drop indexes
DROP INDEX IF EXISTS idx_templates_category_active;
DROP INDEX IF EXISTS idx_templates_id_active;

-- Remove all seeded templates
DELETE FROM templates WHERE template_id IN (
	'REN60','REN45','REN30','REN15','REN0',
	'TPL-PAY-PRE14','TPL-PAY-PRE7','TPL-PAY-PRE3',
	'TPL-PAY-POST1','TPL-PAY-POST4','TPL-PAY-POST8','TPL-PAY-POST15',
	'TPL-CHECKIN-FORM','TPL-CHECKIN-CALL',
	'NPS1','NPS2','NPS3','REFERRAL',
	'CS_H7','CS_H14','CS_H21','CS_H30','CS_H45','CS_H60','CS_H75','CS_H90',
	'CS_LT1','CS_LT2','CS_LT3',
	'LOW_USAGE','LOW_NPS'
);
