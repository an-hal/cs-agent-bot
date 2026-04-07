-- Migration: Seed workspaces
-- Version: 20260406000003
-- Description: Seeds default workspaces (dealls, kantorku, holding)

INSERT INTO workspaces (id, name, slug, logo, color, plan) VALUES
  (gen_random_uuid(), 'Dealls', 'dealls', 'DE', '#534AB7', 'Enterprise'),
  (gen_random_uuid(), 'KantorKu', 'kantorku', 'KK', '#1D9E75', 'Pro');

INSERT INTO workspaces (id, name, slug, logo, color, plan, is_holding, member_ids) VALUES
  (gen_random_uuid(), 'Bumi Holdings', 'holding', 'BH', '#0EA5E9', 'Holding', TRUE,
   ARRAY(SELECT id FROM workspaces WHERE slug IN ('dealls', 'kantorku')));
