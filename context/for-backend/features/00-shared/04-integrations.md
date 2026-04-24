# External Integrations — Configuration & Architecture

## 5 Integrations

```
┌─────────────────────────────────────────────────────────────┐
│                    OUTBOUND (Send)                           │
│                                                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ 1. HaloAI   │  │ 2. Telegram  │  │ 3. SMTP (Email)  │  │
│  │ (WA send)   │  │ (DM to PIC)  │  │                  │  │
│  │ per-workspace│  │ per-workspace│  │ GLOBAL           │  │
│  └──────┬──────┘  └──────┬───────┘  └────────┬─────────┘  │
│         │                │                    │             │
│         ▼                ▼                    ▼             │
│  Client/Lead/     Internal PIC        Client/Lead/         │
│  Prospect WA      Telegram DM         Prospect Email       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐  ┌──────────────────────────────┐    │
│  │ 4. Paper.id      │  │ 5. HaloAI Webhook            │    │
│  │ (Invoice send)   │  │ (WA reply receive)            │    │
│  │ per-workspace    │  │ per-workspace                 │    │
│  └──────┬───────────┘  └──────────────┬───────────────┘    │
│         │                             │                     │
│         ▼                             ▼                     │
│  Client receives          Bot receives WA reply             │
│  invoice via Paper.id     → process → update Master Data    │
│                                                             │
│                    INBOUND (Receive)                         │
└─────────────────────────────────────────────────────────────┘
```

## Config Storage Strategy

| Integration | Scope | Where Config Lives | Why |
|-------------|-------|-------------------|-----|
| **HaloAI** (WA) | Per workspace | `workspace_integrations` table | Each workspace has own WA number, own API key |
| **Telegram** | Per workspace | `workspace_integrations` table | Each workspace has own bot token or group |
| **SMTP** (Email) | Global | Go backend env vars | 1 SMTP server, workspace differentiated by From address |
| **Paper.id** | Per workspace | `workspace_integrations` table | Each workspace has own Paper.id account |
| **HaloAI Webhook** | Per workspace | Generated URL with workspace_id | Each workspace has unique webhook endpoint |

## Database Schema

### Table: `workspace_integrations`

Stores per-workspace API credentials. **Encrypted at rest.**

```sql
CREATE TABLE workspace_integrations (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  
  -- HaloAI (WhatsApp)
  haloai_api_url      TEXT,                      -- e.g. https://api.haloai.id
  haloai_api_key      TEXT,                      -- encrypted
  haloai_wa_number    VARCHAR(20),               -- e.g. 628123456789
  haloai_webhook_secret TEXT,                    -- for verifying inbound webhooks
  
  -- Telegram
  telegram_bot_token  TEXT,                      -- encrypted
  telegram_default_chat_id VARCHAR(50),          -- default group/channel for alerts
  
  -- Paper.id (Invoices)
  paperid_api_url     TEXT,                      -- e.g. https://api.paper.id
  paperid_api_key     TEXT,                      -- encrypted
  paperid_webhook_secret TEXT,                   -- for verifying payment webhooks
  
  -- Email (per-workspace From address, SMTP server is global)
  email_from_name     VARCHAR(100),              -- e.g. "Dealls Team"
  email_from_address  VARCHAR(255),              -- e.g. noreply@dealls.com
  email_reply_to      VARCHAR(255),              -- e.g. support@dealls.com
  
  -- Status
  haloai_active       BOOLEAN DEFAULT FALSE,
  telegram_active     BOOLEAN DEFAULT FALSE,
  paperid_active      BOOLEAN DEFAULT FALSE,
  email_active        BOOLEAN DEFAULT FALSE,
  
  -- Meta
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(workspace_id)
);

CREATE INDEX idx_wi_workspace ON workspace_integrations(workspace_id);
```

### Global SMTP Config (env vars — Go backend)

```env
# SMTP Server (global — shared by all workspaces)
SMTP_HOST=smtp.yourdomain.com
SMTP_PORT=587
SMTP_USERNAME=noreply@sejutacita.id
SMTP_PASSWORD=xxx
SMTP_USE_TLS=true
```

Per-workspace From address comes from `workspace_integrations.email_from_name` + `email_from_address`.

---

## API Endpoints

### GET `/integrations`
Get integration config for active workspace.

```
Response 200:
{
  "haloai": {
    "active": true,
    "wa_number": "628123456789",
    "api_url": "https://api.haloai.id",
    "has_api_key": true            // never expose actual key
  },
  "telegram": {
    "active": true,
    "default_chat_id": "-100123456789",
    "has_bot_token": true
  },
  "paperid": {
    "active": true,
    "api_url": "https://api.paper.id",
    "has_api_key": true
  },
  "email": {
    "active": true,
    "from_name": "Dealls Team",
    "from_address": "noreply@dealls.com",
    "reply_to": "support@dealls.com"
  },
  "webhook_url": "https://api.bumi-dashboard.com/api/v1/webhook/haloai/ws-dealls-001"
}
```

### PUT `/integrations`
Update integration config. Only admin/owner can update.

```json
{
  "haloai": {
    "api_url": "https://api.haloai.id",
    "api_key": "new-key-xxx",
    "wa_number": "628123456789",
    "active": true
  },
  "telegram": {
    "bot_token": "123456:ABC-xxx",
    "default_chat_id": "-100123456789",
    "active": true
  },
  "paperid": {
    "api_url": "https://api.paper.id",
    "api_key": "new-key-xxx",
    "active": true
  },
  "email": {
    "from_name": "Dealls Team",
    "from_address": "noreply@dealls.com",
    "reply_to": "support@dealls.com",
    "active": true
  }
}
```

### POST `/integrations/test/{provider}`
Test connection to a provider.

```
POST /integrations/test/haloai
POST /integrations/test/telegram
POST /integrations/test/paperid
POST /integrations/test/email

Response 200: { "success": true, "message": "Connected successfully" }
Response 400: { "success": false, "message": "Invalid API key" }
```

---

## Webhook Endpoints (Inbound)

### POST `/webhook/haloai/{workspace_id}`
Receive WA reply from HaloAI.

```json
// HaloAI sends:
{
  "phone_number": "628987654321",
  "message": "Oke, saya tertarik. Kapan bisa meeting?",
  "timestamp": "2026-04-12T10:30:00Z",
  "conversation_id": "conv-xxx"
}

// Backend actions:
// 1. Verify webhook_secret (HMAC signature)
// 2. Find record in master_data by phone_number + workspace_id
// 3. Update: reply_wa = TRUE, Last_Interaction_Date = NOW()
// 4. If sequence_status = 'ACTIVE' → trigger HALOAI_HANDOFF
// 5. Log to action_logs
// 6. Create notification: "Reply received from [Company_Name]"

Response 200: { "received": true }
```

### POST `/webhook/paperid/{workspace_id}`
Receive payment notification from Paper.id.

```json
// Paper.id sends:
{
  "invoice_id": "INV-DE-2026-001",
  "status": "paid",
  "paid_at": "2026-04-12T10:00:00Z",
  "amount_paid": 25000000,
  "payment_method": "bank_transfer"
}

// Backend actions:
// 1. Verify webhook_secret (HMAC signature)
// 2. Update invoice: payment_status = 'Paid', paid_at, payment_method
// 3. Update master_data: Payment_Status = 'Paid'
// 4. If Stage = 'PROSPECT' + closing_status = 'CLOSING' → transition to CLIENT
// 5. Log to payment_logs + action_logs
// 6. Create notification: "Payment confirmed: [invoice_id]"

Response 200: { "received": true }
```

---

## Go Service Interfaces

```go
// Each integration has its own service interface
type WAService interface {
    SendMessage(ctx context.Context, wsID uuid.UUID, phone string, templateID string, vars map[string]string) error
}

type TelegramService interface {
    SendDM(ctx context.Context, wsID uuid.UUID, chatID string, message string) error
    SendToDefault(ctx context.Context, wsID uuid.UUID, message string) error  // uses default_chat_id
}

type EmailService interface {
    Send(ctx context.Context, wsID uuid.UUID, to string, subject string, bodyHTML string) error
}

type InvoiceService interface {
    Create(ctx context.Context, wsID uuid.UUID, invoice InvoiceCreateRequest) (*PaperIDInvoice, error)
    GetPaymentURL(ctx context.Context, wsID uuid.UUID, invoiceID string) (string, error)
}
```

Each service reads credentials from `workspace_integrations` table at runtime.
SMTP credentials come from env vars (global).

---

## Settings UI Location

Integration config should be managed at:
**Settings → Integrations tab** (new tab in settings page)

```
Settings Page:
  ├── Appearance (theme, dark mode)
  ├── Account (email, password)
  ├── Workspace (name, logo)
  └── Integrations (NEW)
       ├── WhatsApp (HaloAI)
       │   ├── API URL
       │   ├── API Key (masked)
       │   ├── WA Number
       │   └── [Test Connection]
       ├── Telegram
       │   ├── Bot Token (masked)
       │   ├── Default Chat ID
       │   └── [Test Connection]
       ├── Paper.id
       │   ├── API URL
       │   ├── API Key (masked)
       │   └── [Test Connection]
       └── Email
            ├── From Name
            ├── From Address
            ├── Reply-To
            └── [Send Test Email]
```
