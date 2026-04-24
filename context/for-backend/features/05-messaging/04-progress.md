# Messaging — Implementation Progress

## 2026-04-23 — HaloAI + reply classification sync

### FE (shipped)
- Approval dropdown overhaul — single dropdown across SEND/SKIP/EDIT actions with state preview
- HaloAI Conversations page — surfaces inbound WhatsApp threads with reply classification labels
- Telegram alert sent indicator — chip on activity rows when D7 alert fired
- Active Branch indicator — shows which workflow branch a client is currently on
- Fireflies recording link — surfaces meeting recording on client drawer when present
- NPS Survey Status panel — tracks survey sent / responded / score / gap to next survey

### Backend spec (documented, implementation pending)
- 4 new sections added to `01-overview.md`:
  - **Inbound Reply Classification** — 11-category taxonomy (Interested / Pricing / Reschedule / Reject_Price / Reject_Fit / Reject_Timing / Reject_Competitor / Reject_NoNeed / Reject_Other / Question / Spam)
  - **Rejection Analysis Pipeline** — Claude-based extraction of rejection reason → category + freeform note
  - **D7 Telegram Alert** — alert spec for unreplied D7 message (silent client signal)
  - **D10 DM Escalation** — escalate to direct message channel after D10 silence
- `03-api-endpoints.md`: added `POST /webhook/haloai/inbound` (raw inbound) + `POST /webhook/haloai/rejection` (classified rejection signal)

### Open dependencies (backend)
- Implement HaloAI integration — webhook receiver, signature verification, message normalization
- Implement Claude rejection analysis pipeline — async job that classifies inbound text into 11 categories
- Implement Telegram alert cron — fires D7/D10 windows, writes to `action_logs` with manual_send=false
- Wire NPS survey status into messaging templates (NPS uses messaging delivery infra)

### Cross-refs
- FE: `app/dashboard/[workspace]/messaging/conversations/page.tsx`, `components/features/ApprovalDropdown.tsx`, `components/features/NPSSurveyPanel.tsx`
- 01-auth §2d escalation severity matrix consumes the rejection-category counts surfaced here
- Backend gap doc: `claude/for-backend/05-messaging/gap-haloai-integration.md`, `gap-rejection-analysis.md`

---

> **Overall: 30% complete** (15/50 items done or partial)
> - Frontend: 65% done (13 done + 2 partial)
> - Backend (Go): 0% done (28 items not started)
> - Optional improvements: 0% done (7 items)

---

## DONE — Frontend ✅ (13 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | Message template list page (WA/Telegram) | `app/dashboard/[workspace]/message-template/page.tsx` | Full table with filters by role, phase, channel, category, workspace |
| 2 | Message template data model (TypeScript) | Same file | `MessageTemplate` interface matches spec schema fields exactly |
| 3 | Hardcoded Dealls AE templates (P0-P6) | Same file | ~25 templates: onboarding, NPS, check-in, cross-sell, renewal, payment, overdue |
| 4 | Hardcoded KantorKu AE templates | Same file | Workspace-specific HRIS/Payroll templates mirroring Dealls structure |
| 5 | Hardcoded SDR templates (WA + Email) | Same file | SDR outreach, nurture, broadcast, seasonal, snooze templates |
| 6 | Template detail drawer | Same file | Shows all fields: message body, variables, timing, condition, sentFlag |
| 7 | Edit log display in drawer | Same file | Shows `EditLog[]` with field, old/new values, editor, timestamp |
| 8 | Template variable highlighting | Same file | Variables like `[Company_Name]` rendered with visual tags |
| 9 | Email template list page | `app/dashboard/[workspace]/email-template/page.tsx` | Table with filters by role, category, status, workspace |
| 10 | Email template data model (TypeScript) | Same file | `EmailTemplate` interface with subject, body_html, status, variables |
| 11 | TipTap rich-text editor for email HTML | Same file + `components/features/EmailTipTapEditor` | Dynamic import, HTML editing with sanitization |
| 12 | Hardcoded Dealls + KantorKu email templates | Same file | SDR outreach, BD proposal, AE renewal, overdue emails per workspace |
| 13 | Holding view (combined workspace display) | Both template pages | Shows templates from both workspaces when holding workspace active |

## PARTIAL ⚠️ (2 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 14 | Template inline editing | Edit modal with form fields for message, variables, etc. | Changes are client-side only (setState), no API call to persist |
| 15 | Edit log generation | `EditLog` / `EmailEditLog` types defined, mock logs displayed | Logs are hardcoded mock data, not generated from actual edits |

## NOT DONE — Backend (Go) Required 🔴 (28 items)

### Critical (blocks real data)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 16 | `message_templates` table | 02-database-schema | id (VARCHAR PK), workspace_id, trigger_id, phase, channel, role, category, message, variables, timing, condition, sent_flag, etc. |
| 17 | `email_templates` table | 02-database-schema | id (VARCHAR PK), workspace_id, name, subject, body_html, role, category, status, variables |
| 18 | `template_variables` table | 02-database-schema | Variable definitions per workspace (key, source_type, source_field, example_value) |
| 19 | `template_edit_logs` table | 02-database-schema | Audit trail: template_id, template_type, field, old_value, new_value, edited_by |
| 20 | GET `/templates/messages` | 03-api-endpoints | List message templates with filter (role, phase, channel, category, search) |
| 21 | GET `/templates/messages/{id}` | 03-api-endpoints | Get single message template |
| 22 | GET `/templates/emails` | 03-api-endpoints | List email templates with filter (role, category, status, search) |
| 23 | GET `/templates/emails/{id}` | 03-api-endpoints | Get single email template |

### High Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 24 | POST `/templates/messages` | 03-api-endpoints | Create new message template |
| 25 | PUT `/templates/messages/{id}` | 03-api-endpoints | Update message template + auto-generate edit logs for changed fields |
| 26 | DELETE `/templates/messages/{id}` | 03-api-endpoints | Delete message template + log deletion |
| 27 | POST `/templates/emails` | 03-api-endpoints | Create email template + sanitize body_html |
| 28 | PUT `/templates/emails/{id}` | 03-api-endpoints | Update email template + auto-generate edit logs |
| 29 | DELETE `/templates/emails/{id}` | 03-api-endpoints | Delete email template |
| 30 | body_html sanitization | 03-api-endpoints | Whitelist tags, strip script/event handlers on write |

### Medium Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 31 | POST `/templates/preview` | 03-api-endpoints | Render template with sample_data, report missing_variables |
| 32 | POST `/templates/render` | 03-api-endpoints | Render template with real master_data record (for actual sending) |
| 33 | GET `/templates/edit-logs` | 03-api-endpoints | List edit logs per workspace with filters |
| 34 | GET `/templates/edit-logs/{template_id}` | 03-api-endpoints | Edit logs for a specific template |
| 35 | GET `/templates/variables` | 03-api-endpoints | List variable definitions (autocomplete for editor) |
| 36 | Holding view aggregation | 03-api-endpoints | When is_holding=true, return templates from all member workspaces |

### Low Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 37 | POST `/templates/messages/import` | 03-api-endpoints | Bulk import (JSON), upsert or insert_only mode |
| 38 | POST `/templates/messages/upload` | 03-api-endpoints | Upload from Excel/CSV file with column mapping |
| 39 | Seed data: Dealls AE templates | 01-overview | Migrate ~25 hardcoded Dealls AE templates to DB |
| 40 | Seed data: KantorKu AE templates | 01-overview | Migrate ~25 hardcoded KantorKu templates to DB |
| 41 | Seed data: SDR templates | 01-overview | Migrate SDR WA + Email templates to DB |
| 42 | Seed data: Email templates | 01-overview | Migrate ~10 hardcoded email templates to DB |
| 43 | Seed data: Template variable definitions | 02-database-schema | Populate template_variables with all known variable keys |

## NOT DONE — Optional Frontend Improvements 🟡 (7 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 44 | Connect template list to GET API | High | Replace hardcoded arrays with fetch from `/templates/messages` and `/templates/emails` |
| 45 | Connect template edit to PUT API | High | Save edits via API instead of client-side setState |
| 46 | Connect template create to POST API | Medium | Create new templates via API |
| 47 | Template preview with sample data | Medium | Call `/templates/preview` to show rendered output before sending |
| 48 | Variable autocomplete in editor | Low | Fetch `/templates/variables` and show picker/autocomplete in message editor |
| 49 | Bulk import UI (JSON upload) | Low | UI for `/templates/messages/import` endpoint |
| 50 | Excel/CSV upload UI | Low | UI for `/templates/messages/upload` endpoint |

---

## Recommended Implementation Order (Backend)

```
Week 1: #16 message_templates + #17 email_templates + #19 template_edit_logs
         #20 GET /templates/messages + #22 GET /templates/emails
Week 2: #21 GET single + #23 GET single + #24 POST message + #27 POST email
         #25 PUT message (with edit log) + #28 PUT email
Week 3: #26 DELETE message + #29 DELETE email + #30 body_html sanitization
         #18 template_variables + #35 GET /templates/variables
Week 4: #31 preview + #32 render + #33-34 edit logs endpoints
         #36 holding view
Later:  #37-38 bulk import/upload + #39-43 seed data migration
```

## Dependency Chain

```
message_templates ──→ GET/POST/PUT/DELETE /templates/messages
  │                          │
  └──→ template_edit_logs ──→ GET /templates/edit-logs
  │
  └──→ template_variables ──→ GET /templates/variables
                                    │
email_templates ──→ GET/POST/PUT/DELETE /templates/emails
  │                          │
  └──→ body_html sanitization│
                             └──→ /templates/preview + /templates/render
                                        │
                                        └──→ master_data (for real rendering)
```
