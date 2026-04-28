# FE Agent Prompt — Import Wizard Implementation

> Copy-paste this entire block into your FE agent (Cursor / Claude Code / etc.) as the initial prompt. The agent should not need to ask follow-up questions because the spec doc + endpoints are referenced inline.

---

## Prompt

You are implementing an OneSchema-style spreadsheet import wizard in the dashboard. The backend is fully built and tested (Phase A schema + detect, Phase B type coercion, Phase C inline cell editing via sessions).

**Repo:** `/Users/macbook/Project/dealls/project-bumi-dashboard` (Next.js + React + TypeScript)
**Mount point:** `/dashboard/{workspace}/data-master` — the existing "Import" card opens this wizard
**Backend repo (read-only reference):** `/Users/macbook/Project/dealls/cs-agent-bot`
**Full spec:** `context/for-frontend/09-import-wizard.md` — read this first, all endpoint contracts and component architecture are documented there.

### Goal

User opens import → drops xlsx (any sheet/headers, locale-ID dates and currency) → wizard auto-suggests column mapping → user fixes any bad cells inline → submits for approval. **No re-upload required when a cell is wrong.**

### Constraints

1. **Use existing project conventions** — fetch wrappers, design system components, styling, state library (zustand or existing pattern). Read 2-3 existing complex page components first to match style.
2. **Do not re-implement** API client primitives — use whatever `/lib/api.ts` (or equivalent) the project already exports.
3. **TypeScript strict mode** — no `any`. Copy the type definitions from the spec doc verbatim.
4. **Virtualize** the preview table for >100 rows (`@tanstack/react-virtual`).
5. **No new modals on errors** — surface errors inline (toast for transient, banner for blocking).

### Deliverable order

Implement in this strict order — each must compile + render before moving on:

1. `components/import-wizard/types.ts` (all interfaces from spec)
2. `components/import-wizard/lib/import-api.ts` (6 typed fetch wrappers)
3. `components/import-wizard/hooks/useImportWizard.ts` (zustand store, step machine)
4. `components/import-wizard/ImportWizard.tsx` (shell + step routing)
5. `Step1Upload.tsx` (drag-drop, xlsx validation)
6. `Step2Map.tsx` + `parts/SheetSelector.tsx` + `parts/TargetFieldDropdown.tsx`
7. `Step3Validate.tsx` + `parts/PreviewTable.tsx` + `parts/CellEditPopover.tsx` + `parts/PreviewStatsBar.tsx`
8. `Step4Submit.tsx` (summary + submit)
9. Wire wizard to existing "Import" card on data-master page

### Smoke test plan (run after each step)

After each numbered deliverable, manually verify in the browser:

- **After step 5:** drop `/tmp/phaseb_test.xlsx` (3 rows) → wizard advances
- **After step 6:** mapping pre-filled with high-confidence suggestions; required-field banner accurate
- **After step 7:** row 4 shows 4 red cells (Stage=FOO, Start Date=not-a-date, Final Price=abc, Bot Active=maybe). Click red cell → popover with original value → edit "FOO" → "CLIENT" → save → cell turns green, stats update.
- **After step 8:** submit returns approval id; toast shows; wizard closes.

### Acceptance criteria

A user opens the wizard with the `phaseb_test.xlsx` fixture and completes a successful submission in ≤4 clicks past upload, with row 4's bad cells fixed inline. No backend re-upload happens during cell fixes.

### What the spec doc covers (read it)

- Full TS type definitions for all 6 endpoint responses
- Component file tree
- zustand store shape
- API wrapper code (copy verbatim)
- Per-step UX details (visual layout, what to show in each panel)
- Edge cases (session expiry, network errors, multi-edit batching)

### What's intentionally NOT in scope

- Approval queue UI (existing page; not touched)
- Phase D (multi-tenant import templates) — not built
- Inline custom-field definition (CFDs registered separately via `/master-data/field-definitions`)

### When done

Reply with:
1. Path of every file created/modified
2. Screenshot of each step (the FE engineer will validate)
3. Any deviation from the spec + reason
4. Cell-edit response time observed (FE→BE→FE round trip)
