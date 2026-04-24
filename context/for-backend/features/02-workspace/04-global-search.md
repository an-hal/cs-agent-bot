# Global Search

## Overview
Global search (Cmd+K atau klik search di Header) query ke multiple data sources sekaligus.
Frontend: `Header.tsx` (search modal) + `CommandPalette.tsx` (Cmd+K page navigation).

## Current Frontend Implementation
```
GET /api/search?q=keyword&workspace_id=UUID

→ Backend searches in parallel:
  1. Master Data clients (company_name, company_id, pic_name)
  2. Invoices (invoice_id, company_name)

→ Response grouped by type
```

## API Endpoint

### GET `/search`
Full-text search across multiple entities.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}

Query:
  ?q=keyword          (required, min 2 chars)
  &types=clients,invoices,templates   (optional, default: all)
  &limit=10           (per type, default: 10)

Response 200:
{
  "clients": [
    {
      "id": "uuid",
      "company_id": "DE-001",
      "company_name": "PT Dealls Tech",
      "pic_name": "John",
      "stage": "CLIENT",
      "match_field": "company_name"
    }
  ],
  "invoices": [
    {
      "id": "uuid",
      "invoice_id": "INV-DE-2026-001",
      "company_name": "PT Dealls Tech",
      "amount": 25000000,
      "payment_status": "Lunas",
      "match_field": "invoice_id"
    }
  ],
  "templates": [
    {
      "id": "TPL-OB-WELCOME",
      "name": "Welcome Message",
      "channel": "whatsapp",
      "match_field": "name"
    }
  ]
}
```

## Database Query
```sql
-- Parallel queries, UNION or separate:

-- Clients
SELECT id, company_id, company_name, pic_name, stage
FROM master_data
WHERE workspace_id = $1
  AND (
    company_name ILIKE '%' || $2 || '%'
    OR company_id ILIKE '%' || $2 || '%'
    OR pic_name ILIKE '%' || $2 || '%'
  )
LIMIT 10;

-- Invoices
SELECT id, invoice_id, company_name, amount, payment_status
FROM invoices
WHERE workspace_id = $1
  AND (
    invoice_id ILIKE '%' || $2 || '%'
    OR company_name ILIKE '%' || $2 || '%'
  )
LIMIT 10;

-- Templates (optional)
SELECT template_id, name, channel
FROM message_templates
WHERE workspace_id = $1
  AND (
    template_id ILIKE '%' || $2 || '%'
    OR name ILIKE '%' || $2 || '%'
  )
LIMIT 10;
```

## Performance
- Use `pg_trgm` extension for fuzzy search: `CREATE EXTENSION IF NOT EXISTS pg_trgm;`
- Add GIN trigram indexes:
  ```sql
  CREATE INDEX idx_md_search ON master_data USING GIN(company_name gin_trgm_ops);
  CREATE INDEX idx_inv_search ON invoices USING GIN(invoice_id gin_trgm_ops);
  ```
- Execute queries in parallel (Go goroutines + errgroup)
- Cache hot results in Redis with 60s TTL (optional)

## Holding View
If workspace is holding → search across all member workspace IDs:
```sql
WHERE workspace_id IN (SELECT unnest(member_ids) FROM workspaces WHERE id = $1)
```
