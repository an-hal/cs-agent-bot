# Workspace — Backend Implementation Guide

## Context
Dashboard ini adalah **multi-workspace CRM**. Setiap workspace mewakili satu bisnis unit (Dealls, KantorKu). Ada konsep **holding** yang mengagregasi data dari beberapa workspace (Sejutacita = Dealls + KantorKu).

Setiap user bisa punya akses ke beberapa workspace, dan setiap workspace punya konfigurasi sendiri (warna tema, custom fields, pipeline settings).

## Multi-Workspace Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     HOLDING                              │
│               Sejutacita (is_holding=true)               │
│                                                          │
│    ┌─────────────────┐    ┌─────────────────┐           │
│    │   WORKSPACE      │    │   WORKSPACE      │          │
│    │   Dealls          │    │   KantorKu       │         │
│    │                   │    │                   │         │
│    │ • Master Data     │    │ • Master Data     │        │
│    │ • Pipeline        │    │ • Pipeline        │        │
│    │ • Custom Fields   │    │ • Custom Fields   │        │
│    │ • Settings        │    │ • Settings        │        │
│    │ • Theme Color     │    │ • Theme Color     │        │
│    └─────────────────┘    └─────────────────┘           │
│                                                          │
│    Holding = READ ONLY aggregated view                   │
│    dari semua member workspaces                          │
└─────────────────────────────────────────────────────────┘
```

## Data Isolation

```
Setiap API request yang workspace-scoped harus include:
  Header: X-Workspace-ID: {uuid}

Backend HARUS:
  1. Validate X-Workspace-ID exists
  2. Verify user has access to workspace (via workspace_members)
  3. Filter ALL queries by workspace_id
  4. NEVER return data from other workspaces (kecuali holding view)

Holding workspace:
  1. Check workspace.is_holding = true
  2. Get member_workspace_ids
  3. Query WHERE workspace_id IN (member_ids)
  4. Add workspace_name ke setiap record untuk display
```

## Holding Concept

```
┌────────────────────┐
│     Holding         │
│   Sejutacita        │
│                     │
│  member_ids:        │
│   [ws-dealls-001,   │
│    ws-kantorku-001] │
└────────┬────────────┘
         │
         │ aggregates data from
         │
    ┌────┴──────────────────┐
    │                       │
    ▼                       ▼
┌──────────┐          ┌──────────┐
│ Dealls    │          │ KantorKu  │
│ ws-001    │          │ ws-002    │
│           │          │           │
│ 85 clients│          │ 43 clients│
└──────────┘          └──────────┘

Holding view = 128 clients total
Each record tagged with source workspace name
```

Holding workspace characteristics:
- `is_holding = true` di database
- `member_ids` = array of workspace UUIDs
- **READ ONLY** — tidak punya data sendiri di `master_data`
- Dashboard stats = aggregated dari semua members
- Pipeline view = merged dari semua members
- User TIDAK bisa create/edit data langsung di holding view

## UUID Routing Strategy

### URL Pattern
```
Primary:   /dashboard/{uuid}/pipeline/...
Legacy:    /dashboard/{slug}/pipeline/...  → 301 redirect to UUID

UUID ensures URLs survive workspace renames.
```

### Slug-to-UUID Resolution
```
Middleware (proxy.ts):
  1. Request masuk: /dashboard/dealls/pipeline/...
  2. Check: is "dealls" a known slug?
  3. YES → lookup UUID: dealls → ws-dealls-001
  4. 301 Redirect: /dashboard/ws-dealls-001/pipeline/...

Static mapping (hardcoded, fallback):
  dealls   → ws-dealls-001
  kantorku → ws-kantorku-001
  holding  → ws-holding-001

Dynamic resolution (client-side, for new workspaces):
  CompanyContext.findByParam(param):
    1. Try match by UUID
    2. Try match by slug
    3. Try match by id
    Return CompanyProfile or undefined
```

### URL Parameter Resolution in Pages
```
Page: /dashboard/[workspace]/pipeline/[slug]

1. Get `workspace` param from URL
2. findByParam(workspace) → CompanyProfile
3. If not found → redirect to /dashboard
4. If found → set as active workspace
5. Use profile.uuid for X-Workspace-ID header
```

## Workspace Switching Flow

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐
│ Sidebar   │     │ CompanyCtx   │     │ localStorage  │
│ Workspace │     │ setActive()  │     │              │
│ Selector  │     │              │     │              │
└─────┬─────┘     └──────┬───────┘     └──────┬───────┘
      │                  │                    │
      │ 1. User clicks  │                    │
      │    "KantorKu"   │                    │
      │─────────────────>│                    │
      │                  │                    │
      │                  │ 2. Apply theme     │
      │                  │    color override  │
      │                  │                    │
      │                  │ 3. Persist         │
      │                  │────────────────────>│
      │                  │  active_workspace   │
      │                  │  = "kantorku"       │
      │                  │                    │
      │                  │ 4. Update CSS vars │
      │                  │    applyBrandCSS() │
      │                  │                    │
      │ 5. Re-render     │                    │
      │    with new      │                    │
      │    workspace     │                    │
      │<─────────────────│                    │
```

## Theme System

### Per-Workspace Theme Colors
```
Setiap workspace punya brand color (disimpan di backend).
User bisa override theme per workspace (disimpan di localStorage).

Storage key: workspace_themes
Format: { [workspaceId]: themeId }

Contoh:
  { "dealls": "amethyst", "kantorku": "emerald" }
```

### Available Theme Presets
```
amethyst  → #534AB7  (default Bumi — ungu klasik)
emerald   → #059669  (hijau sejuk)
ocean     → #0EA5E9  (biru laut)
sunset    → #F97316  (oranye warm)
rose      → #E11D48  (merah mawar)
lavender  → #8B5CF6  (ungu muda)
mint      → #14B8A6  (tosca segar)
golden    → #D97706  (emas hangat)
sakura    → #EC4899  (pink cherry blossom)
```

### Brand Color → Full Palette Generation
```
Input: 1 hex color (e.g. #534AB7)
Output: full palette (50, 100, 200, 400, 600, 800, 900, foreground-on-light, foreground-on-dark)

Algorithm:
  1. Convert hex → HSL
  2. Generate shades by adjusting lightness:
     50  = (h, min(s,30), 95)    — very light
     100 = (h, min(s,50), 88)
     200 = (h, min(s,55), 78)
     400 = (h, min(s,65), 60)
     600 = original hex           — base color
     800 = (h, min(s+5,100), 35) — dark shade
     900 = (h, min(s+5,100), 22) — very dark
  3. Apply as CSS custom properties on <html> element
```

### CSS Custom Properties Applied
```css
--color-brand-50:  ...
--color-brand-100: ...
--color-brand-200: ...
--color-brand-400: ...
--color-brand-600: ... (= accent hex)
--color-brand-800: ...
--color-brand-900: ...
--color-brand-DEFAULT: ...
--color-brand-foreground-on-light: ...
--color-brand-foreground-on-dark: ...
--brand-rgb: R, G, B
```

## Workspace Data Flow

```
┌───────────────────────────────────────────────────────────────┐
│ App Startup                                                    │
│                                                                │
│  1. CompanyProvider mounts                                     │
│  2. Restore active_workspace from localStorage                 │
│  3. Fetch GET /api/workspaces (with auth_session cookie)      │
│  4. Parse response → split into regular[] + holdings[]        │
│  5. Resolve member_ids (UUIDs → slugs)                         │
│  6. Restore or match active workspace                          │
│  7. Apply theme color (localStorage override)                  │
│  8. Apply CSS custom properties                                │
│                                                                │
│  Error handling:                                               │
│   - 401 → handleSessionExpired() (only on /dashboard pages)  │
│   - Network error → fallback to static FALLBACK_COMPANIES     │
│   - Empty response → keep static fallbacks                     │
└───────────────────────────────────────────────────────────────┘
```

## Environment Variables

| Variabel | Wajib | Keterangan |
|---|---|---|
| `DASHBOARD_API_URL` | Ya | Base URL untuk workspace API |
