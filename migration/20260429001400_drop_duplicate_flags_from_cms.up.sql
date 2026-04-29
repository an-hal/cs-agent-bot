-- Resolve the duplicate-flag conflict between client_message_state and
-- client_flags. The 9 columns below have been the responsibility of
-- client_flags since 2025-03-30 (migration 20250330000003) — flags_repo
-- writes them, the bot lifecycle reads them. Migration 700 (Phase 6 split)
-- inadvertently relocated identically-named columns from clients to cms,
-- creating a second source of truth.
--
-- Production data: both tables hold 0 rows at the time of writing, so we
-- can drop without data migration. After this migration:
--   - bot writes via flags_repo → client_flags  (unchanged)
--   - bot reads via client_repo → client_flags via the new JOIN in
--     clientColumns (see internal/repository/client_repo.go change paired
--     with this migration)

ALTER TABLE client_message_state
    DROP COLUMN IF EXISTS pre14_sent,
    DROP COLUMN IF EXISTS pre7_sent,
    DROP COLUMN IF EXISTS pre3_sent,
    DROP COLUMN IF EXISTS post1_sent,
    DROP COLUMN IF EXISTS post4_sent,
    DROP COLUMN IF EXISTS post8_sent,
    DROP COLUMN IF EXISTS post15_sent,
    DROP COLUMN IF EXISTS checkin_replied,
    DROP COLUMN IF EXISTS feature_update_sent;
