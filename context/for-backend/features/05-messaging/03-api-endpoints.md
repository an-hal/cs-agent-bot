# API Endpoints — Messaging Templates

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header.
Workspace-scoped endpoints require `X-Workspace-ID: {uuid}` header.

---

## 1. Message Templates (WA/Telegram) CRUD

### GET `/templates/messages`
List message templates dengan filter.

```
Query params:
  ?role=sdr                        (optional: sdr, bd, ae)
  &phase=P0,P1                     (optional, comma-separated)
  &channel=whatsapp                (optional: whatsapp, telegram)
  &category=onboarding             (optional)
  &search=keyword                  (optional — searches id, action, message)

Response 200:
{
  "data": [
    {
      "id": "TPL-OB-WELCOME",
      "workspace_id": "uuid",
      "trigger_id": "Onboarding_Welcome",
      "phase": "P0",
      "phase_label": "Onboarding",
      "channel": "whatsapp",
      "role": "ae",
      "category": "onboarding",
      "action": "Send welcome message",
      "timing": "D+0 to D+5",
      "condition": "days_since_activation BETWEEN 0 AND 5 AND onboarding_sent = FALSE",
      "message": "Halo [contact_name_prefix_manual] [contact_name_primary] ...",
      "variables": ["Company_Name", "contact_name_prefix_manual", "contact_name_primary", "link_wiki"],
      "stop_if": "-",
      "sent_flag": "onboarding_sent",
      "priority": "P0",
      "updated_at": "2026-04-03T14:22:00Z",
      "updated_by": "arief.faltah@dealls.com",
      "created_at": "2026-03-01T10:00:00Z"
    }
  ],
  "meta": {
    "total": 35
  }
}
```

### GET `/templates/messages/{id}`
Get single message template by ID.

```
Response 200:
{
  "data": { ... full template ... }
}

Response 404:
{
  "error": "Template not found"
}
```

### POST `/templates/messages`
Create new message template.

```json
// Request body:
{
  "id": "TPL-CUSTOM-001",
  "trigger_id": "Custom_Trigger",
  "phase": "P2",
  "phase_label": "Warming Up",
  "channel": "whatsapp",
  "role": "ae",
  "category": "warmup",
  "action": "Custom check-in message",
  "timing": "D+100 to D+105",
  "condition": "days_since_activation BETWEEN 100 AND 105 AND custom_sent = FALSE",
  "message": "Halo [contact_name_prefix_manual] [contact_name_primary], ...",
  "variables": ["Company_Name", "contact_name_prefix_manual", "contact_name_primary"],
  "stop_if": "custom_replied = TRUE",
  "sent_flag": "custom_sent",
  "priority": "P2"
}

// Response 201:
{
  "data": { "id": "TPL-CUSTOM-001", ... }
}

// Response 409:
{
  "error": "Template ID TPL-CUSTOM-001 already exists in this workspace"
}
```

### PUT `/templates/messages/{id}`
Update message template. Partial update (merge).

```json
// Request body — hanya field yang berubah:
{
  "message": "Updated message text...",
  "variables": ["Company_Name", "PIC_Name"]
}

// Response 200:
{
  "data": { ... updated template ... },
  "changed_fields": ["message", "variables"],
  "edit_log_ids": ["uuid-1"]
}
```

Backend harus:
1. Compare old vs new values per field
2. Create `template_edit_logs` entry untuk setiap field yang berubah
3. Set `updated_at` dan `updated_by`

### DELETE `/templates/messages/{id}`

```
Response 200:
{
  "message": "Deleted",
  "id": "TPL-CUSTOM-001"
}
```

Backend harus: create edit log dengan `field = 'deleted'`.

---

## 2. Email Templates CRUD

### GET `/templates/emails`
List email templates dengan filter.

```
Query params:
  ?role=sdr                        (optional: sdr, bd, ae)
  &category=outreach               (optional)
  &status=active                   (optional: active, draft, archived)
  &search=keyword                  (optional — searches id, name, subject)

Response 200:
{
  "data": [
    {
      "id": "ETPL-DE-SDR-001",
      "workspace_id": "uuid",
      "name": "SDR Cold Outreach — Intro Dealls",
      "subject": "Solusi Rekrutmen untuk [Company_Name]",
      "body_html": "<h2>Halo [PIC_Name],</h2>...",
      "category": "outreach",
      "role": "sdr",
      "variables": ["PIC_Name", "Company_Name", "SDR_Name", "link_deck"],
      "status": "active",
      "updated_at": "2026-04-07T09:00:00Z",
      "updated_by": "rina@dealls.com",
      "created_at": "2026-03-01T10:00:00Z"
    }
  ],
  "meta": {
    "total": 10
  }
}
```

### GET `/templates/emails/{id}`
Get single email template.

```
Response 200:
{
  "data": { ... full email template ... }
}
```

### POST `/templates/emails`
Create new email template.

```json
// Request body:
{
  "id": "ETPL-DE-CUSTOM-001",
  "name": "Custom Outreach Email",
  "subject": "Hello [PIC_Name] from [Company_Name]",
  "body_html": "<p>Hello <strong>[PIC_Name]</strong>,</p>...",
  "category": "outreach",
  "role": "sdr",
  "variables": ["PIC_Name", "Company_Name"],
  "status": "draft"
}

// Response 201:
{
  "data": { "id": "ETPL-DE-CUSTOM-001", ... }
}
```

Backend harus:
1. **Sanitize `body_html`** — strip dangerous tags/attributes
2. Create edit log with `field = 'created'`

### PUT `/templates/emails/{id}`
Update email template. Partial update.

```json
// Request body:
{
  "subject": "Updated Subject for [Company_Name]",
  "body_html": "<p>Updated content...</p>",
  "status": "active"
}

// Response 200:
{
  "data": { ... updated template ... },
  "changed_fields": ["subject", "body_html", "status"],
  "edit_log_ids": ["uuid-1", "uuid-2", "uuid-3"]
}
```

### DELETE `/templates/emails/{id}`

```
Response 200:
{
  "message": "Deleted",
  "id": "ETPL-DE-CUSTOM-001"
}
```

---

## 3. Template Preview / Render

### POST `/templates/preview`
Render template dengan sample data untuk preview di frontend.

```json
// Request body:
{
  "template_type": "message",
  "template_id": "TPL-OB-WELCOME",
  "sample_data": {
    "Company_Name": "PT Maju Digital",
    "contact_name_prefix_manual": "Pak",
    "contact_name_primary": "Budi",
    "link_wiki": "https://wiki.dealls.com/onboarding"
  }
}

// Response 200:
{
  "rendered": "Halo Pak Budi 👋\n\nSelamat datang di Dealls! Kami senang PT Maju Digital sudah bergabung...",
  "missing_variables": [],
  "template_type": "message"
}
```

Jika ada variabel yang tidak tersedia di `sample_data`:
```json
{
  "rendered": "Halo [contact_name_prefix_manual] Budi 👋\n\n...",
  "missing_variables": ["contact_name_prefix_manual"],
  "template_type": "message"
}
```

### POST `/templates/render`
Render template dengan data dari Master Data (untuk actual sending).

```json
// Request body:
{
  "template_type": "message",
  "template_id": "TPL-OB-WELCOME",
  "master_data_id": "uuid-of-company-record"
}

// Response 200:
{
  "rendered": "Halo Pak Budi 👋\n\nSelamat datang di Dealls! Kami senang PT Maju Digital sudah bergabung...",
  "missing_variables": [],
  "variables_used": {
    "Company_Name": "PT Maju Digital",
    "contact_name_prefix_manual": "Pak",
    "contact_name_primary": "Budi",
    "link_wiki": "https://wiki.dealls.com/onboarding"
  }
}
```

Backend logic:
1. Load template by ID
2. Load master_data record by master_data_id
3. Resolve variables: core fields + custom_fields + computed + workspace config
4. Replace `[Variable_Name]` placeholders
5. Return rendered content + metadata

---

## 4. Edit Logs / Version History

### GET `/templates/edit-logs`
Riwayat perubahan template per workspace.

```
Query params:
  ?template_id=TPL-OB-WELCOME     (optional — filter by specific template)
  &template_type=message           (optional: message, email)
  &limit=50                        (default 50)
  &since=2026-04-01T00:00:00Z     (optional)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "template_id": "TPL-OB-WELCOME",
      "template_type": "message",
      "field": "message",
      "old_value": "Halo [contact_name_prefix_manual]...",
      "new_value": "Halo Tim [Company_Name]...",
      "edited_by": "arief.faltah@dealls.com",
      "edited_at": "2026-04-03T14:22:00Z"
    }
  ],
  "meta": {
    "total": 12
  }
}
```

### GET `/templates/edit-logs/{template_id}`
Edit logs untuk satu template spesifik.

```
Query: ?limit=20

Response 200:
{
  "data": [ ... logs for this template, sorted by edited_at DESC ... ]
}
```

---

## 5. Template Variables (Definitions)

### GET `/templates/variables`
List semua variable definitions untuk workspace.

```
Response 200:
{
  "data": [
    {
      "id": "uuid",
      "variable_key": "Company_Name",
      "display_label": "Nama Perusahaan",
      "source_type": "master_data_core",
      "source_field": "company_name",
      "description": "Nama perusahaan dari Master Data",
      "example_value": "PT Maju Digital"
    },
    {
      "id": "uuid",
      "variable_key": "link_wiki",
      "display_label": "Link Wiki",
      "source_type": "workspace_config",
      "source_field": null,
      "description": "URL wiki/tutorial produk",
      "example_value": "https://wiki.dealls.com/onboarding"
    }
  ]
}
```

Endpoint ini dipakai oleh frontend saat user membuat/edit template —
menampilkan daftar variabel yang tersedia sebagai autocomplete/reference.

---

## 6. Bulk Operations

### POST `/templates/messages/import`
Bulk import message templates dari JSON file.

```json
// Request body:
{
  "templates": [
    {
      "id": "TPL-CUSTOM-001",
      "trigger_id": "Custom_1",
      "phase": "P2",
      "phase_label": "Warming Up",
      "channel": "whatsapp",
      "role": "ae",
      "category": "warmup",
      "action": "Custom action",
      "timing": "D+100",
      "condition": "...",
      "message": "...",
      "variables": ["Company_Name"],
      "stop_if": "-",
      "sent_flag": "custom_sent",
      "priority": "P2"
    }
  ],
  "mode": "upsert"
}

// mode: "upsert" = insert new, update existing by ID
// mode: "insert_only" = skip if ID exists

// Response 200:
{
  "inserted": 5,
  "updated": 2,
  "skipped": 0,
  "errors": []
}
```

### POST `/templates/messages/upload`
Upload template dari file (Excel/CSV) — untuk migrasi dari spreadsheet.

```
Content-Type: multipart/form-data
Fields:
  file: .xlsx or .csv file
  channel: whatsapp | telegram
  role: sdr | bd | ae

Response 200:
{
  "imported": 10,
  "skipped": 1,
  "errors": [
    { "row": 5, "error": "Missing required field: trigger_id" }
  ]
}
```

---

## 7. Holding View

Jika workspace adalah holding (`is_holding = true`):
- `GET /templates/messages` dan `GET /templates/emails` otomatis return data dari semua member workspaces
- Setiap template di-enrich dengan `workspace_name` field
- Same logic as master-data holding view: check `workspace.is_holding`, get `member_ids`, query across all

---

## 8. HaloAI Webhooks

### POST `/webhook/haloai/inbound` [Gap #32]
Receive classified inbound WA reply from HaloAI agent.

```
Headers:
  X-Haloai-Signature: {HMAC-SHA256(raw_body, HALOAI_WEBHOOK_SECRET_INBOUND)}
  X-Workspace-Id: {uuid}

Request body:
{
  "conversation_id": "halo-conv-12345",
  "client_id": "uuid",
  "inbound_message": "Boleh kirim proposalnya?",
  "received_at": "2026-04-22T10:15:00+07:00",
  "classification": "demo_request",
  "classification_confidence": 0.92,
  "competitor_mentioned": null
}

Response 200:
{
  "received": true,
  "escalation_fired": "ESC-BD-002",
  "next_action": "continue_bot"
}
```

Backend flow:
1. **Verify HMAC signature** — reject 401 if invalid
2. Update `clients`:
   - `last_reply_classification = classification`
   - `replied_count = replied_count + 1`
   - `competitor_mentioned = TRUE` if applicable
3. Apply escalation rule from `01-overview.md` → Inbound Reply Classification table
4. If `classification = 'reject'` → trigger Rejection Analysis pipeline (forward to Claude)
5. If `classification = 'angry'` → set `bot_active = FALSE`, `blacklisted = TRUE`, fire ESC-BD-006
6. Insert escalation row in `bd_escalations` (see `06-workflow-engine/02-database-schema.md` Agent F2)

### POST `/webhook/haloai/rejection` [Gap #31]
HaloAI fires this when prospect explicitly rejects OR conversation goes DORMANT (>14d no reply).

```
Headers:
  X-Haloai-Signature: {HMAC-SHA256(raw_body, HALOAI_WEBHOOK_SECRET_REJECTION)}
  X-Workspace-Id: {uuid}

Request body:
{
  "conversation_id": "halo-conv-12345",
  "prospect_id": "uuid",                 // = clients.id
  "trigger_source": "reject_classified", // or 'dormant_14d'
  "conversation_json": [
    { "from": "bot", "text": "...", "ts": "..." },
    { "from": "prospect", "text": "...", "ts": "..." }
  ],
  "last_reply_at": "2026-04-22T09:00:00+07:00",
  "signature": "{redundant body field for double-check}"
}

Response 200:
{
  "received": true,
  "analysis_id": "uuid",
  "rejection_category": "competitor_locked",
  "recommended_action": "winback_in_90d",
  "scheduled_followup_at": "2026-07-21T00:00:00Z"
}
```

Backend flow:
1. **Verify HMAC** — reject 401 if `X-Haloai-Signature` does not match
2. Forward `conversation_json` → Claude with system prompt `HALOAI_REJECTION_ANALYSIS_PROMPT`
3. Parse Claude's structured JSON response (10-category enum)
4. UPDATE `clients` with `rejection_category`, `rejection_detail`, `rejection_confidence`, `prospect_sentiment`, `reengagement_signal`, `reengagement_timeframe`, `recommended_action`, `rejection_analyzed_at`
5. INSERT into `rejection_analysis_log` with full Claude request/response
6. Apply decision tree (see `01-overview.md` → Decision Tree per category)
7. Schedule downstream action (winback / nurture / blacklist) via workflow engine

### Webhook Security (HaloAI)
- Two distinct secrets: `HALOAI_WEBHOOK_SECRET_INBOUND`, `HALOAI_WEBHOOK_SECRET_REJECTION`
- Verify: `HMAC-SHA256(raw_body, secret)` must equal `X-Haloai-Signature` (hex)
- Idempotent: re-deliveries with same `conversation_id` + `received_at` are no-ops
- Log every inbound payload (success or fail) to `webhook_inbound_log`

---

## 9. Telegram Alerts (Outbound)

These are NOT webhooks — they are server-initiated. Documented here for completeness.

### Internal: D7 High BD-Score Alert [Gap #65]
Fired by D7 cron when `bd_score >= BD_SCORE_ALERT_THRESHOLD`.
Backend calls `POST {TELEGRAM_BOT_API}/sendMessage` to `clients.bd_owner_telegram_chat_id` and sets `clients.bd_d10_dm_alert_sent = TRUE` for idempotency. See `01-overview.md` → D7 Telegram Alert.

### Internal: D10 DM Escalation Alert [Gap #63]
Fired by D10 cron when `dm_followup_needed = TRUE AND dm_present_in_call = TRUE`.
After firing, increments `dm_followup_count`; if reaches 3, fires ESC-BD-004. See `01-overview.md` → D10 DM Escalation.

---

## Error Responses (Standard)

```json
// 400 Bad Request
{
  "error": "Validation failed",
  "details": [
    { "field": "channel", "message": "Must be 'whatsapp' or 'telegram'" }
  ]
}

// 401 Unauthorized
{
  "error": "Token expired or invalid"
}

// 403 Forbidden
{
  "error": "No access to this workspace"
}

// 404 Not Found
{
  "error": "Template not found"
}

// 409 Conflict
{
  "error": "Template ID already exists in this workspace"
}
```
