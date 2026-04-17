-- Migration: 20260415000611 — Create payment_logs table (append-only)
-- Per spec §3. Tracks all invoice events: payment, status change, reminder, stage change, webhook.
-- REVOKE prevents accidental UPDATE/DELETE — logs are immutable by design.

CREATE TABLE IF NOT EXISTS payment_logs (
  id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID         NOT NULL REFERENCES workspaces(id),
  invoice_id      VARCHAR(50)  NOT NULL REFERENCES invoices(invoice_id),

  -- ══ Event classification ══
  event_type      VARCHAR(30)  NOT NULL,
  -- Allowed values: payment_received, payment_failed, status_change,
  --                 reminder_sent, stage_change, paper_id_webhook,
  --                 manual_mark_paid, created, updated

  -- ══ Payment details (payment_received / paper_id_webhook events) ══
  amount_paid     BIGINT,
  payment_method  VARCHAR(100),
  payment_channel VARCHAR(50),
  payment_ref     VARCHAR(200),

  -- ══ Status / stage change details ══
  old_status      VARCHAR(20),
  new_status      VARCHAR(20),
  old_stage       VARCHAR(30),
  new_stage       VARCHAR(30),

  -- ══ Context ══
  actor           VARCHAR(255),   -- user email | 'system' | 'paper_id_webhook'
  notes           TEXT,
  raw_payload     JSONB,          -- raw webhook payload preserved for paper_id_webhook events

  -- ══ Meta ══
  timestamp       TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- ══ Indexes (per spec) ══
CREATE INDEX IF NOT EXISTS idx_pl_workspace   ON payment_logs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_pl_invoice     ON payment_logs(invoice_id);
CREATE INDEX IF NOT EXISTS idx_pl_event_type  ON payment_logs(workspace_id, event_type);
CREATE INDEX IF NOT EXISTS idx_pl_timestamp   ON payment_logs(workspace_id, timestamp DESC);

-- ══ Append-only enforcement ══
-- Revoking UPDATE and DELETE from PUBLIC makes accidental mutations fail at the DB level.
-- The migration runner role still has INSERT and SELECT (not affected by PUBLIC revoke).
REVOKE UPDATE, DELETE ON payment_logs FROM PUBLIC;
