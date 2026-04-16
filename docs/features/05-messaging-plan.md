# Plan — feat/05-messaging

> **Branch base**: `master` &nbsp;&nbsp;|&nbsp;&nbsp; **Migration range**: `20260414000400`–`000499` &nbsp;&nbsp;|&nbsp;&nbsp; **Spec dir**: `~/dealls/project-bumi-dashboard/context/for-backend/features/05-messaging/`

## Scope

Per-workspace template management for WA, Telegram (plaintext + emoji), and Email (TipTap HTML). Variable system, edit-log audit, sanitization, render+send pipeline. Reuses existing `internal/usecase/haloai`, `internal/usecase/telegram`, and template resolver.

**Read first**: `01-overview.md`, `02-database-schema.md`, `03-api-endpoints.md`, `04-progress.md`, `00-shared/03-html-sanitization.md`, `00-shared/04-integrations.md`.

> **Existing repo has `templates` table + `usecase/template/resolver.go`** which handles `[variable]` substitution + the unresolved-variable guard. **Reuse the resolver.** This feature extends the schema and adds CRUD/audit, not a new render engine.

## Migrations

| # | File | Purpose |
|---|---|---|
| 400 | `extend_templates_to_message_templates.{up,down}.sql` | If existing `templates` table fits, ALTER to add: `trigger_id`, `phase`, `phase_label`, `channel`, `role`, `category`, `action`, `timing`, `condition`, `variables TEXT[]`, `stop_if`, `sent_flag`, `priority`, `updated_by`. Otherwise create `message_templates` per spec §1 and migrate existing rows. UNIQUE(ws, trigger_id, channel). |
| 401 | `create_email_templates.{up,down}.sql` | Per spec §2. PK = `id VARCHAR(50)`, UNIQUE(ws, id). `body_html TEXT`, `status` enum, `subject VARCHAR(500)`. |
| 402 | `create_template_variables.{up,down}.sql` | Per spec §3. UNIQUE(ws, variable_key). `source_type` enum-like check. |
| 403 | `create_template_edit_logs.{up,down}.sql` | Per spec §4. `template_type IN ('message','email')`. INSERT-only — `REVOKE UPDATE, DELETE`. Indexes per spec. |
| 404 | `seed_template_variables.{up,down}.sql` | Seed common variables per workspace (Company_Name, PIC_Name, link_*, computed). Pull from spec § Variable System table. |

## Entities

`internal/entity/message_template.go`:
```go
type MessageTemplate struct {
    ID          string  // human-readable, e.g. TPL-OB-WELCOME
    WorkspaceID uuid.UUID
    TriggerID   string
    Phase       string  // P0..P6, HANDOFF, DORMANT
    PhaseLabel  string
    Channel     string  // whatsapp | telegram
    Role        string  // sdr | bd | ae
    Category    string  // onboarding|assessment|...|escalation
    Action      string
    Timing      string
    Condition   string
    Message     string
    Variables   []string
    StopIf      string
    SentFlag    string
    Priority    string
    UpdatedAt   time.Time
    UpdatedBy   string
    CreatedAt   time.Time
}

type EmailTemplate struct {
    ID          string  // ETPL-*
    WorkspaceID uuid.UUID
    Name        string
    Role        string
    Category    string
    Status      string  // active|draft|archived
    Subject     string
    BodyHTML    string  // sanitized
    Variables   []string
    UpdatedAt   time.Time
    UpdatedBy   string
}

type TemplateVariable struct {
    ID           uuid.UUID
    WorkspaceID  uuid.UUID
    VariableKey  string
    DisplayLabel string
    SourceType   string  // master_data_core | master_data_custom | invoice | computed | workspace_config | generated
    SourceField  string
    Description  string
    ExampleValue string
}

type TemplateEditLog struct {
    ID           uuid.UUID
    WorkspaceID  uuid.UUID
    TemplateID   string
    TemplateType string  // message | email
    Field        string
    OldValue     *string
    NewValue     *string
    EditedBy     string
    EditedAt     time.Time
}
```

## Repositories

```
internal/repository/
  message_template_repo.go   // List(ws,filter), Get(ws,id), Create, Update (returns diff), Delete, ExistsByID
  email_template_repo.go     // List(ws,filter), Get, Create, Update, Delete, ListActive
  template_variable_repo.go  // List(ws), Upsert, Delete
  template_edit_log_repo.go  // Append (INSERT-only), List(template_id, limit)
```

## Usecases

`internal/usecase/message_template/usecase.go`:
- CRUD with diff-based edit logs. On `Update`, compare each field; for every changed field, append an edit log row inside the same txn.
- `Delete` writes edit log with `field='deleted'`, `old_value=<json>`, `new_value=null`.

`internal/usecase/email_template/usecase.go`:
- CRUD. On Create/Update: **sanitize `body_html` via bluemonday** with the allowlist in `00-shared/03`. Reject if sanitized output drops critical content (length delta > 50% triggers a warning log but still saves sanitized version).
- Variable extraction: regex `/\[[A-Za-z_][A-Za-z0-9_]*\]/g` to populate `variables` automatically when caller doesn't supply it.

`internal/usecase/template_variable/usecase.go`:
- CRUD + `ListByWorkspace` for the variable picker UI.

`internal/usecase/template_render/usecase.go`:
- Wraps existing `usecase/template/resolver.go`. Adds:
  - `RenderMessage(ctx, templateID, masterDataID) (string, error)` — fetches template + master_data + invoice (if needed), substitutes, **aborts if any unresolved `[variable]` remains** (CLAUDE.md rule 12)
  - `RenderEmail(ctx, templateID, masterDataID) (subject, html string, err error)` — same guard

`internal/usecase/messaging_send/usecase.go`:
- `SendWA(ctx, wsID, masterDataID, templateID)` — render, then call existing `usecase/haloai.SendWA`. Records `action_logs` (workflow trace from feat/03) with `channel='whatsapp'`, `template_id`, `phase`.
- `SendEmail(ctx, wsID, masterDataID, templateID)` — render, then call SMTP service (via `pkg/smtp` — new lightweight wrapper around `net/smtp` using global SMTP env from feat/02). `from`/`reply_to` come from `workspace_integrations`.
- `SendTelegram(ctx, wsID, chatID, templateID, vars)` — render, call existing `usecase/telegram.SendMessage`. For escalation alerts.

## HTTP routes

```go
tpl := api.Group("/templates")
tpl.Handle(GET,    "/messages",                wsRequired(jwtAuth(msgTplH.List)))
tpl.Handle(GET,    "/messages/{id}",           wsRequired(jwtAuth(msgTplH.Get)))
tpl.Handle(POST,   "/messages",                wsRequired(jwtAuth(msgTplH.Create)))
tpl.Handle(PUT,    "/messages/{id}",           wsRequired(jwtAuth(msgTplH.Update)))
tpl.Handle(DELETE, "/messages/{id}",           wsRequired(jwtAuth(msgTplH.Delete)))
tpl.Handle(GET,    "/messages/{id}/history",   wsRequired(jwtAuth(msgTplH.History)))

tpl.Handle(GET,    "/emails",                  wsRequired(jwtAuth(emailTplH.List)))
tpl.Handle(GET,    "/emails/{id}",             wsRequired(jwtAuth(emailTplH.Get)))
tpl.Handle(POST,   "/emails",                  wsRequired(jwtAuth(emailTplH.Create)))
tpl.Handle(PUT,    "/emails/{id}",             wsRequired(jwtAuth(emailTplH.Update)))
tpl.Handle(DELETE, "/emails/{id}",             wsRequired(jwtAuth(emailTplH.Delete)))
tpl.Handle(GET,    "/emails/{id}/history",     wsRequired(jwtAuth(emailTplH.History)))
tpl.Handle(POST,   "/emails/{id}/preview",     wsRequired(jwtAuth(emailTplH.Preview)))   // render with sample data

tpl.Handle(GET,    "/variables",               wsRequired(jwtAuth(varH.List)))
tpl.Handle(POST,   "/variables",               wsRequired(jwtAuth(varH.Create)))
tpl.Handle(PUT,    "/variables/{id}",          wsRequired(jwtAuth(varH.Update)))
tpl.Handle(DELETE, "/variables/{id}",          wsRequired(jwtAuth(varH.Delete)))

api.Handle(POST,   "/messaging/send/wa",       wsRequired(jwtAuth(sendH.WA)))             // manual trigger
api.Handle(POST,   "/messaging/send/email",    wsRequired(jwtAuth(sendH.Email)))
api.Handle(POST,   "/messaging/send/telegram", wsRequired(jwtAuth(sendH.Telegram)))
```

## Tests

- `usecase/message_template/usecase_test.go` — diff-based edit log on partial update; create with duplicate ID returns Conflict
- `usecase/email_template/usecase_test.go` — bluemonday strips `<script>`, preserves `[variable]` placeholders, variable auto-extraction
- `usecase/template_render/usecase_test.go` — unresolved `[variable]` aborts send (must match existing resolver behavior)
- `usecase/messaging_send/usecase_test.go` — error path: render fail blocks send; success path writes action_logs row
- `pkg/smtp/smtp_test.go` — mock SMTP server (or use `net/smtp` test helper); from-address per workspace

## Risks / business-rule conflicts with CLAUDE.md

- **Resolver guard**: rule 12 — abort send on any unresolved `[variable]`. Existing resolver already does this; messaging_send must propagate the error, not swallow it.
- **Webhook 200-first**: rule 11 — all replies handled async after returning 200. Messaging features are outbound only; inbound is feat/02's webhook handler. No conflict.
- **Body HTML sanitization** is mandatory at write time. Frontend already sanitizes but **never trust client input** — the bluemonday pass on the backend is the source of truth.
- **Template ID is human-readable PK** (not UUID). Make sure repo `Get` accepts string, not uuid.UUID.
- **Edit log is INSERT-only** like `action_log`. Apply `REVOKE UPDATE, DELETE` in migration 403.

## File checklist

- [ ] migrations 400–404
- [ ] entities (message_template, email_template, template_variable, template_edit_log)
- [ ] repos + mocks (4)
- [ ] usecases: message_template, email_template, template_variable, template_render (wraps existing resolver), messaging_send
- [ ] pkg/smtp (new lightweight wrapper) + tests
- [ ] handlers: msg_template, email_template, variable, send (4 files)
- [ ] route.go + deps.go + main.go wiring
- [ ] swag regen
- [ ] `make lint && make unit-test` green
- [ ] commit + push `feat/05-messaging`
