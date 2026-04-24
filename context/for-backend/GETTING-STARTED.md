# Getting Started — Backend Engineer Guide

## Cara Pakai Folder Ini

Folder `context/for-backend/features/` berisi **complete specification** untuk semua 9 feature yang perlu diimplementasi di backend Go + PostgreSQL.

### Langkah Kerja

```
1. Baca PROGRESS-SUMMARY.md        → lihat status keseluruhan
2. Baca DATABASE-FULL-SCHEMA.md     → semua CREATE TABLE dalam 1 file
3. Pilih feature sesuai urutan (01 → 09)
4. Baca folder feature tersebut:
   ├── 01-overview.md         → arsitektur + data flow
   ├── 02-database-schema.md  → CREATE TABLE + indexes
   ├── 03-api-endpoints.md    → REST API contract (request/response)
   ├── 03-golang-models.md    → Go structs + repository (jika ada)
   └── *-progress.md          → checklist TODO (DONE/NOT DONE)
5. Implement di Go project
6. Update progress.md setelah selesai
```

### Prompt untuk Claude Code di Backend Project

Copy-paste prompt ini di Claude Code backend project kamu:

```
Baca semua file di folder ./context/for-backend/features/{feature-name}/
Implement semua item yang berstatus "NOT DONE — Backend Required 🔴" di file progress.md.
Gunakan:
- PostgreSQL dari file 02-database-schema.md
- API endpoints dari file 03-api-endpoints.md  
- Go models dari file 03-golang-models.md (jika ada)
- Pastikan semua endpoint return response sesuai spec
```

### Tech Stack yang Diharapkan

| Component | Technology |
|-----------|-----------|
| Language | Go (Golang) |
| Framework | Gin atau Echo (HTTP) |
| Database | PostgreSQL 15+ |
| ORM/Query | sqlx atau pgx (raw SQL preferred) |
| Auth | JWT access + refresh tokens |
| Cache | Redis (optional, untuk rate limiting + search cache) |
| File Storage | S3-compatible (untuk Excel import/export) |
| Cron | Google Cloud Scheduler → HTTP endpoint |
| Messaging | HaloAI API (WA), Apollo API (Email), Telegram Bot API |
| Payment | Paper.id API + webhook |

### Environment Variables

```env
# Database
DATABASE_URL=postgres://user:pass@host:5432/bumi_dashboard

# Auth
JWT_SECRET=your-jwt-secret-256bit
SESSION_SECRET=your-session-secret
GOOGLE_CLIENT_ID=416879028044-xxx.apps.googleusercontent.com

# External APIs
HALOAI_API_URL=https://api.haloai.id
HALOAI_API_KEY=xxx
APOLLO_API_KEY=xxx
TELEGRAM_BOT_TOKEN=xxx
PAPERID_API_KEY=xxx
PAPERID_WEBHOOK_SECRET=xxx

# Storage
S3_BUCKET=bumi-dashboard-files
S3_REGION=ap-southeast-1

# Redis (optional)
REDIS_URL=redis://localhost:6379
```

### Database Migration Order

```sql
-- Run in this order (foreign key dependencies):
-- NOTE: No users/sessions tables — auth delegated to ms-auth-proxy
1.  workspaces
2.  whitelist, login_attempts (optional)
3.  workspace_members, workspace_settings, workspace_invitations, workspace_integrations
4.  user_preferences
5.  master_data, custom_field_definitions
6.  roles, role_permissions, team_members, member_workspace_assignments, role_workspace_scope
7.  message_templates, email_templates, template_variables, template_edit_logs
8.  workflows, workflow_nodes, workflow_edges, workflow_steps
9.  automation_rules, rule_change_logs
10. pipeline_tabs, pipeline_stats, pipeline_columns
11. invoices, invoice_line_items, invoice_sequences, payment_logs
12. action_logs, data_mutation_logs, team_activity_logs
13. notifications
14. approval_requests
```

### API Base URL

```
Production: https://api.bumi-dashboard.com/api/v1
Staging:    https://api-staging.bumi-dashboard.com/api/v1

All endpoints require:
  Authorization: Bearer {access_token}
  X-Workspace-ID: {workspace_uuid}    (except auth endpoints)
  Content-Type: application/json
```
