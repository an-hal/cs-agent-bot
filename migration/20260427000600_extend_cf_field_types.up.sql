-- Phase 5 (continued): extend custom_field_definitions.field_type enum to
-- support cross-business modelling. Adds:
--   - money:        numeric value with currency code, e.g. {"value": 50000, "currency": "USD"}
--   - phone:        normalized phone number (E.164 or local)
--   - multi_select: multiple values from `options` array
--   - percentage:   number constrained 0-100, rendered with % suffix in UI
--
-- Existing types (text/number/date/boolean/select/url/email) keep working.

ALTER TABLE custom_field_definitions
    DROP CONSTRAINT IF EXISTS custom_field_definitions_field_type_check;

ALTER TABLE custom_field_definitions
    ADD CONSTRAINT custom_field_definitions_field_type_check
    CHECK (field_type IN (
        'text',
        'number',
        'date',
        'boolean',
        'select',
        'multi_select',
        'url',
        'email',
        'money',
        'phone',
        'percentage'
    ));
