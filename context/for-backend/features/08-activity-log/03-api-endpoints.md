# API Endpoints — Activity Log

## Base URL
```
{BACKEND_API_URL}/api/v1
```

All endpoints require `Authorization: Bearer {token}` header.
Workspace-scoped endpoints require `X-Workspace-ID: {uuid}` header.

---

## 1. Unified Activity Feed

### GET `/activity-log/feed`
Unified feed gabungan dari semua log types. Dipakai halaman Activity Log utama.

```
Query params:
  ?limit=50                          (default 50, max 200)
  &offset=0                          (pagination)
  &since=2026-04-01T00:00:00Z       (optional, return entries after this timestamp)
  &category=bot                      (optional: bot | data | team)
  &actor_type=human                  (optional: bot | human)
  &workspace_id=uuid                 (optional, for holding view — filter specific workspace)
  &search=keyword                    (optional, searches target, action, actor, detail)
  &sort_dir=desc                     (optional: asc/desc, default desc)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "workspace_id": "uuid",
      "workspace_slug": "dealls",
      "category": "bot",
      "actor_type": "bot",
      "actor": "Bot Otomasi",
      "action": "Kirim WA Renewal",
      "target": "PT Maju Digital",
      "detail": "Trigger: Renewal_H30 · Terkirim, belum reply",
      "status": "delivered",
      "timestamp": "2026-04-05T15:44:00Z"
    },
    {
      "id": "uuid",
      "workspace_id": "uuid",
      "workspace_slug": "dealls",
      "category": "data",
      "actor_type": "human",
      "actor": "Arief Faltah",
      "action": "edit_client",
      "target": "PT Surya Gemilang",
      "detail": "Ubah: Payment_Status, Last_Payment_Date",
      "status": null,
      "timestamp": "2026-04-05T11:05:00Z"
    }
  ],
  "meta": {
    "offset": 0,
    "limit": 50,
    "total": 312
  },
  "stats": {
    "total": 312,
    "today": 13,
    "bot": 145,
    "human": 167,
    "data_mutations": 95,
    "team_actions": 72,
    "escalations": 8
  }
}
```

Backend logic:
1. If holding workspace → query all member workspace_ids
2. Build UNION ALL query across `action_logs`, `data_mutation_logs`, `team_activity_logs`
3. Apply category / actor_type / search filters
4. Order by timestamp DESC
5. Compute stats from unfiltered (but workspace-scoped) data

---

## 2. Bot Action Logs

### GET `/action-log/recent`
Bot action log feed — dipakai oleh ActivityFeed sidebar component.

```
Query params:
  ?limit=50                          (default 50, max 200)
  &since=2026-04-01T00:00:00Z       (optional — for polling, return entries after this)
  &phase=P0                          (optional: P0..P6, ESC)
  &status=delivered                  (optional: delivered, escalated, manual, failed)

Response 200:
{
  "logs": [
    {
      "log_id": "uuid",
      "company_id": "DE-001",
      "company_name": "PT Maju Digital",
      "trigger_id": "Renewal_H30",
      "template_id": "TPL-RENEWAL-H30",
      "status": "delivered",
      "channel": "whatsapp",
      "replied": false,
      "reply_text": null,
      "timestamp": "2026-04-05T15:44:00Z"
    }
  ],
  "total_today": 5,
  "replied_today": 2,
  "reply_rate_today": 40,
  "escalations_today": 1
}
```

### GET `/action-log/summary`
Per-company aggregated summary. Dipakai sidebar company list.

```
Response 200:
[
  {
    "company_id": "DE-001",
    "company_name": "PT Maju Digital",
    "total_sent": 12,
    "total_replied": 5,
    "reply_rate": 42,
    "last_sent_at": "2026-04-05T15:44:00Z",
    "last_trigger_id": "Renewal_H30",
    "last_status": "delivered",
    "has_active_escalation": false,
    "current_phase": "P4"
  }
]
```

Backend logic:
- Group action_logs by company_id (via master_data_id -> master_data.company_id)
- For each company: count sent/replied, find latest, check escalation in last 7 days
- Sort by last_sent_at DESC

---

## 3. Data Mutation Logs

### GET `/data-mutations`
Data mutation feed — riwayat perubahan Master Data.

Sudah didefinisikan di master-data spec sebagai `GET /master-data/mutations`.
Endpoint ini adalah **alias** yang sama.

```
Query params:
  ?limit=50                          (default 50, max 200)
  &since=2026-04-01T00:00:00Z       (optional)
  &action=edit_client                (optional: add_client, edit_client, delete_client, import_bulk, export_bulk)
  &actor=arief@bumi.id              (optional: filter by actor email)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "action": "edit_client",
      "actor_email": "arief@bumi.com",
      "actor_name": "Arief Faltah",
      "timestamp": "2026-04-05T11:05:00Z",
      "company_id": "DE-014",
      "company_name": "PT Surya Gemilang",
      "changed_fields": ["Payment_Status", "Last_Payment_Date"],
      "previous_values": { "Payment_Status": "Menunggu", "Last_Payment_Date": null },
      "new_values": { "Payment_Status": "Lunas", "Last_Payment_Date": "2026-04-05" }
    },
    {
      "id": "uuid",
      "action": "import_bulk",
      "actor_email": "arief@bumi.com",
      "actor_name": "Arief Faltah",
      "timestamp": "2026-04-04T13:00:00Z",
      "company_id": null,
      "company_name": null,
      "count": 3,
      "note": "Import dari Google Sheets"
    }
  ],
  "meta": {
    "offset": 0,
    "limit": 50,
    "total": 95
  }
}
```

Holding view: jika holding workspace, return logs dari semua member workspaces
(sorted by timestamp DESC, merged).

---

## 4. Team Activity Logs

### GET `/team/activity`
Team activity feed — riwayat manajemen tim & RBAC changes.

```
Query params:
  ?limit=50                          (default 50, max 200)
  &since=2026-04-01T00:00:00Z       (optional)
  &action=invite_member              (optional filter by action type)
  &workspace_id=uuid                 (optional, for holding view)

Response 200:
{
  "data": [
    {
      "id": "uuid",
      "action": "invite_member",
      "actor_email": "arief@bumi.id",
      "actor_name": "Arief",
      "target_name": "Galih Nugroho",
      "target_email": "galih@kantorku.id",
      "workspace_id": "uuid",
      "workspace_slug": "kantorku",
      "detail": "Role: SDR Officer",
      "timestamp": "2026-04-05T09:14:00Z"
    }
  ],
  "meta": {
    "offset": 0,
    "limit": 50,
    "total": 20
  },
  "today_stats": {
    "total": 2,
    "invite": 1,
    "role": 0,
    "policy": 1,
    "access": 0
  }
}
```

---

## 5. Today Stats (quick summary)

### GET `/activity-log/today`
Stats singkat untuk hari ini — dipakai stat cards di halaman Activity Log.

```
Response 200:
{
  "total": 13,
  "bot": 5,
  "human": 8,
  "data_mutations": 6,
  "team_actions": 2,
  "escalations": 1,
  "by_workspace": {
    "dealls": 9,
    "kantorku": 4
  }
}
```

---

## Design Notes

### Automatic Log Creation
Backend HARUS otomatis mencatat log setiap kali:
- Workflow engine mengirim pesan (action_logs — sudah ada)
- User mengubah master_data (data_mutation_logs — triggered oleh PUT/POST/DELETE master-data)
- User melakukan aksi tim (team_activity_logs — triggered oleh team management endpoints)

### Retention Policy
- action_logs: keep forever (audit trail)
- data_mutation_logs: keep 1 year, archive ke cold storage setelahnya
- team_activity_logs: keep forever (compliance)

### Performance
- Unified feed query bisa berat (UNION ALL 3 tables)
- Rekomendasi: cache today stats per workspace (invalidate on new log)
- Polling endpoint (`?since=`) harus cepat — index pada timestamp sudah ada
- Untuk holding workspace, pertimbangkan pre-aggregated materialized view
  yang di-refresh tiap 5 menit
