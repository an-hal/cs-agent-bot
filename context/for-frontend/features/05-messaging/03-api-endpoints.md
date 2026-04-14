# Messaging Templates — API Endpoints

Base prefix: `/templates`
Auth: `Authorization: Bearer {jwt}` + `X-Workspace-ID: {uuid}`

All successful responses use the standard envelope
`{ success, message, data, meta? }`.

---

## Message Templates (WhatsApp / Telegram)

### `GET /templates/messages`

List message templates in the active workspace.

**Query params**

| Name      | Type    | Notes                             |
|-----------|---------|-----------------------------------|
| `role`    | string  | `sdr` \| `bd` \| `ae`             |
| `phase`   | string  | Comma-separated (`P0,P1`)         |
| `channel` | string  | `whatsapp` \| `telegram`          |
| `category`| string  | Exact match                       |
| `search`  | string  | ILIKE on `id`, `action`, `message`|

**Response 200** — `ApiResponseWithMeta<MessageTemplate[]>`

### `GET /templates/messages/{id}`

Fetch a single message template.

**Response 200** — `ApiResponse<MessageTemplate>`
**Response 404** — `{ error: { code: "NOT_FOUND" } }`

### `POST /templates/messages`

Create a new message template.

**Request body**

```json
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
  "condition": "days_since_activation BETWEEN 100 AND 105",
  "message": "Halo [contact_name_prefix_manual] [contact_name_primary], ...",
  "variables": ["contact_name_prefix_manual", "contact_name_primary"],
  "stop_if": "custom_replied = TRUE",
  "sent_flag": "custom_sent",
  "priority": "P2"
}
```

`variables` is optional — the backend extracts placeholders from
`message` when omitted.

**Response 201** — `ApiResponse<MessageTemplate>`
**Response 409** — duplicate `id` in workspace
**Response 400** — missing `id` or malformed JSON

### `PUT /templates/messages/{id}`

Partial update. Only non-empty fields in the body are applied.

**Response 200**

```json
{
  "success": true,
  "message": "Message template updated",
  "data": {
    "data": { "...updated MessageTemplate..." },
    "changed_fields": ["message", "variables"]
  }
}
```

The backend appends one row to `template_edit_logs` per entry in
`changed_fields`.

### `DELETE /templates/messages/{id}`

**Response 200** — `{ success: true, data: { id: "..." } }`
A `deleted` row is appended to `template_edit_logs`.

---

## Email Templates

### `GET /templates/emails`

**Query params**

| Name       | Type   | Notes                             |
|------------|--------|-----------------------------------|
| `role`     | string | `sdr` \| `bd` \| `ae`             |
| `category` | string | Exact match                       |
| `status`   | string | `active` \| `draft` \| `archived` |
| `search`   | string | ILIKE on `id`, `name`, `subject`  |

**Response 200** — `ApiResponseWithMeta<EmailTemplate[]>`

### `GET /templates/emails/{id}`
**Response 200** — `ApiResponse<EmailTemplate>`

### `POST /templates/emails`

**Request body**

```json
{
  "id": "ETPL-DE-CUSTOM-001",
  "name": "Custom Outreach Email",
  "subject": "Hello [PIC_Name] from [Company_Name]",
  "body_html": "<p>Hello <strong>[PIC_Name]</strong>,</p>...",
  "category": "outreach",
  "role": "sdr",
  "status": "draft"
}
```

`body_html` is sanitized server-side. `variables` is auto-extracted
when omitted.

**Response 201** — `ApiResponse<EmailTemplate>`
**Response 409** — duplicate `id`

### `PUT /templates/emails/{id}`

Partial update. Returns `data.data` (updated template) and
`data.changed_fields`.

### `DELETE /templates/emails/{id}`

---

## Preview

### `POST /templates/preview`

Render a template with caller-supplied sample data.

**Request body**

```json
{
  "template_type": "message",
  "template_id": "TPL-OB-WELCOME",
  "sample_data": {
    "Company_Name": "PT Maju Digital",
    "contact_name_prefix_manual": "Pak",
    "contact_name_primary": "Budi"
  }
}
```

For `template_type: "email"` the response also includes a rendered
`subject`.

**Response 200** — `ApiResponse<PreviewResult>`

```ts
type PreviewResult = {
  template_type:     "message" | "email";
  rendered?:         string;   // for message
  subject?:          string;   // for email
  body_html?:        string;   // for email
  missing_variables: string[];
};
```

Unresolved `[variable]` placeholders are listed in
`missing_variables` and left intact in the rendered output.

---

## Edit Logs

### `GET /templates/edit-logs`

Workspace-wide feed of edits.

**Query params**

| Name            | Type    | Notes                          |
|-----------------|---------|--------------------------------|
| `template_id`   | string  | Filter by a specific template  |
| `template_type` | string  | `message` \| `email`           |
| `limit`         | int     | Default 50                     |
| `since`         | string  | RFC 3339 timestamp             |

**Response 200** — `ApiResponseWithMeta<TemplateEditLog[]>`

### `GET /templates/edit-logs/{template_id}`

Single-template history (for the detail drawer).

**Response 200** — `ApiResponse<TemplateEditLog[]>`

---

## Variables Catalog

### `GET /templates/variables`

List all substitution variables available in this workspace. Use
this to build an autocomplete/reference picker in the template
editor.

**Response 200** — `ApiResponse<TemplateVariable[]>`

The catalog is seeded for every existing workspace (see migration
`20260414000504_seed_template_variables`) with common keys like
`Company_Name`, `PIC_Name`, `link_wiki`, etc.

---

## Error Responses

Standard error envelope:

```json
{
  "success": false,
  "message": "Template not found",
  "data": null,
  "error": { "code": "NOT_FOUND", "message": "Template not found" }
}
```

| Status | `error.code`        | When                                       |
|--------|---------------------|--------------------------------------------|
| 400    | `VALIDATION_ERROR`  | Missing/invalid body or params             |
| 401    | `UNAUTHORIZED`      | Missing/invalid JWT                        |
| 403    | `FORBIDDEN`         | Workspace access denied                    |
| 404    | `NOT_FOUND`         | Template does not exist in this workspace  |
| 409    | `CONFLICT`          | Duplicate `id` on create                   |
| 500    | `INTERNAL_ERROR`    | Unexpected server error                    |
