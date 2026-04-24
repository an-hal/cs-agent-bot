# Manual Actions (GUARD queue)

20 flows per role (SDR/BD/AE/admin) that require human composition at
relationship-critical moments. The bot detects a trigger and inserts a
queue entry; the human opens the dashboard, edits the suggested draft,
and sends via personal WhatsApp/email/etc.

See `context/for-backend/features/06-workflow-engine/07-manual-flows.md`
for the full flow inventory (20 triggers) + business rationale.

## Endpoints

All require `Authorization: Bearer {jwt}` + `X-Workspace-ID`.

### List queue
```
GET /manual-actions?mine=true&status=pending&limit=20
```
Query params:
- `status` — `pending|in_progress|sent|skipped|expired`
- `assigned_to_user` — filter by assignee email
- `mine=true` — shortcut: filter to caller
- `role` — `sdr|bd|ae|admin`
- `priority` — `P0|P1|P2`
- `flow_category` — one of the 17 categories (see below)
- `limit` (default 50, max 200)
- `offset`

### Get one
```
GET /manual-actions/{id}
```
Returns the full entry including `suggested_draft` and `context_summary`.

### Mark as sent (after human sends)
```
PATCH /manual-actions/{id}/mark-sent
{
  "channel": "wa",                          // wa|email|call|meeting
  "actual_message": "Pak, terkait renewal...",
  "notes": "positive tone"
}
```
Side effects:
- Status → `sent`, `sent_at` populated
- Logs `action_logs` with `sender_type=human`
- Stamps the matching `sent_flag` on `master_data` (if trigger maps to one)

### Skip
```
PATCH /manual-actions/{id}/skip
{"reason": "client already responded via call"}
```
`reason` min 5 chars. No penalty — next trigger in sequence still fires.

## Flow categories (17)

| Category | When fired |
|---|---|
| `bant_qualification` | Bot finishes BANT collection |
| `enterprise_personalisation` | HC ≥ 200 AND no reply 7d |
| `internal_politics_escalation` | BD D10 no progress |
| `final_check_in` | BD D14 silent |
| `bd_ae_handoff` | `first_payment_confirmed` webhook |
| `onboarding_checkin` | D+14 after activation |
| `warmup_call_invite` | P2 form not filled 7d |
| `referral_pitch` | NPS ≥ 8 in P3 window |
| `renewal_opener` | Contract_End H-90 working day |
| `renewal_call_invite` | Renewal opener unanswered H-83 |
| `renewal_followup` | H-60 active renewal negotiation |
| `renewal_decision` | H-45 pre-decision |
| `overdue_empathy` | Invoice D+8 overdue |
| `overdue_final` | Invoice D+15 overdue (pre-suspend) |
| `admin_pricing_edit` | User edits product price (2-person approval) |
| `admin_blacklist_edit` | User adds/removes blacklist (2-person approval) |
| `bd_dm_absent` | DM missed meeting (4 scenarios A/B/C/D) |

Priority tiers:
- **P0** — renewal_opener, overdue_final, renewal_decision, admin_* (critical)
- **P1** — renewal_call_invite, renewal_followup, overdue_empathy, bd_d10/d14
- **P2** — everything else

## FE UX

**Queue UI ideas:**
- Sort by priority then due_at ascending — P0 at top
- Show `suggested_draft` in an editable textarea
- `context_summary` as a sidebar panel (client name, NPS, last interaction, etc.)
- After sending in personal WA, user clicks "Mark sent" with channel + actual message
- Skipped items hide by default but show a count bubble

**Telegram alert integration:**
BE also sends a Telegram DM to the assignee when an entry is queued (when
Telegram integration is configured). FE can trust this or poll the queue.

**Auto-expiry:**
Pending entries older than 48h past `due_at` get auto-expired by a cron.
Escalation Telegram goes to the manager. Currently wired via
`usecase.ExpirePastDue(ctx, 48*time.Hour)`.
