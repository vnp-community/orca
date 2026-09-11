-- Mirrors postgres/0002_backfill_legacy_bootstrap_company.up.sql — same
-- sentinel row, same no-op-everywhere-else intent. INSERT IGNORE is MySQL's
-- equivalent of Postgres's ON CONFLICT (id) DO NOTHING (skips silently on
-- the PRIMARY KEY collision, no other error suppressed by this statement).
INSERT IGNORE INTO companies (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'Legacy Bootstrap Company');
