# Activity Log — Implementation Progress

## 2026-04-23 — Division filter + manual_send sync

### FE (shipped)
- **Division segmented filter** — BD / AE / SDR / CS / GUARD chips for filtering activity feed by division
- **manual_send flag indicator** — distinguishes human-initiated SEND (via Daily Task) vs cron-fired auto-blast on each row

### Backend spec (documented, implementation pending)
- Division-derivation prefix table added in `02-database-schema.md` — maps `trigger_id` prefix (e.g. `BD_`, `AE_`, `SDR_`) → division for fast filter queries without joins
- `manual_send` column ALTER + backfill plan added in `02-database-schema.md` — boolean column on `action_logs`, default false, backfill from existing `actor_type='human'` rows
- 5 recommended indexes added in `02-database-schema.md`:
  - `idx_action_logs_workspace_division_ts (workspace_id, division, timestamp DESC)`
  - `idx_action_logs_workspace_manual_send (workspace_id, manual_send, timestamp DESC)`
  - `idx_data_mutation_logs_workspace_actor (workspace_id, actor_email, timestamp DESC)`
  - `idx_team_activity_logs_workspace_action (workspace_id, action, timestamp DESC)`
  - `idx_unified_activity_logs_workspace_division_ts` (covering index for unified view)

### Open dependencies (backend)
- Implement filter query optimization — division filter must hit indexed prefix, not LIKE scan
- Implement `manual_send` write path from Daily Task SEND button — every manual SEND must INSERT with `manual_send=true`
- Backfill `manual_send` for historical rows where `actor_type='human'`
- Surface division derivation in unified feed response (so FE can render division chip without re-deriving)

### Cross-refs
- FE: `app/dashboard/[workspace]/activity-log/page.tsx`, `components/features/ActivityFeed.tsx`
- Daily Task SEND originates in 06-workflow-engine (Daily Task Validation Dashboard)
- 06-workflow-engine cron writes use `manual_send=false`; Daily Task SEND uses `manual_send=true`
- Backend gap doc: `claude/for-backend/08-activity-log/gap-division-derivation.md`, `gap-manual-send-flag.md`

---

> **Overall: 42% complete** (19/45 items done or partial)
> - Frontend/BFF: 76% done (16 done + 3 partial)
> - Backend (Go): 0% done (21 items not started)
> - Optional improvements: 0% done (5 items)

---

## DONE — Frontend/BFF ✅ (16 items)

| # | Item | File | Notes |
|---|------|------|-------|
| 1 | Unified activity log page | `app/dashboard/[workspace]/activity-log/page.tsx` | Full page with tab system: All / Bot / Human (Data + Team sub-tabs) |
| 2 | Three log categories (bot, data, team) | `activity-log/page.tsx` | UnifiedLog type with `category: 'bot' \| 'data' \| 'team'` and `actorType: 'bot' \| 'human'` |
| 3 | Tab system (All / Bot Otomasi / Aktivitas Pengguna) | `activity-log/page.tsx` | Main tabs + human sub-tabs (All, Data Changes, Team Activity) with counts |
| 4 | Workspace filter (holding view) | `activity-log/page.tsx` | Chips: All / Dealls / KantorKu / Sejutacita — only shown when isHolding |
| 5 | Stat cards (Total, Bot, Human, Escalation) | `activity-log/page.tsx` | 4 stat cards scoped to workspace, ignores tab/search filter |
| 6 | Free-text search | `activity-log/page.tsx` | Searches target, action, actor, detail fields |
| 7 | Day-grouped timeline | `activity-log/page.tsx` | Groups: "Today", "Yesterday", "3 April 2025", etc. Sorted desc |
| 8 | Bot action log API (BFF) | `app/api/action-log/recent/route.ts` | GET with limit, since, phase, status filters. Returns logs + today stats |
| 9 | Per-company summary API (BFF) | `app/api/action-log/summary/route.ts` | Aggregated per-company: total_sent, reply_rate, last_sent, has_active_escalation, current_phase |
| 10 | Data mutation log API (BFF) | `app/api/data-mutations/route.ts` | GET with company + limit. Supports holding (merged dealls + kantorku) |
| 11 | Per-company action log API (BFF) | `app/api/clients/[company_id]/action-log/route.ts` | Company-scoped logs with filter (all/replied/escalated/no_reply), pagination, summary stats |
| 12 | Per-company escalation API (BFF) | `app/api/clients/[company_id]/escalations/route.ts` | Returns mock escalation data for a company |
| 13 | Action log data store | `app/api/_lib/action-log-store.ts` | Loads seed JSON (132 entries) or falls back to mockActionLogs |
| 14 | Mutation log data store | `app/api/_lib/mutation-log-store.ts` | Per-company mutable stores (dealls 20 entries, kantorku 11 entries). `recordMutation()` for runtime inserts |
| 15 | Log filter/summary utilities | `app/api/_lib/log-utils.ts` | `applyFeedFilters`, `applyCompanyFilter`, `buildCompanySummary`, `buildTodayStats` |
| 16 | ActivityFeed sidebar component | `components/features/ActivityFeed.tsx` | Real-time feed with 60s polling, day grouping, filter pills, stats mini bar, escalation banner, fade-in animation for new entries |

## PARTIAL ⚠️ (3 items)

| # | Item | What's Done | What's Missing |
|---|------|-------------|----------------|
| 17 | Unified feed API | BFF endpoints exist separately for bot, data, team logs | Spec wants single `GET /activity-log/feed` with unified response shape (category, actor_type, search, stats). Currently 3 separate endpoints |
| 18 | Stat cards completeness | 4 cards shown (Total, Bot, Human, Escalation) | Spec wants 7 stats: Total, Today, Bot, Human, Data Mutations, Team Actions, Escalations. Missing: Today count as separate card, Data Mutations card, Team Actions card |
| 19 | Holding workspace badges | Workspace filter chips implemented | Log entries don't show workspace badge (DE/KK/SC) per-entry as spec requires. Workspace chip only shown on hover via border accent |

## NOT DONE — Backend (Go) Required 🔴 (21 items)

### Critical (blocks real data)

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 20 | `action_logs` table extensions | 02-database-schema | Add columns: actor_type, actor_name, actor_email, reply_text to existing table |
| 21 | `data_mutation_logs` table | 02-database-schema | New table: action, actor_email, actor_name, master_data_id, company_id/name, changed_fields, previous_values, new_values, count, note |
| 22 | `team_activity_logs` table | 02-database-schema | New table: action, actor_email/name, target_name/email/id, detail |
| 23 | `unified_activity_logs` view | 02-database-schema | UNION ALL view across all 3 log tables for unified query |
| 24 | GET `/activity-log/feed` | 03-api-endpoints | Unified feed endpoint with category/actor_type/search/workspace filters + pagination + stats |

### High Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 25 | GET `/action-log/recent` (backend) | 03-api-endpoints | Bot action log feed from real DB with limit/since/phase/status. Currently BFF mock |
| 26 | GET `/action-log/summary` (backend) | 03-api-endpoints | Per-company aggregated summary from real action_logs |
| 27 | GET `/data-mutations` (backend) | 03-api-endpoints | Data mutation feed from real DB with limit/since/action/actor filters + pagination meta |
| 28 | GET `/team/activity` (backend) | 03-api-endpoints | Team activity feed from real DB with limit/since/action/workspace filters + today_stats |
| 29 | GET `/activity-log/today` (backend) | 03-api-endpoints | Quick today stats endpoint: total, bot, human, data_mutations, team_actions, escalations, by_workspace |
| 30 | Auto-record data mutations | 02-database-schema | INSERT into data_mutation_logs on PUT/POST/DELETE master-data endpoints |
| 31 | Auto-record team activity | 02-database-schema | INSERT into team_activity_logs on team management endpoints (invite, role change, policy update, etc.) |

### Escalation System

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 32 | `escalations` table | 04-escalation | id, workspace_id, master_data_id, trigger_id, priority (P0/P1/P2), trigger_condition, reason, assigned_to, status, resolved_at/by, notified_via/at |
| 33 | GET `/escalations` | 04-escalation | List escalations with status/priority/limit filters |
| 34 | GET `/clients/{company_id}/escalations` | 04-escalation | Per-company escalation history. Currently BFF returns mock data |
| 35 | PUT `/escalations/{id}/resolve` | 04-escalation | Resolve/dismiss escalation with resolution_note |
| 36 | POST `/escalations` (internal) | 04-escalation | Auto-created by workflow engine cron when conditions met (bd_score >= 4, SLA breach, etc.) |
| 37 | Escalation trigger conditions | 04-escalation | 7 trigger rules: SDR silent high-value, BD score, FP overdue, AE usage, AE overdue, CS SLA, CS CSAT |
| 38 | Telegram notification | 04-escalation | Send escalation alerts via Telegram to assigned team lead |

### Medium Priority

| # | Item | Spec File | Description |
|---|------|-----------|-------------|
| 39 | DB indexes for activity log tables | 02-database-schema | All indexes defined in spec: workspace+timestamp, action, actor_email, master_data_id |
| 40 | Retention policy implementation | 03-api-endpoints | data_mutation_logs: 1 year retention + archive to cold storage |

## NOT DONE — Optional Frontend Improvements 🟡 (5 items)

| # | Item | Priority | Description |
|---|------|----------|-------------|
| 41 | Summary panel (sidebar) | High | "Ringkasan Hari Ini" — bot vs human split, per-company aggregation with reply_rate, last_sent, has_active_escalation, current_phase |
| 42 | Real-time polling for team activity | Medium | Spec: poll team activity every 30s. Currently only bot feed polled at 60s |
| 43 | Workspace badge per log entry | Medium | Show DE/KK/SC badge on each log entry in holding view |
| 44 | Previous/new value diff display | Low | Show before/after values for data mutations (previous_values, new_values from spec) |
| 45 | Materialized view for holding | Low | Pre-aggregated view refreshed every 5 min for holding workspace performance |

---

## Recommended Implementation Order (Backend)

```
Week 1: #20 action_logs extensions + #21 data_mutation_logs + #22 team_activity_logs + #23 unified view
Week 2: #24 unified feed endpoint + #25 action-log/recent + #29 today stats
Week 3: #26 summary + #27 data-mutations + #28 team/activity + #30 auto-record mutations + #31 auto-record team
Week 4: #32 escalations table + #33-36 escalation CRUD + #37 trigger conditions
Week 5: #38 Telegram notification + #39 indexes + #40 retention
```

## Dependency Chain

```
action_logs (extend) ──→ unified_activity_logs view ──→ GET /activity-log/feed
                                │
data_mutation_logs ─────────────┤
  │                             │
  └──→ auto-record on PUT/POST/DELETE master-data
                                │
team_activity_logs ─────────────┘
  │
  └──→ auto-record on team management endpoints

escalations table ──→ GET /escalations ──→ trigger conditions (cron)
  │                                              │
  └──→ PUT /resolve                              └──→ Telegram notification
```
