-- Reverse: restore original 7 field types. Any def using new types
-- (money/phone/multi_select/percentage) gets coerced to 'text' to avoid
-- losing the row.

UPDATE custom_field_definitions
SET field_type = 'text'
WHERE field_type IN ('money', 'phone', 'multi_select', 'percentage');

ALTER TABLE custom_field_definitions
    DROP CONSTRAINT IF EXISTS custom_field_definitions_field_type_check;

ALTER TABLE custom_field_definitions
    ADD CONSTRAINT custom_field_definitions_field_type_check
    CHECK (field_type IN ('text', 'number', 'date', 'boolean', 'select', 'url', 'email'));
