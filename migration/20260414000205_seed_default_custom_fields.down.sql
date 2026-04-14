DELETE FROM custom_field_definitions
WHERE workspace_id IN (SELECT id FROM workspaces WHERE slug = 'dealls');
