# CLAUDE.md — Project Bumi: CS Agent Bot (Production)

## 1. What This Service Does

`cs-agent-bot` is a WhatsApp automation bot for Kantorku.id HRIS SaaS. It acts as a virtual Account Executive — managing the full client lifecycle through automated WhatsApp outreach via HaloAI across six sequential phases:

```
P0   Health & Risk
P0.5 Check-in (health dialogue before renewal)
P1   Renewal Negotiation (stops on reply)
P2   Invoice & Payment  (never stops on reply)
P3   Overdue Recovery   (never stops on reply)
P4   NPS & Referral
P5   Cross-sell — Dealls ATS
```

**The bot never marks a client as Paid, Renewed, or Churned. Only the AE does that via the Dashboard.**

---

## 2. Two Entry Points Only

No third entry point exists. Every system behaviour originates from one of these two triggers.

### Outbound — GCP Cloud Scheduler

```
GCP Cloud Scheduler (09:00 WIB)
  → GET /cron/run  [OIDC-authenticated]
    → processClient() per active client
      → PostgreSQL (read state)
        → HaloAI Send API (WhatsApp delivery)
          → PostgreSQL (write flags + audit)
            → Telegram API (if escalation)
```

### Inbound — HaloAI Webhook

```
Client replies on WhatsApp
  → HaloAI captures reply
    → POST /webhook/wa
      → HTTP 200 immediately (must be ≤5s)
        → goroutine: classify intent → apply rule → update DB
          → HaloAI Send API (reply if needed)
            → Telegram API (AE notification — always)
```

### All Entry Points

| Endpoint | Initiator | Purpose |
|---|---|---|
| `GET /cron/run` | GCP Cloud Scheduler, 09:00 WIB | Proactive outbound message loop |
| `POST /webhook/wa` | HaloAI on client reply | Inbound reply classification and handling |
| `POST /webhook/checkin-form` | Check-in form platform | Sets `checkin_replied=TRUE`, notifies AE |
| `POST /api/handoff/new-client` | BD system on CLOSED_WON | Creates client record, activates bot |
| Dashboard "Mark Resolved" | AE (manual) | Re-activates bot on next cron cycle |

---

## 3. Project Structure

```
cs-agent-bot/
├── cmd/
│   ├── server/main.go            # Entry point: HTTP server, Redis start, cron init
│   └── migrate/main.go           # Migration CLI — run independently of server
├── config/config.go               # Typed config from env. Fails fast on missing keys.
├── docs/                          # Swagger (auto-generated via swaggo)
├── internal/
│   ├── delivery/http/
│   │   ├── deps/                  # DI container — wires usecase, repo, pkg
│   │   ├── health/                # GET /health
│   │   ├── middleware/            # OIDC validation (cron), HaloAI sig verify (webhook), logging
│   │   ├── router/                # HTTP router config
│   │   └── route.go               # Routes: /cron/run, /webhook/wa,
│   │                              #         /webhook/checkin-form, /api/handoff/new-client
│   ├── delivery/response/         # Standardised API response helpers
│   ├── entity/                    # Client, Invoice, ClientFlags, ActionLog, Escalation, SystemConfig
│   ├── migration/                 # Migration runner — executes /migration SQL files in sequence
│   ├── pkg/
│   │   ├── database/postgres.go   # pgx/v5 connection pool
│   │   ├── database/redis.go      # go-redis/v9 connection pool
│   │   └── logger/                # log/slog structured JSON
│   ├── repository/
│   │   ├── client_repo.go         # GetClient(), FindByWA(), SetBotActive(), SentTodayAlready()
│   │   ├── invoice_repo.go        # GetActiveInvoice(), CreateInvoice()
│   │   ├── flags_repo.go          # GetFlags(), UpdateFlags(), ResetCycleFlags()
│   │   ├── conversation_state_repo.go  # GetByCompanyID(), CreateOrUpdate(), RecordMessage()
│   │   ├── log_repo.go            # AppendLog() — INSERT only, no UPDATE/DELETE ever
│   │   ├── escalation_repo.go     # OpenEscalation(), IsResolved(), SetStatus()
│   │   ├── cron_repo.go           # MarkPending(), MarkDone(), MarkError()
│   │   ├── config_repo.go         # GetConfig()
│   │   └── mocks/                 # Repository mocks for testing
│   └── usecase/
│       ├── cron/runner.go         # processClient() — P0→P5 priority loop
│       ├── trigger/health.go      # P0: EvalHealthRisk()
│       ├── trigger/checkin.go     # P0.5: EvalCheckIn() — Branch A + B
│       ├── trigger/negotiation.go # P1: EvalNegotiation()
│       ├── trigger/invoice.go     # P2: EvalInvoice()
│       ├── trigger/overdue.go     # P3: EvalOverdue()
│       ├── trigger/expansion.go   # P4: EvalExpansion()
│       ├── trigger/crosssell.go   # P5: EvalCrossSell()
│       ├── classifier/reply.go    # ClassifyReply() — 9 intent categories
│       ├── template/resolver.go   # ResolveTemplate() — substitution + guard
│       ├── haloai/client.go       # SendWA()
│       ├── telegram/notify.go     # SendMessage(), escalation formatters, 30-min fallback
│       └── escalation/handler.go  # TriggerEscalation()
├── migration/                     # 001_create_clients.sql, 002_create_invoices.sql, ...
├── scripts/                       # Seed system_config, manual cron trigger
├── .env.example
├── .env.docker.example
├── docker-compose.yml             # app + PostgreSQL + Redis
├── Dockerfile
├── go.mod / go.sum
└── README.md
```

---

## 4. Domain Entities

### `entity.Client`

```go
type Client struct {
    // Identity
    CompanyID           string    // PK. e.g. KTK-001. Never changes.
    CompanyName         string
    PICName             string
    PICWA               string    // WA number — HaloAI webhook lookup key. UNIQUE in DB.

    // Ownership
    OwnerName           string
    OwnerWA             *string
    OwnerTelegramID     string    // Primary target for all Telegram escalation alerts.

    // Contract & commercial
    Segment             string    // High | Mid | Low
    ContractMonths      int       // 6|12|24 — determines check-in branch (>=9=A, <9=B)
    ContractStart       time.Time
    ContractEnd         time.Time // DaysToExpiry = ContractEnd - today
    ActivationDate      time.Time // D+N anchor: NPS D+15/45/60, cross-sell D+30, referral D+104
    QuotationLink       string    // Must be set by AE before H-30

    // Payment & status — AE writes, bot reads only
    PaymentStatus       string    // Paid | Pending | Overdue | Partial
    NPSScore            int       // 1–10. <=5=P0 risk+ESC-003. >=8=referral eligible.
    UsageScore          int       // 0–100. <40=P0 risk trigger.
    LastInteractionDate *time.Time

    // Bot control
    BotActive           bool      // Master gate. Checked AFTER Blacklisted.
    Blacklisted         bool      // Checked FIRST — before all other logic. Always.
    ResponseStatus      string    // Pending | Replied — P1 reads this. P2/P3 ignore it.

    // AE-only status
    Renewed             bool      // AE sets. Never bot. Triggers resetCycleFlags().
    Rejected            bool      // AE sets. Never bot.

    // Cross-sell state
    SequenceCS          string    // ACTIVE | LONGTERM | SNOOZED | REJECTED | CONVERTED
    CrossSellRejected   bool
    CrossSellInterested bool      // When TRUE: AE takes over, no more P5 messages.

    // Check-in
    CheckinReplied      bool      // Reset by resetCycleFlags(). Skips REN60+REN45 when TRUE.

    // Payment reminder flags — stored on clients table (per-client, reset on new invoice cycle)
    Pre14Sent  bool
    Pre7Sent   bool
    Pre3Sent   bool
    Post1Sent  bool
    Post4Sent  bool
    Post8Sent  bool
    Post15Sent bool
}
```

### `entity.Invoice`

```go
type Invoice struct {
    InvoiceID     string
    CompanyID     string
    DueDate       time.Time // = ContractEnd. Set at invoice creation.
    Amount        float64
    PaymentStatus string    // Pending | Paid | Overdue | Partial — AE writes only
}
```

### `entity.ClientFlags`

```go
type ClientFlags struct {
    CompanyID string

    // Renewal — reset by resetCycleFlags() when Renewed=TRUE
    Ren60Sent, Ren45Sent, Ren30Sent, Ren15Sent, Ren0Sent bool

    // Check-in Branch A (ContractMonths >= 9)
    CheckinA1FormSent, CheckinA1CallSent bool
    CheckinA2FormSent, CheckinA2CallSent bool

    // Check-in Branch B (ContractMonths < 9)
    CheckinB1FormSent, CheckinB1CallSent bool
    CheckinB2FormSent, CheckinB2CallSent bool
    CheckinReplied bool // Also on Client — both must be set together.

    // NPS + Referral — reset each cycle
    NPS1Sent, NPS2Sent, NPS3Sent bool
    NPSReplied            bool
    ReferralSentThisCycle bool

    // Risk
    LowUsageMsgSent bool
    LowNPSMsgSent   bool

    // Cross-sell 90-day initial sequence — NOT reset on renewal, ever
    CSH7, CSH14, CSH21, CSH30, CSH45, CSH60, CSH75, CSH90 bool

    // Cross-sell long-term rotation — reset after CSLT3 only (angle rotation, not lifecycle)
    CSLT1, CSLT2, CSLT3 bool

    // Feature update tracking — suppresses scheduled P5 this week; reset every Monday
    FeatureUpdateSent bool
}
```

### `entity.ConversationState`

```go
type ConversationState struct {
    CompanyID              string
    CompanyName            *string
    ActiveFlow             *string    // RENEWAL | INVOICE | CHECKIN | CROSS_SELL | NPS
    CurrentStage           *string
    LastMessageType        *string
    LastMessageDate        *time.Time
    ResponseStatus         string     // Pending | Replied
    ResponseClassification *string    // POSITIVE | DELAY | REJECT | ANGRY | PAID | NPS | OOO
    AttemptCount           int
    CooldownUntil          *time.Time // Bot paused until this time
    BotActive              bool       // Per-conversation gate (separate from clients.bot_active)
    ReasonBotPaused        *string
    NextScheduledAction    *string
    NextScheduledDate      *time.Time
    HumanOwnerNotified     bool
}
```

`ShouldSend()` returns `false` when `BotActive=false` OR `CooldownUntil > now`. Both `EvalInvoice` and `EvalOverdue` check this before sending.

---

## 5. The Main Loop — `usecase/cron/runner.go`

```go
func processClient(ctx context.Context, companyID string) error {
    c, _ := db.GetClient(ctx, companyID)

    // Gate 1: Blacklist — ALWAYS FIRST, before everything else. Rule 2.
    if c.Blacklisted { return nil }

    // Gate 2: Bot suspended by escalation or AE manual toggle.
    if !c.BotActive { return nil }

    // Gate 3: Max 1 WA per client per calendar day.
    if db.SentTodayAlready(ctx, companyID) { return nil }

    f, _ := db.GetFlags(ctx, companyID)
    inv, _ := db.GetActiveInvoice(ctx, companyID)

    // Strict priority — first match fires, function returns. No fall-through.
    if sent, _ := trigger.EvalHealthRisk(ctx, c, f);   sent { return nil }
    if sent, _ := trigger.EvalCheckIn(ctx, c, f);      sent { return nil }
    if sent, _ := trigger.EvalNegotiation(ctx, c, f);  sent { return nil }
    if sent, _ := trigger.EvalInvoice(ctx, c, f, inv); sent { return nil }
    if sent, _ := trigger.EvalOverdue(ctx, c, f, inv); sent { return nil }
    if sent, _ := trigger.EvalExpansion(ctx, c, f);    sent { return nil }
    trigger.EvalCrossSell(ctx, c, f)
    return nil
}
```

**Never violate these invariants:**
- `Blacklisted` gate is unconditionally first. Never move it.
- Never reorder the trigger evaluation sequence.
- First trigger that fires causes immediate `return nil` — never fall through to lower priority on the same day.
- 300ms sleep between clients for WhatsApp Business API rate-limit compliance.

---

## 6. Trigger Priority Reference

| Priority | Phase | Activation Condition | Halts on Reply? | Key Constraint |
|---|---|---|---|---|
| **P0** | Health & Risk | `UsageScore < 40` OR `NPSScore <= 5` | No | Not bound to renewal cycle. Any time. |
| **P0.5** | Check-in | DaysToExpiry in check-in windows | Yes — `CheckinReplied=TRUE` | Branch A (>=9mo) or Branch B (<9mo) |
| **P1** | Renewal Negotiation | H-60 to H-45 | Yes — any meaningful reply | Skipped entirely if `CheckinReplied=TRUE` |
| **P2** | Invoice + Payment | H-30 onwards; pre-due at H-14/7/3 | **Never** | Payment chased regardless of any reply |
| **P3** | Overdue Recovery | Past `DueDate`, payment outstanding | **Never** | AE handover at D+15 |
| **P4** | NPS + Referral | D+15/45/60 post-activation; D+104 referral | Yes — `NPSReplied=TRUE` | Referral only if `NPSScore >= 8` + `CSH90Sent=TRUE` |
| **P5** | Cross-sell ATS | D+30 post-activation | Yes — rejected/interested | NPS gate: blocked if `NPSReplied=TRUE AND NPSScore < 8` |

---

## 7. Check-in Branch Logic

```
ContractMonths >= 9  →  Branch A
  H-120 to H-115  form sent          (checkin_a1_form_sent)
  H-113 to H-108  call if no reply   (checkin_a1_call_sent)
  H-90  to H-85   form again         (checkin_a2_form_sent)
  H-83  to H-78   call if no reply   (checkin_a2_call_sent)

ContractMonths < 9   →  Branch B
  H-90  to H-85   form sent          (checkin_b1_form_sent)
  H-83  to H-78   call if no reply   (checkin_b1_call_sent)
  H-60  to H-55   form again         (checkin_b2_form_sent)
  H-53  to H-48   call if no reply   (checkin_b2_call_sent)

When CheckinReplied = TRUE:
  → Skip REN60 and REN45 entirely
  → Client is eligible for REN30 at H-30 directly
```

`CheckinReplied` is set by:
1. Bot — form webhook fires (`POST /webhook/checkin-form`)
2. AE — clicks "Mark Check-in Replied" in Dashboard

---

## 8. Full Message Lifecycle Timeline

| Timing | Phase | Trigger ID | Template | Flag Set | Condition |
|---|---|---|---|---|---|
| H-120 to H-115 | P0.5 | CheckIn_A1_Form | TPL-CHECKIN-FORM | checkin_a1_form_sent | Branch A only |
| H-113 to H-108 | P0.5 | CheckIn_A1_Call | TPL-CHECKIN-CALL | checkin_a1_call_sent | Branch A; form sent, no reply 7d |
| H-90 to H-85 | P0.5 | CheckIn_A2/B1_Form | TPL-CHECKIN-FORM | checkin_a2/b1_form_sent | Both branches |
| H-83 to H-78 | P0.5 | CheckIn_A2/B1_Call | TPL-CHECKIN-CALL | checkin_a2/b1_call_sent | No reply 7d after respective form |
| H-65 to H-60 | P1 | Renewal_REN60 | TPL-REN60 | ren60_sent | Branch A only; CheckinReplied=FALSE |
| H-60 to H-55 | P0.5 | CheckIn_B2_Form | TPL-CHECKIN-FORM | checkin_b2_form_sent | Branch B only |
| H-53 to H-48 | P0.5 | CheckIn_B2_Call | TPL-CHECKIN-CALL | checkin_b2_call_sent | Branch B; no reply 7d |
| H-50 to H-45 | P1 | Renewal_REN45 | TPL-REN45 | ren45_sent | All branches; CheckinReplied=FALSE; PROMO_DEADLINE>today |
| H-44 to H-30 | P2 | Renewal_REN30 | TPL-REN30 | ren30_sent | quotation_link IS NOT NULL; invoice created |
| H-29 to H-15 | P2 | Renewal_REN15 | TPL-REN15 | ren15_sent | payment_status!=Paid AND renewed!=TRUE |
| H-14 to H-9 | P2 | PAY_PRE14 | TPL-PAY-PRE14 | pre14_sent (invoice) | payment_status=Pending |
| H-7 to H-4 | P2 | PAY_PRE7 | TPL-PAY-PRE7 | pre7_sent (invoice) | payment_status=Pending |
| H-3 to H-1 | P2 | PAY_PRE3 | TPL-PAY-PRE3 | pre3_sent (invoice) | payment_status=Pending |
| H-14 to H-0 | P2 | Renewal_REN0 | TPL-REN0 | ren0_sent | payment_status!=Paid; bot suspends; ESC-004 |
| D+1 to D+3 overdue | P3 | Overdue_POST1 | TPL-PAY-POST1 | post1_sent (invoice) | payment_status=Overdue |
| D+4 to D+7 overdue | P3 | Overdue_POST4 | TPL-PAY-POST4 | post4_sent (invoice) | payment_status=Overdue |
| D+8 to D+14 overdue | P3 | Overdue_POST8 | TPL-PAY-POST8 | post8_sent (invoice) | payment_status=Overdue |
| D+15+ overdue | P3 | Overdue_POST15 | Manual (AE sends) | — | Bot stops; ESC-001 raised |
| D+10 to D+15 | P4 | NPS_Survey_1 | TPL-EXP-NPS1 | nps1_sent | nps_replied=FALSE |
| D+30 to D+35 | P5 | CS_H7 | TPL-CS-H7 | cs_h7_sent | NPS gate: nps_replied=FALSE OR nps_score>=8 |
| D+37 to D+42 | P5 | CS_H14 | TPL-CS-H14 | cs_h14_sent | cs_h7_sent=TRUE |
| D+40 to D+45 | P4 | NPS_Survey_2 | TPL-EXP-NPS2 | nps2_sent | nps1_sent=TRUE; nps_replied=FALSE |
| D+44 to D+49 | P5 | CS_H21 | TPL-CS-H21 | cs_h21_sent | cs_h14_sent=TRUE |
| D+52 to D+57 | P5 | CS_H30 | TPL-CS-H30 | cs_h30_sent | cs_h21_sent=TRUE |
| D+55 to D+60 | P4 | NPS_Survey_3 | TPL-EXP-NPS3 | nps3_sent | nps2_sent=TRUE; nps_replied=FALSE |
| D+60 to D+65 | P5 | CS_H45 | TPL-CS-H45 | cs_h45_sent | cs_h30_sent=TRUE |
| D+67 to D+72 | P5 | CS_H60 | TPL-CS-H60 | cs_h60_sent | PROMO_DEADLINE>today |
| D+75 to D+80 | P5 | CS_H75 | TPL-CS-H75 | cs_h75_sent | cs_h60_sent=TRUE |
| D+90 to D+95 | P5 | CS_H90 | TPL-CS-H90 | cs_h90_sent | No decision → sequence_cs=LONGTERM |
| D+104 to D+109 | P4 | Referral_Ask | TPL-EXP-NPS-HIGH-REFERRAL | referral_sent_this_cycle | nps_score>=8; nps_replied=TRUE; cs_h90_sent=TRUE |
| D+95+ every 60d | P5 LT | CS_LT1/LT2/LT3 | TPL-CS-LT1/2/3 | cs_lt1/lt2/lt3_sent | sequence_cs=LONGTERM; 60-day gap |

---

## 9. Payment Flag Rules

**Payment flags are stored on `clients`, read via `entity.Client`.**

```
Flags:     pre14_sent, pre7_sent, pre3_sent, post1_sent, post4_sent, post8_sent, post15_sent
Table:     clients
Scope:     Per company_id
Reset:     Must be reset explicitly when a new invoice cycle begins (e.g. on renewal)
Written:   Bot only via ClientRepo.UpdateInvoiceReminderFlags()
```

> **Note vs FSD Rule 9:** The FSD specifies payment flags should be per `invoice_id` on `invoices`. The current implementation stores them on `clients` for simplicity. This means `resetCycleFlags()` must also reset these flags on renewal to prevent stale state carrying into the next cycle.

P2 and P3 triggers **never check `ResponseStatus`**. Even if the client replied positively, payment collection continues.

---

## 10. Reply Intent Classification

Nine categories. First match wins.

| Intent | Keywords | Bot Action | State Written |
|---|---|---|---|
| `angry` | ancam, lawyer, hukum, lapor, bohong, tipu | Blacklist. ESC-006. No WA reply. | `Blacklisted=TRUE`, `BotActive=FALSE`, ESC-006 |
| `paid_claim` | sudah bayar, lunas, transfer, trf, sdh bayar | Ask for proof. Telegram to AE. Never auto-mark Paid. | `ResponseStatus=Replied`. AE verifies. |
| `nps` | Numeric reply to NPS survey | Record score. <=5 → ESC-003. >=8 → referral gate. | `NPSScore` updated, `NPSReplied=TRUE` |
| `cs_interested` | demo, info, mau, jadwal, tertarik, share, siapkan, coba, boleh *(in response to a P5 message)* | Set `CrossSellInterested=TRUE`. Suspend P5. AE takes over. | `CrossSellInterested=TRUE` |
| `reject` | tidak, cancel, stop, ga perlu, tidak perlu | Stop all automation. `Rejected=TRUE`. | `Rejected=TRUE`, `BotActive=FALSE` |
| `delay` | nanti, bulan depan, next, minggu depan | Snooze 30d (renewal) or set `CrossSellResumeDate`. | Resume date recorded |
| `positive` | mau, oke, bisa, setuju, lanjut, boleh | Continue flow. Log. Telegram to AE. | `ResponseStatus=Replied` |
| `ooo` | out of office, auto reply, sedang tidak | Ignore. Do NOT update `ResponseStatus`. Retry in 3 days. | No state change |
| `wants_human` | Voice note, image, video, ends with `?` | Suspend bot. Telegram to AE. | `BotActive=FALSE` |

Voice notes, images, and videos are always `wants_human` — no exceptions.

### CS Reply Routing — `usecase/classifier/reply.go`

When `SequenceCS` is `ACTIVE` or `LONGTERM`, route to `handleCSReply()`:

```go
func handleIncomingReply(ctx context.Context, p WAPayload) {
    client, _ := repo.FindByWA(ctx, p.FromNumber)
    if client.Blacklisted { return }

    isCSActive := client.SequenceCS == "ACTIVE" || client.SequenceCS == "LONGTERM"
    if isCSActive {
        handleCSReply(ctx, client.CompanyID, p.MessageText)
        return
    }
    intent := classifier.ClassifyReply(p.MessageText)
    applyReplyRules(ctx, client, intent, p.MessageText)
}
```

---

## 11. Template Resolution — `usecase/template/resolver.go`

| Variable | Source | Fallback |
|---|---|---|
| `[Company_Name]` | `clients.company_name` | REQUIRED — no fallback |
| `[PIC_Name]` | `clients.pic_name` | REQUIRED — no fallback |
| `[Owner_Name]` | `clients.owner_name` | REQUIRED |
| `[Owner_WA]` | `clients.owner_wa` | REQUIRED |
| `[Invoice_ID]` | `invoices.invoice_id` | REQUIRED for payment templates |
| `[Due_Date]` | `invoices.due_date` formatted | REQUIRED for payment templates |
| `[link_quotation]` | `clients.quotation_link` | **Abort send if NULL at H-30** |
| `[Next_Discount_%_Manual]` | `clients.next_discount_pct_manual` | — AE sets each cycle |
| `[First_Time_Discount_%]` | `clients.first_time_discount_pct` | — Set at contract start |
| `[link_survey]` | `system_config.SURVEY_PLATFORM_URL + ?cid=` | REQUIRED |
| `[link_checkin_form]` | `system_config.CHECKIN_FORM_URL + ?cid=` | REQUIRED |
| `[months_active]` | `FLOOR((today - contract_start) / 30)` | Omit phrase if < 1 month |
| `[Benefit_Referral]` | `system_config.REFERRAL_BENEFIT` | REQUIRED |
| `[feature_update_text]` | Manual input by product team via Dashboard | REQUIRED for feature blast |

After all substitutions, run the guard:

```go
// Any remaining bracket = unresolved required variable → abort
if strings.Contains(msg, "[") {
    return "", fmt.Errorf("unresolved variable in template %s — send aborted", tmplID)
}
```

---

## 12. Escalation — `usecase/escalation/handler.go`

Every escalation atomically does four things:
1. Sets `BotActive=FALSE` in `clients`
2. Appends `ESCALATED_*` entry to `action_log`
3. Sends Telegram to `OwnerTelegramID`
4. Sets `escalations.status = Open`

**Deduplication:** Before `TriggerEscalation()`, check for existing Open row with same `esc_id` + `company_id`. If found: send Telegram reminder only, do not insert a new row.

**Fallback:** `time.AfterFunc(30 * time.Minute)` re-sends to `system_config.TELEGRAM_AE_LEAD_ID` if owner unacknowledged.

**Re-activation:** Only when AE sets `escalations.status = Resolved` in Dashboard. Detected on next cron cycle.

| ESC ID | Trigger | Notified | Telegram Format |
|---|---|---|---|
| ESC-001 | Overdue >= D+15 | AE + AE Lead | `🔴 ESKALASI PEMBAYARAN *[Company]* \| Invoice: [Invoice_ID] \| Rp [Amount] ([X] hari) \| Bot sudah berhenti.` |
| ESC-002 | Reply = objection/complaint | AE | `🟡 OBJECTION / KOMPLAIN *[Company]* \| [PIC_Name] \| Reply: "[Reply_Excerpt]" \| Bot sudah berhenti.` |
| ESC-003 | NPSScore <= 5 | AE Lead | `🔴 NPS RENDAH *[Company]* \| NPS: [score]/10 \| AE Lead follow up 24 jam.` |
| ESC-004 | REN0 sent, no reply (Mid/High) | AE | `🟠 RENEWAL H-14 TANPA RESPONS *[Company]* \| Kontrak: [Contract_End] \| Bot stop.` |
| ESC-005 | High-value churn (ACV > Rp 20jt) | AE + AE Lead + Founder | `🔴 CHURN HIGH VALUE *[Company]* \| ACV: Rp [ACV] \| Eskalasi ke AE Lead + Founder.` |
| ESC-006 | Angry reply detected | AE + AE Lead | `🚨 DARURAT — KLIEN MARAH *[Company]* \| [PIC_WA] \| Reply: "[Reply_Excerpt]" \| ⛔ JANGAN BALAS APAPUN.` |

---

## 13. resetCycleFlags() — Complete Reference

Called atomically when AE sets `clients.renewed = TRUE`. Run in the same transaction as `contract_end` and `renewal_date` updates.

| Flag Group | Reset to FALSE | NOT Reset | Why |
|---|---|---|---|
| Renewal | `ren60`, `ren45`, `ren30`, `ren15`, `ren0` | — | Full restart each cycle |
| Check-in Branch A | `checkin_a1_form`, `checkin_a1_call`, `checkin_a2_form`, `checkin_a2_call` | — | New cycle = new check-in window |
| Check-in Branch B | `checkin_b1_form`, `checkin_b1_call`, `checkin_b2_form`, `checkin_b2_call` | — | New cycle = new check-in window |
| Check-in shared | `checkin_replied` | — | Client must re-engage in new period |
| NPS | `nps1_sent`, `nps2_sent`, `nps3_sent`, `nps_replied` | — | Re-surveyed each cycle |
| Risk | `low_usage_msg_sent`, `low_nps_msg_sent` | — | Risk interventions restart |
| Referral | `referral_sent_this_cycle` | — | Referral is per-cycle |
| Cross-sell LT | `cs_lt1`, `cs_lt2`, `cs_lt3` (after LT3 only) | **`cs_h7`–`cs_h90` are NEVER reset** | 90-day initial sequence is one-time. LT rotation resets angles, not lifecycle. |
| Permanent | — | `blacklisted`, `cross_sell_rejected`, `cross_sell_interested`, `sequence_cs` | Deliberate client decisions persist across cycles |

---

## 14. Cross-sell Sequence Logic

```
NPS gate:
  nps_replied=TRUE AND nps_score < 8  →  block ALL P5
  nps_replied=FALSE                   →  allow P5 (silence is not a negative signal)

No-overlap rule:
  feature_update_sent=TRUE  →  skip scheduled P5 this week
  Resets every Monday via weekly cron

Snooze:
  sequence_cs=SNOOZED  →  skip until cross_sell_resume_date
  AE sets cross_sell_resume_date in Dashboard

LONGTERM (after cs_h90_sent=TRUE with no decision):
  sequence_cs = LONGTERM
  Every 60 days (days_since_cs_last_sent >= 60):
    CS_LT1 (feature angle)
    CS_LT2 (efficiency angle)
    CS_LT3 (commercial angle)
    After LT3: reset cs_lt1/cs_lt2/cs_lt3=FALSE  →  rotation restarts from LT1

Stop conditions:
  cross_sell_rejected=TRUE       →  stop all P5 permanently
  cross_sell_interested=TRUE     →  stop bot P5, AE handles manually
```

---

## 15. Database Schema

Eight tables. Deploy in order to satisfy FK dependencies.

```sql
-- 1. clients
CREATE TABLE clients (
  company_id               VARCHAR(20)    PRIMARY KEY,
  company_name             VARCHAR(200)   NOT NULL,
  pic_name                 VARCHAR(100)   NOT NULL,
  pic_wa                   VARCHAR(20)    NOT NULL UNIQUE,
  pic_email                VARCHAR(100),
  pic_role                 VARCHAR(100),
  hc_size                  VARCHAR(20),
  owner_name               VARCHAR(100)   NOT NULL,
  owner_wa                 VARCHAR(20),
  owner_telegram_id        VARCHAR(30)    NOT NULL,
  backup_owner_telegram_id VARCHAR(30),
  ae_assigned              BOOLEAN        DEFAULT FALSE,
  ae_telegram_id           VARCHAR(30),
  segment                  VARCHAR(10),
  plan_type                VARCHAR(50),
  payment_terms            VARCHAR(20),
  contract_start           DATE           NOT NULL,
  contract_end             DATE           NOT NULL,
  contract_months          SMALLINT       NOT NULL,
  activation_date          DATE           NOT NULL,
  first_time_discount_pct  DECIMAL(5,2),
  next_discount_pct_manual DECIMAL(5,2),
  final_price              DECIMAL(12,2),
  quotation_link           VARCHAR(500),
  quotation_link_expires   DATE,
  renewal_date             DATE,
  bd_prospect_id           VARCHAR(50),
  notes                    TEXT,
  payment_status           VARCHAR(20)    DEFAULT 'Paid',
  last_payment_date        DATE,
  nps_score                SMALLINT,
  usage_score              SMALLINT,
  usage_score_avg_30d      SMALLINT,
  last_interaction_date    DATE,
  risk_flag                BOOLEAN        DEFAULT FALSE,
  bot_active               BOOLEAN        DEFAULT TRUE,
  blacklisted              BOOLEAN        DEFAULT FALSE,
  wa_undeliverable         BOOLEAN        DEFAULT FALSE,
  response_status          VARCHAR(20)    DEFAULT 'Pending',
  renewed                  BOOLEAN        DEFAULT FALSE,
  rejected                 BOOLEAN        DEFAULT FALSE,
  churn_reason             VARCHAR(200),
  sequence_cs              VARCHAR(20)    DEFAULT 'ACTIVE',
  cross_sell_rejected      BOOLEAN        DEFAULT FALSE,
  cross_sell_interested    BOOLEAN        DEFAULT FALSE,
  cross_sell_resume_date   DATE,
  days_since_cs_last_sent  SMALLINT       DEFAULT 0,
  checkin_replied          BOOLEAN        DEFAULT FALSE,
  -- Payment reminder flags (per-client; reset on new invoice cycle)
  pre14_sent               BOOLEAN        DEFAULT FALSE,
  pre7_sent                BOOLEAN        DEFAULT FALSE,
  pre3_sent                BOOLEAN        DEFAULT FALSE,
  post1_sent               BOOLEAN        DEFAULT FALSE,
  post4_sent               BOOLEAN        DEFAULT FALSE,
  post8_sent               BOOLEAN        DEFAULT FALSE,
  post15_sent              BOOLEAN        DEFAULT FALSE,
  created_at               TIMESTAMP      DEFAULT NOW(),
  CONSTRAINT chk_renewal_date CHECK (renewed = FALSE OR renewal_date IS NOT NULL)
);

-- 2. invoices
CREATE TABLE invoices (
  invoice_id       VARCHAR(30)    PRIMARY KEY,
  company_id       VARCHAR(20)    NOT NULL REFERENCES clients(company_id),
  issue_date       DATE           NOT NULL,
  due_date         DATE           NOT NULL,
  amount           DECIMAL(12,2)  NOT NULL,
  payment_status   VARCHAR(20)    DEFAULT 'Pending',
  created_at       TIMESTAMP      DEFAULT NOW()
);

-- Payment reminder flags are stored on clients, not invoices.
-- See Section 9 for rationale and reset responsibility.

-- 3. client_flags
CREATE TABLE client_flags (
  company_id                VARCHAR(20)  PRIMARY KEY REFERENCES clients(company_id),
  ren60_sent, ren45_sent, ren30_sent, ren15_sent, ren0_sent  BOOLEAN DEFAULT FALSE,
  checkin_a1_form_sent, checkin_a1_call_sent                  BOOLEAN DEFAULT FALSE,
  checkin_a2_form_sent, checkin_a2_call_sent                  BOOLEAN DEFAULT FALSE,
  checkin_b1_form_sent, checkin_b1_call_sent                  BOOLEAN DEFAULT FALSE,
  checkin_b2_form_sent, checkin_b2_call_sent                  BOOLEAN DEFAULT FALSE,
  checkin_replied           BOOLEAN DEFAULT FALSE,
  nps1_sent, nps2_sent, nps3_sent                             BOOLEAN DEFAULT FALSE,
  nps_replied               BOOLEAN DEFAULT FALSE,
  referral_sent_this_cycle  BOOLEAN DEFAULT FALSE,
  quotation_acknowledged    BOOLEAN DEFAULT FALSE,
  low_usage_msg_sent        BOOLEAN DEFAULT FALSE,
  low_nps_msg_sent          BOOLEAN DEFAULT FALSE,
  cs_h7, cs_h14, cs_h21, cs_h30, cs_h45, cs_h60, cs_h75, cs_h90  BOOLEAN DEFAULT FALSE,
  cs_lt1, cs_lt2, cs_lt3   BOOLEAN DEFAULT FALSE,
  feature_update_sent       BOOLEAN DEFAULT FALSE
);

-- 4. conversation_states
CREATE TABLE conversation_states (
  company_id               VARCHAR(20) PRIMARY KEY REFERENCES clients(company_id) ON DELETE CASCADE,
  company_name             VARCHAR(200),
  active_flow              VARCHAR(30),
  current_stage            VARCHAR(30),
  last_message_type        VARCHAR(50),
  last_message_date        TIMESTAMP,
  response_status          VARCHAR(20)  DEFAULT 'Pending',
  response_classification  VARCHAR(30),
  attempt_count            SMALLINT     DEFAULT 0,
  cooldown_until           TIMESTAMP,
  bot_active               BOOLEAN      DEFAULT TRUE,
  reason_bot_paused        VARCHAR(200),
  next_scheduled_action    VARCHAR(50),
  next_scheduled_date      TIMESTAMP,
  human_owner_notified     BOOLEAN      DEFAULT FALSE,
  created_at               TIMESTAMP    DEFAULT NOW(),
  updated_at               TIMESTAMP    DEFAULT NOW()
);
CREATE INDEX idx_conv_state_bot_active ON conversation_states(bot_active);
CREATE INDEX idx_conv_state_response_status ON conversation_states(response_status);

-- 5. escalations
CREATE TABLE escalations (
  id                    SERIAL       PRIMARY KEY,
  esc_id                VARCHAR(10)  NOT NULL,
  company_id            VARCHAR(20)  NOT NULL REFERENCES clients(company_id),
  trigger_condition     TEXT,
  priority              VARCHAR(20),
  notified_party        TEXT,
  telegram_message_sent TEXT,
  status                VARCHAR(20)  DEFAULT 'Open',
  triggered_at          TIMESTAMP    DEFAULT NOW(),
  resolved_at           TIMESTAMP,
  resolved_by           VARCHAR(100),
  notes                 TEXT,
  CONSTRAINT uq_open_escalation UNIQUE (esc_id, company_id, status)
    DEFERRABLE INITIALLY DEFERRED
);

-- 6. action_log — INSERT only
CREATE TABLE action_log (
  id                       BIGSERIAL    PRIMARY KEY,
  company_id               VARCHAR(20)  REFERENCES clients(company_id),
  company_name             VARCHAR(200),
  triggered_at             TIMESTAMP    DEFAULT NOW(),
  trigger_type             VARCHAR(50)  NOT NULL,
  template_id              VARCHAR(50),
  message_id               VARCHAR(100),
  channel                  VARCHAR(10)  DEFAULT 'WA',
  sent_to_wa               VARCHAR(20),
  message_sent             BOOLEAN,
  response_received        BOOLEAN,
  response_classification  VARCHAR(30),
  next_action_triggered    VARCHAR(50),
  status                   VARCHAR(20),
  intent                   VARCHAR(30),
  by_human                 VARCHAR(100),
  log_notes                TEXT
);
CREATE INDEX idx_action_log_message_id ON action_log(message_id);
CREATE INDEX idx_action_log_company_date ON action_log(company_id, (triggered_at::date));
REVOKE UPDATE, DELETE ON action_log FROM cs_agent_app;

-- 8. cron_log
CREATE TABLE cron_log (
  id            SERIAL      PRIMARY KEY,
  run_date      DATE        NOT NULL,
  company_id    VARCHAR(20) NOT NULL REFERENCES clients(company_id),
  status        VARCHAR(20) DEFAULT 'pending',
  processed_at  TIMESTAMP,
  error_message TEXT,
  CONSTRAINT uq_cron_log_run UNIQUE (run_date, company_id)
);

-- 7. system_config
CREATE TABLE system_config (
  key        VARCHAR(100) PRIMARY KEY,
  value      TEXT         NOT NULL,
  description TEXT,
  updated_at TIMESTAMP    DEFAULT NOW(),
  updated_by VARCHAR(100)
);
```

### system_config Required Keys

| Key | Purpose |
|---|---|
| `PROMO_DEADLINE` | Check before REN45 + CS_H60. Skip + alert AE Lead if expired. |
| `PROMO_DISCOUNT_PCT` | Discount % shown in TPL-REN45 |
| `SURVEY_PLATFORM_URL` | Base URL for NPS surveys. Append `?cid=` |
| `CHECKIN_FORM_URL` | Base URL for check-in form. Append `?cid=` |
| `REFERRAL_BENEFIT` | Copy for referral template (e.g. "1 bulan gratis") |
| `LINK_WA_AE` | AE WhatsApp link. Used in TPL-REN0 + urgent escalation templates. |
| `ROI_SUMMARY_TEMPLATE_URL` | ROI summary doc. Used in 24-month TPL-REN30 variant. |
| `QUOTATION_URL` | Fallback quotation URL if `quotation_link` not yet set. |
| `ACV_HIGH_THRESHOLD` | High segment threshold (e.g. 50000000). Used by `deriveSegment()`. |
| `ACV_MID_THRESHOLD` | Mid segment threshold (e.g. 5000000). Used by `deriveSegment()`. |
| `TELEGRAM_AE_LEAD_ID` | 30-min escalation fallback recipient. |
| `SENIOR_AE_TELEGRAM_IDS` | Comma-separated IDs. Used by `assignOwner()` for High segment. |
| `AE_TEAM_TELEGRAM_IDS` | Comma-separated IDs. Used by `assignOwner()` for Mid segment. |
| `ANGRY_KEYWORDS_EXTRA` | Additional angry classifier keywords beyond defaults. |
| `HANDOFF_WEBHOOK_SECRET` | HMAC secret for `POST /api/handoff/new-client`. |
| `SILENCE_THRESHOLD_DAYS` | Inactivity warning threshold (default 30). Reporting only, not a trigger. |

---

## 16. Environment Variables

```env
# HaloAI
HALOAI_API_URL=https://api.haloai.id
WA_API_TOKEN=                         # Bearer token from HaloAI dashboard
WA_WEBHOOK_SECRET=                    # Webhook signature secret from HaloAI

# Telegram
TELEGRAM_BOT_TOKEN=                   # From @BotFather
TELEGRAM_AE_LEAD_ID=                  # Numeric Telegram user ID (NOT @username)

# Database
DATABASE_URL=postgresql://user:pass@host:5432/dbname
REDIS_URL=redis://localhost:6379

# GCP Cloud Scheduler OIDC
APP_URL=https://your-domain.com       # Audience must exactly match APP_URL + "/cron/run"
SCHEDULER_SA_EMAIL=cs-agent-scheduler@your-project.iam.gserviceaccount.com

# BD Handoff
HANDOFF_WEBHOOK_SECRET=               # openssl rand -hex 16

# Business config
PROMO_DEADLINE=2025-03-31             # YYYY-MM-DD. Check before REN45 + CS_H60.
SURVEY_PLATFORM_URL=                  # NPS form base URL
CHECKIN_FORM_URL=                     # Check-in form base URL
REFERRAL_BENEFIT=1 bulan gratis

PORT=8080
```

---

## 17. The 12 Business Rules

| # | Rule | Never Do |
|---|---|---|
| 1 | Trigger & Template = ROOT. Every template_id must exist there. | Send any message not in the Trigger & Template sheet. |
| 2 | `blacklisted` checked before `bot_active`, always first. | Check `bot_active` before `blacklisted` in any code path. |
| 3 | Invoice issued at H-30. `due_date = contract_end`. Payment flags on `invoices` per `invoice_id`. | Store `pre14_sent` or `post*_sent` on `client_flags`. |
| 4 | P1 halts on reply. P2 and P3 never halt on reply. | Check `ResponseStatus` inside any P2 or P3 trigger evaluator. |
| 5 | Check-in branch determined by `contract_months` (>=9=A, <9=B). | Send Branch A messages to a Branch B client. |
| 6 | `CheckinReplied=TRUE` skips REN60 and REN45 entirely. Client goes directly to REN30 at H-30. | Send REN60 or REN45 when `CheckinReplied=TRUE`. |
| 7 | `resetCycleFlags()` on `Renewed=TRUE`. `cs_h*` flags are never reset. | Include `cs_h7`–`cs_h90` in the reset list. |
| 8 | Bot never writes `payment_status`, `renewed`, or `rejected`. AE only via Dashboard. | Set any of these three fields from bot code. |
| 9 | Payment flags are per `invoice_id`, not per client. | Store payment flags on `client_flags`. |
| 10 | `quotation_link` must be non-null before REN30 send. If null: defer + alert AE. | Send REN30 with a null or empty quotation link. |
| 11 | Check `PROMO_DEADLINE` before REN45 and CS_H60. If expired: skip + alert AE Lead. | Send a promotional message with an expired deadline. |
| 12 | Bot activates only on BD CLOSED_WON webhook. Never pursues first payment. | Activate the bot manually or chase initial payment. |

---

## 18. Non-Functional Requirements

| Requirement | Target | Implementation |
|---|---|---|
| Webhook response latency | HTTP 200 to HaloAI within 5 seconds | Return 200 immediately; goroutine handles all processing |
| Cron execution window | All clients processed by 10:00 WIB | 300ms inter-client delay; ~30s for 100 clients |
| WA messages per client per day | Maximum 1 | `SentTodayAlready()` gate before every trigger evaluation |
| Audit log integrity | Permanent, immutable | INSERT only; `REVOKE UPDATE, DELETE` on app DB role |
| Escalation Telegram delivery | < 30 seconds from trigger | Telegram call is synchronous inside `TriggerEscalation()` |
| Escalation fallback | AE Lead notified after 30 min if unacknowledged | `time.AfterFunc(30 * time.Minute)` re-sends to `TELEGRAM_AE_LEAD_ID` |
| Blacklist enforcement | Unconditional — no message ever sent | `Blacklisted` is Gate 1 in `processClient()`, before all else |
| Template send guard | Zero messages with `[variable]` remaining | `strings.Contains(msg, "[")` check; returns error → send aborted |
| Cross-sell no-duplicate | Feature blast and scheduled P5 never same week | `feature_update_sent=TRUE` suppresses P5; resets every Monday |
| Escalation deduplication | No duplicate Open escalation rows per client per type | Check for existing Open row before `TriggerEscalation()`; reminder only if found |
| DB transaction integrity | Flag writes + audit log atomic per client per cycle | All state changes in one transaction per `processClient()` |
| Cron idempotency | Safe to restart mid-run | `cron_log` marks done clients; skip on re-run |

---

## 19. Common Mistakes to Avoid

| Mistake | Why It's Wrong |
|---|---|
| Checking `BotActive` before `Blacklisted` | Rule 2: blacklisted is unconditionally first, no exceptions |
| Checking `ResponseStatus` in P2 or P3 | Payment collection never halts on reply. Rule 4. |
| Writing `payment_status = "Paid"` from bot code | Bot never marks paid. Ask for proof and notify AE. Rule 8. |
| Storing `pre14_sent` / `post1_sent` on `client_flags` | Payment flags must be scoped per `invoice_id`. Rule 9. |
| Sending REN60 or REN45 when `checkin_replied=TRUE` | Rule 6: skip both, advance directly to REN30 at H-30. |
| Sending REN30 when `quotation_link` is NULL | Rule 10: defer + alert AE, never send with a broken link. |
| Sending a promo when `PROMO_DEADLINE` has passed | Rule 11: skip + alert AE Lead. |
| Including `cs_h7`–`cs_h90` in `resetCycleFlags()` | 90-day sequence is one-time. These flags never reset on renewal. Rule 7. |
| Returning HTTP 200 to HaloAI after processing | Must return 200 before any processing — goroutine handles the rest. |
| Using a custom secret header for `/cron/run` auth | GCP Cloud Scheduler uses OIDC JWT. Use `idtoken.Validate()`. |
| Creating a new escalation row when one is already Open | Check first. If Open row exists: Telegram reminder only, no new row. |
| Not calling `resetCycleFlags()` when `Renewed=TRUE` | Client enters new cycle with stale flags and silently skips messages. |
| Calling UPDATE or DELETE on `action_log` | Audit log is append-only. DB role has REVOKE UPDATE, DELETE. |
