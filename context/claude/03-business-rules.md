# Critical Business Rules

These rules are load-bearing and must not be relaxed without explicit product-level decision.

| # | Rule |
|---|---|
| 1 | `blacklisted` checked before `bot_active`, always first. Never reverse this order. |
| 2 | P1 halts on reply. **P2 and P3 never halt on reply** — payment collection continues regardless. |
| 3 | Bot never writes `payment_status`, `renewed`, or `rejected`. AE only via Dashboard. |
| 4 | Invoice issued at H-30. `due_date = contract_end`. Payment flags (`pre14_sent`, `post*_sent`) live on `clients`, not `client_flags`. Reset on new invoice cycle. |
| 5 | Check-in branch: `contract_months >= 9` → Branch A, `< 9` → Branch B. |
| 6 | `CheckinReplied=TRUE` skips REN60 and REN45 entirely; client goes directly to REN30 at H-30. |
| 7 | `resetCycleFlags()` on `Renewed=TRUE`. `cs_h7`–`cs_h90` flags are **never** reset (90-day sequence is one-time). |
| 8 | `quotation_link` must be non-null before REN30. If null: defer + alert AE. |
| 9 | Check `PROMO_DEADLINE` before REN45 and CS_H60. If expired: skip + alert AE Lead. |
| 10 | `action_log` is INSERT only. `REVOKE UPDATE, DELETE` is granted at DB level. |
| 11 | Return HTTP 200 to HaloAI webhook **before** any processing. All logic runs in a goroutine. |
| 12 | Template send guard: any remaining `[variable]` in the resolved message → abort send. |

## Trigger Priority Loop (`usecase/cron/runner.go`)

```go
// Gate 1: Blacklist — ALWAYS FIRST
if c.Blacklisted { return nil }
// Gate 2: Bot suspended
if !c.BotActive { return nil }
// Gate 3: Max 1 WA per client per calendar day
if db.SentTodayAlready(ctx, companyID) { return nil }

// Strict priority — first match fires, returns immediately.
EvalHealthRisk → EvalCheckIn → EvalNegotiation → EvalInvoice → EvalOverdue → EvalExpansion → EvalCrossSell
```

**Never reorder these triggers.** 300ms sleep between clients (WA Business API rate-limit).

## Trigger Priority Reference

| Priority | Phase | Trigger Hook | Halts on Reply? |
|---|---|---|---|
| P0 | Health & Risk | `trigger/health.go` — low usage/NPS, risk flags | No |
| P0.5 | Check-in | `trigger/checkin.go` — 90d / 180d + A/B branches | Yes (`CheckinReplied=TRUE`) |
| P1 | Renewal Negotiation | `trigger/negotiation.go` — quotation H-7, H-3, H-0 | Yes (any meaningful reply) |
| P2 | Invoice + Payment | `trigger/invoice.go` — pre-14, pre-7, pre-3 days | **Never** |
| P3 | Overdue Recovery | `trigger/overdue.go` — post-1, post-4, post-8 days | **Never** |
| P4 | NPS + Expansion | `trigger/expansion.go` — 90d / 180d / 270d milestones | Yes (`NPSReplied=TRUE`) |
| P5 | Cross-sell ATS | `trigger/crosssell.go` — HT: 7–90d, LT: 30–90d | Yes (rejected/interested) |

## Reply Intent Classification (`usecase/classifier/reply.go`)

Nine categories, first match wins:
`angry`, `paid_claim`, `nps`, `cs_interested`, `wants_human`, `reject`, `delay`, `positive`, `ooo`.

- Voice notes, images, videos → always `wants_human`.
- When `SequenceCS == "ACTIVE"` or `"LONGTERM"`, replies are routed to `handleCSReply()` before general classification.

## Escalation (`usecase/escalation/handler.go`)

Every escalation atomically:
1. Set `BotActive = FALSE`
2. Append to `action_log`
3. Send Telegram
4. Set `escalations.status = Open`

**Deduplication:** if an Open row exists for the same `(esc_id, company_id)`, only send a Telegram reminder (no new row). Re-activation only when AE sets `status = Resolved` in Dashboard. 30-minute fallback re-sends to `TELEGRAM_AE_LEAD_ID`.

| ESC ID | Trigger |
|---|---|
| ESC-001 | Overdue ≥ D+15 |
| ESC-002 | Objection/complaint reply |
| ESC-003 | NPSScore ≤ 5 |
| ESC-004 | REN0 sent, no reply (Mid/High value) |
| ESC-005 | High-value churn (ACV > threshold) |
| ESC-006 | Angry reply |
