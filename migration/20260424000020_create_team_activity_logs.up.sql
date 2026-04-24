-- Team activity logs — separate from master_data mutations + action_log so
-- admins can audit team-scoped actions (role changes, member invites,
-- permission updates) without cluttering the client-activity feed.
CREATE TABLE IF NOT EXISTS team_activity_logs (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_email    VARCHAR(255) NOT NULL,
    action         VARCHAR(64)  NOT NULL,
                                -- invite_member | change_role | remove_member |
                                -- update_permissions | create_role | delete_role |
                                -- update_assignments
    target_email   VARCHAR(255) NOT NULL DEFAULT '',
    role_id        UUID,
    detail         JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_team_activity_workspace_time
    ON team_activity_logs (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_team_activity_actor
    ON team_activity_logs (actor_email, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_team_activity_target
    ON team_activity_logs (target_email, created_at DESC);
