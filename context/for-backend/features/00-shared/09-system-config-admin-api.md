# System Config — Admin API

## Overview

Backend contract for the `system_config` key-value table that Admin/Lead users edit via Settings → System Config tab. Holds workspace-scoped tunables that change without code deploy: campaign deadlines, pricing anchors, prompt templates (`CLAUDE_*_PROMPT`), wiki/demo links, holiday calendar, competitor keyword list, etc.

Frontend already shipped:
- `lib/system-config-store.ts` — Zustand store for key/value editing
- `lib/hooks/use-system-config.ts` — React hook with 60s SWR cache
- Settings UI: `app/dashboard/[workspace]/settings/page.tsx` → System Config tab

This spec defines the backend contract. FE expects the API exactly as described below.

## DB Schema

```sql
CREATE TYPE system_config_data_type AS ENUM (
  'string',
  'number',
  'percentage',
  'date',
  'url',
  'json',
  'boolean'
);

CREATE TABLE system_config (
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  key             VARCHAR(100) NOT NULL,
  value           TEXT NOT NULL,                       -- always TEXT; cast at read time per data_type
  data_type       system_config_data_type NOT NULL,
  description     TEXT NOT NULL,
  is_secret       BOOLEAN NOT NULL DEFAULT FALSE,      -- if true, GET masks value
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by      VARCHAR(255) NOT NULL,
  PRIMARY KEY (workspace_id, key)
);

CREATE INDEX idx_sc_workspace ON system_config(workspace_id);
```

Composite PK on `(workspace_id, key)` — workspaces are isolated; same key can hold different values per workspace.

## Per-Key Validation

Backend MUST validate `value` against `data_type` on every write. Reject with 400 if invalid.

| `data_type` | Validation rule |
|---|---|
| `string` | Length 1–10000 |
| `number` | Parses as numeric AND `> 0` (positive) |
| `percentage` | Parses as numeric AND `0 ≤ v ≤ 100` |
| `date` | ISO 8601 format (`YYYY-MM-DD` or full RFC3339); for keys ending in `_DEADLINE` MUST be in future |
| `url` | Starts with `https://`; passes `url.Parse` without error |
| `json` | Parses as valid JSON (object OR array) |
| `boolean` | `'true'` or `'false'` literal |

Special key-specific rules (enforced in addition to data_type):

| Key | Extra rule |
|---|---|
| `PROMO_DEADLINE` | `data_type=date` AND must be `> NOW()` on write |
| `PROMO_DISCOUNT_PCT` | `data_type=percentage` AND `0 < v ≤ 50` (no full-discount foot-guns) |
| `LINK_*` | `data_type=url` AND HEAD request returns 2xx within 5s (advisory — log warning, don't block) |
| `CLAUDE_*_PROMPT` | `data_type=string` AND length 100–50000 |
| `KNOWN_COMPETITOR_KEYWORDS` | `data_type=json` AND must parse as `string[]` |
| `INDONESIAN_HOLIDAYS_2026` | `data_type=json` AND must parse as `string[]` of `YYYY-MM-DD` |

## API Endpoints

All scoped to `/{workspace_id}` and require `auth_session` cookie.

### GET `/system-config`

Returns all entries for the active workspace.

```
Headers:
  Cache-Control: max-age=60, must-revalidate    — FE re-fetches every 60s

Response 200:
{
  "data": [
    { "key": "HARGA_START", "value": "1500000", "data_type": "number", "description": "...", "is_secret": false, "updated_at": "...", "updated_by": "alice@dealls.com" },
    { "key": "PROMO_DEADLINE", "value": "2026-05-31", "data_type": "date", ..., "is_secret": false },
    { "key": "CLAUDE_EXTRACTION_PROMPT", "value": "***MASKED***", "data_type": "string", ..., "is_secret": true }
  ],
  "cache_ttl": 60
}
```

`is_secret=true` entries return `value="***MASKED***"`. To read actual value, Admin/Lead must call `GET /system-config/{key}/reveal` (audit-logged).

### PUT `/system-config/{key}`

Update. Admin/Lead only. Body `{ "value": "1750000" }`. Returns the updated entry.

- 400: `{ "error": "Value must be > 0 for data_type=number", "field": "value" }`
- 403: `{ "error": "Requires Admin or Lead role" }`
- 404: `{ "error": "Key not found. Use POST /system-config to create." }`

### POST `/system-config`

Create new key. Admin/Lead only. Body `{ key, value, data_type, description }`. 201 on success; 409 if key exists.

### DELETE `/system-config/{key}`

Admin only. Soft-delete (sets `deleted_at`); hard-delete via migration only.

### GET `/system-config/{key}/reveal`

For `is_secret=true` keys, returns plaintext value. Admin/Lead only. Audit-logged as `action='system_config.secret_revealed'`.

## Audit Trail

Every write logged immutably.

```sql
CREATE TABLE system_config_audit (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  key             VARCHAR(100) NOT NULL,
  operation       VARCHAR(20) NOT NULL,            -- 'create' | 'update' | 'delete' | 'secret_revealed'
  old_value       TEXT,                             -- null on create
  new_value       TEXT,                             -- null on delete
  actor_email     VARCHAR(255) NOT NULL,
  actor_role      VARCHAR(20) NOT NULL,
  performed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ip_address      INET
);

CREATE INDEX idx_sca_workspace_key ON system_config_audit(workspace_id, key, performed_at DESC);
CREATE INDEX idx_sca_actor ON system_config_audit(actor_email, performed_at DESC);
```

For `is_secret=true` keys, `old_value` and `new_value` store SHA-256 hash, not plaintext.

## Initial Seed Migration

Run once per new workspace. Insert 9 default keys with same defaults the FE uses (`lib/system-config-store.ts` `DEFAULT_CONFIG`).

```sql
INSERT INTO system_config (workspace_id, key, value, data_type, description, is_secret, updated_by) VALUES
  ($1, 'HARGA_START',                '1500000',                                  'number',     'Starting price (IDR) used in BD pricing templates',                       false, 'system_seed'),
  ($1, 'PROMO_DEADLINE',             '2026-12-31',                               'date',       'End date of current promo campaign',                                      false, 'system_seed'),
  ($1, 'PROMO_DISCOUNT_PCT',         '15',                                       'percentage', 'Promo discount percentage applied during current campaign',               false, 'system_seed'),
  ($1, 'BENEFIT_REFERRAL',           'Voucher Rp 500.000 untuk referral pertama','string',     'Benefit text shown in referral templates',                                false, 'system_seed'),
  ($1, 'CAMPAIGN_THIS_MONTH',        'Cashback 10% all annual plans',            'string',     'Active campaign tagline for this month',                                  false, 'system_seed'),
  ($1, 'LINK_DASHBOARD_DEMO',        'https://demo.kantorku.id',                 'url',        'Public demo dashboard link',                                              false, 'system_seed'),
  ($1, 'LINK_NPS_SURVEY',            'https://kantorku.id/nps',                  'url',        'NPS survey link sent post-onboarding',                                    false, 'system_seed'),
  ($1, 'LINK_WIKI',                  'https://wiki.kantorku.id',                 'url',        'Internal wiki / docs link',                                               false, 'system_seed'),
  ($1, 'KNOWN_COMPETITOR_KEYWORDS',  '["Talenta","Mekari","Gadjian","BambooHR"]','json',       'List of competitor product names — used for objection trigger matching',  false, 'system_seed');
```

Additional keys seeded by other features (NOT included in the 9-key FE seed but inserted by their own migrations):
- `CLAUDE_EXTRACTION_PROMPT`, `CLAUDE_BANTS_SCORING_PROMPT` (06-claude-extraction-pipeline) — `is_secret=true`
- `INDONESIAN_HOLIDAYS_2026` (10-working-day-timezone)
- `FIREFLIES_API_KEY`, `FIREFLIES_WEBHOOK_SECRET` — env vars, NOT in `system_config`

## Backend MUST

- Enforce composite PK `(workspace_id, key)` — no cross-workspace leakage
- Validate `value` against `data_type` AND key-specific rules on every write
- Mask `is_secret=true` entries in `GET /system-config` listing
- Write to `system_config_audit` on every create/update/delete/reveal
- Set `Cache-Control: max-age=60` on GET response so FE SWR aligns with backend cache
- Restrict write endpoints to `actor.role IN ('Admin','Lead')`

## Backend MAY

- Cache the per-workspace config in Redis with 60s TTL keyed by `system_config:{workspace_id}`
- Push a Server-Sent Event `system_config.updated` so FE invalidates immediately on remote edits (instead of polling)
- Provide a `POST /system-config/import` admin endpoint accepting JSON dump from another workspace (for cloning config)
