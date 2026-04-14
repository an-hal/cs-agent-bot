# Messaging Templates — Data Model

## `MessageTemplate`

```ts
type MessageTemplate = {
  id:            string;   // e.g. "TPL-OB-WELCOME"
  workspace_id:  string;   // uuid
  trigger_id:    string;   // e.g. "Onboarding_Welcome"
  phase:         "P0" | "P1" | "P2" | "P3" | "P4" | "P5" | "P6" | "HANDOFF" | "DORMANT";
  phase_label:   string;   // "Onboarding", "First Assessment", ...
  channel:       "whatsapp" | "telegram";
  role:          "sdr" | "bd" | "ae";
  category:
    | "onboarding"
    | "assessment"
    | "warmup"
    | "promo"
    | "renewal"
    | "payment"
    | "first_payment"
    | "overdue"
    | "outreach"
    | "qualification"
    | "nurture"
    | "blast"
    | "escalation";
  action:        string;
  timing:        string;
  condition:     string;
  message:       string;          // body with [Variable_Name] placeholders
  variables:     string[];        // auto-extracted if omitted on write
  stop_if:       string | null;
  sent_flag:     string;
  priority:      string | null;
  updated_at:    string | null;   // ISO 8601
  updated_by:    string | null;   // editor email
  created_at:    string;
};
```

## `EmailTemplate`

```ts
type EmailTemplate = {
  id:            string;   // e.g. "ETPL-DE-SDR-001"
  workspace_id:  string;
  name:          string;
  role:          "sdr" | "bd" | "ae";
  category:
    | "outreach"
    | "onboarding"
    | "nurture"
    | "renewal"
    | "payment"
    | "overdue"
    | "blast"
    | "qualification"
    | "escalation";
  status:        "active" | "draft" | "archived";
  subject:       string;     // may contain [Variable_Name] placeholders
  body_html:     string;     // sanitized on write
  variables:     string[];   // auto-extracted if omitted on write
  updated_at:    string | null;
  updated_by:    string | null;
  created_at:    string;
};
```

## `TemplateVariable`

```ts
type TemplateVariable = {
  id:            string;
  workspace_id:  string;
  variable_key:  string;   // "Company_Name"
  display_label: string;   // "Nama Perusahaan"
  source_type:
    | "master_data_core"
    | "master_data_custom"
    | "invoice"
    | "computed"
    | "workspace_config"
    | "generated";
  source_field:  string | null;  // "company_name"
  description:   string | null;
  example_value: string | null;  // "PT Maju Digital"
  created_at:    string;
};
```

## `TemplateEditLog`

```ts
type TemplateEditLog = {
  id:             string;
  workspace_id:   string;
  template_id:    string;
  template_type:  "message" | "email";
  field:          string;        // e.g. "message", "status", "created", "deleted"
  old_value:      string | null;
  new_value:      string | null;
  edited_by:      string;        // editor email
  edited_at:      string;        // ISO 8601
};
```

## Response Envelope

All endpoints use the project-standard response envelope:

```ts
type ApiResponse<T> = {
  success: boolean;
  message: string;
  data: T | null;
  error?: { code: string; message: string; details?: unknown };
};

type ApiResponseWithMeta<T> = ApiResponse<T> & { meta: { total: number } };
```

## Validation Rules

- `id`: required on create, max 50 chars, unique within workspace.
- `channel`: only `whatsapp` or `telegram` for message templates.
- `role`: only `sdr`, `bd`, or `ae`.
- `status` (email): only `active`, `draft`, or `archived`.
- `source_type` (variable): constrained to the enum above.
- `body_html`: sanitized on write. Scripts, event handlers, and
  `javascript:` URLs are stripped silently.
- `variables`: if omitted on create/update, the backend extracts them
  from `message` (and `subject` + `body_html` for email).
