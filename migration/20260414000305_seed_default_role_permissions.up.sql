-- Feature 04: seed default role_permissions matrix for each existing workspace.
-- Runs once — subsequent workspaces get their matrix seeded by the team usecase
-- at workspace-create time (not this migration). Idempotent via ON CONFLICT.

-- Super Admin: full access, view_list='all' on every module for every workspace.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       'all', TRUE, TRUE, TRUE, TRUE, TRUE, TRUE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'Super Admin'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- Admin: full within workspace, no delete on team.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       'true', TRUE, TRUE, TRUE,
       CASE WHEN m.module_id = 'team' THEN FALSE ELSE TRUE END,
       TRUE,
       CASE WHEN m.module_id IN ('data_master','ae','sdr','bd','cs') THEN TRUE ELSE FALSE END
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'Admin'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- Manager: view/create/edit, no delete, no team module.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE WHEN m.module_id = 'team' THEN 'false' ELSE 'true' END,
       m.module_id <> 'team',
       m.module_id <> 'team',
       m.module_id <> 'team',
       FALSE,
       m.module_id <> 'team',
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'Manager'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- AE Officer: full ae+data_master, read overview, no team.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE WHEN m.module_id = 'team' THEN 'false' ELSE 'true' END,
       m.module_id <> 'team',
       m.module_id IN ('ae', 'data_master'),
       m.module_id IN ('ae', 'data_master'),
       FALSE,
       m.module_id IN ('ae', 'data_master'),
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'AE Officer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- SDR Officer: full sdr, read overview + data_master.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE WHEN m.module_id = 'team' THEN 'false' ELSE 'true' END,
       m.module_id <> 'team',
       m.module_id = 'sdr',
       m.module_id = 'sdr',
       FALSE,
       m.module_id IN ('sdr', 'data_master'),
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'SDR Officer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- CS Officer: full cs, read rest except team.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE WHEN m.module_id = 'team' THEN 'false' ELSE 'true' END,
       m.module_id <> 'team',
       m.module_id = 'cs',
       m.module_id = 'cs',
       FALSE,
       m.module_id = 'cs',
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'CS Officer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- Finance: read-only + export, holding scope via view_list='all'.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE WHEN m.module_id = 'team' THEN 'false'
            WHEN m.module_id IN ('reports','analytics','ae','data_master') THEN 'all'
            ELSE 'true' END,
       m.module_id <> 'team',
       FALSE, FALSE, FALSE,
       m.module_id IN ('reports','analytics','ae','data_master'),
       FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'Finance'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;

-- Viewer: read-only except team.
INSERT INTO role_permissions
    (role_id, workspace_id, module_id,
     view_list, view_detail, can_create, can_edit, can_delete, can_export, can_import)
SELECT r.id, w.id, m.module_id,
       CASE WHEN m.module_id = 'team' THEN 'false' ELSE 'true' END,
       m.module_id <> 'team',
       FALSE, FALSE, FALSE, FALSE, FALSE
FROM roles r
CROSS JOIN workspaces w
CROSS JOIN (VALUES
    ('dashboard'), ('analytics'), ('reports'),
    ('ae'), ('sdr'), ('bd'), ('cs'),
    ('data_master'), ('team')
) AS m(module_id)
WHERE r.name = 'Viewer'
ON CONFLICT (role_id, workspace_id, module_id) DO NOTHING;
