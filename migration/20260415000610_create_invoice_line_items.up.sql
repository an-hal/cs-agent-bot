-- Migration: 20260415000610 — Create invoice_line_items table
-- Per spec §2: each invoice can have multiple line items (description, qty, unit_price, subtotal).
-- subtotal is computed in app layer (qty × unit_price) before INSERT.

CREATE TABLE IF NOT EXISTS invoice_line_items (
  id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id   VARCHAR(50)  NOT NULL REFERENCES invoices(invoice_id) ON DELETE CASCADE,
  workspace_id UUID         NOT NULL REFERENCES workspaces(id),

  description  VARCHAR(500) NOT NULL,
  qty          INT          NOT NULL DEFAULT 1,
  unit_price   BIGINT       NOT NULL DEFAULT 0,
  subtotal     BIGINT       NOT NULL DEFAULT 0,   -- computed: qty × unit_price (written by app)

  sort_order   INT          DEFAULT 0,
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ili_invoice   ON invoice_line_items(invoice_id);
CREATE INDEX IF NOT EXISTS idx_ili_workspace ON invoice_line_items(workspace_id);
