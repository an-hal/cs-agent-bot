# Manual-Flow Overlay (GUARD)

## Overview

Not all automation is bot-sent. Twenty flows per role require human composition
at relationship-critical moments (~30–45 min/day per AE/BD). The system surfaces
these as reminders in the dashboard, suggests a draft, and logs the send —
but the actual message is composed + sent by the human via personal WA/email.

Bot = reminder + data layer. Human = voice + judgment. A wrong tone at renewal
turns a retention conversation into a price negotiation; a bot "audit" at D+8
overdue causes permanent churn.

See `context/for-business/GUARD-Manual-Flows-CEO-Brief.md` for business philosophy
and the full 20-flow inventory. Frontend dashboard contract lives in
`context/claude/08-workflow-and-pipeline.md` § "GUARD: Manual-Flow Overlay".

```
Bot detects trigger → inserts manual_action_queue row + Telegram DM to owner
       │
       ▼
Human opens dashboard → sees context_summary + suggested_draft → edits → sends via personal WA
       │
       ▼
Human clicks "Mark Sent" → PATCH /manual-actions/{id}/mark-sent
       │
       ▼
Backend logs action_logs (sender_type='human') + stamps sent_flag on master_data
```

---

## Database Schema

### Table: `manual_action_queue`

```sql
CREATE TABLE manual_action_queue (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id        UUID NOT NULL REFERENCES workspaces(id),
  master_data_id      UUID NOT NULL REFERENCES master_data(id) ON DELETE CASCADE,

  -- Trigger identity (matches automation_rules.trigger_id convention)
  trigger_id          VARCHAR(64) NOT NULL,       -- e.g. 'AE_P4_REN90_OPENER'

  -- Closed enum (17 values) — see "20 Flow Inventory" table
  flow_category       VARCHAR(32) NOT NULL,
                      -- bant_qualification | enterprise_personalisation |
                      -- internal_politics_escalation | final_check_in |
                      -- bd_ae_handoff | onboarding_checkin | warmup_call_invite |
                      -- referral_pitch | renewal_opener | renewal_call_invite |
                      -- renewal_followup | renewal_decision | overdue_empathy |
                      -- overdue_final | admin_pricing_edit | admin_blacklist_edit |
                      -- bd_dm_absent

  role                VARCHAR(8)  NOT NULL,       -- 'sdr' | 'bd' | 'ae' | 'admin'
  assigned_to_user    VARCHAR(255) NOT NULL,      -- owner email (from master_data.Owner_Name)

  suggested_draft     TEXT NOT NULL,              -- pre-rendered template, placeholders filled
  context_summary     JSONB NOT NULL DEFAULT '{}',
                      -- {nps_score, days_since_activation, last_interaction,
                      --  usage_pct, contract_end, payment_status, days_overdue, notes}

  status              VARCHAR(16) NOT NULL DEFAULT 'pending',
                      -- pending | in_progress | sent | skipped | expired
  priority            VARCHAR(4)  NOT NULL DEFAULT 'P2',  -- P0 | P1 | P2

  due_at              TIMESTAMPTZ NOT NULL,
  sent_at             TIMESTAMPTZ,
  sent_channel        VARCHAR(16),                -- 'wa' | 'email' | 'call' | 'meeting'
  actual_message      TEXT,                       -- what the human sent (audit)
  skipped_reason      TEXT,                       -- required when status='skipped'

  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_maq_workspace_status  ON manual_action_queue(workspace_id, status);
CREATE INDEX idx_maq_assigned_pending  ON manual_action_queue(assigned_to_user, status)
                                         WHERE status = 'pending';
CREATE INDEX idx_maq_due_at            ON manual_action_queue(due_at)
                                         WHERE status = 'pending';
CREATE INDEX idx_maq_master_data       ON manual_action_queue(master_data_id);
```

> `flow_category` is a **closed enum** — not free-text. Multiple triggers can
> share a category (e.g. all four `BD_DM_ABSENT_*` triggers → `bd_dm_absent`).
> Use `trigger_id` for exact rule identity, `flow_category` for UX grouping.

---

## 20 Flow Inventory

| # | Category                       | Role  | Trigger ID                    | When it fires                               | Human action                                     |
|---|--------------------------------|-------|-------------------------------|---------------------------------------------|--------------------------------------------------|
| 1 | `bant_qualification`           | sdr   | `SDR_BANT_QUALIFY_REVIEW`     | Bot finishes BANT data collection           | SDR Lead approves/rejects qualified lead         |
| 2 | `enterprise_personalisation`   | sdr   | `SDR_ENTERPRISE_FOLLOWUP`     | `HC_Size >= 200` AND no reply 7 days        | Custom FU referencing company context            |
| 3 | `internal_politics_escalation` | bd    | `BD_D10_DM_ESCALATION`        | BD D10 no progress                          | BD drafts DM escalation via PIC                  |
| 4 | `final_check_in`               | bd    | `BD_D14_FINAL_CHECKIN`        | BD D14 prospect silent                      | Read intent: alive vs. dormant                   |
| 5 | `bd_ae_handoff`                | bd    | `BD_AE_HANDOFF_INTRO`         | `first_payment_confirmed` webhook           | BD personally introduces AE to client            |
| 6 | `onboarding_checkin`           | ae    | `AE_P02_CHECKIN_D14`          | D+14 after activation                       | AE writes client-specific onboarding check-in    |
| 7 | `warmup_call_invite`           | ae    | `AE_P22_CALL_INVITE`          | P2 form not filled after 7 days             | AE invites 15–20 min call, genuine tone          |
| 8 | `referral_pitch`               | ae    | `AE_P33_REFERRAL_ASK`         | `NPS_Score >= 8` (P3 window)                | AE pitches referral program naturally            |
| 9 | `renewal_opener`               | ae    | `AE_P4_REN90_OPENER`          | Contract_End H-90 (working day)             | **MOST CRITICAL** — value-vs-revenue framing     |
|10 | `renewal_call_invite`          | ae    | `AE_P42_REN_CALL`             | Renewal opener unanswered H-83              | AE invites renewal discussion call               |
|11 | `renewal_followup`             | ae    | `AE_P45_REN60`                | H-60 active renewal negotiation             | AE reads negotiation state, adjusts tone         |
|12 | `renewal_decision`             | ae    | `AE_P47_REN45_DECIDE`         | H-45 pre-decision                           | AE pushes decision or offers space               |
|13 | `overdue_empathy`              | ae    | `AE_P6_OVERDUE_D8`            | Invoice D+8 overdue                         | Confrontational — firm + empathetic              |
|14 | `overdue_final`                | ae    | `AE_P6_OVERDUE_D15`           | Invoice D+15 overdue (pre-suspend)          | Final escalation to owner/finance                |
|15 | `admin_pricing_edit`           | admin | `ADMIN_PRICING_EDIT`          | User edits product price                    | 2-person approval (checker-maker)                |
|16 | `admin_blacklist_edit`         | admin | `ADMIN_BLACKLIST_EDIT`        | User adds/removes blacklist entry           | 2-person approval (checker-maker)                |
|17 | `bd_dm_absent`                 | bd    | `BD_DM_ABSENT_FOLLOWUP_A`     | DM missed meeting (PIC exclusion scenario)  | BD reads political nuance                        |
|18 | `bd_dm_absent`                 | bd    | `BD_DM_ABSENT_FOLLOWUP_B`     | DM missed meeting (unavailability scenario) | BD reads political nuance                        |
|19 | `bd_dm_absent`                 | bd    | `BD_DM_ABSENT_FOLLOWUP_C`     | DM missed meeting (briefed-by-PIC scenario) | BD reads political nuance                        |
|20 | `bd_dm_absent`                 | bd    | `BD_DM_ABSENT_FOLLOWUP_D`     | DM post-meeting direct engagement           | BD chooses direct vs. via-PIC approach           |

---

## API Endpoints

All scoped to the active workspace via `X-Workspace-ID` header.

### GET `/manual-actions?status=pending`
List reminders assigned to the current user.
```
Query: ?status=pending|in_progress|sent|skipped|expired  &priority=P0  &role=ae  &offset=0&limit=20
Response 200:
{ "data": [ { id, trigger_id, flow_category, role, priority, status, due_at,
             master_data: { id, company_name, company_id } } ],
  "meta": { offset, limit, total } }
```

### GET `/manual-actions/{id}`
Detail with full `context_summary` + `suggested_draft` + embedded master_data reference.

### PATCH `/manual-actions/{id}/mark-sent`
```
Request: { channel: 'wa'|'email'|'call'|'meeting',
           actual_message: string,
           notes: string }
Response 200: { data: { id, status: 'sent', sent_at, sent_channel } }
```
**Side effects:** inserts `action_logs` row with `sender_type='human'` · stamps
corresponding `sent_flag` on `master_data` (if `trigger_id` maps to one) ·
Telegram confirmation to `assigned_to_user`.

### PATCH `/manual-actions/{id}/skip`
```
Request: { reason: string (min 5 chars) }
Response 200: { data: { id, status: 'skipped' } }
Response 400: { error: "reason must be at least 5 characters" }
```
Flow moves to the next trigger in sequence if applicable (no penalty).

---

## Cron Integration

The cron engine (see `05-cron-engine.md`) dispatches based on trigger kind: if
the `trigger_id` is in the manual-flow set, enqueue a row instead of calling the
bot sender.

```go
func (s *CronService) dispatchAction(ctx context.Context, rule *Rule, record *DataMaster) error {
    if cat, isManual := manualFlowTriggers[rule.TriggerID]; isManual {
        return s.manualService.CreatePending(ctx, ManualActionInput{
            WorkspaceID:    record.WorkspaceID,
            MasterDataID:   record.ID,
            TriggerID:      rule.TriggerID,
            FlowCategory:   cat,
            Role:           rule.Role,
            AssignedToUser: record.OwnerName,          // email resolved from owner field
            Priority:       manualFlowPriority(rule.TriggerID),
            DueAt:          computeDueAt(rule, record),
            ContextSummary: buildContextSummary(record),
            SuggestedDraft: renderTemplate(rule.TemplateID, record),
        })
    }
    return s.botService.Send(ctx, rule, record)
}

// Centralised — the single source of truth for what is manual.
var manualFlowTriggers = map[string]string{
    "SDR_BANT_QUALIFY_REVIEW":   "bant_qualification",
    "SDR_ENTERPRISE_FOLLOWUP":   "enterprise_personalisation",
    "BD_D10_DM_ESCALATION":      "internal_politics_escalation",
    "BD_D14_FINAL_CHECKIN":      "final_check_in",
    "BD_AE_HANDOFF_INTRO":       "bd_ae_handoff",
    "AE_P02_CHECKIN_D14":        "onboarding_checkin",
    "AE_P22_CALL_INVITE":        "warmup_call_invite",
    "AE_P33_REFERRAL_ASK":       "referral_pitch",
    "AE_P4_REN90_OPENER":        "renewal_opener",
    "AE_P42_REN_CALL":           "renewal_call_invite",
    "AE_P45_REN60":              "renewal_followup",
    "AE_P47_REN45_DECIDE":       "renewal_decision",
    "AE_P6_OVERDUE_D8":          "overdue_empathy",
    "AE_P6_OVERDUE_D15":         "overdue_final",
    "ADMIN_PRICING_EDIT":        "admin_pricing_edit",
    "ADMIN_BLACKLIST_EDIT":      "admin_blacklist_edit",
    "BD_DM_ABSENT_FOLLOWUP_A":   "bd_dm_absent",
    "BD_DM_ABSENT_FOLLOWUP_B":   "bd_dm_absent",
    "BD_DM_ABSENT_FOLLOWUP_C":   "bd_dm_absent",
    "BD_DM_ABSENT_FOLLOWUP_D":   "bd_dm_absent",
}
```

---

## Go Service Skeleton

```go
package manualflows

type Status string

const (
    StatusPending    Status = "pending"
    StatusInProgress Status = "in_progress"
    StatusSent       Status = "sent"
    StatusSkipped    Status = "skipped"
    StatusExpired    Status = "expired"
)

type ManualActionInput struct {
    WorkspaceID    uuid.UUID
    MasterDataID   uuid.UUID
    TriggerID      string
    FlowCategory   string
    Role           string
    AssignedToUser string
    Priority       string
    DueAt          time.Time
    ContextSummary map[string]any
    SuggestedDraft string
}

type ManualActionService struct {
    repo        Repository
    telegram    TelegramNotifier
    activityLog ActivityLogger
    masterData  MasterDataWriter
}

// CreatePending is invoked by the cron engine when a manual-flow trigger matches.
// Inserts a queue row and fires a Telegram DM to the assignee.
func (s *ManualActionService) CreatePending(ctx context.Context, in ManualActionInput) error {
    action := &ManualAction{
        ID: uuid.New(), WorkspaceID: in.WorkspaceID, MasterDataID: in.MasterDataID,
        TriggerID: in.TriggerID, FlowCategory: in.FlowCategory, Role: in.Role,
        AssignedToUser: in.AssignedToUser, SuggestedDraft: in.SuggestedDraft,
        ContextSummary: in.ContextSummary, Status: StatusPending,
        Priority: in.Priority, DueAt: in.DueAt,
    }
    if err := s.repo.Insert(ctx, action); err != nil {
        return err
    }
    return s.telegram.NotifyQueued(ctx, action)
}

// MarkSent confirms the human sent the message. Writes action_logs + sent_flag.
func (s *ManualActionService) MarkSent(ctx context.Context, id uuid.UUID, channel, actualMsg, notes string) error {
    action, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return err
    }
    if action.Status != StatusPending && action.Status != StatusInProgress {
        return errors.New("manual action not in pending/in_progress state")
    }
    now := time.Now()
    action.Status = StatusSent
    action.SentAt = &now
    action.SentChannel = &channel
    action.ActualMessage = &actualMsg
    if err := s.repo.Update(ctx, action); err != nil {
        return err
    }
    _ = s.activityLog.LogHumanSend(ctx, action, notes)             // sender_type='human'
    _ = s.masterData.StampSentFlag(ctx, action.MasterDataID, action.TriggerID)
    return nil
}

// Skip marks the action as skipped with a reason (min 5 chars). No penalty.
func (s *ManualActionService) Skip(ctx context.Context, id uuid.UUID, reason string) error {
    if len(reason) < 5 {
        return errors.New("reason must be at least 5 characters")
    }
    action, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return err
    }
    if action.Status != StatusPending && action.Status != StatusInProgress {
        return errors.New("manual action not in pending/in_progress state")
    }
    action.Status = StatusSkipped
    action.SkippedReason = &reason
    return s.repo.Update(ctx, action)
}

// ExpirePastDue is a periodic job: pending rows past due_at + 48h → expired + P0 escalation.
func (s *ManualActionService) ExpirePastDue(ctx context.Context) error {
    cutoff := time.Now().Add(-48 * time.Hour)
    expired, err := s.repo.ListPastDue(ctx, cutoff)
    if err != nil {
        return err
    }
    for _, a := range expired {
        a.Status = StatusExpired
        _ = s.repo.Update(ctx, &a)
        _ = s.telegram.NotifyEscalation(ctx, &a)                   // → manager, P0
    }
    return nil
}
```

---

## Telegram Notification Contract

Uses the workspace's Telegram bot integration (per-workspace, see external-integrations docs).

**On queue insert (status=pending):**
```
📋 Manual action required: {flow_category}
Client: {Company_Name}
Due: {due_at, WIB}
Priority: {priority}
→ {dashboard_link}/manual-actions/{id}
```

**12h after due_at without sent/skip — re-notify & escalate priority one tier:**
```
⚠️ Overdue manual action: {flow_category} · Client: {Company_Name} · was due {due_at}
```

**due_at + 48h without action — status='expired', P0 to manager:**
```
🚨 ESCALATION — manual action expired: {flow_category}
Client: {Company_Name} · Assignee: {assigned_to_user} → {manager_email} (P0)
```

---

## Dashboard Contract

Full FE spec in `context/claude/08-workflow-and-pipeline.md` § GUARD. Key points:

- Dashboard polls `GET /manual-actions?status=pending` every **60s**
- Sidebar badge shows count of pending actions for current user
- Modal renders `suggested_draft` in editable textarea; "Kirim" button →
  `PATCH /mark-sent` with channel selector + edited text
- Activity Feed shows sent actions with **👤 human-composed** badge (vs 🤖
  bot-sent, sourced from `action_logs.sender_type`)
- Approval Drawer handles `admin_pricing_edit` / `admin_blacklist_edit` via the
  checker-maker flow (see `00-shared/05-checker-maker.md`)

---

## Rules & Invariants

1. **No bot quota impact.** Manual actions do NOT consume bot send quota — the
   max-6-WA/SDR and 1-WA/day/PIC rules apply to bot sends only.

2. **Skip has no penalty.** `sequence_status` unchanged; flow moves to next
   trigger in sequence if applicable.

3. **Expiry = due_at + 48h.** Status flips to `expired`, priority escalates to
   `P0`, manager Telegram'd. Human can still mark-sent retroactively.

4. **Category is a closed enum** (17 values). Multiple triggers can share a
   category. Never accept free-text categories from the frontend.

5. **Cron boundary is static.** A `trigger_id` is either always manual (in
   `manualFlowTriggers`) or always bot — never both. Switching requires a code
   change, not runtime config.

6. **Ownership at enqueue time.** `assigned_to_user` resolved from
   `master_data.Owner_Name` (or role-specific owner field) when the row is
   inserted. Re-assignment afterwards is a manual admin action.

7. **Stage transitions stay cron-authoritative.** Marking `sent` does NOT
   transition `master_data.stage`. Stage changes implied by a flow (e.g.
   renewal signed) go through `POST /approvals` with
   `request_type='stage_transition'` (see `04-api-endpoints.md`).

---

## Edge Cases & High-Risk Scenarios (#36)

The 32 BD edge cases that the engine must detect, log to `edge_case_log` (see
`02-database-schema.md` §12), and route to the appropriate handler. Severity
buckets: **CRITICAL** (4) · **HIGH** (10) · **MEDIUM** (7) · **LOW** (4) ·
**FP-SPECIFIC** (7).

Owners use the codes below as `case_code`. Trigger conditions are checked at the
end of `processProspect` (after blast/FP/nurture evaluation) so the case fires
even when no message is sent.

### CRITICAL (4) — block bot, alert BD Lead immediately

| Code           | Trigger condition                                                           | Resolution logic                                              | Owner    |
|----------------|-----------------------------------------------------------------------------|---------------------------------------------------------------|----------|
| EC-BD-CRIT-01  | `legal_complaint_received = TRUE`                                           | Set `Bot_Active=FALSE`, `blacklisted=TRUE`, alert AE Lead+Legal| AE Lead  |
| EC-BD-CRIT-02  | DM explicitly forbids contact AND BD attempts continue                      | Force `Bot_Active=FALSE`, fire ESC-BD-002, escalate to BD Lead | BD Lead  |
| EC-BD-CRIT-03  | Wrong company invoice issued (Paper.id mismatch)                            | Void invoice, pause sequence, finance manual fix              | Finance  |
| EC-BD-CRIT-04  | PIC death / org collapse signal in reply                                    | Bot stops, transition to DORMANT, manual review               | BD Lead  |

### HIGH (10) — log + reroute, may continue with caution

| Code           | Trigger condition                                                           | Resolution logic                                                           | Owner   |
|----------------|-----------------------------------------------------------------------------|----------------------------------------------------------------------------|---------|
| EC-BD-HIGH-01  | DM absent across all 4 BD_DM_ABSENT followups                               | Insert manual_action_queue → BD owner reads political nuance               | BD      |
| EC-BD-HIGH-02  | Multiple PICs at same company conflict                                       | Lock to senior PIC, suppress duplicate sends                               | BD      |
| EC-BD-HIGH-03  | Competitor poaching mid-cycle (price war signal)                             | Switch to TPL-BD-{day}-VS-FREE variant, alert BD Lead                      | BD Lead |
| EC-BD-HIGH-04  | Prospect is also a competitor employee                                       | Mask pricing details in template, alert BD Lead                            | BD Lead |
| EC-BD-HIGH-05  | Reply detected outside business hours but bot fires WD msg                   | Defer next send to next WD 09:00, queue HaloAI handoff                     | Engine  |
| EC-BD-HIGH-06  | `buying_intent` flips high → low mid-sequence                                | Truncate to low-intent path, fire ESC-BD-007 if D >= D10                   | BD      |
| EC-BD-HIGH-07  | Owner email NULL when assigning manual action                                | Fall back to BD Lead email, log warning                                    | Engine  |
| EC-BD-HIGH-08  | Dealls existing customer being prospected by BD (double-touch)               | Stop BD sequence, hand to AE pipeline immediately                          | AE Lead |
| EC-BD-HIGH-09  | HaloAI takeover but no human follow-up after 48h                             | Re-enable bot to BD_D{next}, alert BD owner                                | BD      |
| EC-BD-HIGH-10  | Phone number invalid (HaloAI 4xx)                                            | Mark `pic_wa_invalid=TRUE`, fall back to email channel                     | Engine  |

### MEDIUM (7) — log + soft-handle

| Code            | Trigger condition                                                          | Resolution logic                                                | Owner   |
|-----------------|----------------------------------------------------------------------------|-----------------------------------------------------------------|---------|
| EC-BD-MED-01    | Reply in foreign language (non-ID/EN)                                       | Route to HaloAI translate flow, defer next send by 1 WD         | HaloAI  |
| EC-BD-MED-02    | Two-week silence after positive reply                                       | Insert `BD_D14_FINAL_CHECKIN` manual action                     | BD      |
| EC-BD-MED-03    | Implementation timeline pushed to next quarter                              | Re-anchor `intake_at` to new quarter start, fire `BD_D0` again  | BD      |
| EC-BD-MED-04    | Prospect requests demo with non-DM IT-only attendee                         | Switch to `TPL-BD-{day}-IT-ATTENDEE` variant                    | BD      |
| EC-BD-MED-05    | Budget allocated but PO process > 60 days                                   | Transition to FP_LONG_TAIL, suppress urgency templates          | BD      |
| EC-BD-MED-06    | Holiday week falls inside D7-D14 window                                     | Pause WD counter, resume after holiday block                    | Engine  |
| EC-BD-MED-07    | Prospect changes contact mid-sequence                                       | Update PIC_WA, reset WA dedupe window, log handoff              | BD      |

### LOW (4) — auto-handle, pure log

| Code           | Trigger condition                                  | Resolution logic                                       | Owner  |
|----------------|----------------------------------------------------|--------------------------------------------------------|--------|
| EC-BD-LOW-01   | Out-of-office auto-reply detected                  | Defer next send by `OOO.until` if parseable, else +3 WD| Engine |
| EC-BD-LOW-02   | Read-receipt only, no reply                        | Continue normal sequence, log `read_only=TRUE`         | Engine |
| EC-BD-LOW-03   | Emoji-only reply                                   | Treat as positive ack, advance to next D-day           | Engine |
| EC-BD-LOW-04   | Duplicate prospect intake (Apollo + manual)        | Merge to first `id`, log dup                           | Engine |

### FP-SPECIFIC (7) — first-payment chase edge cases

| Code         | Trigger condition                                                | Resolution logic                                                       | Owner    |
|--------------|------------------------------------------------------------------|------------------------------------------------------------------------|----------|
| EC-FP-01     | Invoice issued but Paper.id webhook never fires                  | Reconcile via Paper.id polling job, alert finance after 24h            | Finance  |
| EC-FP-02     | Partial payment received                                         | Flag `payment_status='Partial'`, suppress overdue templates            | Finance  |
| EC-FP-03     | Payment received but to wrong invoice number                     | Manual reconcile, mark both invoices in review                         | Finance  |
| EC-FP-04     | Refund requested mid-FP chase                                    | Stop FP sequence, fire ESC-BD-005, transition to CLOSED_LOST           | BD Lead  |
| EC-FP-05     | FP D+15+ overdue with no contact                                 | Auto-fire ESC-BD-005 (P0), restrict access via product API             | BD Lead  |
| EC-FP-06     | Prospect upgrades plan during FP chase                           | Recompute invoice, restart FP_D0 with new amount                       | BD       |
| EC-FP-07     | Currency mismatch (USD invoice for IDR client)                   | Flag for finance review, suppress reminders until corrected            | Finance  |

> **Engine contract:** every fire of an edge case must INSERT one row into
> `edge_case_log` with `trigger_context` capturing the relevant master_data
> snapshot. Resolution updates the same row (UPDATE outcome + resolved_at) — do
> NOT insert a new row when an open one exists for the same `(master_data_id, case_code)`.
