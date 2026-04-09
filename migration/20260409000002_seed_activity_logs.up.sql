-- Migration: Seed activity_log
-- Version: 20260409000002
-- Description: Seed sample data for activity logs

WITH dealls_ws (id) AS (
    SELECT id FROM workspaces WHERE slug = 'dealls' LIMIT 1
)
INSERT INTO activity_log (workspace_id, category, actor_type, actor, action, target, detail, ref_id, status, occurred_at)
VALUES
  ( (SELECT id FROM dealls_ws), 'bot', 'bot', 'bot', 'Send Document Request', 'PT. ABC Indonesia', 'Sent annual KYC update request via WA', 'COMPANY-101', 'delivered', NOW() - INTERVAL '1 day' ),
  ( (SELECT id FROM dealls_ws), 'team', 'human', 'admin@dealls.com', 'Update Client Plan', 'PT. DEF Utama', 'Upgraded from Basic to Premium plan', 'COMPANY-102', NULL, NOW() - INTERVAL '5 hours' ),
  ( (SELECT id FROM dealls_ws), 'data', 'human', 'operator@dealls.com', 'Import Client Master', '150 Records', 'Successfully imported clients from CSV upload', 'JOB-001', NULL, NOW() - INTERVAL '2 hours' ),
  ( (SELECT id FROM dealls_ws), 'bot', 'bot', 'bot', 'Follow Up Invoice', 'PT. XYZ Sinergi', 'Sent 3rd reminder for invoice INV-2026-001', 'INV-2026-001', 'escalated', NOW() - INTERVAL '30 minutes' ),
  ( (SELECT id FROM dealls_ws), 'team', 'human', 'manager@dealls.com', 'Resolve Escalation', 'Ticket #8844', 'Manual resolve after client confirmation', 'TCK-8844', NULL, NOW() - INTERVAL '10 minutes' );
