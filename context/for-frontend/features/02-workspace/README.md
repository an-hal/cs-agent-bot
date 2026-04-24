# feat/02 — Workspaces

Multi-tenant workspaces + member invitations + per-workspace theme + holding
hierarchy for cross-workspace aggregation.

## Status

**✅ 100%** — workspaces, members, invitations, theme, holding all live.

## Endpoints

### Workspaces

```
GET    /workspaces                        # List workspaces the user belongs to
POST   /workspaces                        # Create workspace
GET    /workspaces/{id}                   # Get by ID
PUT    /workspaces/{id}                   # Update (name, description, settings)
DELETE /workspaces/{id}                   # Soft delete
POST   /workspaces/{id}/switch            # "Switch" — sets default workspace for user
```

Create body:
```json
{"slug": "acme-id", "name": "Acme Indonesia", "description": "..."}
```

### Members + invitations

```
GET    /workspaces/{id}/members
POST   /workspaces/{id}/members/invite
PUT    /workspaces/{id}/members/{member_id}
DELETE /workspaces/{id}/members/{member_id}
POST   /workspaces/invitations/{token}/accept
```

Invite body:
```json
{"email": "new@example.com", "role": "member"}  // role varies per team feature
```

### Theme

Per-workspace UI theme — opaque JSONB, FE owns the shape.

```
GET /workspace/theme                # current workspace (from X-Workspace-ID header)
PUT /workspace/theme
  {"theme": {"mode": "dark", "accent": "blue", "logo_url": "..."}}
```

### Holding expansion

When a workspace is part of a holding (multiple sibling workspaces share a
parent `holding_id`), this endpoint returns all workspace IDs FE should
query for cross-workspace aggregation.

```
GET /workspace/holding/expand
→ {"workspace_ids": ["ws-1", "ws-2", "ws-3"], "count": 3}
```

- If no holding configured: returns just `[current_workspace_id]`.
- Use the result to fan out parallel reads for holding dashboards.

## Data model

See [../../05-data-models.md](../../05-data-models.md#workspace) for
`Workspace` shape.

## FE UX

**Workspace switcher:**
- Dropdown in top nav
- Lists user's workspaces (from `/workspaces`)
- On switch: optionally `POST /workspaces/{id}/switch` (persist preference)
- Set `X-Workspace-ID` on all subsequent calls

**Invitations:**
- Email link: `/invitations/{token}/accept`
- FE accept page calls `POST /workspaces/invitations/{token}/accept`
- Redirect to the newly-joined workspace

**Theme:**
- Apply on app load via `GET /workspace/theme`
- Admin settings page with color/mode picker → `PUT /workspace/theme`
- Theme store locally too for instant apply (hydrate from API)

**Holding dashboards:**
- Admin toggle "View holding aggregate" → call `/workspace/holding/expand`
- Parallel-fetch analytics/reports for each workspace_id returned
- Merge client-side
