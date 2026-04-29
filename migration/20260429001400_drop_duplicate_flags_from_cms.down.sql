-- Restore the duplicate columns on client_message_state. Backfill from
-- client_flags so the rollback doesn't lose any state the bot has already
-- written via flags_repo since the up migration.

ALTER TABLE client_message_state
    ADD COLUMN IF NOT EXISTS pre14_sent          BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pre7_sent           BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pre3_sent           BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS post1_sent          BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS post4_sent          BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS post8_sent          BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS post15_sent         BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS checkin_replied     BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS feature_update_sent BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE client_message_state cms SET
    pre14_sent          = cf.pre14_sent,
    pre7_sent           = cf.pre7_sent,
    pre3_sent           = cf.pre3_sent,
    post1_sent          = cf.post1_sent,
    post4_sent          = cf.post4_sent,
    post8_sent          = cf.post8_sent,
    post15_sent         = cf.post15_sent,
    checkin_replied     = cf.checkin_replied,
    feature_update_sent = cf.feature_update_sent
FROM client_flags cf, clients c
WHERE cf.company_id = c.company_id AND c.master_id = cms.master_id;
