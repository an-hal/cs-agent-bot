# cs-agent-bot — Project Context (for Claude)

Snapshot aktual 2026-04-24. Coverage spec `context/for-backend/` **±97%**
(3% sisa = swap mock→real untuk 4 integrasi eksternal yang butuh API key).

| File | Contents |
|---|---|
| [01-overview.md](01-overview.md) | Mission, capabilities, tech stack, external integrations |
| [02-architecture.md](02-architecture.md) | Clean Architecture layers, entry points, directory tree, DI wiring |
| [03-business-rules.md](03-business-rules.md) | 12 critical business rules, P0–P5 trigger priority, intent classifier, escalation ESC-001…ESC-006 |
| [04-api-and-auth.md](04-api-and-auth.md) | HTTP server, middleware stack, auth by route, response format |
| [05-database.md](05-database.md) | PostgreSQL engine, migration strategy, core table catalog, payment-flag locations |
| [06-domain-features.md](06-domain-features.md) | Workspaces, master data, teams, templates, workflow engine, invoices, activity log, analytics, collections |
| [07-dev-workflow.md](07-dev-workflow.md) | Makefile commands, required env vars, testing approach, Docker, observability |
| [08-gap-vs-fe-specs.md](08-gap-vs-fe-specs.md) | Initial gap analysis — kini mayoritas CLOSED |
| [09-session-coverage-2026-04-23.md](09-session-coverage-2026-04-23.md) | Sesi A→B→C→Postman→Wave 1-5: 9 tabel, 27 endpoint, 12 usecase, 11 test baru |
| [10-session-coverage-2026-04-24.md](10-session-coverage-2026-04-24.md) | Wave A (10 item) + Wave B1-3 (11 item) → push 85% ke 97% |
| [11-endpoint-catalog.md](11-endpoint-catalog.md) | Katalog lengkap ±245 route by kategori |
| [12-integration-state.md](12-integration-state.md) | Status 6 integrasi eksternal — mana real, mana mock, cara swap |

## Quick Facts (current state)

- **Language:** Go 1.25 — custom `net/http` router, no framework.
- **Data plane:** PostgreSQL (`pgx/v5`) + Redis (`go-redis/v9`).
- **Entry points:** GCP Cloud Scheduler (cron) + HaloAI webhook + JWT dashboard.
- **Test suite:** 34/34 packages pass, 0 FAIL.
- **Routes:** ±245 (up from ~170 pre-April-23).
- **Approval types wired:** 8 (create_invoice, mark_invoice_paid, collection_schema_change, delete_client_record, toggle_automation_rule, bulk_import_master_data, stage_transition, integration_key_change).
- **External integrations:** 6 total — 2 real (Paper.id, Telegram), 4 mock+real-ready (Claude/Fireflies/HaloAI-send/SMTP).
- **Security features:** AES-256-GCM secret vault, PDP erasure + retention, session revocation, audit cross-workspace, mutation log source tagging, CSV formula injection guards.

## Critical invariants (never relax without product decision)

1. `blacklisted` checked BEFORE `bot_active`, always first.
2. P2 + P3 never halt on reply (payment collection continues).
3. Bot never writes `payment_status`, `renewed`, `rejected` — only AE via Dashboard (exception: Paper.id webhook for `payment_status=Lunas`).
4. `action_log` is INSERT only (DB-level REVOKE UPDATE/DELETE).
5. Template send guard: any remaining `[variable]` in resolved body → abort send.
6. Return HTTP 200 to HaloAI webhook ≤5s; processing runs in goroutine.
7. Manual-flow triggers (20 in registry) go to `manual_action_queue` — never bot-sent.

## Directory structure highlights (current)

```
internal/
  usecase/         # 40 subpackages
    approval/           # central dispatcher for 8 types
    audit_workspace_access/
    claude_client/      # mock + noop; real SDK swap ready
    claude_extraction/  # + FirefliesBridge
    coaching/
    fireflies/
    fireflies_client/   # mock + noop
    haloai_mock/
    manual_action/      # GUARD queue
    mockoutbox/         # shared in-memory outbox
    pdp/                # erasure + retention + SQL enforcer
    reactivation/
    rejection_analysis/
    smtp_client/        # real + mock
    user_preferences/
    workspace_integration/  # + AES vault
  pkg/
    rediscache/         # analytics 15-min cache
    secretvault/        # AES-256-GCM
    xlsxexport/         # + CSV injection guard
    xlsximport/
    apperror/, ctxutil/, conditiondsl/, filterdsl/, workday/, htmlsanitize/, etc.
  repository/      # 40+ repos incl. new: activity_feed, team_activity, revoked_sessions, workspace_theme, pdp
  entity/          # 30+ entities incl. new: UserPreference, WorkspaceIntegration, ManualAction, PDPErasureRequest, CoachingSession, RejectionAnalysis, etc.
migration/         # 160+ files, latest: 20260424000030_create_revoked_sessions
docs/postman/      # cs-agent-bot.postman_collection.json (228 requests) + environment
```

## Next steps (optional, outside code-only scope)

See `12-integration-state.md` for the 4 third-party swap items when
credentials are available. Each swap is ~2-4 hours of work with the
interface already defined.
