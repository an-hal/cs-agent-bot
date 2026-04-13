-- Migration: Seed whitelist with super admin emails
-- Version: 20260413000002

INSERT INTO whitelist (email, is_active, added_by, notes) VALUES
  ('arief.faltah@dealls.com',   TRUE, 'system', 'Super Admin'),
  ('dhimas.priyadi@sejutacita.id', TRUE, 'system', 'Super Admin')
ON CONFLICT (email) DO NOTHING;
