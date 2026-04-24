# Activity Log — Backend Implementation Guide

## Context
Activity Log adalah audit trail terpusat untuk seluruh aktivitas di dashboard.
Semua aksi — baik otomatis (bot) maupun manual (pengguna) — dicatat dan bisa ditelusuri.
Ini penting untuk compliance, debugging workflow, dan visibilitas operasional.

## Tiga Jenis Log

### 1. Bot Action Logs (`action_logs`)
Aksi otomatis dari workflow engine: kirim WA, eskalasi, NPS survey, dll.
Sudah didefinisikan di master-data spec (`action_logs` table) — diperluas di sini.

Contoh entry:
- Bot Otomasi > Kirim WA Renewal > PT Maju Digital > Trigger: Renewal_H30 > delivered
- Bot Otomasi > Eskalasi ke AE > PT Techno Building > Trigger: Overdue_H14 > escalated
- Bot Otomasi > Reply Diterima > PT Sentosa Office > Klasifikasi: Positive

### 2. Data Mutation Logs (`data_mutation_logs`)
Setiap perubahan pada master_data yang dilakukan oleh pengguna:
- `add_client` — tambah klien baru
- `edit_client` — ubah field (tracked: field mana yang berubah)
- `delete_client` — hapus klien
- `import_bulk` — import Excel/CSV (tracked: jumlah baris)
- `export_bulk` — export data (tracked: jumlah baris)

### 3. Team Activity Logs (`team_activity_logs`)
Aksi yang berkaitan dengan manajemen tim & RBAC:
- `invite_member` — undang member baru
- `update_role` — ubah role member
- `update_policy` — ubah permission matrix suatu role
- `activate_member` / `deactivate_member` — toggle status member
- `remove_member` — hapus member dari workspace
- `create_role` — buat role baru
- `reset_password` — reset password member

## Arsitektur Unified Log View

Frontend menampilkan ketiga jenis log dalam satu halaman dengan tab system:

```
┌──────────────────────────────────────────────┐
│  Activity Log                                │
│                                              │
│  [Semua] [Bot Otomasi] [Aktivitas Pengguna]  │
│                            ├─ [Semua]        │
│                            ├─ [Mutasi Data]  │
│                            └─ [Aktivitas Tim] │
│                                              │
│  Workspace filter (holding only):            │
│  [Semua] [Dealls] [KantorKu] [Sejutacita]   │
└──────────────────────────────────────────────┘
```

## Filtering & Grouping

### Filter Dimensions
| Dimension     | Values                                              | Applicable to      |
|---------------|------------------------------------------------------|---------------------|
| Actor Type    | `bot`, `human`                                       | All logs            |
| Category      | `bot`, `data`, `team`                                | All logs            |
| Workspace     | `dealls`, `kantorku`, `holding`                      | All logs            |
| Phase         | `P0`..`P6`, `ESC`                                    | Bot action logs     |
| Status        | `delivered`, `escalated`, `manual`, `failed`          | Bot action logs     |
| Action        | `add_client`, `edit_client`, `delete_client`, etc.   | Data mutation logs  |
| Team Action   | `invite_member`, `update_role`, `update_policy`, etc.| Team activity logs  |
| Search        | Free text (target, action, actor, detail)            | All logs            |

### Grouping
- Logs dikelompokkan per hari: "Hari ini", "Kemarin", "3 April 2025", dst.
- Dalam setiap grup, diurutkan descending by timestamp.

## Stat Cards (di atas timeline)
Dihitung dari log yang masuk scope workspace (bukan dari filter tab/search):

| Stat                  | Kalkulasi                                              |
|-----------------------|--------------------------------------------------------|
| Total Log             | Count semua log dalam scope workspace                  |
| Hari ini              | Count log dengan timestamp = today                     |
| Bot Otomasi           | Count log dengan actorType = 'bot'                     |
| Aktivitas Pengguna    | Count log dengan actorType = 'human'                   |
| Mutasi Data           | Count log dengan category = 'data'                     |
| Aktivitas Tim         | Count log dengan category = 'team'                     |
| Eskalasi              | Count log dengan status = 'escalated'                  |

## Polling & Real-time
- Frontend melakukan polling setiap 60 detik ke `/action-log/recent?since={lastTimestamp}`
- Baru masuk entries di-highlight dengan animasi (fade-in-down)
- Team activity juga di-poll setiap 30 detik

## Holding View
Kalau workspace aktif = holding (Sejutacita):
- Tampilkan log dari **semua** member workspaces
- Tambah workspace filter chips: [Semua] [Dealls] [KantorKu] [Sejutacita]
- Setiap log entry memiliki workspace badge (DE / KK / SC)

## Summary Panel (sidebar kanan)
- "Ringkasan Hari Ini" — bot vs human split, today action count
- Company-level aggregation (per company_id):
  - total_sent, total_replied, reply_rate
  - last_sent_at, last_trigger_id, last_status
  - has_active_escalation (any escalated in last 7 days)
  - current_phase (derived from latest trigger_id)
