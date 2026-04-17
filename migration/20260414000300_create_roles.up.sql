-- Feature 04: Team management — roles table.

CREATE TABLE IF NOT EXISTS roles (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT         NOT NULL DEFAULT '',
    color       VARCHAR(7)   NOT NULL DEFAULT '',
    bg_color    VARCHAR(7)   NOT NULL DEFAULT '',
    is_system   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_roles_is_system ON roles(is_system);

-- Seed default system roles. Custom roles may also be created via API.
INSERT INTO roles (name, description, color, bg_color, is_system) VALUES
    ('Super Admin', 'Akses penuh ke semua modul dan semua workspace.', '#EF4444', '#FEF2F2', TRUE),
    ('Admin',       'Akses penuh dalam 1 workspace.',                  '#F59E0B', '#FFFBEB', TRUE),
    ('Manager',     'Multi-workspace view/create/edit (no delete).',   '#0EA5E9', '#F0F9FF', TRUE),
    ('AE Officer',  'Account Executive officer.',                      '#8B5CF6', '#F5F3FF', TRUE),
    ('SDR Officer', 'Sales Development Rep officer.',                  '#10B981', '#ECFDF5', TRUE),
    ('CS Officer',  'Customer Service officer.',                       '#14B8A6', '#F0FDFA', TRUE),
    ('Finance',     'Read-only reports, analytics, AE, data master.',  '#6366F1', '#EEF2FF', TRUE),
    ('Viewer',      'Read-only across most modules.',                  '#6B7280', '#F3F4F6', TRUE)
ON CONFLICT (name) DO NOTHING;
