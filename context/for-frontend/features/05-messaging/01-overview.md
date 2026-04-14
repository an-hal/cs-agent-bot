# Messaging Templates — Overview

The Messaging feature (feat/05-messaging) provides workspace-scoped CRUD
for **message templates** (WhatsApp + Telegram) and **email templates**
(HTML), plus template preview, edit-log auditing, and a catalog of
available substitution variables.

All endpoints live under the `/templates` prefix in
`internal/delivery/http/route.go` and require:

- `Authorization: Bearer {jwt}`
- `X-Workspace-ID: {uuid}`

---

## Data Domains

```
message_templates     ── WhatsApp + Telegram templates (plaintext + emoji)
email_templates       ── HTML email templates (subject + body_html)
template_variables    ── Variable catalog per workspace (autocomplete)
template_edit_logs    ── INSERT-only audit trail of every field change
```

Each row is keyed on the composite `(workspace_id, id)` — the `id` is a
human-readable string like `TPL-OB-WELCOME` or `ETPL-DE-SDR-001`, not
a UUID. This makes workflow-node references and log entries easy to
read.

---

## Variable Placeholders

Templates use `[Variable_Name]` placeholders. Valid syntax:

```
[A-Za-z_][A-Za-z0-9_]*
```

Examples: `[Company_Name]`, `[PIC_Name]`, `[link_wiki]`, `[Due_Date]`.

The backend automatically extracts placeholders from `message`,
`subject`, and `body_html` when the client does not supply a
`variables` array. The frontend can also fetch the canonical variable
catalog via `GET /templates/variables` and render an autocomplete
picker.

---

## HTML Sanitization

`body_html` on email templates is passed through a bluemonday allowlist
on every create and update. Dangerous content (scripts, inline event
handlers, `javascript:` URLs) is stripped server-side even if the
frontend already sanitized it. `[Variable_Name]` placeholders are
preserved verbatim.

---

## Edit-Log Audit Trail

Every field change on a message or email template appends a row to
`template_edit_logs`, including:

- `template_id`, `template_type` (`message` | `email`)
- `field` (e.g. `message`, `body_html`, `status`), or `created`/`deleted`
- `old_value` (nullable) and `new_value` (nullable)
- `edited_by` (editor email from the JWT)
- `edited_at` (timestamp)

Use `GET /templates/edit-logs` for the workspace feed, or
`GET /templates/edit-logs/{template_id}` for a single template's
history drawer.

---

## Preview vs Render

- `POST /templates/preview` — pass `sample_data` and the backend
  substitutes placeholders. Any placeholder that cannot be resolved
  is returned in `missing_variables` and left intact in the output.
- **Render against master data** (`POST /templates/render` in the
  spec) is **not yet implemented**. Use the legacy cron runner for
  scheduled automation; manual send endpoints are out of scope for
  this PR.

---

## Deferred / Not Implemented Yet

- Holding-view aggregation (spec §7)
- Bulk JSON import (`POST /templates/messages/import`)
- Excel/CSV upload (`POST /templates/messages/upload`)
- Full seed of Dealls + KantorKu template library
- Render against live master_data (workflow execution)
