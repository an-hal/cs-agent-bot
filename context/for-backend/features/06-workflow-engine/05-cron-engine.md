# Cron Engine — Workflow Execution

## Overview

The cron engine is the runtime that evaluates workflow nodes against Master Data.
It runs on a schedule (Google Cloud Scheduler -> HTTP endpoint) and processes
every active record through its corresponding workflow pipeline.

```
Cloud Scheduler (every hour, 09:00-17:00 WIB on working days)
    │
    ▼
POST /api/v1/cron/evaluate
    │
    ▼
For each workspace:
    │
    ├─► Load all active master_data records
    │
    ├─► For each record:
    │     ├─► Gate checks (blacklisted? Bot_Active? snooze?)
    │     ├─► Route by Stage → correct pipeline
    │     ├─► Evaluate all nodes in order
    │     ├─► Execute matching actions
    │     └─► WRITE results back to master_data + action_logs
    │
    └─► Summary log
```

---

### AE Phase Reference Table

Canonical reference for the AE lifecycle phases used by `evaluateAE`. Timing-only — no manual stage change needed; the engine derives `current_phase` every run.

| Phase | Window | Timing Basis | Semantics | Trigger Count |
|-------|--------|--------------|-----------|---------------|
| P0 Onboarding | D+0 to D+35 | days since contract_start | Welcome, setup check, adoption baseline | 3 |
| P1 Assessment | D+42 to D+60 | D+ | NPS baseline, cross-sell awareness | 3 |
| P2 Warming Up | D+90 to D+122 | D+ | Monthly check-in, feature discovery, cross-sell seed | 4 |
| P3 Promo Selling | H-120 to H-94 | days before contract_end | Early renewal promo, benefit messaging | 4 |
| P4 Renewal Negotiation | H-90 to H-41 | H- (stops on reply) | Renewal conversation start, quotation sent at H-30 (mandatory) | 7 |
| P5 Renewal Ops | H-35 to H-0 | H- (never stops on reply) | Invoice sent, reminder sequence, bot pauses after payment confirmed | 7 |
| P6 Overdue | D+1 to D+45+ | days since due_date | Payment tracking, escalation from soft reminder to access restriction (no working-day rule) | 4 |

> Phase transition is **automatic by timing** — no manual stage change required. The cron engine computes `current_phase` from `contract_start` / `contract_end` / `Payment_Status` / `Days_Overdue` on every run. Phase is not a stored column; it's derived.

---

### Operating Constraints (Global Gates)

Cross-cutting rules enforced globally by the engine. These compose with the per-gate details in the **Gate Checks (detailed)** section below — this subsection is the **authoritative summary**; the Gate Checks section is the implementation reference.

**Working-Day Rule**
- All SDR/BD/AE WA/Email triggers (except `SEASONAL_*` variants) execute only on working days.
- Working day = Mon-Fri in `Asia/Jakarta` timezone, excluding Indonesian national holidays.
- Backend helper: `isWorkingDay(today Time) bool` — checked inside `tryNode` before sending.
- P6 Overdue is **exempt** (runs every day — payment chase doesn't respect weekends).

**WA Send Window**
- Morning: 09:00–11:30 WIB
- Afternoon: 13:30–16:30 WIB
- Outside window → queue the message for next valid send window (not drop).
- Email has **no window constraint** (background delivery OK).
- Helper: `isInSendWindow(now Time) bool`

**Max Touches**
- SDR: max 6 WA + 2 email per lead per sequence (hard limit; sequence enters NURTURE after cap)
- BD: no hard cap (sequence-based — ends at D21 or NURTURE via condition)
- AE: no hard cap (phase-gated)
- **Per-recipient rate:** max 1 WA per day per `PIC_WA` across all pipelines. Enforced by `lastWAAt` lookup + 24h window check.

**Stop Conditions (any pauses the bot)**
- `blacklisted = TRUE` — permanent stop
- `Bot_Active = FALSE` — manual pause (can be resumed manually)
- Any prospect/client reply — hand off to HaloAI classification or manual AE
- `sequence_status` not in `('ACTIVE')` — paused / snoozed / completed

**Resume Rules**
- After `snooze_until` passes → auto-resume if other conditions still met
- Manual `SNOOZE_RESUME` trigger runs: sets `Bot_Active=TRUE` + `sequence_status='ACTIVE'` + fires next-due trigger
- No auto-resume from blacklist (requires admin approval via `change_role_permission`-style flow)

---

## Cron Schedule

| Trigger                     | Schedule                           | Notes                        |
|-----------------------------|------------------------------------|------------------------------|
| Main cron (working day)     | Every hour 09:00-17:00 WIB Mon-Fri| Skips holidays               |
| Days_to_Expiry update       | Daily at 00:05 WIB                 | Recalculate for all CLIENTs  |
| Overdue check               | Every 4 hours (including weekends) | P6 has no working day rule   |
| Snooze resume check         | Daily at 09:00 WIB                 | Resume snoozed records       |
| Seasonal blast              | Custom (manual or scheduled)       | New Year: 31 Dec 23:35 WIB  |
| **BD Blast Sequence (#25)** | Daily at 09:00 WIB Mon-Fri         | D0/D2/D4/D7/D10/D12/D14/D21 evaluation w/ branching |
| **AE Escalation SLA (#17)** | Every 5 minutes (24/7)             | Scans `escalations`, fires reminders + Telegram fallback |
| **SDR Nurture Recycle (#18)** | Daily at 09:00 WIB                | D+60/90/120/180/270/365 nurture touches (max 3 cycles) |
| **Intake Rate Limit (#45)** | At intake API entry (event-driven) | Caps `MAX_LEADS_PER_SDR_PER_DAY * active_sdr_count`; queue/pause Apollo |

---

### #25 BD Blast Sequences cron (daily 09:00 WIB)

Evaluates every active `Stage='PROSPECT'` record and fires the next-due D-day
template based on `intake_at` + branching state. Sequence:
`D0 → D2 → D4 → D7 → D10 → D12 → D14 → D21 → NURTURE`.

Low-intent shortened path (when `buying_intent='low'`):
`D0 → D2 → D4 → D7 → D10 → NURTURE` (skip D12/D14/D21).

```go
func cronBDBlast(ctx context.Context, repo *DataMasterRepo) error {
    if !helpers.IsWorkingDay(time.Now()) { return nil }
    prospects, _ := repo.ListByStage(ctx, "PROSPECT", /*activeOnly=*/true)
    for _, p := range prospects {
        days := helpers.DaysSince(p.GetCustomTime("intake_at"))
        if !inWindow(days) { continue }                // see D-day matrix
        bd := evaluateBlast(ctx, p, p.Flags())         // branch resolver
        if bd == nil { continue }                      // no fire today
        tplID, _ := resolveTemplate(bd.TemplateBase, p, p.Flags())
        _ = sendWAMessage(ctx, p.PIC_WA, render(tplID, p), meta(p))
    }
    return nil
}
```

D-day eligibility matrix (working-day count from `intake_at`):

| D-day | Window (WD) | Trigger ID prefix | Skip if low-intent |
|-------|-------------|-------------------|--------------------|
| D0    | 0           | `BD_D0`           | No                 |
| D2    | 2           | `BD_D2`           | No                 |
| D4    | 4           | `BD_D4`           | No                 |
| D7    | 7           | `BD_D7`           | No                 |
| D10   | 10          | `BD_D10`          | No                 |
| D12   | 12          | `BD_D12`          | **Yes**            |
| D14   | 14          | `BD_D14`          | **Yes**            |
| D21   | 21          | `BD_D21`          | **Yes**            |

### #17 AE Escalation SLA cron (every 5min, 24/7)

Scans `escalations` table for rows with `ack_at IS NULL` past their `sla_seconds`
deadline. Sends Telegram reminder to `assigned_to_email`; if still un-acked at
2× SLA, escalates to AE Lead via `TELEGRAM_AE_LEAD_ID`.

| Priority | Ack SLA | Escalation hop                        |
|----------|---------|----------------------------------------|
| P0       | 1 hour  | BD owner → AE Lead at +1h               |
| P1       | 24 hours| BD owner → AE Lead at +24h              |
| P2       | 24 hours| BD owner → AE Lead at +24h (informational)|

```sql
SELECT * FROM escalations
WHERE ack_at IS NULL
  AND created_at + (sla_seconds || ' seconds')::interval < NOW()
  AND resolved_at IS NULL;
```

For each row: insert a `notifications` reminder, and if `NOW() - created_at > 2 * sla_seconds`,
re-route to `TELEGRAM_AE_LEAD_ID` and set `escalations.fallback_at = NOW()`.

### #18 SDR Nurture Recycling cron (daily 09:00 WIB)

Fires re-engagement touches for `Stage='LEAD'` records in `sequence_status='NURTURE_POOL'`
(or `NURTURE`). Uses `lead_intake_at` as anchor + `nurture_count` cycle counter.

Touch schedule (working-day-aligned offsets from `lead_intake_at`):

| Day offset | Trigger ID       | Notes                                       |
|------------|------------------|---------------------------------------------|
| D+60       | `NURTURE_D60`    | Referral offer                              |
| D+90       | `NURTURE_D90`    | Casual touch                                |
| D+120      | `NURTURE_D120`   | Pain-based (end-of-quarter)                 |
| D+180      | `NURTURE_D180`   | Social proof (6-month milestone)            |
| D+270      | `NURTURE_D270`   | Referral 2nd touch                          |
| D+365      | `NURTURE_D365`   | Anniversary → `nurture_count++`, reset to D60 cycle |

After D+365 → bump `nurture_count`. When `nurture_count >= 3` → transition to
`Stage='DORMANT'` (permanent, requires manual revive).

### #45 Intake Rate Limiting (event-driven at intake API)

Apollo and manual `POST /master-data/intake` calls are gated by per-day cap:

```go
func enforceIntakeRateLimit(ctx context.Context, wsID uuid.UUID) error {
    cfg, _ := repo.GetSystemConfig(ctx, wsID, "MAX_LEADS_PER_SDR_PER_DAY")
    active, _ := userRepo.CountActiveSDRs(ctx, wsID)
    cap := cfg.IntValue * active
    todayCount, _ := repo.CountIntakeToday(ctx, wsID)
    if todayCount >= cap {
        // Option A (Apollo): pause Apollo webhook until tomorrow 00:05 WIB
        return svcApollo.Pause(ctx, wsID, untilNextDay())
        // Option B (manual): queue → process tomorrow
    }
    return nil
}
```

Config keys (table `system_config`):

| Key                            | Type | Default | Notes                              |
|--------------------------------|------|---------|------------------------------------|
| `MAX_LEADS_PER_SDR_PER_DAY`    | INT  | 30      | Hard cap per SDR per calendar day  |
| `INTAKE_PAUSE_BEHAVIOR`        | TEXT | `queue` | `queue` \| `pause_apollo` \| `drop`|

---

## processClient Flow

The core execution flow for a single master_data record:

```go
// POST /api/v1/cron/evaluate
// Called by Cloud Scheduler

func EvaluateCron(ctx context.Context, repo *DataMasterRepo, wfRepo *WorkflowRepo, wsID uuid.UUID) error {
    // 1. Check if today is a working day (for main cron — overdue cron skips this)
    if !helpers.IsWorkingDay(time.Now()) {
        log.Info("Not a working day, skipping main cron")
        return nil
    }

    // 2. Get all active records for this workspace
    records, _, err := repo.GetByWorkspace(ctx, wsID, nil, 10000, 0)
    if err != nil {
        return err
    }

    // 3. Get all active workflows for this workspace
    workflows, err := wfRepo.ListByWorkspace(ctx, wsID, stringPtr("active"))
    if err != nil {
        return err
    }

    // 4. Process each record
    var processed, skipped, errors int
    for _, record := range records {
        err := processClient(ctx, repo, workflows, &record)
        if err != nil {
            errors++
            log.Error("processClient failed", "company", record.CompanyName, "err", err)
        } else if err == nil {
            processed++
        }
    }

    log.Info("Cron complete", "processed", processed, "skipped", skipped, "errors", errors)
    return nil
}
```

### processClient — single record processing

```go
func processClient(ctx context.Context, repo *DataMasterRepo, workflows []Workflow, record *DataMaster) error {
    // ═══ GATE 1: Blacklist check ═══
    if record.Blacklisted {
        return nil // permanently excluded
    }

    // ═══ GATE 2: Bot_Active check ═══
    if !record.BotActive {
        // Exception: check if snooze_until has passed
        if record.SnoozeUntil != nil && time.Now().After(*record.SnoozeUntil) {
            // Resume from snooze — process SNOOZE_RESUME trigger
            return processSnoozeResume(ctx, repo, record)
        }
        return nil // bot is off, skip
    }

    // ═══ GATE 3: Working day check ═══
    // (already checked at cron level, but P6 overdue skips this)

    // ═══ GATE 4: Snooze check ═══
    if record.SequenceStatus == SeqSnoozed {
        if record.SnoozeUntil != nil && time.Now().Before(*record.SnoozeUntil) {
            return nil // still snoozed
        }
        // Snooze expired — resume
        return processSnoozeResume(ctx, repo, record)
    }

    // ═══ ROUTE by Stage ═══
    for _, wf := range workflows {
        if containsStage(wf.StageFilter, string(record.Stage)) {
            return evaluateWorkflow(ctx, repo, &wf, record)
        }
    }

    return nil // no matching workflow
}
```

---

## Node Execution Order

Within a workflow, nodes are evaluated in **phase order** (P0 -> P1 -> P2 -> ... -> P6).
Within each phase, nodes are evaluated in **chronological order** (by timing window).

**Important**: Only ONE action per record per cron run. Once a node matches and executes,
the record is done for this cycle. This prevents message flooding.

```go
func evaluateWorkflow(ctx context.Context, repo *DataMasterRepo, wf *Workflow, record *DataMaster) error {
    // Get all active automation rules for this workflow's role
    rules, err := ruleRepo.GetActiveByRole(ctx, wf.WorkspaceID, determineRole(wf))
    if err != nil {
        return err
    }

    // Evaluate rules in phase order
    for _, rule := range rules {
        matched, err := evaluateRule(ctx, repo, &rule, record)
        if err != nil {
            log.Error("rule evaluation failed", "rule", rule.RuleCode, "err", err)
            continue
        }
        if matched {
            // ONE action per record per cron run
            return nil
        }
    }

    return nil
}
```

---

## Gate Checks (detailed)

### Gate 1: Blacklist
```go
// Permanently excluded from all automation
// Set by: admin manual, angry reply detection, legal request
// Check: record.Blacklisted == true → EXIT immediately
```

### Gate 2: Bot_Active
```go
// Temporary pause of all automation for this record
// Set by:
//   - HaloAI takeover (reply received → Bot_Active = FALSE)
//   - AE manual pause
//   - P5.7 H-0 bot stop
//   - P6.4 overdue bot stop
//   - BD CLOSED_LOST
// Resume by:
//   - RESUME_AFTER_SILENCE (BD — X days after last HaloAI msg)
//   - SNOOZE_RESUME (SDR — after snooze_until date)
//   - Manual re-enable
//   - Stage transition (new stage resets Bot_Active = TRUE)
```

### Gate 3: isWorkingDay
```go
// Working day = Monday-Friday, NOT a national holiday
// Applied to: ALL nodes EXCEPT P6 (Overdue)
// P6 Overdue runs 7 days a week (no working day restriction)
// Seasonal exceptions:
//   - SEASONAL_NEWYEAR: 31 Dec 23:35 regardless of day
//   - SEASONAL_LEBARAN: H-1 Lebaran (must be working day)

func isWorkingDay(t time.Time) bool {
    wd := t.Weekday()
    if wd == time.Saturday || wd == time.Sunday {
        return false
    }
    // Check Indonesian national holidays
    return !isNationalHoliday(t)
}
```

### Gate 4: Snooze
```go
// SDR/BD can snooze a record to a future date
// When snoozed:
//   - sequence_status = 'SNOOZED'
//   - snooze_until = future date
//   - Bot_Active = FALSE (usually)
// When snooze_until passes:
//   - SNOOZE_RESUME trigger fires
//   - sequence_status = 'ACTIVE'
//   - Bot_Active = TRUE
```

---

## Phase Routing — SDR Pipeline

SDR processes records with `Stage = 'LEAD'` (and `DORMANT` for reporting).

### Phase Order

```
P1 — Email Outreach (Apollo)
  EMAIL_H0            H+0 WD — cold email
  EMAIL_H4            H+4 WD — follow up pain-focused

P2 — WA Blast Sequence (HaloAI)
  WA_H0               H+0 WD — cek identitas
  WA_H1               H+1 WD — perkenalan panjang
  WA_H3               H+3 WD — follow up jadwal diskusi
  WA_H7               H+7 WD — thank you + izin kontak lagi
  WA_H8               H+8 WD — follow up sesuai janji
  WA_H14              H+14 WD — penutup graceful -> NURTURE_POOL
  WA_COMPETITOR_H0    H+0 override — contract timing (competitor segment)
  HALOAI_HANDOFF      Immediate — reply detected -> bot stops

P3 — Feature Broadcast (Manual)
  FEATURE_BROADCAST   Manual trigger — SDR Lead initiates
  SNOOZE_RESUME       On snooze_until — resume from snooze

P4 — Nurture Cycle (every 60-365 days, repeating)
  NURTURE_D60         D+60 WD — referral offer
  NURTURE_D90         D+90 WD — casual touch
  NURTURE_D120        D+120 WD — pain-based (end of quarter)
  NURTURE_D180        D+180 WD — social proof (6 month milestone)
  NURTURE_D270        D+270 WD — referral 2nd touch
  NURTURE_D365        D+365 WD — anniversary -> cycle reset (back to D60)
  SDR_NURTURE_TO_DORMANT  nurture_count >= 3 -> permanent DORMANT

Seasonal (no phase — manual triggers)
  SEASONAL_NEWYEAR    31 Dec 23:35 WIB
  SEASONAL_LEBARAN    H-1 Lebaran 08:00 WIB
  SEASONAL_CUSTOM     Custom date

HANDOFF
  SDR_QUALIFY_HANDOFF  Lead -> Prospect (HC >= MIN_HC_SIZE + role match + interest)
```

### SDR Evaluation Logic

```go
func evaluateSDR(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    // Determine lead segment for template variant
    segment := record.GetCustomString("lead_segment") // FRESH, RECYCLED, COMPETITOR_TIMED

    // Check channel availability
    channelAvail := record.GetCustomString("channel_availability") // BOTH, WA_ONLY, EMAIL_ONLY

    // P1 — Email (if channel supports)
    if channelAvail == "BOTH" || channelAvail == "EMAIL_ONLY" {
        if tryNode(ctx, repo, record, "EMAIL_H0") { return nil }
        if tryNode(ctx, repo, record, "EMAIL_H4") { return nil }
    }

    // P2 — WA Blast (if channel supports)
    if channelAvail == "BOTH" || channelAvail == "WA_ONLY" {
        // Competitor override check
        if segment == "COMPETITOR_TIMED" && !record.GetCustomBool("wa_h0_sent") {
            if tryNode(ctx, repo, record, "WA_COMPETITOR_H0") { return nil }
        }

        if tryNode(ctx, repo, record, "WA_H0") { return nil }
        if tryNode(ctx, repo, record, "WA_H1") { return nil }
        if tryNode(ctx, repo, record, "WA_H3") { return nil }
        if tryNode(ctx, repo, record, "WA_H7") { return nil }
        if tryNode(ctx, repo, record, "WA_H8") { return nil }
        if tryNode(ctx, repo, record, "WA_H14") { return nil }
    }

    // P4 — Nurture (if in nurture pool)
    if record.SequenceStatus == SeqNurturePool || record.SequenceStatus == SeqNurture {
        if tryNode(ctx, repo, record, "NURTURE_D60") { return nil }
        if tryNode(ctx, repo, record, "NURTURE_D90") { return nil }
        if tryNode(ctx, repo, record, "NURTURE_D120") { return nil }
        if tryNode(ctx, repo, record, "NURTURE_D180") { return nil }
        if tryNode(ctx, repo, record, "NURTURE_D270") { return nil }
        if tryNode(ctx, repo, record, "NURTURE_D365") { return nil }

        // Check dormant threshold
        nurtureCount := int(record.GetCustomFloat("nurture_count"))
        if nurtureCount >= 3 {
            return handleSDRNurtureToDormant(ctx, repo, record)
        }
    }

    // Qualify check (always — regardless of where in sequence)
    if record.GetCustomBool("reply_wa") || record.GetCustomBool("reply_email") {
        hcSize := record.GetCustomFloat("hc_size")
        roleMatch := record.GetCustomBool("role_match")
        if hcSize >= float64(config.MinHCSize) && roleMatch {
            return handleSDRQualifyHandoff(ctx, repo, record)
        }
    }

    return nil
}
```

---

## Phase Routing — BD Pipeline

BD processes records with `Stage = 'PROSPECT'`.

### Phase Order

```
P1 — HaloAI Handoff (event-driven, not cron)
  HALOAI_HANDOFF        Immediate — any reply -> bot stops, HaloAI takes over
  RESUME_AFTER_SILENCE  X days after last HaloAI msg -> resume blast

P2 — WA Blast Sequence D0-D21
  BD_D0                 D+0 WD — post-call recap (within 2h)
  BD_D2                 D+2 WD — ringkasan untuk manajemen
  BD_D4                 D+4 WD — dashboard analytics showcase
  BD_D7                 D+7 WD — follow up presentasi
  BD_D10                D+10 WD — timeline urgency
  BD_D12                D+12 WD — promo deadline + slot scarcity
  BD_D14                D+14 WD — soft check-in, open snooze
  BD_D21                D+21 WD — wind-down -> NURTURE

P2.5 — First Payment Chase
  FP_D0                 FP D+0 WD — invoice + early bird offer
  FP_D7                 FP D+7 WD — early bird last day
  FP_D21                FP D+21 WD — mid-cycle payment reminder
  FP_D27                FP D+27 WD — final pre-due

P3 — Nurture (post-D21)
  BD_NURTURE_D45        D+45 WD — re-engage
  BD_NURTURE_D75        D+75 WD — soft re-engage
  BD_NURTURE_D90        D+90 WD — goodbye -> DORMANT -> SDR queue

P3.5 — Overdue (First Payment)
  FP_POST1              FP D+31 — soft (no WD rule)
  FP_POST4              FP D+34 — firm (no WD rule)
  FP_POST15             FP D+44+ — wind-down -> CLOSED_LOST (no WD rule)

HANDOFF
  BD_PAYMENT_HANDOFF    Prospect -> Client (Payment_Status = Paid)

RECYCLE
  BD_DORMANT_TO_SDR     Dormant -> re-enter SDR queue (90d after NURTURE_D90)
```

### BD Evaluation Logic

```go
func evaluateBD(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    closingStatus := record.GetCustomString("closing_status")

    // P2.5 — First Payment (if deal closed)
    if closingStatus == "CLOSING" {
        invoiceID := record.GetCustomString("invoice_id")
        if invoiceID != "" {
            // Check payment
            if record.PaymentStatus == "Paid" {
                return handleBDPaymentHandoff(ctx, repo, record)
            }
            // Payment chase sequence
            if tryNode(ctx, repo, record, "FP_D0") { return nil }
            if tryNode(ctx, repo, record, "FP_D7") { return nil }
            if tryNode(ctx, repo, record, "FP_D21") { return nil }
            if tryNode(ctx, repo, record, "FP_D27") { return nil }

            // Overdue (no working day check)
            if record.PaymentStatus == "Overdue" {
                if tryNode(ctx, repo, record, "FP_POST1") { return nil }
                if tryNode(ctx, repo, record, "FP_POST4") { return nil }
                if tryNode(ctx, repo, record, "FP_POST15") { return nil }
            }
        }
        return nil
    }

    // P2 — Blast Sequence D0-D21
    if record.SequenceStatus == SeqActive {
        if tryNode(ctx, repo, record, "BD_D0") { return nil }
        if tryNode(ctx, repo, record, "BD_D2") { return nil }
        if tryNode(ctx, repo, record, "BD_D4") { return nil }
        if tryNode(ctx, repo, record, "BD_D7") { return nil }
        if tryNode(ctx, repo, record, "BD_D10") { return nil }
        if tryNode(ctx, repo, record, "BD_D12") { return nil }
        if tryNode(ctx, repo, record, "BD_D14") { return nil }
        if tryNode(ctx, repo, record, "BD_D21") { return nil }
    }

    // P3 — Nurture
    if record.SequenceStatus == SeqNurture {
        if tryNode(ctx, repo, record, "BD_NURTURE_D45") { return nil }
        if tryNode(ctx, repo, record, "BD_NURTURE_D75") { return nil }
        if tryNode(ctx, repo, record, "BD_NURTURE_D90") { return nil }
    }

    // Recycle check (DORMANT -> SDR)
    if record.SequenceStatus == SeqDormant {
        lastNurture := record.GetCustomString("last_nurture_date")
        if lastNurture != "" {
            t, _ := time.Parse(time.RFC3339, lastNurture)
            if helpers.DaysSince(&t) >= 90 {
                return handleBDDormantToSDR(ctx, repo, record)
            }
        }
    }

    return nil
}
```

---

## Phase Routing — AE Pipeline

AE processes records with `Stage = 'CLIENT'`.

### Phase Order

```
P0 — Onboarding (D+0 to D+35)
  Onboarding_Welcome     D+0~5 WD
  Onboarding_CheckIn     D+14~19 WD
  Onboarding_UsageCheck  D+30~35 WD

P1 — First Assessment (D+42 to D+60)
  NPS_Survey_1           D+42~47 WD
  NPS_FU_1               D+49~54 WD (if NPS not replied)
  CS_Awareness           D+55~60 WD (ATS cross-sell awareness)

P2 — Warming Up (D+90 to D+122)
  Warmup_CheckIn_Form    D+90~95 WD
  Warmup_CheckIn_Call    D+97~102 WD (if form not replied)
  CS_SocialProof         D+105~112 WD (HOLD if NPS < 8)
  CS_Efficiency          D+115~122 WD (HOLD if NPS < 8)

P3 — Promo Selling (H-120 to H-94)
  Promo_CheckIn_Form     H-120~115 WD (NPS-2)
  Promo_CheckIn_Call     H-113~108 WD (if form not replied)
  Promo_Referral         H-106~101 WD (NPS >= 8 only)
  CS_Pricing_Promo       H-99~94 WD (ATS pricing + promo)

P4 — Renewal Negotiation (H-90 to H-41, STOPS ON REPLY)
  Renewal_REN90          H-90~85 WD — soft opener
  Renewal_CallInvite     H-83~78 WD — call invite
  Renewal_FormFallback   H-76~71 WD — form fallback
  CS_Upsell_Renewal      H-76~71 WD — ATS upsell (parallel)
  Renewal_REN60          H-60~55 WD — status update FU
  Renewal_REN52          H-53~48 WD — quotation + invoice
  Renewal_REN45          H-46~41 WD — decision forcing

P5 — Renewal Ops (H-35 to H-0, NEVER STOPS)
  Renewal_REN35          H-35~30 WD — checkpoint
  Renewal_REN15          H-15~29 WD — nudge
  PAY_PRE14              H-14~9 WD — payment reminder
  PAY_PRE7               H-7~4 WD — payment reminder
  PAY_PRE3               H-3~1 WD — final pre-due
  PAY_VERIF              Immediate — payment verified (webhook or manual)
  Renewal_REN0           H-0 WD — bot stop, AE takeover

P6 — Overdue (D+1 to D+15+, NO WORKING DAY RULE)
  Overdue_POST1          D+1~3 — sopan
  Overdue_POST4          D+4~7 — tegas
  Overdue_POST8          D+8~14 — warning akses dibatasi
  Overdue_POST15         D+15+ — bot stop, AE manual, ESC-001

Cycle:
  PAY_VERIF → renewed=TRUE → reset all flags → back to P0 (new cycle)
```

### AE Evaluation Logic

```go
func evaluateAE(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    daysSinceActivation := helpers.DaysSince(record.ContractStart)
    daysToExpiry := 0
    if record.DaysToExpiry != nil {
        daysToExpiry = *record.DaysToExpiry
    }

    // P6 — Overdue (NO working day check — evaluated first for urgency)
    if record.PaymentStatus == "Overdue" {
        if tryNode(ctx, repo, record, "Overdue_POST1") { return nil }
        if tryNode(ctx, repo, record, "Overdue_POST4") { return nil }
        if tryNode(ctx, repo, record, "Overdue_POST8") { return nil }
        if tryNode(ctx, repo, record, "Overdue_POST15") { return nil }
    }

    // Working day gate for P0-P5
    if !helpers.IsWorkingDay(time.Now()) {
        return nil
    }

    // P5 — Renewal Ops (H-35 to H-0) — NEVER stops on reply
    if daysToExpiry >= 0 && daysToExpiry <= 35 {
        if tryNode(ctx, repo, record, "Renewal_REN35") { return nil }
        if tryNode(ctx, repo, record, "Renewal_REN15") { return nil }
        if tryNode(ctx, repo, record, "PAY_PRE14") { return nil }
        if tryNode(ctx, repo, record, "PAY_PRE7") { return nil }
        if tryNode(ctx, repo, record, "PAY_PRE3") { return nil }
        if tryNode(ctx, repo, record, "Renewal_REN0") { return nil }
    }

    // P4 — Renewal Negotiation (H-90 to H-41)
    if daysToExpiry >= 41 && daysToExpiry <= 90 {
        if tryNode(ctx, repo, record, "Renewal_REN90") { return nil }
        if tryNode(ctx, repo, record, "Renewal_CallInvite") { return nil }
        if tryNode(ctx, repo, record, "Renewal_FormFallback") { return nil }
        if tryNode(ctx, repo, record, "CS_Upsell_Renewal") { return nil }
        if tryNode(ctx, repo, record, "Renewal_REN60") { return nil }
        if tryNode(ctx, repo, record, "Renewal_REN52") { return nil }
        if tryNode(ctx, repo, record, "Renewal_REN45") { return nil }
    }

    // P3 — Promo Selling (H-120 to H-94)
    if daysToExpiry >= 94 && daysToExpiry <= 120 {
        if tryNode(ctx, repo, record, "Promo_CheckIn_Form") { return nil }
        if tryNode(ctx, repo, record, "Promo_CheckIn_Call") { return nil }
        if tryNode(ctx, repo, record, "Promo_Referral") { return nil }
        if tryNode(ctx, repo, record, "CS_Pricing_Promo") { return nil }
    }

    // P2 — Warming Up (D+90 to D+122)
    if daysSinceActivation >= 90 && daysSinceActivation <= 122 {
        if tryNode(ctx, repo, record, "Warmup_CheckIn_Form") { return nil }
        if tryNode(ctx, repo, record, "Warmup_CheckIn_Call") { return nil }
        if tryNode(ctx, repo, record, "CS_SocialProof") { return nil }
        if tryNode(ctx, repo, record, "CS_Efficiency") { return nil }
    }

    // P1 — First Assessment (D+42 to D+60)
    if daysSinceActivation >= 42 && daysSinceActivation <= 60 {
        if tryNode(ctx, repo, record, "NPS_Survey_1") { return nil }
        if tryNode(ctx, repo, record, "NPS_FU_1") { return nil }
        if tryNode(ctx, repo, record, "CS_Awareness") { return nil }
    }

    // P0 — Onboarding (D+0 to D+35)
    if daysSinceActivation >= 0 && daysSinceActivation <= 35 {
        if tryNode(ctx, repo, record, "Onboarding_Welcome") { return nil }
        if tryNode(ctx, repo, record, "Onboarding_CheckIn") { return nil }
        if tryNode(ctx, repo, record, "Onboarding_UsageCheck") { return nil }
    }

    return nil
}
```

> **Note**: P5 and P6 are evaluated BEFORE P4-P0 because payment urgency takes priority.
> P6 has no working day gate.

---

## Phase Routing — CS Pipeline

CS is **event-driven** (not cron). Actions trigger on ticket lifecycle events.

```
CS_TICKET_CREATED    Immediate — ticket created from client WA/email/form
CS_AUTO_ASSIGN       Immediate — round-robin assignment after ticket created
CS_SLA_BREACH        2 hours — no response after ticket created
CS_RESOLUTION        On event — ticket_status = RESOLVED
CS_CSAT_RECEIVED     On event — CSAT survey submitted -> WRITE to Data Master
```

### CS Event Handler

```go
// Called by webhook or internal event bus — NOT by cron
func handleCSEvent(ctx context.Context, repo *DataMasterRepo, event CSEvent) error {
    record, err := repo.GetByID(ctx, event.MasterDataID)
    if err != nil {
        return err
    }

    // Gate: CS only processes CLIENT records
    if record.Stage != StageClient {
        return fmt.Errorf("CS ticket rejected: Stage=%s, expected CLIENT", record.Stage)
    }

    switch event.Type {
    case "ticket_created":
        return processCSTicketCreated(ctx, repo, record, event)
    case "sla_check":
        return processCSSlaBreach(ctx, repo, record, event)
    case "ticket_resolved":
        return processCSResolution(ctx, repo, record, event)
    case "csat_received":
        return processCSCSATReceived(ctx, repo, record, event)
    }

    return nil
}
```

---

## tryNode — Generic Node Evaluation

The core function that evaluates a single node against a record:

```go
// tryNode attempts to execute a node for a record.
// Returns true if the node matched and executed (action sent).
// Returns false if conditions not met (skip to next node).
func tryNode(ctx context.Context, repo *DataMasterRepo, record *DataMaster, triggerID string) bool {
    // 1. Get automation rule by trigger_id
    rule, err := ruleRepo.GetByTriggerID(ctx, record.WorkspaceID, triggerID)
    if err != nil || rule == nil {
        return false
    }

    // 2. Check if rule is active
    if !rule.IsExecutable() {
        return false
    }

    // 3. Evaluate condition
    if !evaluateCondition(record, rule.Condition) {
        return false
    }

    // 4. Check stop_if
    if rule.StopIf != "-" && rule.StopIf != "" {
        if evaluateCondition(record, rule.StopIf) {
            return false // stop condition met, skip
        }
    }

    // 5. Check sent_flag (prevent duplicate sends)
    if rule.SentFlag != nil && *rule.SentFlag != "" {
        primaryFlag := strings.Split(*rule.SentFlag, "\n")[0]
        primaryFlag = strings.TrimSpace(primaryFlag)
        if record.GetCustomBool(primaryFlag) {
            return false // already sent
        }
    }

    // 6. Execute action
    err = executeAction(ctx, repo, record, rule)
    if err != nil {
        logAction(ctx, record, triggerID, "failed", nil)
        return false
    }

    // 7. Write results (sent flag + any extra writes)
    writeResults(ctx, repo, record, rule)

    // 8. Log action
    logAction(ctx, record, triggerID, "delivered", extractWrittenFields(rule))

    return true
}
```

### Timing Format Parser (dual-format)

The `timing` field on a node can be either **legacy code** (from Excel seeds) or **Indonesian text** (from the FE `TimingBuilder` widget). Parser must accept both:

| Format | Example | Meaning |
|---|---|---|
| Legacy: `D+N` / `D+N to D+M` | `D+5`, `D+0 to D+5` | N days after trigger event |
| Legacy: `H-N` / `H-N to H-M` | `H-120`, `H-90 to H-41` | N days BEFORE contract end (for AE renewal) |
| New: `N Hari Setelah` | `5 Hari Setelah` | N days after trigger |
| New: `A-B Hari Setelah` | `0-5 Hari Setelah` | between A and B days after trigger |
| New: `N Hari Sebelum` | `120 Hari Sebelum` | N days before a reference date (contract_end for AE) |
| New: `N Jam Setelah` / `N Menit Setelah` / `N Detik Setelah` | `30 Menit Setelah` | finer-grained timing (not yet used by seeds, but FE emits) |

```go
// ParseTiming returns a normalized timing window.
//   { min, max: offset amount (nil = unbounded), unit, direction }
// Backend scheduler compares against `days_since_activation` (or contract-end
// offset for "Hari Sebelum" variants) to decide whether a node is eligible now.
type TimingWindow struct {
    Min       *int           // lower bound, nil if exact
    Max       *int           // upper bound, nil if exact
    Unit      string         // "Hari" | "Jam" | "Menit" | "Detik"
    Direction string         // "Setelah" | "Sebelum"
    Raw       string         // original string for audit
}

func ParseTiming(raw string) (TimingWindow, error) {
    s := strings.TrimSpace(raw)

    // Indonesian format: "5 Hari Setelah", "0-5 Hari Setelah", "120 Hari Sebelum"
    if m := indonesianRe.FindStringSubmatch(s); m != nil {
        min, max := parseRange(m[1])  // "0-5" or "5"
        return TimingWindow{
            Min: &min, Max: &max,
            Unit:      m[2],           // Hari | Jam | Menit | Detik
            Direction: m[3],           // Setelah | Sebelum
            Raw:       s,
        }, nil
    }

    // Legacy "D+0 to D+5" or "H-120" or "H-90 to H-41"
    if m := legacyRe.FindStringSubmatch(s); m != nil {
        // D means days after; H- means days before.
        // (Yes, "D" here is "Day", not "Detik" — legacy from Excel.)
        ...
    }

    return TimingWindow{}, fmt.Errorf("unrecognized timing format: %q", raw)
}
```

> Note on naming conflict: legacy uses `D` for **Day**, whereas the new Indonesian format uses `D` for **Detik** (second). The parser disambiguates by checking for `+`/`-` (legacy is always `D+N` or `H-N`) vs whitespace + Indonesian keyword (`5 Hari`, `30 Detik`).

### DSL Field Catalog (FE contract)

The FE `ConditionBuilder` / `StopIfBuilder` widgets write expressions using these fields ONLY — backend evaluator must implement all of them. Map FE's 21-field catalog (see `components/features/WorkflowBuilderWrapper.tsx` `CONDITION_FIELDS`):

| Field (LHS expression) | Type     | Backend source                       |
|------------------------|----------|--------------------------------------|
| `days_since_activation`| number   | computed: `today - contract_start`   |
| `days_to_expiry`       | number   | computed: `contract_end - today`     |
| `days_overdue`         | number   | computed: `today - invoice.due_date` |
| `NPS_Score`            | number   | `master_data.nps_score`              |
| `Usage_Score`          | number   | `master_data.usage_score`            |
| `Bot_Active`           | boolean  | `master_data.bot_active`             |
| `blacklisted`          | boolean  | `master_data.blacklisted`            |
| `renewed`              | boolean  | `master_data.renewed`                |
| `onboarding_sent`      | boolean  | `master_data.custom_fields->>'onboarding_sent'` |
| `ob_checkin_sent`      | boolean  | `master_data.custom_fields->>'ob_checkin_sent'` |
| `ob_usage_sent`        | boolean  | `master_data.custom_fields->>'ob_usage_sent'` |
| `checkin_replied`      | boolean  | `master_data.custom_fields->>'checkin_replied'` |
| `nps_replied`          | boolean  | `master_data.custom_fields->>'nps_replied'` |
| `cross_sell_interested`| boolean  | `master_data.custom_fields->>'cross_sell_interested'` |
| `cross_sell_rejected`  | boolean  | `master_data.custom_fields->>'cross_sell_rejected'` |
| `Stage`                | enum     | `master_data.stage` — `LEAD`, `PROSPECT`, `CLIENT`, `DORMANT` |
| `Payment_Status`       | enum     | `master_data.payment_status` — `Lunas`, `Menunggu`, `Belum bayar`, `Terlambat` |
| `sequence_status`      | enum     | `master_data.sequence_status` — `ACTIVE`, `PAUSED`, `NURTURE`, `NURTURE_POOL`, `SNOOZED`, `DORMANT` |
| `Risk_Flag`            | enum     | `master_data.risk_flag` — `High`, `Mid`, `Low`, `None` |
| `Value_Tier`           | enum     | `master_data.value_tier` — `High`, `Mid`, `Low` |
| `isWorkingDay(TODAY())`| function | helper: non-Sat/Sun in the Jakarta calendar |

**Operators (7)**: `=`, `<>`, `>`, `<`, `>=`, `<=`, `BETWEEN ... AND ...`. Legacy seeds also use `IN (...)` and `IS NULL` — keep supporting those for Excel-seeded nodes.

**Combinators**: `AND` (for `condition`) and `OR` (for `stopIf`). Note that the builders enforce this separation — `stopIf` is OR-joined because short-circuit-on-any-stop-condition is the natural semantic ("stop sending if ANY of these is true").

**Type coercion**:
- Booleans compared against `TRUE` / `FALSE` (uppercase, unquoted).
- Enums compared against single-quoted strings: `Stage = 'LEAD'`.
- Numbers compared unquoted.
- `isWorkingDay(TODAY())` resolves to the helper's boolean return.

### evaluateCondition — Condition Parser

```go
// evaluateCondition parses a condition string and evaluates it against a record.
// Supports:
//   field = value
//   field != value
//   field >= value
//   field <= value
//   field BETWEEN low AND high
//   field IN ('a','b','c')
//   field IS NULL / IS NOT NULL
//   AND / OR combinators (AND for condition, OR for stopIf)
func evaluateCondition(record *DataMaster, condition string) bool {
    if condition == "" || condition == "-" {
        return true
    }

    // Split by AND (all must be true)
    parts := strings.Split(condition, "\nAND ")
    for _, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }

        if !evaluateSingleCondition(record, part) {
            return false
        }
    }
    return true
}

func evaluateSingleCondition(record *DataMaster, expr string) bool {
    // Parse field, operator, value from expression
    // Check core fields first, then custom_fields
    //
    // Examples:
    //   "days_since_activation BETWEEN 0 AND 5"
    //   "onboarding_sent = FALSE"
    //   "Bot_Active = TRUE"
    //   "NPS_Score >= 8"
    //   "Days_to_Expiry BETWEEN 85 AND 90"
    //   "Payment_Status = 'Overdue'"
    //   "isWorkingDay(TODAY()) = TRUE"
    //   "workingDaysSince(call_date) >= 3"

    // Special functions
    if strings.Contains(expr, "isWorkingDay") {
        return helpers.IsWorkingDay(time.Now())
    }
    if strings.Contains(expr, "workingDaysSince") {
        // Extract field name, compute working days
        fieldName := extractFunctionArg(expr, "workingDaysSince")
        dateVal := getDateField(record, fieldName)
        if dateVal == nil { return false }
        wd := helpers.WorkingDaysSince(*dateVal)
        return evaluateNumericOp(float64(wd), extractOp(expr), extractNumericVal(expr))
    }

    // Standard field comparison
    fieldName, op, value := parseExpression(expr)
    fieldVal := getFieldValue(record, fieldName)
    return compareValues(fieldVal, op, value)
}
```

---

## Stage Transition Handlers

### SDR -> BD Handoff (LEAD -> PROSPECT)

```go
func handleSDRQualifyHandoff(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    err := repo.TransitionStage(ctx, record.ID, StageProspect, map[string]interface{}{
        "qualified_at":    time.Now().Format(time.RFC3339),
        "qualified_by":    *record.OwnerName,
        "sequence_status": "ACTIVE",
        // Reset SDR-specific flags
        "wa_h0_sent": false, "wa_h1_sent": false, "wa_h3_sent": false,
        "wa_h7_sent": false, "wa_h8_sent": false, "wa_h14_sent": false,
    })
    if err != nil { return err }

    // Schedule BD meeting
    bookBDCalendar(ctx, record)

    // Notify BD via Telegram
    notifyTelegram(ctx, fmt.Sprintf(
        "New prospect from SDR: %s (%s) — HC: %.0f",
        record.CompanyName, record.CompanyID, record.GetCustomFloat("hc_size"),
    ))

    logAction(ctx, record, "SDR_QUALIFY_HANDOFF", "delivered", map[string]interface{}{
        "previous_stage": "LEAD", "new_stage": "PROSPECT",
    })
    return nil
}
```

### BD -> AE Handoff (PROSPECT -> CLIENT)

```go
func handleBDPaymentHandoff(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    // Calculate contract dates
    contractStart := time.Now()
    contractMonths := 12 // default, may come from deal
    contractEnd := contractStart.AddDate(0, contractMonths, 0)
    daysToExpiry := int(contractEnd.Sub(time.Now()).Hours() / 24)

    err := repo.TransitionStage(ctx, record.ID, StageClient, map[string]interface{}{
        "closing_status":  "CLOSED_WON",
        "sequence_status": "ACTIVE",
        "contract_start":  contractStart,
        "contract_end":    contractEnd,
        "contract_months": contractMonths,
        "days_to_expiry":  daysToExpiry,
        // Reset BD-specific flags
        "d0_sent": false, "d2_sent": false, "d4_sent": false,
    })
    if err != nil { return err }

    // Enable bot for AE pipeline
    repo.UpdateCoreField(ctx, record.ID, "bot_active", true)

    // Notify AE via Telegram
    notifyTelegram(ctx, fmt.Sprintf(
        "New client from BD: %s — %s — Price: Rp %s",
        record.CompanyName, record.CompanyID, formatCurrency(record.FinalPrice),
    ))

    logAction(ctx, record, "BD_PAYMENT_HANDOFF", "delivered", map[string]interface{}{
        "previous_stage": "PROSPECT", "new_stage": "CLIENT",
    })
    return nil
}
```

### BD Dormant -> SDR Recycle

```go
func handleBDDormantToSDR(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    err := repo.TransitionStage(ctx, record.ID, StageLead, map[string]interface{}{
        "lead_segment":    "RECYCLED",
        "sequence_status": "ACTIVE",
        // Reset BD flags
        "d0_sent": false, "d2_sent": false, "d4_sent": false,
        "d7_sent": false, "d10_sent": false, "d12_sent": false,
        "d14_sent": false, "d21_sent": false,
        "nurture_d45_sent": false, "nurture_d75_sent": false, "nurture_d90_sent": false,
    })
    if err != nil { return err }

    repo.UpdateCoreField(ctx, record.ID, "bot_active", true)

    logAction(ctx, record, "BD_DORMANT_TO_SDR", "delivered", map[string]interface{}{
        "previous_stage": "PROSPECT", "new_stage": "LEAD", "segment": "RECYCLED",
    })
    return nil
}
```

### AE Renewal Cycle Reset

```go
func handleRenewalCycleReset(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    // Reset ALL AE phase flags for a fresh lifecycle cycle
    resetFields := map[string]interface{}{
        // P0
        "onboarding_sent": false, "ob_checkin_sent": false, "ob_usage_sent": false,
        // P1
        "nps1_sent": false, "nps1_fu_sent": false, "nps_replied": false, "cs_awareness_sent": false,
        // P2
        "warmup_form_sent": false, "warmup_call_sent": false, "checkin_replied": false,
        "cs_socialproof_sent": false, "cs_efficiency_sent": false,
        // P3
        "promo_checkin_form_sent": false, "promo_checkin_call_sent": false,
        "referral_sent": false, "cs_pricing_sent": false,
        // P4
        "ren90_sent": false, "ren_call_sent": false, "ren_form_sent": false,
        "cs_upsell_sent": false, "ren60_sent": false, "ren52_sent": false, "ren45_sent": false,
        // P5
        "ren35_sent": false, "ren15_sent": false,
        "pre14_sent": false, "pre7_sent": false, "pre3_sent": false,
        "ren0_sent": false, "payment_verified": false,
        // P6
        "post1_sent": false, "post4_sent": false, "post8_sent": false,
        // Reset state
        "renewed": false, "invoice_created": false,
    }

    err := repo.MergeCustomFields(ctx, record.ID, resetFields)
    if err != nil { return err }

    // Re-enable bot
    repo.UpdateCoreField(ctx, record.ID, "bot_active", true)
    repo.UpdateCoreField(ctx, record.ID, "renewed", false)

    // Update contract dates for new cycle
    newStart := time.Now()
    newEnd := newStart.AddDate(0, 12, 0) // assume 12-month renewal
    repo.UpdateCoreField(ctx, record.ID, "contract_start", newStart)
    repo.UpdateCoreField(ctx, record.ID, "contract_end", newEnd)

    logAction(ctx, record, "RENEWAL_CYCLE_RESET", "delivered", map[string]interface{}{
        "action": "cycle_reset",
    })
    return nil
}
```

---

### Handoff Data Carry-Through Contract

Defines the **required fields, optional fields, and post-transition side effects** for the four critical stage transitions. The Go handler functions above (`handleSDRQualifyHandoff`, `handleBDPaymentHandoff`, `handleBDDormantToSDR`, and the AE dormant path) implement these contracts; this subsection is the authoritative field-level spec they must satisfy.

**1. SDR → BD (LEAD → PROSPECT)**
Trigger: `lead_qualified = TRUE AND meeting_booked = TRUE`

Required carry-through (must be non-NULL for transition to succeed):
- `Company_Name`, `Company_ID`, `Industry`, `HC_Size`
- `PIC_Name`, `PIC_WA`, `PIC_Email`, `PIC_Role`
- `pain_point_sdr` (free-text from qualification notes)
- `sdr_score`, `verdict_sdr`
- `first_blast_date`

Optional: `cross_sell_interested`, custom fields.

Post-transition side effects:
- Reset SDR sequence flags (`wa_h0_sent`, ..., `wa_h14_sent` = FALSE)
- Set `sequence_status = 'ACTIVE'`, `bd_meeting_date = {calendar.nextAvailable}`
- Record `SDR_QUALIFY_HANDOFF` in action_logs

**2. BD → AE (PROSPECT → CLIENT)**
Trigger: `first_payment_confirmed = TRUE` (via Paper.id webhook)

Required:
- All BD fields preserved
- `Payment_Info` (amount, date, method)
- `Contract_Start`, `Contract_End`, `Final_Price`, `Payment_Terms`
- `buying_intent`, `bants_score`, `bants_classification` (for AE context on how deal closed)

Post-transition side effects:
- Reset BD sequence flags
- Set `sequence_status = 'ACTIVE'`, `renewed = FALSE`
- **Trigger manual action:** `manual_action_queue` insert for `AE_INTRO` flow (see `07-manual-flows.md`)
- Record `BD_PAYMENT_HANDOFF` in action_logs

**3. AE → SDR (CLIENT → DORMANT/RECYCLED)**
Trigger: churned <6 months OR `days_since_last_interaction > 90`

Required:
- All AE fields preserved (for re-engagement context)
- `Stage` set to `'DORMANT'`
- `churn_reason` (free-text, captured from exit conversation)
- Preserve `NPS_Score`, `Usage_Score` history for SDR nurture decision

Post-transition side effects:
- Clear `Owner_Name` (SDR team takes over — or set to SDR lead's email)
- Add to SDR recycle queue with D+90 cooldown before first contact
- Record `AE_DORMANT_TO_SDR` in action_logs

**4. BD Nurture → SDR (PROSPECT → DORMANT, BD_D90 triggered)**
Trigger: `BD_D90` reached + no progress (no reply, no scheduled follow-up)

Required:
- Preserve BD contact attempts (for SDR to see what was tried)
- `Stage` set to `'DORMANT'`
- Reset all BD sent flags

Post-transition side effects:
- Set D+90 cooldown timestamp
- After cooldown, eligible for SDR `NURTURE_RECYCLE` trigger
- Record `BD_DORMANT_TO_SDR` in action_logs

**Go-signature outline** (shared by all four handoff paths):

```go
type HandoffResult struct {
    FromStage string
    ToStage   string
    CarriedFields map[string]interface{}
    TriggeredSideEffects []string
}

func (s *WorkflowEngine) HandleHandoff(ctx context.Context, recordID uuid.UUID, handoffType string) (*HandoffResult, error)
```

---

## Working Day Rules Summary

| Phase / Trigger       | Working Day Required? | Notes                                      |
|-----------------------|-----------------------|--------------------------------------------|
| SDR P1-P2             | YES                   | Mon-Fri, no holidays                       |
| SDR P3 Broadcast      | YES (manual)          | SDR Lead triggers on working day           |
| SDR P4 Nurture        | YES                   | Working days only                          |
| SDR Seasonal NYE      | NO                    | 31 Dec 23:35 WIB regardless               |
| SDR Seasonal Lebaran  | YES                   | H-1 Lebaran must be working day            |
| BD P2 Blast           | YES                   | Mon-Fri, no holidays                       |
| BD P2.5 First Payment | YES                   | Working days only                          |
| BD P3 Nurture         | YES                   | Working days only                          |
| BD P3.5 Overdue       | NO                    | Payment urgency — 7 days/week              |
| AE P0-P4              | YES                   | Mon-Fri, no holidays                       |
| AE P5 Renewal Ops     | YES                   | Working days, but NEVER stops on reply     |
| AE P6 Overdue         | NO                    | Payment urgency — 7 days/week              |
| CS all                 | NO                    | Event-driven, responds any time            |

---

## Daily Maintenance Cron

### Days_to_Expiry Update (daily 00:05 WIB)

```go
// Recalculate days_to_expiry for all CLIENT records
func UpdateDaysToExpiry(ctx context.Context, repo *DataMasterRepo, wsID uuid.UUID) error {
    _, err := repo.db.ExecContext(ctx,
        `UPDATE master_data
         SET days_to_expiry = EXTRACT(DAY FROM (contract_end - NOW()))::INT
         WHERE workspace_id = $1
         AND stage = 'CLIENT'
         AND contract_end IS NOT NULL`,
        wsID)
    return err
}
```

### Payment Status Overdue Check (daily 00:10 WIB)

```go
// Auto-set Payment_Status = 'Overdue' when due_date has passed
func CheckOverduePayments(ctx context.Context, repo *DataMasterRepo, wsID uuid.UUID) error {
    _, err := repo.db.ExecContext(ctx,
        `UPDATE master_data
         SET payment_status = 'Overdue'
         WHERE workspace_id = $1
         AND payment_status IN ('Pending', 'Menunggu')
         AND contract_end < NOW()
         AND stage = 'CLIENT'`,
        wsID)
    return err
}
```
