# Plan — feat/07-invoices

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000600`–`000699` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/07-invoices/`

## Scope

Invoice CRUD, line items, Paper.id integration (create + webhook), 5-stage collection lifecycle, payment logs, sync to master_data, reminder system. Reuses existing `internal/usecase/payment/` verifier and `clients` payment flag semantics.

**Read first**: `01-overview.md`, `02-database-schema.md`, `03-api-endpoints.md`, `04-progress.md`, `00-shared/04-integrations.md` (Paper.id), `00-shared/05-checker-maker.md`.

> **Existing repo has an `invoices` table** (used by the AE P2 invoice issuance at H-30) and `usecase/payment.PaymentVerifier`. CLAUDE.md rule: **bot never writes `payment_status`, `renewed`, or `rejected`** — AE only via Dashboard. Paper.id webhook is the **one exception** because it's an external system event, not the bot. Document this clearly in webhook handler.

## Migrations

| # | File | Purpose |
|---|---|---|
| 600 | `extend_invoices_for_billing.{up,down}.sql` | ALTER existing `invoices` to add: `payment_terms INT DEFAULT 30`, `payment_method`, `amount_paid BIGINT`, `days_overdue INT`, `collection_stage VARCHAR(30)`, `reminder_count INT`, `last_reminder_date`, `paper_id_url`, `paper_id_ref`, `created_by`. Add indexes per spec. |
| 601 | `create_invoice_line_items.{up,down}.sql` | Per spec §2. CASCADE on invoice delete. `subtotal` computed in app, not generated column. |
| 602 | `create_payment_logs.{up,down}.sql` | Per spec §3. **Append-only**: `REVOKE UPDATE, DELETE`. Indexes per spec. |
| 603 | `create_invoice_sequences.{up,down}.sql` | Per spec §4. Composite PK (workspace_id, year). Used by atomic ID generation. |
| 604 | `update_invoice_status_check.{up,down}.sql` | Add CHECK constraint for `payment_status IN ('Lunas','Menunggu','Belum bayar','Terlambat')` and `collection_stage IN (...)` per spec |

## Entities

`internal/entity/invoice.go` (extend existing):
```go
type Invoice struct {
    ID                  string  // INV-DE-2026-001
    WorkspaceID         uuid.UUID
    CompanyID           string  // logical FK to clients/master_data
    Amount              int64   // IDR, no cents
    IssueDate           *time.Time
    DueDate             *time.Time
    PaymentTerms        int     // net days
    Notes               string
    PaymentStatus       string  // Lunas|Menunggu|Belum bayar|Terlambat
    PaymentDate         *time.Time
    PaymentMethod       string
    AmountPaid          int64
    DaysOverdue         int
    CollectionStage     string  // Stage 0..4
    ReminderCount       int
    LastReminderDate    *time.Time
    PaperIDURL          string
    PaperIDRef          string
    CreatedBy           string
    CreatedAt           time.Time
    UpdatedAt           time.Time
}

type InvoiceLineItem struct {
    ID          uuid.UUID
    InvoiceID   string
    WorkspaceID uuid.UUID
    Description string
    Qty         int
    UnitPrice   int64
    Subtotal    int64
    SortOrder   int
}

type PaymentLog struct {
    ID            uuid.UUID
    WorkspaceID   uuid.UUID
    InvoiceID     string
    EventType     string
    AmountPaid    *int64
    PaymentMethod string
    PaymentChannel string
    PaymentRef    string
    OldStatus     string
    NewStatus     string
    OldStage      string
    NewStage      string
    Actor         string  // user email | "system" | "paper_id_webhook"
    Notes         string
    RawPayload    map[string]any
    Timestamp     time.Time
}
```

## Repositories

```
internal/repository/
  invoice_repo.go              // List(ws, filter), Get, Create, Update, ListByCompany, ListOverdue, Stats
  invoice_line_item_repo.go    // CRUD scoped to invoice; BulkUpsert (replace all)
  payment_log_repo.go          // Append-only
  invoice_sequence_repo.go     // NextSeq(ws, year) — atomic
```

## Usecases

`internal/usecase/invoice/usecase.go`:
- `List(ctx, ws, filter, pag)`, `Get`, `GetByCompany`
- `Create(ctx, req)` — atomic: NextSeq → format ID per workspace prefix → INSERT invoice + line items in txn → optionally call Paper.id `Create()` to get URL → store. **checker-maker**: creates `create_invoice` approval first.
- `Update(ctx, id, partial)` — partial update. **Forbids manual `payment_status` change** unless caller has admin permission (cross-checks with feat/04 RBAC: `invoices.edit` action).
- `MarkPaid(ctx, id, manualReq)` — **checker-maker**: creates `mark_invoice_paid` approval. On approval: txn updates invoice + writes `payment_log` (event=`manual_mark_paid`) + syncs `clients.payment_status='Paid'` + `last_payment_date`.
- `SendReminder(ctx, id)` — increments `reminder_count`, sets `last_reminder_date`, calls feat/05 `messaging_send.SendWA` with `TPL-PAY-PRE14` or appropriate template, writes payment_log
- `Stats(ctx, ws)` — 4 stat cards per spec
- `BySite()` — 5 collection stage breakdowns

`internal/usecase/invoice/cron.go`:
- `UpdateOverdueStatuses(ctx)` — runs daily 00:05 WIB. SQL per spec § Cron Job 1. Writes payment_log per row.
- `AutoEscalateCollectionStages(ctx)` — runs daily 00:10 WIB. SQL per spec § Cron Job 2. Writes payment_log per row.
- `SyncToMasterData(ctx, ws, companyID)` — called after every invoice status change. SQL per spec § Cron Job 3.
- Hook into existing `cron/runner.go` if useful; or run via separate scheduler endpoint.

`internal/usecase/invoice/paperid.go`:
- `Create(ctx, ws, invoice) (paperIDURL, paperIDRef string, err error)` — calls Paper.id API per workspace_integrations creds. Encrypted key decrypted at call time.
- `Test(ctx, ws)` — health-check endpoint (used by feat/02 integrations test)
- `HandleWebhook(ctx, ws, payload)` — verifies HMAC, matches `external_id` to invoice ID, updates status to `Lunas` + payment_date + payment_method, writes payment_log (event=`paper_id_webhook`, raw_payload), syncs master_data, fires notification (feat/02).

## HTTP routes

```go
inv := api.Group("/invoices")
inv.Handle(GET,    "",                          wsRequired(jwtAuth(invH.List)))
inv.Handle(GET,    "/{id}",                     wsRequired(jwtAuth(invH.Get)))
inv.Handle(POST,   "",                          wsRequired(jwtAuth(invH.Create)))   // checker-maker
inv.Handle(PUT,    "/{id}",                     wsRequired(jwtAuth(invH.Update)))
inv.Handle(DELETE, "/{id}",                     wsRequired(jwtAuth(invH.Delete)))
inv.Handle(POST,   "/{id}/mark-paid",           wsRequired(jwtAuth(invH.MarkPaid)))   // checker-maker
inv.Handle(POST,   "/{id}/send-reminder",       wsRequired(jwtAuth(invH.SendReminder)))
inv.Handle(GET,    "/{id}/payment-logs",        wsRequired(jwtAuth(invH.PaymentLogs)))
inv.Handle(GET,    "/stats",                    wsRequired(jwtAuth(invH.Stats)))
inv.Handle(GET,    "/by-stage",                 wsRequired(jwtAuth(invH.ByStage)))

// Paper.id webhook — per workspace, HMAC-protected
api.Handle(POST,   "/webhook/paperid/{workspace_id}", paperidH.Handle)  // no JWT, HMAC verify

// Cron triggers (called by Cloud Scheduler with OIDC, like existing /cron/run)
api.Handle(GET,    "/cron/invoices/overdue",     oidcAuth(invCronH.UpdateOverdue))
api.Handle(GET,    "/cron/invoices/escalate",    oidcAuth(invCronH.EscalateStages))
```

## Tests

- `usecase/invoice/usecase_test.go` — atomic ID generation, line item txn rollback, MarkPaid approval gate, sync to master_data
- `usecase/invoice/cron_test.go` — overdue marker sets `Terlambat` only on past-due rows; escalation respects current stage (no skipping)
- `usecase/invoice/paperid_test.go` — webhook signature verify, payload-to-status mapping, payment_log raw_payload preserved
- Integration test: full flow Create → Paper.id stub → webhook → status change → master_data sync

## Risks / business-rule conflicts with CLAUDE.md

- **CRITICAL**: rule 3 — bot never writes `payment_status`. Paper.id webhook is the **only** exception (external system event). All other paths must require either:
  - Dashboard JWT user with explicit RBAC permission, OR
  - Approved checker-maker request
- **Existing payment flags on `clients`** (`pre14_sent`, `post*_sent`, etc.) belong to the AE message cron loop (P2/P3 in `cron/runner.go`). This feature does NOT touch those flags directly — only the AE cron does. Reminder count + last_reminder_date are NEW columns on `invoices`, not on `clients`.
- **Existing `usecase/payment/PaymentVerifier`** handles inbound payment proof. Its responsibility: read OCR/manual proof → flag for AE review. Keep it separate from this feature; the new `MarkPaid` flow goes through approval.
- **`due_date = contract_end`** rule (CLAUDE.md rule 4). When invoice is auto-issued at H-30 by the AE cron, due_date must equal `clients.contract_end`. Reuse existing logic; this feature only handles dashboard-initiated invoices.
- **Reset-on-new-cycle**: existing `pre14_sent`/`post*_sent` flags reset when a new invoice cycle starts. Make sure manual invoice creation calls the same reset helper.

## File checklist

- [ ] migrations 600–604
- [ ] entities (invoice, line_item, payment_log)
- [ ] repos + mocks (4)
- [ ] usecases: invoice (CRUD/stats/reminder), invoice/cron (overdue/escalate/sync), invoice/paperid (create/webhook/test)
- [ ] handlers: invoice_handler, paperid_webhook_handler, invoice_cron_handler
- [ ] route.go + deps.go + main.go wiring
- [ ] swag regen
- [ ] `make lint && make unit-test` green
- [ ] commit + push `feat/07-invoices`
