# Notifications System

## Overview

Notifications adalah **cross-cutting feature** — event dari semua feature lain (workflow, invoices, team, escalation) menghasilkan notifikasi yang ditampilkan di bell icon header.

```
Events from:                          Notification System
┌──────────────────┐                  ┌─────────────────────┐
│ 06-Workflow      │──── stage ────→  │                     │
│   (cron engine)  │    transition    │  notifications DB   │
├──────────────────┤                  │                     │
│ 07-Invoices      │──── payment ──→  │  GET /notifications │ → Bell Icon (frontend)
│   (webhook)      │    confirmed     │                     │
├──────────────────┤                  │  + Telegram Bot     │ → Team Lead phone
│ 08-Activity Log  │── escalation ──→ │  + Email (optional) │
│   (escalation)   │                  │                     │
├──────────────────┤                  └─────────────────────┘
│ 04-Team          │── member ────→
│   (invite/role)  │   changes
└──────────────────┘
```

## Notification Types

| Type | Icon | Source Feature | Example |
|------|------|---------------|---------|
| `alert` | 🚨 | 08-escalation | "PT Maju Jaya — bd_score=4, no reply after D+7" |
| `alert` | ⚠️ | 06-workflow | "5 clients have contracts ending within 30 days" |
| `alert` | 🛑 | 06-workflow | "Bot stopped: PT Logistik — P6.4 Overdue D+15" |
| `success` | ✅ | 07-invoices | "Payment confirmed: INV-KK-2026-012 marked as Paid" |
| `success` | 📧 | 05-messaging | "Email template 'AE Renewal' updated by arief@dealls.com" |
| `success` | 👤 | 04-team | "New team member: budi@kantorku.id joined as SDR Officer" |
| `workflow` | 🤝 | 06-workflow | "PT Tech Global moved from LEAD → PROSPECT" |
| `workflow` | 🔄 | 06-workflow | "Workflow 'AE Client Lifecycle' canvas saved" |
| `info` | 💬 | 06-workflow | "WA blast: SDR H+0 sent to 12 leads, 3 replies" |
| `info` | 📊 | 09-analytics | "Weekly report ready: Revenue Rp 2.4B MTD" |

## Deep-Link Mapping

Each notification navigates to specific context when clicked:

| Event | Navigate To |
|-------|-------------|
| Escalation (company) | `/pipeline/{pipeline-uuid}?search={company_name}` |
| Payment confirmed | `/invoices?search={invoice_id}` |
| Stage transition LEAD→PROSPECT | `/pipeline/{BD-uuid}?search={company_name}` |
| Stage transition PROSPECT→CLIENT | `/pipeline/{AE-uuid}?search={company_name}` |
| WA blast completed | `/activity-log?filter=bot` |
| Contract expiring | `/data-master?tab=attention` |
| Template updated | `/email-template?search={template_name}` |
| Canvas saved | `/workflow` |
| Bot stopped (overdue) | `/pipeline/{AE-uuid}?search={company_name}` |
| New member joined | `/team` |
| Weekly report ready | `/analytics` |

## Channels

Notifications delivered via 3 channels:

| Channel | When | Implementation |
|---------|------|----------------|
| **In-app** (bell icon) | All events | Store in DB, fetch via API, display in dropdown |
| **Telegram** | Escalations + urgent alerts | Telegram Bot API → team lead chat/group |
| **Email** | Weekly reports + critical alerts (optional) | SMTP or SendGrid |

---

## Database Schema

```sql
CREATE TABLE notifications (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  
  -- Who
  recipient_id    UUID REFERENCES users(id),     -- NULL = broadcast to workspace
  
  -- What
  type            VARCHAR(20) NOT NULL,           -- alert, success, workflow, info
  icon            VARCHAR(10) NOT NULL,           -- emoji: 🚨, ✅, 🤝, 💬
  message         TEXT NOT NULL,
  
  -- Where to go when clicked
  href            TEXT,                           -- deep-link path with query params
  
  -- Source
  source_feature  VARCHAR(30),                    -- workflow-engine, invoices, team, etc.
  source_id       UUID,                           -- related record ID (invoice, escalation, etc.)
  
  -- State
  read            BOOLEAN NOT NULL DEFAULT FALSE,
  read_at         TIMESTAMPTZ,
  
  -- Delivery
  telegram_sent   BOOLEAN DEFAULT FALSE,
  email_sent      BOOLEAN DEFAULT FALSE,
  
  -- Meta
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notif_recipient   ON notifications(workspace_id, recipient_id, read, created_at DESC);
CREATE INDEX idx_notif_unread      ON notifications(workspace_id, recipient_id) WHERE read = FALSE;
CREATE INDEX idx_notif_created     ON notifications(created_at DESC);
```

---

## API Endpoints

### GET `/notifications`
List notifications for current user.

```
Headers:
  Authorization: Bearer {token}
  X-Workspace-ID: {uuid}

Query:
  ?unread_only=true         (optional, default: false)
  &limit=20                 (default: 20, max: 50)
  &before={timestamp}       (cursor for pagination)
  &type=alert,success       (optional, comma-separated filter)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "type": "alert",
      "icon": "🚨",
      "message": "Escalation: PT Maju Jaya — bd_score=4, no reply after D+7.",
      "href": "/pipeline/0c85261e-.../search=PT+Maju+Jaya",
      "read": false,
      "source_feature": "escalation",
      "created_at": "2026-04-12T10:30:00Z"
    }
  ],
  "unread_count": 4,
  "meta": {
    "has_more": true,
    "next_cursor": "2026-04-12T08:00:00Z"
  }
}
```

### GET `/notifications/count`
Quick unread count (for badge).

```
Response 200:
{
  "unread": 4
}
```

### PUT `/notifications/{id}/read`
Mark single notification as read.

```
Response 200:
{
  "id": "uuid",
  "read": true,
  "read_at": "2026-04-12T10:35:00Z"
}
```

### PUT `/notifications/read-all`
Mark all notifications as read for current user in workspace.

```
Response 200:
{
  "updated": 4
}
```

### POST `/notifications` (internal — from other features)
Create notification from backend services. Not called from frontend.

```json
{
  "workspace_id": "uuid",
  "recipient_id": "uuid or null for broadcast",
  "type": "alert",
  "icon": "🚨",
  "message": "Escalation: PT Maju Jaya — bd_score=4",
  "href": "/pipeline/{bd-uuid}?search=PT+Maju+Jaya",
  "source_feature": "escalation",
  "source_id": "escalation-uuid",
  "channels": ["in_app", "telegram"]
}
```

---

## How Other Features Create Notifications

### From Workflow Engine (06):
```go
// In processNode() after stage transition:
notifService.Create(ctx, notif.CreateRequest{
    WorkspaceID:   wsID,
    RecipientID:   nil, // broadcast
    Type:          "workflow",
    Icon:          "🤝",
    Message:       fmt.Sprintf("Stage transition: %s moved from %s → %s", companyName, oldStage, newStage),
    Href:          fmt.Sprintf("/pipeline/%s?search=%s", targetPipelineUUID, url.QueryEscape(companyName)),
    SourceFeature: "workflow-engine",
    SourceID:      &recordID,
    Channels:      []string{"in_app"},
})
```

### From Invoices (07):
```go
// In Paper.id webhook handler after payment confirmed:
notifService.Create(ctx, notif.CreateRequest{
    WorkspaceID:   wsID,
    Type:          "success",
    Icon:          "✅",
    Message:       fmt.Sprintf("Payment confirmed: %s — Invoice %s marked as Paid", companyName, invoiceID),
    Href:          fmt.Sprintf("/invoices?search=%s", invoiceID),
    SourceFeature: "invoices",
    SourceID:      &invoiceUUID,
    Channels:      []string{"in_app"},
})
```

### From Escalation (08):
```go
// In escalation trigger:
notifService.Create(ctx, notif.CreateRequest{
    WorkspaceID:   wsID,
    RecipientID:   &teamLeadID, // specific person
    Type:          "alert",
    Icon:          "🚨",
    Message:       fmt.Sprintf("Escalation: %s — %s", companyName, reason),
    Href:          fmt.Sprintf("/pipeline/%s?search=%s", pipelineUUID, url.QueryEscape(companyName)),
    SourceFeature: "escalation",
    SourceID:      &escalationID,
    Channels:      []string{"in_app", "telegram"},
})
```

### From Team (04):
```go
// In member invite accepted:
notifService.Create(ctx, notif.CreateRequest{
    WorkspaceID:   wsID,
    Type:          "success",
    Icon:          "👤",
    Message:       fmt.Sprintf("New team member: %s joined as %s", email, roleName),
    Href:          "/team",
    SourceFeature: "team",
    Channels:      []string{"in_app"},
})
```

---

## Telegram Integration

For `alert` type notifications (escalations, bot stops, overdue):

```go
func sendTelegram(chatID string, message string) error {
    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", os.Getenv("TELEGRAM_BOT_TOKEN"))
    body := map[string]interface{}{
        "chat_id":    chatID,
        "text":       message,
        "parse_mode": "Markdown",
    }
    // POST to Telegram API
}
```

Telegram chat_id comes from `team_members.telegram_id` or `master_data.owner_telegram_id`.

---

## Frontend Implementation (Current)

```
Header.tsx:
  - Bell icon with unread badge count
  - Dropdown panel (396px) with notification list
  - 4 types: alert (red), success (green), workflow (purple), info (blue)
  - Click → mark read + navigate to href
  - "Mark all read" button
  - "View all activity →" footer link
  - Currently uses mock data (buildNotifications function)
  - TODO: Replace with GET /notifications API call
```

## Polling vs WebSocket

**Phase 1 (recommended)**: Polling
- Frontend polls `GET /notifications/count` every 30 seconds
- Full list fetched on bell click
- Simple, no infra needed

**Phase 2 (later)**: WebSocket / SSE
- Real-time push via Server-Sent Events
- `GET /notifications/stream` → EventSource
- Only if polling latency becomes a problem
