# Theme System

## Overview
Setiap workspace punya brand color sendiri. User bisa pilih dari 9 preset theme.
Theme disimpan per-user per-workspace di localStorage (frontend) + opsional di backend.

## Theme Presets
| ID | Name | Hex Color |
|----|------|-----------|
| amethyst | Amethyst | #534AB7 |
| emerald | Emerald | #1D9E75 |
| ocean | Ocean | #0EA5E9 |
| sunset | Sunset | #F97316 |
| rose | Rose | #F43F5E |
| lavender | Lavender | #8B5CF6 |
| mint | Mint | #10B981 |
| golden | Golden | #EAB308 |
| sakura | Sakura | #EC4899 |

## Palette Generation
Dari 1 hex color → 9 CSS custom properties via HSL manipulation:
```
--color-brand-50   → lightness 95%
--color-brand-100  → lightness 88%
--color-brand-200  → lightness 78%
--color-brand-400  → lightness 60%
--color-brand-600  → base color
--color-brand-800  → lightness 35%
--color-brand-900  → lightness 22%
--color-brand-foreground-on-light  → lightness 35%
--color-brand-foreground-on-dark   → lightness 88%
```

## Dark/Light Mode
- Stored in localStorage key `theme` → `'light'` | `'dark'`
- Applied via `document.documentElement.classList.add('dark')`
- Script runs `beforeInteractive` to prevent flash

## Backend Storage (optional)

### Table: `user_preferences`
```sql
CREATE TABLE user_preferences (
  user_id       UUID NOT NULL REFERENCES users(id),
  workspace_id  UUID NOT NULL REFERENCES workspaces(id),
  theme_id      VARCHAR(20),       -- 'amethyst', 'emerald', etc.
  dark_mode     BOOLEAN DEFAULT TRUE,
  PRIMARY KEY (user_id, workspace_id)
);
```

### API Endpoints
Already covered in `02-workspace/03-api-endpoints.md`:
- `GET /workspaces/{id}/theme` → current theme
- `PUT /workspaces/{id}/theme` → update theme
