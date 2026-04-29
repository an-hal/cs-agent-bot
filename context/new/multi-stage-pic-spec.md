# Multi-Stage PIC Spec — Per-Lifecycle Internal & Client-Side Contacts

> Status: design + initial implementation
> Author: BE refactor (sesi 2026-04-29)
> Related: `crm_database_spec.md` Table 1 (clients), Table 3 (bot_state)

## Problem

Schema sebelum spec ini:
- `clients.pic_*` = **single** PIC client-side (1 row)
- `clients.owner_*` + `cms.owner_telegram_id` = **single** internal owner
- `clients.stage` = single value

Real-world flow di team Bumi:
```
PT Dua Tiga
├─ Stage LEAD  (SDR phase)  → Internal: Icha    + Client: Baba  (Owner)
├─ Stage PROSPECT (BD phase) → Internal: Shafira + Client: Fina  (HR)
└─ Stage CLIENT (AE phase)  → Internal: Anggi   + Client: Caca  (Finance)
```

Saat transisi `LEAD → PROSPECT`, schema lama paksa **overwrite** `pic_*` + `owner_*` → history Icha–Baba hilang. Tidak bisa audit "siapa yang follow up Baba zaman SDR?".

## Solution: `client_contacts` table

Normalisasi kontak ke tabel terpisah, scoped by `(master_id, stage, kind)`.

### Schema

```sql
CREATE TABLE client_contacts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    master_id    UUID NOT NULL REFERENCES clients(master_id) ON DELETE CASCADE,

    stage        VARCHAR(20) NOT NULL,  -- LEAD / PROSPECT / CLIENT / DORMANT
    kind         VARCHAR(20) NOT NULL,  -- internal | client_side
    role         VARCHAR(100),          -- SDR / BD / AE / Owner / HR / Finance / …

    name         VARCHAR(255) NOT NULL,
    wa           VARCHAR(20),
    email        VARCHAR(100),
    telegram_id  VARCHAR(100),

    is_primary   BOOLEAN NOT NULL DEFAULT TRUE,
    notes        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 1 primary per (client, stage, kind); backup contacts use is_primary=FALSE
CREATE UNIQUE INDEX uq_client_contacts_primary
    ON client_contacts (master_id, stage, kind)
    WHERE is_primary = TRUE;
```

### Why Option B (table) over Option A (stage-keyed columns) or C (JSONB)?

| Concern | A (columns) | B (table) ✓ | C (JSONB) |
|---|---|---|---|
| Add new stage (e.g. `RETENTION`) | ALTER TABLE × N cols | INSERT row | edit JSON shape |
| Query "all PICs di stage X" | per-stage SELECT | indexed lookup | sequential scan |
| Backup contacts | hard-coded `_backup` cols | `is_primary=false` row | array in JSON |
| Audit history | overwritten | preserved | overwritten |
| FK integrity | n/a | full | none |

Single deal-breaker for **A**: 12+ extra columns on clients yang sudah heavy. Single deal-breaker for **C**: tidak bisa di-index efisien di Postgres for our query patterns.

## Use case mapping

### Use case 1 — Internal team rotation per stage

| Workflow phase | Lifecycle stage | Internal PIC | Role | WA |
|---|---|---|---|---|
| SDR | LEAD | Icha | SDR | 6282225942841 |
| BD | PROSPECT | Shafira | BD | 6287812312345 |
| AE | CLIENT | Anggi | AE | 6285247571288 |

Stored as 3 rows in `client_contacts` with `kind='internal'`, different `stage`/`name`/`role`.

### Use case 2 — Client-side contact rotation per stage

| Workflow phase | Lifecycle stage | Client PIC | Role |
|---|---|---|---|
| SDR | LEAD | Baba | Owner |
| BD | PROSPECT | Fina | HR |
| AE | CLIENT | Caca | Finance |

Stored as 3 rows with `kind='client_side'`.

Untuk PT Dua Tiga, total 6 rows di `client_contacts` (3 internal × 3 client_side).

## Stage transition flow

Saat client transisi `LEAD → PROSPECT`:

```
1. Stage transition handler runs
2. Check: ada row `client_contacts WHERE master_id=X AND stage=PROSPECT AND kind=internal/client_side`?
   YES → reuse (e.g., klien sudah pernah di PROSPECT)
   NO  → AE/operator HARUS isi via UI sebelum stage transit (atau auto-clone dari LEAD jika opsi `--inherit-pic`)
3. clients.stage = PROSPECT
4. Bot reads master_data view → view JOINs client_contacts WHERE stage = clients.stage → returns Shafira/Fina
```

History stays: row LEAD/Icha + LEAD/Baba tidak dihapus.

## Master_data view backward compatibility

View tetap expose `pic_name`/`pic_wa`/`owner_name`/`owner_wa`/`owner_telegram_id` — sourcing dari `client_contacts` at current stage:

```sql
CREATE VIEW master_data AS
SELECT
    c.master_id AS id,
    ...,
    -- Current-stage client-side primary contact
    cc_pic.name        AS pic_name,
    cc_pic.role        AS pic_role,
    cc_pic.wa          AS pic_wa,
    cc_pic.email       AS pic_email,
    -- Current-stage internal primary contact
    cc_own.name        AS owner_name,
    cc_own.wa          AS owner_wa,
    cc_own.telegram_id AS owner_telegram_id,
    ...
FROM clients c
LEFT JOIN LATERAL (
    SELECT * FROM client_contacts
    WHERE master_id = c.master_id AND stage = c.stage AND kind = 'client_side' AND is_primary
    LIMIT 1
) cc_pic ON true
LEFT JOIN LATERAL (
    SELECT * FROM client_contacts
    WHERE master_id = c.master_id AND stage = c.stage AND kind = 'internal' AND is_primary
    LIMIT 1
) cc_own ON true;
```

Implications:
- FE existing tetap baca `pic_name`/`owner_name` lewat view — **tidak perlu diubah**
- Bot routing `cms.owner_telegram_id` deprecated → bot baca dari view
- Old `clients.pic_*` + `clients.owner_*` columns → **deprecated** tapi tetap ada untuk backfill/legacy

## Migration plan

1. Migration 001 (this round): create `client_contacts` table + backfill from current `clients.pic_*`/`owner_*` (1 row per kind, stage=clients.stage at backfill time) + rebuild view
2. Migration 002 (future): drop `clients.pic_*` + `clients.owner_*` after FE/bot fully sourcing from view
3. Migration 003 (future): drop `cms.owner_telegram_id` after bot routing fully sources from view

## API surface (new endpoints)

### `GET /api/master-data/clients/{id}/contacts`

List semua kontak untuk client tersebut. Optional filter `stage`, `kind`, `only_primary`.

```jsonc
// Response
{
  "data": [
    {
      "id": "uuid",
      "stage": "LEAD",
      "kind": "internal",
      "role": "SDR",
      "name": "Icha",
      "wa": "6282225942841",
      "email": null,
      "telegram_id": "@ichaaa",
      "is_primary": true,
      "notes": null,
      "created_at": "2026-04-29T...",
      "updated_at": "..."
    }
    // ... 5 more for PT Dua Tiga
  ]
}
```

### `POST /api/master-data/clients/{id}/contacts`

```jsonc
{
  "stage": "PROSPECT",
  "kind": "internal",
  "role": "BD",
  "name": "Shafira",
  "wa": "6287812312345",
  "email": "shafira@dealls.com",
  "telegram_id": "@firaaa",
  "is_primary": true
}
// → 201 returns the new contact
// Auto-demotes any existing primary in same (stage, kind) to is_primary=false
```

### `PATCH /api/master-data/clients/{id}/contacts/{contact_id}`

Edit one contact. Body is partial — only fields provided are touched.

### `DELETE /api/master-data/clients/{id}/contacts/{contact_id}`

Hard delete. (Soft-delete bisa ditambah nanti pakai `deleted_at`.)

### `POST /api/master-data/clients/{id}/transition` (existing)

No body change. After stage transit, view auto-resolves to new stage's contacts.

## Bot integration

Saat stage transition (`LEAD → PROSPECT`):
- Bot **wajib** verify ada primary contact (internal + client_side) di stage baru sebelum transit. Kalau tidak ada → throw `ESC-XXX: missing PIC for new stage`, AE diberitahu.
- Setelah transit sukses, bot reads `master_data` view → sees Shafira/Fina, sends WA ke Fina, escalation alert ke Shafira's telegram_id.

## FE implications

### Dashboard "Add/Edit Client" drawer — tab baru "Contacts per Stage"

```
┌─────────────────────────────────────────────────────┐
│ Stage: LEAD                          [+ Add contact] │
├─────────────────────────────────────────────────────┤
│ Internal | Icha     | SDR  | 62822... | @ichaaa     │
│ Client   | Baba     | Owner| 62...    |             │
├─────────────────────────────────────────────────────┤
│ Stage: PROSPECT                       [+ Add contact]│
├─────────────────────────────────────────────────────┤
│ Internal | Shafira  | BD   | 6287...  | @firaaa     │
│ Client   | Fina     | HR   | 62...    |             │
├─────────────────────────────────────────────────────┤
│ Stage: CLIENT                         [+ Add contact]│
├─────────────────────────────────────────────────────┤
│ Internal | Anggi    | AE   | 62852... | @anggiii    │
│ Client   | Caca     | Fin. | 62...    |             │
└─────────────────────────────────────────────────────┘
```

### Stage transition button

Sebelum klik "Move to PROSPECT", FE check:
- Ada contact `(stage=PROSPECT, kind=internal, is_primary=true)`? Tidak → modal "Assign internal PIC for PROSPECT first"
- Ada contact `(stage=PROSPECT, kind=client_side, is_primary=true)`? Tidak → modal "Assign client-side PIC for PROSPECT first"

## Backward compatibility

| Aspek | Status |
|---|---|
| `master_data` view tetap return `pic_name`/`owner_name` | ✓ via LATERAL JOIN |
| FE existing `GET /clients/{id}` tidak perlu diubah | ✓ |
| Existing `POST /clients` create flow | ✓ — Create method auto-creates 1 contact row at clients.stage |
| Existing `PUT /clients/{id}` update | ✓ — Patch routes pic_*/owner_* changes ke contact at current stage |
| Bot's `owner_telegram_id` lookup | ✓ — view exposes it from current stage's internal contact |
| Import wizard | ✓ — import path creates contact at clients.stage |

## Out of scope (future)

1. Migrasi semua `clients.pic_*` + `clients.owner_*` data permanent ke `client_contacts` lalu drop columns dari `clients`
2. Soft-delete contacts dengan `deleted_at` + retention policy
3. Bulk import stage-keyed kontak via wizard mapping (Phase D)
4. Activity log per contact (last contacted at, last response date)
5. Multi-PIC per kind: 1 client di stage AE bisa punya 2 internal PICs (Anggi senior + asisten) — sudah didukung via `is_primary=false` rows; FE pickup pending

## Why import template stays single-PIC

The xlsx template (`GET /clients/template`) intentionally exposes only ONE
`PIC *` and ONE `Owner *` group. On import, the BE auto-seeds two
`client_contacts` rows (one `internal` + one `client_side`) at the new
client's `stage`. Adding rotated PICs for other stages is done after import,
either via:
- Dashboard drawer "Contacts per Stage" tab → POST `/clients/{id}/contacts`
- API directly

The decision NOT to widen the template to per-stage columns
(`LEAD PIC Name`, `PROSPECT PIC Name`, …):

1. **Stage values vary per workspace.** KantorKu uses LEAD/PROSPECT/CLIENT.
   A future workspace might use SDR/BD/AE/CHURNED; another might use
   ONBOARDING/ACTIVE/RENEWAL/CHURNED. Hard-coding stage names in the
   template forces every workspace into the same lifecycle vocabulary.
2. **Most workspaces don't rotate PIC per stage.** The single-AE-owner
   model (1 internal + 1 client-side, stable across lifecycle) covers the
   90% case. Rotation is a power-user feature.
3. **Schema spec already calls this out.** "Bulk import stage-keyed kontak
   via wizard mapping" lives under "Out of scope" item 3 — explicitly
   deferred to Phase D.

If/when a workspace genuinely needs bulk multi-stage import, the path
forward is **Option C** (multi-row per client with `Contact Stage` +
`Contact Kind` columns) — not widening to fixed per-stage columns. That
preserves per-workspace stage flexibility while supporting the use case.

For now: **template stays simple, contacts API handles the rest**.

## End-to-end verification

Tested 2026-04-29 dengan PT Dua Tiga (master_id `3871393a-…`). Setup:
- LEAD: Icha (internal/AE) + Baba (client_side/Owner)
- PROSPECT: Shafira (internal/BD) + Fina (client_side/HR)
- CLIENT: Anggi (internal/AE) + Caca (client_side/Finance)

Hasil GET `/clients/{id}` setelah PUT `stage`:

| stage | pic_name | pic_role | owner_name | owner_telegram_id |
|---|---|---|---|---|
| LEAD | Baba | Owner | Icha | @ichaaa |
| PROSPECT | Fina | HR | Shafira | @firaaa |
| CLIENT | Caca | Finance | Anggi | @anggiii |
| LEAD (back) | Baba | Owner | Icha | @ichaaa |

History preserved across transitions ✓
