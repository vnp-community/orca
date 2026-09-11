ALTER TABLE accounts
  ADD COLUMN latency_ms          INT NULL,               -- NULL until first health check
  ADD COLUMN health_detail       VARCHAR(32) CHECK (health_detail IN
                                   ('healthy','degraded','quota_exceeded','invalid_key','unreachable')),
  ADD COLUMN quota_warning_sent_date DATE NULL;           -- idempotency guard for the 80% alert

-- Partial index (`WHERE status = 'active' AND deleted_at IS NULL`) has no
-- MySQL equivalent — widened to a composite (status, deleted_at,
-- last_health_check_at) index, NOT just (last_health_check_at) alone
-- (which is what a naive "drop the WHERE" translation would produce, same
-- as every other partial-index translation in this migration set).
--
-- This one needs the extra columns for a real reason beyond selectivity,
-- confirmed empirically against a real MySQL 8 server (EXPLAIN, not
-- guessed): ClaimDue's query is `WHERE status='active' AND deleted_at IS
-- NULL AND (last_health_check_at IS NULL OR last_health_check_at <= ?)
-- ORDER BY last_health_check_at LIMIT ? FOR UPDATE SKIP LOCKED`. With only
-- a single-column index on last_health_check_at, MySQL's optimizer chose a
-- full table scan (`type: ALL` in EXPLAIN) over using that index at all —
-- and per documented InnoDB behavior, `ORDER BY ... LIMIT ... FOR UPDATE`
-- that requires a filesort (i.e. isn't satisfied by an index scan already
-- in that order) locks EVERY row matching the WHERE clause during the
-- scan-and-sort phase, not just the LIMIT rows ultimately returned —
-- unlike Postgres's LockRows executor node, which only locks rows actually
-- pulled through Limit. TestClaimDue_NoDoubleClaimUnderConcurrency caught
-- this directly: claim A (LIMIT 5 of 10 due rows) locked all 10 rows
-- during its scan, leaving claim B 0 rows to claim instead of the other 5.
-- A composite index whose leading columns match the WHERE clause's
-- equality/IS-NULL predicates lets the optimizer narrow the scan to only
-- '(status, deleted_at)'-matching rows already in last_health_check_at
-- order, avoiding the filesort (and the over-locking it causes) —
-- repository.go's ClaimDue also adds a FORCE INDEX hint on this index so a
-- future statistics-driven optimizer choice on a differently-sized table
-- can't silently reintroduce the full scan.
CREATE INDEX idx_accounts_due_for_health_check ON accounts (status, deleted_at, last_health_check_at);

-- `usage` is backtick-quoted — MySQL reserved word, see
-- 0001_init.up.sql's comment for the full explanation.
ALTER TABLE `usage` ADD COLUMN tokens_used BIGINT NOT NULL DEFAULT 0;
