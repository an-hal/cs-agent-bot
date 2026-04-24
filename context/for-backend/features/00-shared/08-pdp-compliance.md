# PDP Compliance (UU 27/2022)

## Overview

Indonesian Personal Data Protection Law (Undang-Undang 27 Tahun 2022 tentang Pelindungan Data Pribadi — "UU PDP") obligates us to (a) record explicit consent for every prospect, (b) honour right-to-be-forgotten requests, (c) enforce data retention tiers with automatic anonymisation, and (d) maintain a 7-year immutable audit log exportable to regulators on demand. Background: `context/claude/20-compliance-and-rollout.md`.

## Consent Model

Every row in `prospects` (and `clients`, `leads`) MUST carry consent metadata. Backend rejects INSERTs without these fields.

```sql
ALTER TABLE prospects
  ADD COLUMN consent_at        TIMESTAMPTZ NOT NULL,
  ADD COLUMN consent_source    consent_source_enum NOT NULL,
  ADD COLUMN consent_version   INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN consent_evidence  JSONB;                      -- IP, user-agent, form_id, etc.

CREATE TYPE consent_source_enum AS ENUM (
  'form_submission',
  'apollo_imported',
  'manual_entry',
  'referral'
);
```

| `consent_source` | Required `consent_evidence` keys |
|---|---|
| `form_submission` | `form_id`, `ip`, `user_agent`, `submitted_at` |
| `apollo_imported` | `apollo_list_id`, `imported_by_email`, `apollo_consent_basis` |
| `manual_entry` | `entered_by_email`, `consent_proof_text` (free-text justification — min 20 char) |
| `referral` | `referrer_id`, `referrer_consent_at` |

### Consent Versioning

`consent_versions` tracks the legal text the prospect actually agreed to. Workspace Admin can publish a new version; existing prospects retain their original `consent_version`.

```sql
CREATE TABLE consent_versions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  version         INTEGER NOT NULL,
  legal_text_id   VARCHAR(50) NOT NULL,         -- e.g. 'kantorku-privacy-2026-04'
  legal_text_html TEXT NOT NULL,
  effective_from  TIMESTAMPTZ NOT NULL,
  published_by    VARCHAR(255) NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(workspace_id, version)
);
```

## Right-to-Be-Forgotten

### POST `/clients/{id}/forget`

Admin/Lead role only. Soft-deletes the client immediately, enqueues a 30-day permanent-purge job, writes audit log. Cannot be undone after 30 days.

```
Headers: Authorization: Bearer {token}

Request:
{
  "reason": "Client requested deletion via email 2026-04-20",
  "request_evidence_url": "https://storage.example.com/dsr/req-001.pdf"  // optional
}

Response 200:
{
  "client_id": "uuid-xxx",
  "soft_deleted_at": "2026-04-22T10:00:00Z",
  "purge_scheduled_at": "2026-05-22T10:00:00Z",
  "audit_log_id": "uuid-yyy"
}

Response 403: { "error": "Requires Admin or Lead role" }
Response 404: { "error": "Client not found" }
Response 409: { "error": "Client already in purge queue" }
```

Backend MUST in one transaction:
1. `UPDATE clients SET deleted_at=NOW(), deleted_by=actor_email, deletion_reason=? WHERE id=?`
2. `INSERT INTO pii_purge_queue (...)`
3. `INSERT INTO action_logs (action='client.forget', ...)` with full payload (retained 7 years)
4. Cancel all scheduled bot sends for this client (`UPDATE message_queue SET status='cancelled'`)

## 30-Day Purge Job

Cron runs daily at 02:00 WIB. Anonymises PII; retains foreign-key shape so historical analytics don't break.

```sql
-- Job query
SELECT id, workspace_id, client_id FROM pii_purge_queue
WHERE purge_scheduled_at <= NOW() AND purged_at IS NULL
LIMIT 50;
```

For each row, anonymise PII columns:

```sql
UPDATE clients SET
  PIC_Email     = encode(digest(PIC_Email, 'sha256'), 'hex'),
  PIC_WA        = encode(digest(PIC_WA, 'sha256'), 'hex'),
  DM_WA         = encode(digest(DM_WA, 'sha256'), 'hex'),
  dm_name       = 'REDACTED',
  Company_Name  = 'REDACTED-' || id::text,
  pii_purged_at = NOW()
WHERE id = $client_id;

UPDATE pii_purge_queue SET purged_at = NOW() WHERE id = $queue_id;
```

`action_logs` rows referencing the purged client retain the SHA-256 hash so regulators can verify a record existed without exposing PII. Audit logs themselves are NEVER anonymised — they are retained 7 years per UU PDP Art. 65.

## Data Retention Tiers

Cron job `archival_sweep` runs nightly, evaluates each row.

| Tier | Condition | Action |
|---|---|---|
| **Active** | `last_interaction_at` within 365 days | None |
| **Inactive** | `last_interaction_at` between 365d and 5y ago | Set `archived_at=NOW()`; move to `clients_archive` partition; remove from default queries (require `?include_archived=true`) |
| **Archived** | `archived_at` > 5 years ago | Auto-enqueue to `pii_purge_queue` (purge in 30 days unless Lead opts to retain) |
| **Deleted (soft)** | `deleted_at` set, `purged_at` null | After 30 days → hard-anonymise per purge job above |

Lead can opt-out a record from auto-archival via `PUT /clients/{id}/retention { "indefinite": true, "reason": "..." }`. Logged.

## Audit Log Requirements

```sql
CREATE TABLE action_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL,
  action          VARCHAR(100) NOT NULL,         -- e.g. 'client.forget', 'extraction.feedback', 'pdp.export'
  entity_type     VARCHAR(50) NOT NULL,
  entity_id       UUID,
  performed_by    VARCHAR(255) NOT NULL,
  performed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ip_address      INET,
  user_agent      TEXT,
  metadata        JSONB NOT NULL,
  -- Tamper-evidence: hash chain
  prev_log_hash   VARCHAR(64) NOT NULL,          -- SHA-256 of previous row
  this_log_hash   VARCHAR(64) NOT NULL           -- SHA-256(prev_log_hash || row_data)
);

CREATE INDEX idx_al_workspace_time ON action_logs(workspace_id, performed_at DESC);
CREATE INDEX idx_al_entity ON action_logs(entity_type, entity_id);
CREATE INDEX idx_al_action ON action_logs(action, performed_at DESC);
```

Backend MUST:
- Compute `this_log_hash = SHA256(prev_log_hash || canonical_json(row))` on INSERT
- Reject UPDATEs to `action_logs` at the DB level (REVOKE UPDATE for app role)
- Retain rows 7 years before allowing DELETE (cron deletes only `performed_at < NOW() - INTERVAL '7 years'`)

## Encryption at Rest

PII columns MUST be encrypted (AES-256). Implementation: PostgreSQL Transparent Data Encryption (TDE) at the tablespace level; OR application-level encryption via `pgcrypto` for these columns:

```
clients.PIC_Email
clients.PIC_WA
clients.DM_WA
clients.dm_name
clients.dm_phone
prospects.PIC_Email
prospects.PIC_WA
prospects.dm_name
prospects.dm_phone
fireflies_transcripts.transcript_text   (contains call participant names)
fireflies_transcripts.participants
```

Encryption key rotation: 12-month rotation cycle, key versions tracked in `encryption_key_versions`. Old keys retained for decrypting historical data.

## DB Schema — Purge Queue

```sql
CREATE TABLE pii_purge_queue (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id        UUID NOT NULL REFERENCES workspaces(id),
  client_id           UUID NOT NULL,
  enqueued_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  purge_scheduled_at  TIMESTAMPTZ NOT NULL,            -- enqueued_at + 30 days
  purged_at           TIMESTAMPTZ,
  enqueue_reason      VARCHAR(50) NOT NULL,            -- 'forget_request' | 'auto_archive_5y'
  enqueued_by         VARCHAR(255) NOT NULL
);

CREATE INDEX idx_ppq_scheduled ON pii_purge_queue(purge_scheduled_at) WHERE purged_at IS NULL;
```

## API Endpoints

### POST `/clients/{id}/forget` — see above

### GET `/clients/{id}/consent-history`

Returns full consent + audit history. Used by FE Settings → PDP tab AND regulator export.

```
Response 200:
{
  "client_id": "uuid-xxx",
  "consent": { "consent_at": "...", "consent_source": "form_submission", "consent_version": 2, "consent_evidence": {...} },
  "audit_trail": [
    { "action": "client.created", "performed_by": "alice@dealls.com", "performed_at": "..." },
    { "action": "client.update.email", "performed_by": "alice@dealls.com", "performed_at": "...", "metadata": {...} }
  ]
}
```

### POST `/admin/pdp-export`

Lead/Admin-only. Generates regulator-ready ZIP (PDF + JSONL) of all data + audit logs for a client. Async — returns `job_id`; FE polls for download URL. Body: `{ client_id, format: 'pdf+jsonl', purpose }`. Returns 202 with `{ job_id, status: 'queued' }`.

## Backend MUST

- Reject any prospect/client INSERT lacking `consent_at` + `consent_source`
- Refuse `POST /clients/{id}/forget` from non-Admin/non-Lead actors
- Anonymise PII per purge job; never DELETE the client row (FK shape preserved)
- Maintain hash-chained `action_logs`; expose verification endpoint `GET /admin/audit-log/verify`
- Retain audit logs 7 years minimum
- Encrypt the PII column list above at rest (AES-256)

## Backend MAY

- Provide a "consent re-confirmation" cron that emails clients on `consent_version` changes (opt-in per workspace via `system_config[CONSENT_REVERIFY_ON_VERSION_BUMP]`)
- Cache `GET /clients/{id}/consent-history` for 5 min (immutable history, safe to cache)
- Auto-redact PII in BD-facing UIs for clients with `archived_at` set, even before purge
