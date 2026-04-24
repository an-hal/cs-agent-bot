# Escalation System

## Overview
Escalation terjadi ketika record memenuhi kondisi kritis berdasarkan priority tier (P0/P1/P2).
Klasifikasi tier mengikuti business spec di `context/for-business/02-BUSINESS-PROCESS-MAP.md`
("AE Escalation Triggers") dan `context/claude/09-data-models.md` §"Escalation Rules & Priority
Tiers" — kedua dokumen tersebut adalah **source of truth**; file ini adalah implementasi backend-nya.

Priority enum: **P0 (HIGH)** · **P1 (MEDIUM)** · **P2 (LOW)**. Alert dikirim via **Telegram** ke
`master_data.owner_telegram_id` (primary) dan ke workspace-level `default_chat_id` (fallback/cc).

## Trigger Conditions (source of truth)

Kondisi di bawah ini adalah kontrak eksak — field + threshold harus match persis saat workflow
engine mengevaluasi kandidat escalation. Jangan generalisasi.

| Priority | Trigger Condition (exact field + threshold) | Expected Action | SLA |
|----------|---------------------------------------------|-----------------|-----|
| **P0 HIGH** | `NPS_Score ≤ 5` | Alert AE Lead via Telegram, dashboard banner, manual intervention required, bot pauses outreach | Immediate |
| **P0 HIGH** | `Payment_Status = 'Terlambat' AND Days_Overdue ≥ 30` | Same as above | Immediate |
| **P0 HIGH** | `Usage_Score < 20 AND days_since_activation > 30` | Same as above (adoption failure) | Immediate |
| **P1 MEDIUM** | Client reply classification = `'angry'` OR `'reject'` | Notify Owner via Telegram; AE opens ticket; bot stops outreach (manual takeover) | 48h response |
| **P1 MEDIUM** | Contract expired without renewal commitment | Same as above (churn prevention) | 48h response |
| **P2 LOW** | `cross_sell_rejected = TRUE` 2 consecutive times | Log only, schedule 90d re-engagement window | No SLA |
| **P2 LOW** | 3 consecutive missed check-ins | Same as above | No SLA |

> Catatan: trigger legacy seperti `bd_score ≥ 4 + no reply` (BD pipeline) dan CS SLA breach masih
> dipakai oleh workflow engine, tetapi tidak mapping ke priority tier AE di atas. Lihat
> `context/for-backend/features/06-workflow-engine/` untuk trigger BD/SDR/CS yang bukan AE-tier.

## Status Machine

```
Open ──(AE Lead/Owner acknowledges)──▶ In Progress ──(notes + action taken)──▶ Resolved
```

| From | To | Allowed Actor | Side effects |
|------|----|--------------:|--------------|
| `Open` | `In Progress` | AE Lead / Owner | `resolved_at=null`; dashboard banner tetap tampil |
| `In Progress` | `Resolved` | AE Lead / Owner | set `resolved_at`, `resolved_by`, `notes` wajib; banner hilang |
| `Open` | `Resolved` | AE Lead / Owner (fast-path) | Boleh skip `In Progress` jika issue selesai instant; `notes` tetap wajib |
| `Resolved` | * | — | Immutable. Buat escalation baru jika isu recur. |

## Notification Contract

Saat escalation **dibuat** (INSERT row dengan `status='Open'`), backend wajib fire notifikasi
Telegram ke dua tujuan secara paralel:

1. **Primary** — `master_data.owner_telegram_id` milik company terkait
2. **Fallback / cc** — workspace-level `integrations.telegram.default_chat_id`

### Payload (JSON dikirim ke Telegram bot + disimpan di `notifications` table)

```json
{
  "priority": "P0",
  "priority_label": "HIGH",
  "trigger_condition": "NPS_Score <= 5",
  "company_name": "PT Example Mandiri",
  "company_id": "CMP001",
  "days_overdue": null,
  "nps_score": 4,
  "usage_score": null,
  "dashboard_link": "https://bumi.dealls.com/dashboard/dealls/bd?company=CMP001&tab=escalations"
}
```

Optional fields (`days_overdue`, `nps_score`, `usage_score`) diisi sesuai trigger — hanya field
relevan yang non-null. Frontend consume notifikasi via:
- `GET /api/action-log/recent` (feed utama)
- Notifications bell dropdown (`components/common/Header.tsx` — memetakan payload ke `NotifItem`)

## Database

### Table: `escalations`
```sql
CREATE TABLE escalations (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  master_data_id  UUID NOT NULL REFERENCES master_data(id),

  trigger_id         VARCHAR(100) NOT NULL,  -- e.g. 'ESC-AE-NPS5', 'ESC-AE-OVERDUE30'
  priority           VARCHAR(4) NOT NULL,    -- 'P0' | 'P1' | 'P2'
  trigger_condition  TEXT NOT NULL,          -- human-readable: "NPS_Score <= 5"

  reason          TEXT NOT NULL,           -- detail kontekstual: "NPS=4 on 2026-04-18"
  assigned_to     UUID REFERENCES users(id),  -- team lead/manager (who_notified)

  status          VARCHAR(20) DEFAULT 'Open',  -- 'Open' | 'In Progress' | 'Resolved'
  resolved_at     TIMESTAMPTZ,
  resolved_by     UUID REFERENCES users(id),
  resolution_note TEXT,

  -- Notification
  notified_via    VARCHAR(20),            -- telegram, email
  notified_at     TIMESTAMPTZ,

  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_esc_workspace ON escalations(workspace_id);
CREATE INDEX idx_esc_status ON escalations(workspace_id, status);
CREATE INDEX idx_esc_priority ON escalations(workspace_id, priority);
CREATE INDEX idx_esc_master_data ON escalations(master_data_id);
```

### Go struct

```go
type EscalationPriority string
const (
    EscalationP0 EscalationPriority = "P0"  // HIGH
    EscalationP1 EscalationPriority = "P1"  // MEDIUM
    EscalationP2 EscalationPriority = "P2"  // LOW
)

type Escalation struct {
    ID                uuid.UUID          `json:"id"`
    WorkspaceID       uuid.UUID          `json:"workspace_id"`
    CompanyID         string             `json:"company_id"`
    Priority          EscalationPriority `json:"priority"`
    PriorityLabel     string             `json:"priority_label"`  // "HIGH" | "MEDIUM" | "LOW" derived from priority
    TriggerCondition  string             `json:"trigger_condition"`
    WhoNotified       string             `json:"who_notified"`
    Status            string             `json:"status"`          // 'Open' | 'In Progress' | 'Resolved'
    CreatedAt         time.Time          `json:"created_at"`
    ResolvedAt        *time.Time         `json:"resolved_at,omitempty"`
    ResolvedBy        *string            `json:"resolved_by,omitempty"`
    Notes             *string            `json:"notes,omitempty"`
}
```

`PriorityLabel` derived mapping:
- `P0` → `"HIGH"`
- `P1` → `"MEDIUM"`
- `P2` → `"LOW"`

## API Endpoints

### GET `/escalations`
```
Query: ?status=Open&priority=P0&limit=20

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "company_id": "CMP001",
      "company_name": "PT Example",
      "trigger_id": "ESC-AE-NPS5",
      "priority": "P0",
      "priority_label": "HIGH",
      "trigger_condition": "NPS_Score <= 5",
      "who_notified": "Budi (AE Lead)",
      "status": "Open",
      "created_at": "2026-04-12T10:00:00Z"
    }
  ]
}
```

### GET `/clients/{company_id}/escalations`
Escalation history untuk satu company.

### PUT `/escalations/{id}/resolve`
```json
{
  "status": "Resolved",
  "notes": "Sudah di-call manual, prospect minta snooze 2 minggu"
}
```

### POST `/escalations` (internal — dari workflow engine)
Created otomatis oleh cron saat salah satu trigger condition di tabel atas terpenuhi. Tidak
di-trigger manual dari UI.

---

## Activity-Log Filtering by Division (#33)

The activity log feed (`action_logs`) is consumed by both per-division dashboards
(BD/AE/SDR/CS) and the org-wide GUARD overlay. Backend must support division
filtering and the `manual_send` distinction so FE can separate bot vs human sends.

### GET `/activity-log`

```
Query params:
  ?division=BD|AE|SDR|CS|GUARD     (required for per-division view)
  &action_type=blast|escalation|handoff|manual_action|...   (optional, multi: ?action_type=blast&action_type=escalation)
  &manual_send=true|false          (optional: filter by sender_type)
  &master_data_id=uuid             (single-record drill-down)
  &since=2026-04-22T00:00:00Z
  &offset=0&limit=50

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "division": "BD",
      "action_type": "blast",
      "trigger_id": "BD_D7",
      "manual_send": false,
      "sender_type": "bot",
      "master_data_id": "uuid",
      "company_name": "PT Example",
      "channel": "whatsapp",
      "status": "delivered",
      "created_at": "2026-04-22T09:05:00Z"
    },
    {
      "id": "uuid",
      "division": "AE",
      "action_type": "manual_action",
      "trigger_id": "AE_P4_REN90_OPENER",
      "manual_send": true,
      "sender_type": "human",
      "master_data_id": "uuid",
      "company_name": "PT Other",
      "channel": "wa",
      "status": "sent",
      "created_at": "2026-04-22T10:30:00Z"
    }
  ],
  "meta": { "offset": 0, "limit": 50, "total": 248 }
}
```

### `manual_send` flag

Boolean column on `action_logs` — set `TRUE` when a human SENT a message via the
**Daily Task Validation** flow (per gap #9) or via `PATCH /manual-actions/{id}/mark-sent`
(see `06-workflow-engine/07-manual-flows.md`). Otherwise `FALSE` for bot-fired sends.

```sql
ALTER TABLE action_logs
  ADD COLUMN IF NOT EXISTS manual_send BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill existing rows from sender_type:
UPDATE action_logs SET manual_send = TRUE WHERE sender_type = 'human';
```

`manual_send` is a denormalised mirror of `sender_type='human'` kept for fast
filtering (boolean indexable; `sender_type` is a wider enum used for analytics).

### Division derivation

`division` is derived at INSERT time from `trigger_id` prefix (or rule.role):

| Prefix / source                  | Division |
|----------------------------------|----------|
| `BD_*`, `FP_*`, `ESC-BD-*`       | `BD`     |
| `AE_*`, `Onboarding_*`, `NPS_*`, `Renewal_*`, `Promo_*`, `Warmup_*`, `PAY_*`, `Overdue_*` | `AE`     |
| `SDR_*`, `WA_*`, `EMAIL_*`, `NURTURE_*`, `FEATURE_BROADCAST`, `SNOOZE_RESUME` | `SDR`    |
| `CS_*`                           | `CS`     |
| `ADMIN_*`, audit, approval flows | `GUARD`  |

### Filter combinations & indexes

Filter combos used by FE that must be cheap on Postgres:

| Filter combo                                              | Recommended index                                                              |
|-----------------------------------------------------------|--------------------------------------------------------------------------------|
| division + created_at DESC (per-division feed)            | `(workspace_id, division, created_at DESC)`                                    |
| division + action_type + created_at DESC                  | `(workspace_id, division, action_type, created_at DESC)`                       |
| division + manual_send=TRUE (GUARD human-only view)       | partial index `(workspace_id, division, created_at DESC) WHERE manual_send`    |
| master_data_id + created_at DESC (record drill-down)      | `(master_data_id, created_at DESC)`                                            |
| trigger_id (rule audit)                                   | `(workspace_id, trigger_id, created_at DESC)`                                  |

```sql
CREATE INDEX IF NOT EXISTS idx_alog_division_created
  ON action_logs(workspace_id, division, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_alog_division_action_created
  ON action_logs(workspace_id, division, action_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_alog_manual_send
  ON action_logs(workspace_id, division, created_at DESC)
  WHERE manual_send = TRUE;

CREATE INDEX IF NOT EXISTS idx_alog_master_data_created
  ON action_logs(master_data_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_alog_trigger
  ON action_logs(workspace_id, trigger_id, created_at DESC);
```

> **Note:** the `division` column should be a denormalised VARCHAR(8) populated
> at INSERT time from the prefix table above — do NOT compute it on every SELECT.
> Backfill via one-shot migration using a CASE statement on `trigger_id`.
