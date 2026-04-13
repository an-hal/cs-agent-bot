-- Migration: Remove seeded whitelist entries
-- Version: 20260413000002

DELETE FROM whitelist
WHERE email IN (
  'arief.faltah@dealls.com',
  'dhimas.priyadi@sejutacita.id'
);
