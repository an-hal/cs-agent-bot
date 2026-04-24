# User Preferences — Shared Storage

## Overview

Beberapa UI preferences perlu disimpan per-user per-workspace:
- Column visibility (Master Data table)
- Theme selection
- Sidebar collapse state
- Activity feed polling interval

Satu table `user_preferences` menangani semuanya via JSONB.

## Database Schema

```sql
CREATE TABLE user_preferences (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_email    VARCHAR(255) NOT NULL,       -- from auth token (no users table)
  workspace_id  UUID NOT NULL REFERENCES workspaces(id),
  
  -- Theme
  theme_id      VARCHAR(20),                 -- amethyst, emerald, ocean, etc.
  dark_mode     BOOLEAN DEFAULT TRUE,
  
  -- UI preferences (JSONB for flexibility)
  preferences   JSONB NOT NULL DEFAULT '{}',
  -- Example contents:
  -- {
  --   "master_data_columns": ["Company_ID","Company_Name","Stage","PIC_Name","Bot_Active"],
  --   "sidebar_collapsed": false,
  --   "activity_feed_interval": 60,
  --   "notification_sound": true,
  --   "default_pipeline": "406e6b25-..."
  -- }
  
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  UNIQUE(user_email, workspace_id)
);

CREATE INDEX idx_uprefs_user ON user_preferences(user_email, workspace_id);
```

## API Endpoints

### GET `/preferences`
Get current user preferences for active workspace.

```
Response 200:
{
  "theme_id": "amethyst",
  "dark_mode": true,
  "preferences": {
    "master_data_columns": ["Company_ID", "Company_Name", "Stage", "PIC_Name"],
    "sidebar_collapsed": false
  }
}
```

### PUT `/preferences`
Update preferences (merge, not replace).

```json
{
  "theme_id": "ocean",
  "preferences": {
    "master_data_columns": ["Company_ID", "Company_Name", "Stage", "PIC_Name", "Bot_Active"]
  }
}
```

### PUT `/preferences/theme`
Shortcut for theme-only update.

```json
{
  "theme_id": "emerald",
  "dark_mode": false
}
```

## Note

Currently stored in localStorage. Backend endpoint enables:
- Preferences sync across devices
- Admin can set default preferences per workspace
- Onboarding can pre-configure preferences for new users
