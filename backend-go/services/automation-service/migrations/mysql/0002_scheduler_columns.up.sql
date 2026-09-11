-- Promotes step_type off the step_config_json blob to a real column, and
-- adds the columns the in-process scheduler ticker needs — see
-- specs/backend-go/services/automation-service.md §5/§7. dtstart already
-- exists (migration 0001); enabled/timezone/next_run_at are new here.
-- MySQL/InnoDB rejects a plain literal DEFAULT on TEXT/BLOB/JSON columns
-- ("Error 1101: ... can't have a default value") — wrapping each literal
-- in parentheses turns it into an expression default, which MySQL
-- 8.0.13+ does accept; confirmed against the real mysql:8 test image (see
-- BE-DB-SOL-012 §Kết quả thực tế).
ALTER TABLE automations
    ADD COLUMN step_type   TEXT NOT NULL DEFAULT (''),
    ADD COLUMN enabled     BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN timezone    TEXT NOT NULL DEFAULT ('UTC'),
    ADD COLUMN next_run_at TIMESTAMP(6) NULL;

-- Postgres's idx_automations_due is a PARTIAL index (WHERE enabled) —
-- MySQL/InnoDB has no partial-index equivalent, so this is a full index
-- over (next_run_at, enabled) instead. This is a performance-only
-- deviation (a bigger index, scanning disabled rows' NULL next_run_at
-- entries too), not a correctness one — the scheduler's claim query
-- (WHERE enabled = true AND next_run_at <= ?) still returns the exact same
-- rows either way. Same category of deviation as annotation-service's
-- idx_annotations_worktree (BE-DB-SOL-005 §5).
CREATE INDEX idx_automations_due ON automations (next_run_at, enabled);

-- trigger records what caused a run's dispatch (scheduled/manual/external)
-- — see automation-service.md §3/§7: RunNow, the scheduler ticker, and
-- HandleExternalTrigger all funnel through the same interactor, and this
-- is how a run's origin is told apart afterward. `trigger` (backtick-quoted
-- throughout this migration and internal/adapter/mysql/repository.go) is a
-- RESERVED keyword in MySQL — unlike Postgres, which accepts it unquoted
-- as an identifier — so every reference needs the quoting; confirmed by a
-- real "Error 1064: ... syntax ... near 'trigger'" against mysql:8 before
-- this fix (see BE-DB-SOL-012 §Kết quả thực tế).
ALTER TABLE automation_runs
    ADD COLUMN `trigger` VARCHAR(16) NOT NULL DEFAULT 'manual';

ALTER TABLE automation_runs
    ADD CONSTRAINT automation_runs_trigger_check CHECK (`trigger` IN ('scheduled', 'manual', 'external'));
