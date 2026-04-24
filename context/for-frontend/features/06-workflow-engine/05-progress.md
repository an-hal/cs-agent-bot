# Workflow Engine — Implementation Progress

Backend scope in `feat/06-workflow-engine` + follow-on items.

## DONE ✅

| # | Item | Notes |
|---|---|---|
| 1 | `workflows` table | Canvas metadata |
| 2 | `workflow_nodes` | Node definitions |
| 3 | `workflow_edges` | Transitions |
| 4 | `workflow_steps` | Step config |
| 5 | `automation_rules` | Rule definitions + change logs |
| 6 | `manual_action_queue` | GUARD overlay (feat/06 §07) |
| 7 | `/workflows/*` CRUD endpoints | Full canvas + steps + config |
| 8 | `/automation-rules/*` CRUD endpoints | + change-log endpoint |
| 9 | `toggle_automation_rule` approval | Status changes gated through checker-maker |
| 10 | ApplyToggleStatus executor | Approvals apply via central dispatcher |
| 11 | Channel dispatcher (WA/email/Telegram/escalate/alert/handoff) | `channel_dispatcher.go` |
| 12 | Manual-flow interception | 20 trigger IDs routed to `manual_action_queue` before channel dispatch |
| 13 | WA sender port | `WASender` interface; mock sender wired in dev |
| 14 | Timing parser (dual format) | `H-90`, `D+14`, "90 hari sebelum kontrak berakhir" etc. |
| 15 | Template resolution priority | `template.ResolvePriority(candidates[])` |
| 16 | Low-intent BD sequence skip | D12/D14/D21 skipped when `bants_classification=cold` |
| 17 | Workday + holiday check | `pkg/workday` integration |
| 18 | Condition DSL | `pkg/conditiondsl` for rule condition evaluation |
| 19 | Manual action endpoints (list/get/mark-sent/skip) | See [../00-shared/04-manual-actions.md](../00-shared/04-manual-actions.md) |
| 20 | Pipeline view | Per-workflow data aggregation endpoint |

## PENDING — requires alignment decisions

| # | Item | Blocker |
|---|---|---|
| 1 | Real WA send from cron dispatcher | Needs `WA_API_TOKEN` + swap adapter in `main.go` (mock works today) |
| 2 | TipTap → HTML server-side | FE extension set needs alignment |
| 3 | Auto-expiry escalation with Telegram fan-out | Telegram notifier wiring for `ManualActionUsecase` is nil today (works without) |

## Summary

Workflow engine is **±95% done**. Canvas + rules + dispatcher + manual
overlay + low-intent skip + timing parser all live. Only the WA-send
adapter swap remains for production.
