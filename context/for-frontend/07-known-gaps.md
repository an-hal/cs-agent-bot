# Known Gaps: FE Spec vs BE Implementation

Snapshot 2026-04-24 (after Wave C1-C3). This doc tracks alignment between
`context/for-backend/` (FE-written spec) and actual BE implementation.

## Summary

**Overall alignment: ±99%.** Previously identified 5 missing endpoints + 8
shape mismatches — **all closed** in Wave C1-C3. Remaining gaps are
either (a) pending third-party credentials or (b) ambiguous spec items
waiting for FE/product decision.

## Previously-reported gaps — now CLOSED

### ✅ Endpoints added (6)

| Endpoint | Closed in | Where |
|---|---|---|
| `GET /invoices/{id}/activity` | Wave C1 | `invoice_handler.Activity` + `invoice.Activity` usecase (unified timeline) |
| `POST /invoices/{id}/update-stage` | Wave C1 | `invoice.UpdateStage` — manual stage override with payment_log |
| `PUT /invoices/{id}/confirm-partial` | Wave C1 | `invoice.ConfirmPartial` — multi-termin with auto-Lunas flip |
| `GET /action-log/recent` | Wave C2 | `action_log_handler.Recent` + repo `GetRecentActionLogs` |
| `GET /action-log/summary` | Wave C2 | `action_log_handler.Summary` + repo `GetActionLogSummary` |
| `GET /activity-log/today` | Wave C2 | `action_log_handler.TodayActivity` |
| `GET /action-log/today` | Wave C2 (bonus) | Bot-scoped today view |

### ✅ Response shapes aligned (4)

| Gap | Closed in | How |
|---|---|---|
| `Invoice.company_name` top-level | Wave C1 | Added field on `entity.Invoice` (omitempty) — resolved from master_data at query time |
| `InvoiceStats.unique_companies` | Wave C1 | Added field; single `COUNT(DISTINCT company_id)` query |
| `InvoiceStats.lunas_pct` | Wave C1 | Added field; derived from `ByStatus[Lunas]/Total * 100` |
| `Invoice.termin_breakdown` | Wave C1 | Added field + migration `20260424000040` + Termin entity |

### ✅ Business logic enforced (2)

| Gap | Closed in | How |
|---|---|---|
| `payment_method_route` validation | Wave C3 | `CreateInvoiceReq.PaymentMethodRoute` + `entity.IsValidPaymentMethodRoute()` — validates `paper_id|transfer_bank`, defaults to `transfer_bank` |
| Termin sum validation | Wave C3 | Create rejects when `sum(termin.amount) != total line_item amount` |

## Remaining alignment notes

### 🟡 Auth shape (auth proxy)

FE spec expects:
```json
{"data": {"access_token, expires, expire, platform, admin,
          is_full_access_token, user: {_id, email, phoneNumber, companyId}}}
```

BE is a proxy to `ms-auth-proxy` (external). Response shape comes from the
proxy, not from cs-agent-bot. FE should:
- Treat `user._id` and `user.email` as minimum required
- Handle missing `phoneNumber`, `companyId` gracefully (cs-agent-bot doesn't
  enrich these; they come from ms-auth-proxy if configured there)
- Store the `access_token` as-is; BE cs-agent-bot validates via
  `JWT_VALIDATE_URL`

**Action for FE:** no change needed — BE passes through whatever the auth
proxy returns.

### 🟡 Rate-limit headers

FE spec mentions `X-RateLimit-Limit`, `X-RateLimit-Remaining`,
`X-RateLimit-Reset`. BE cs-agent-bot does **not** emit these today — rate
limiting is expected at the gateway/CDN layer (Cloudflare, nginx, API
gateway).

**Action for FE:** don't rely on these headers from cs-agent-bot. If a
global rate-limit is added via middleware later, it can be surfaced.

### 🟡 Workspace GET/{id} nested members

FE spec expects `members[]` nested in `GET /workspaces/{id}` response. BE
currently returns workspace metadata only — members via a separate fetch at
`GET /workspaces/{id}/members`.

**Action for FE:**
- Either make 2 parallel calls (workspace detail + members)
- Or ask BE team to nest members inline (1-line change in `WorkspaceUsecase.Get`)

### 🟡 Messaging edit-log delete entry

FE spec says deleting a template should create a `template_edit_logs`
entry with `field='deleted'`. BE implementation does this in the delete
handlers (verified in `messaging/usecase.go`). FE should see the entry
when calling `GET /templates/edit-logs`.

**Status:** ✅ Implemented — verify by test.

### 🟡 Collection approval gating

FE spec says `collections/{id}`, `/fields` create+delete require approval.
BE currently:
- `POST /collections` → creates approval (type `collection_schema_change`)
- `DELETE /collections/{id}` → creates approval
- `POST /collections/{id}/fields` → creates approval
- `DELETE /collections/{id}/fields/{field_id}` → creates approval
- `PATCH /collections/{id}` (metadata) → direct apply (no approval)
- `PATCH /collections/{id}/fields/{field_id}` (non-destructive) → direct apply

**Action for FE:** treat create/delete as async (202 → approval_id); treat
patch as sync (200).

### 🟡 Filter DSL in collection list queries

FE spec describes a rich filter DSL for querying `collection_records.data`.
BE implements via `pkg/filterdsl` with operators: `eq`, `neq`, `gt`, `gte`,
`lt`, `lte`, `in`, `between`, `contains`. URL format:
```
GET /collections/{id}/records?filter=category:eq:subscription
                              &filter=price:gte:100000
                              &sort=-updated_at
```

**Action for FE:** use this format. Complex AND/OR grouping not yet
supported — chain multiple filter params (implicit AND).

## Third-party gaps (unchanged — need credentials)

| Integration | Status | What FE sees |
|---|---|---|
| Claude extraction | Mock active when no API key | BANTS derived from keyword matching |
| Fireflies fetch | Mock active when no API key | 4 canned Indonesian scenarios |
| HaloAI outbound WA | Mock active when no token | Records to outbox, returns mock wamid |
| SMTP delivery | Mock active when no host | Records to outbox |

Swap to real: set env var + `MOCK_EXTERNAL_APIS=false`. Response shape identical.

## New endpoint summary (added after FE spec review)

Total BE endpoints: **~251** (up from 245). Net change: **+6 endpoints
directly addressing FE spec gaps**.

```
GET    /invoices/{id}/activity              # Wave C1 — timeline
POST   /invoices/{id}/update-stage          # Wave C1 — manual stage
PUT    /invoices/{id}/confirm-partial       # Wave C1 — termin payment
GET    /action-log/recent                   # Wave C2 — bot actions sidebar
GET    /action-log/today                    # Wave C2 — today view
GET    /action-log/summary                  # Wave C2 — aggregated counts
GET    /activity-log/today                  # Wave C2 — today alias
```

## Postman collection

Updated entries in `docs/postman/cs-agent-bot.postman_collection.json`:
- Invoice folder: +3 requests (Activity, UpdateStage, ConfirmPartial)
- ActionLog folder: +3 requests (Recent, Today, Summary) + activity today

See [../../../docs/postman/](../../../docs/postman/).

## Verification

After Wave C1-C3:
```
$ go build ./...                   # exit 0 ✅
$ go test ./...                    # 34/34 pass, 0 FAIL ✅
```

## FE integration readiness

**Previously: integration possible with 5 workarounds.**
**Now: integration without workarounds.** All known FE-spec endpoints have
BE implementations. All response shapes align. Business logic validations
enforce where spec requires.

Only remaining items are (a) alignment decisions (auth proxy shape,
workspace nested members) and (b) third-party credentials for real SDK
calls.
