# User Preferences

Per-user, per-workspace UI state store. FE owns the shape of each namespace's
`value` — BE treats it as opaque JSONB.

## Use cases

- Theme (dark/light, accent color)
- Sidebar collapse state
- Column visibility per table (`columns.clients`, `columns.invoices`)
- Feed auto-refresh interval
- Anything else UI state that should persist across logins

## Endpoints

All require `Authorization: Bearer {jwt}` + `X-Workspace-ID`.

### List all preferences for caller
```
GET /preferences
```
Returns array of `{namespace, value, ...}` for the JWT's user in this workspace.

### Get one namespace
```
GET /preferences/{namespace}
```
Returns 404 if absent. Client should handle absence as "use defaults".

### Upsert one namespace (full replace)
```
PUT /preferences/{namespace}
{"value": {"mode": "dark", "accent": "blue"}}
```
`value` can be any JSON. BE validates nothing inside.

### Delete one namespace
```
DELETE /preferences/{namespace}
```
404 if already absent.

## Suggested namespaces (convention)

| Namespace | Shape |
|---|---|
| `theme` | `{mode: "dark"/"light", accent: string, logo_url?: string}` |
| `sidebar` | `{collapsed: bool, pinned_items: [string]}` |
| `columns.clients` | `{visible: [string], order: [string], widths: {col: px}}` |
| `columns.invoices` | same shape |
| `feed_interval` | `{seconds: int}` |
| `shortcuts` | `{[action]: "cmd+k"}` |

## Example flow

```js
// On app load:
const prefs = await fetch('/preferences').then(r => r.json());
// prefs.data = [{namespace: "theme", value: {...}}, ...]

// Apply theme
const theme = prefs.data.find(p => p.namespace === 'theme')?.value;
if (theme) applyTheme(theme);

// On user change:
await fetch('/preferences/theme', {
  method: 'PUT',
  body: JSON.stringify({value: {mode: 'dark', accent: 'blue'}})
});
```
