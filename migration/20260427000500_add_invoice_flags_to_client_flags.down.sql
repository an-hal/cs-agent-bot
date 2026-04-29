ALTER TABLE client_flags
    DROP COLUMN IF EXISTS pre14_sent,
    DROP COLUMN IF EXISTS pre7_sent,
    DROP COLUMN IF EXISTS pre3_sent,
    DROP COLUMN IF EXISTS post1_sent,
    DROP COLUMN IF EXISTS post4_sent,
    DROP COLUMN IF EXISTS post8_sent,
    DROP COLUMN IF EXISTS post15_sent;
