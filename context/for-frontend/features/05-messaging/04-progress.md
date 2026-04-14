# Messaging Templates — Implementation Progress

> Backend scope implemented in `feat/05-messaging`.

## DONE — Backend ✅

| # | Item | Notes |
|---|------|-------|
| 1 | `message_templates` table | Workspace-scoped, VARCHAR PK, unique `(workspace_id, trigger_id, channel)` |
| 2 | `email_templates` table | Unique `(workspace_id, id)`, status check, role check |
| 3 | `template_variables` table | Source-type check, unique `(workspace_id, variable_key)` |
| 4 | `template_edit_logs` table | INSERT-only audit trail |
| 5 | Seed common variables | ~26 keys seeded for every existing workspace |
| 6 | `GET /templates/messages` | Filter by role, phase, channel, category, search |
| 7 | `GET /templates/messages/{id}` | |
| 8 | `POST /templates/messages` | Auto-extracts variables from message body |
| 9 | `PUT /templates/messages/{id}` | Partial merge + diff-based edit logs |
| 10 | `DELETE /templates/messages/{id}` | Appends `deleted` edit log |
| 11 | `GET /templates/emails` | Filter by role, category, status, search |
| 12 | `GET /templates/emails/{id}` | |
| 13 | `POST /templates/emails` | bluemonday sanitize + auto variable extraction |
| 14 | `PUT /templates/emails/{id}` | Sanitize + diff-based edit logs |
| 15 | `DELETE /templates/emails/{id}` | Appends `deleted` edit log |
| 16 | `POST /templates/preview` | Substitutes `[Variable_Name]`, reports missing vars |
| 17 | `GET /templates/edit-logs` | Workspace-wide feed |
| 18 | `GET /templates/edit-logs/{template_id}` | Single-template history |
| 19 | `GET /templates/variables` | Variable catalog for autocomplete |
| 20 | HTML sanitize pkg | `internal/pkg/htmlsanitize` using bluemonday |
| 21 | Unit tests | `ExtractVariables`, `Render`, `SanitizeEmailHTML` |
| 22 | Swagger docs | Regenerated via `make swag` |

## DEFERRED / NOT DONE 🟡

| # | Item | Why |
|---|------|-----|
| 1 | `POST /templates/render` against master_data | Needs workflow runtime integration |
| 2 | `POST /templates/messages/import` (bulk JSON) | Out of scope for initial PR |
| 3 | `POST /templates/messages/upload` (xlsx/csv) | Out of scope for initial PR |
| 4 | Holding view aggregation (spec §7) | Needs cross-workspace query pattern |
| 5 | Full seed (~50 Dealls + KantorKu templates) | Minimal seed only; content curation pending |

## Frontend Integration Checklist

- [ ] Replace hardcoded template arrays with `GET /templates/messages` + `GET /templates/emails`
- [ ] Wire edit modals to `PUT /templates/{messages,emails}/{id}` and surface `changed_fields` in the success toast
- [ ] Wire create modals to `POST /templates/{messages,emails}`
- [ ] Show edit history drawer from `GET /templates/edit-logs/{template_id}`
- [ ] Build variable picker from `GET /templates/variables`
- [ ] Call `POST /templates/preview` when the user clicks "Preview" in the editor
- [ ] Handle `409 CONFLICT` on duplicate IDs with a user-friendly message
- [ ] Render `missing_variables` from preview as a warning badge above the preview pane
