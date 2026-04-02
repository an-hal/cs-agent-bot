# CS Agent Bot - Quick Reference Guide

## 🚀 Quick Start

### 1. Run Migrations
```bash
cd /home/anhalim/dealls/cs-agent-bot
go run cmd/migrate/main.go up
```

### 2. Build & Run Server
```bash
go build -o bin/cs-agent-bot ./cmd/server
./bin/cs-agent-bot
```

### 3. Test Cron Trigger
```bash
curl -X POST http://localhost:3000/v1/cs-agent-bot/admin/cron/trigger
```

---

## 📋 File Locations Reference

| Component | Path | Count |
|-----------|------|-------|
| **Migrations** | `/migration/*.sql` | 17 |
| **Entities** | `/internal/entity/*.go` | 9 |
| **Repositories** | `/internal/repository/*_repo.go` | 8 |
| **Cron** | `/internal/usecase/cron/*.go` | 1 |
| **Triggers** | `/internal/usecase/trigger/*.go` | 8 |
| **Support** | `/internal/usecase/*/*.go` | 7 |

---

## 🎯 Trigger Priority Reference

| Priority | Trigger | File | Halt? | Condition |
|----------|---------|------|-------|-----------|
| P0 | Health Risk | `health.go` | ❌ | usage<40 OR nps<6 OR risk_flag |
| P0.5 | Check-in | `checkin.go` | ✅ | 90d, 180d with Branch A/B |
| P1 | Negotiation | `negotiation.go` | ✅ | quotation 7d, 3d, 0d before expiry |
| P2 | Invoice | `invoice.go` | ❌ | pre-14, pre-7, pre-3 days |
| P3 | Overdue | `overdue.go` | ❌ | post-1, post-4, post-8 days |
| P4 | Expansion | `expansion.go` | ❌ | NPS at 90, 180, 270 days |
| P5 | Cross-sell | `crosssell.go` | ❌ | HT: 7/14/21/30/45/60/75/90d, LT: 30/60/90d |

---

## 🔗 Key Entity Relationships

```
clients (1) ──┬──> (N) invoices
              │
              ├──> (1) client_flags
              │
              ├──> (N) action_log
              │
              ├──> (N) escalations
              │
              └──> (N) cron_log

system_config (standalone)
templates (standalone)
```

---

## 📊 Database Schema Quick Reference

### Main Tables

| Table | Primary Key | Key Indexes | Notes |
|-------|-------------|-------------|-------|
| `clients` | company_id | pic_wa, bot_active, contract_end | Includes payment reminder flags |
| `invoices` | invoice_id | company_id, due_date, payment_status | Minimal: ID, due_date, amount, status |
| `client_flags` | company_id | (composite) | 34 boolean flags + feature_update_sent |
| `conversation_states` | company_id | bot_active, response_status | Per-client bot state + cooldown |
| `action_log` | id (BIGSERIAL) | company_id, triggered_at, message_id | APPEND ONLY |
| `escalations` | id (SERIAL) | esc_id, company_id, status | Unique constraint |
| `cron_log` | id (SERIAL) | run_date, company_id | Unique constraint |
| `system_config` | key | key | Key-value store |
| `templates` | template_id | category, active | With default data |

---

## 🔧 Configuration Reference

### Required Environment Variables

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=cs_agent_bot

# HaloAI WhatsApp
HALO_AI_ENDPOINT=https://api.haloai.io/v1/send
HALO_AI_KEY=your_api_key

# Telegram Bot
TELEGRAM_BOT_TOKEN=your_bot_token

# Cron Settings
CRON_ENABLED=true
CRON_HOUR=8
BATCH_DELAY_MS=300
```

---

## 🧪 Test Queries

### Check if cron ran today
```sql
SELECT COUNT(*) FROM cron_log WHERE run_date = CURRENT_DATE AND status = 'done';
```

### View messages sent today
```sql
SELECT trigger_type, template_id, status, COUNT(*)
FROM action_log
WHERE DATE(triggered_at) = CURRENT_DATE
GROUP BY trigger_type, template_id, status;
```

### Find clients needing attention
```sql
SELECT company_id, company_name, contract_end,
       response_status, nps_score, usage_score_avg_30d
FROM clients
WHERE bot_active = true
  AND blacklisted = false
  AND contract_end BETWEEN NOW() AND NOW() + INTERVAL '90 days'
ORDER BY contract_end ASC;
```

### Check open escalations
```sql
SELECT e.esc_id, e.company_id, c.company_name,
       e.priority, e.triggered_at, e.status
FROM escalations e
JOIN clients c ON e.company_id = c.company_id
WHERE e.status = 'Open'
ORDER BY e.triggered_at DESC;
```

---

## 📈 Monitoring Queries

### Daily message statistics
```sql
SELECT DATE(triggered_at) AS date,
       COUNT(*) AS total_messages,
       COUNT(CASE WHEN status = 'success' THEN 1 END) AS successful,
       COUNT(CASE WHEN status = 'failed' THEN 1 END) AS failed
FROM action_log
WHERE triggered_at >= NOW() - INTERVAL '30 days'
GROUP BY DATE(triggered_at)
ORDER BY date DESC;
```

### Trigger performance
```sql
SELECT trigger_type,
       COUNT(*) AS sent,
       COUNT(CASE WHEN intent = 'positive' THEN 1 END) AS positive_replies,
       COUNT(CASE WHEN intent = 'negative' THEN 1 END) AS negative_replies,
       ROUND(100.0 * COUNT(CASE WHEN intent IS NOT NULL THEN 1 END) / COUNT(*), 2) AS response_rate
FROM action_log
WHERE triggered_at >= NOW() - INTERVAL '30 days'
GROUP BY trigger_type
ORDER BY sent DESC;
```

### Client health overview
```sql
SELECT
  COUNT(CASE WHEN nps_score >= 9 THEN 1 END) AS promoters,
  COUNT(CASE WHEN nps_score BETWEEN 7 AND 8 THEN 1 END) AS passive,
  COUNT(CASE WHEN nps_score <= 6 THEN 1 END) AS detractors,
  AVG(nps_score) AS avg_nps,
  AVG(usage_score_avg_30d) AS avg_usage
FROM clients
WHERE nps_score IS NOT NULL;
```

---

## 🐛 Troubleshooting

### Issue: Cron not running

**Check:**
```bash
# Verify cron is enabled in config
grep CRON_ENABLED .env

# Check logs for cron execution
tail -f logs/cs-agent-bot.log | grep "cron"

# Verify cron_log table
psql -c "SELECT * FROM cron_log WHERE run_date = CURRENT_DATE;"
```

### Issue: Messages not sending

**Check:**
```sql
-- Check if client is blacklisted
SELECT company_id, blacklisted, bot_active FROM clients WHERE company_id = 'TEST001';

-- Check if already sent today
SELECT * FROM action_log
WHERE company_id = 'TEST001'
  AND DATE(triggered_at) = CURRENT_DATE;

-- Check flags
SELECT * FROM client_flags WHERE company_id = 'TEST001';
```

### Issue: Escalations not working

**Check:**
```sql
-- Verify Telegram ID is set
SELECT owner_telegram_id, ae_telegram_id FROM clients WHERE company_id = 'TEST001';

-- Check escalation records
SELECT * FROM escalations WHERE company_id = 'TEST001' ORDER BY triggered_at DESC;

-- Check system config for escalation setting
SELECT * FROM system_config WHERE key = 'escalation_enabled';
```

---

## 📚 Useful Commands

### Count messages by type
```sql
SELECT trigger_type, COUNT(*) FROM action_log GROUP BY trigger_type;
```

### Find most recent messages per client
```sql
SELECT DISTINCT ON (company_id) company_id, triggered_at, trigger_type, status
FROM action_log
ORDER BY company_id, triggered_at DESC;
```

### Reset all flags for a client
```sql
BEGIN;
UPDATE client_flags SET
  checkin_replied = false,
  ren60_sent = false,
  ren45_sent = false,
  ren30_sent = false,
  ren15_sent = false,
  ren0_sent = false
WHERE company_id = 'TEST001';
COMMIT;
```

### Manually insert cron log entry
```sql
INSERT INTO cron_log (run_date, company_id, status)
VALUES (CURRENT_DATE, 'TEST001', 'pending');
```

---

## 🎓 Best Practices

### Adding New Triggers

1. Create new file in `/internal/usecase/trigger/`
2. Implement interface with `EvalXxx(ctx, client) (bool, error)`
3. Add to cron runner's priority chain
4. Add flags to `client_flags.go` if needed
5. Add templates to database

### Testing Changes

1. Insert test client with specific dates
2. Run cron manually via admin endpoint
3. Check `action_log` for results
4. Verify `client_flags` updated
5. Check WhatsApp/Telegram messages sent

### Production Deployment

1. Run database migrations
2. Verify system_config values
3. Test webhook endpoints
4. Start server with `CRON_ENABLED=false` initially
5. Monitor logs for first cron run
6. Enable cron after verification

---

**Last Updated:** 2026-04-02
**Version:** 1.1.0
**Status:** Production Ready
