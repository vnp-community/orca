-- CR-AUTO-002/TASK-BE-AUTO-003: action chain (repeated AutomationAction),
-- and CR-AUTO-007/TASK-BE-AUTO-010/011's retention/timeout/concurrency
-- columns — matches the Postgres variant's decision to merge both tracks
-- into one migration file.
--
-- actions_json/action_results_json are JSON (Postgres: JSONB — MySQL has
-- no binary-JSON type, plain JSON is the dialect-safe equivalent; see
-- BE-DB-SOL-001 §5/CR-DB-002's JSONB->JSON rule). Empty array default so a
-- legacy automation (actions never populated) reads back as [] rather than
-- NULL — usecase.resolveActions treats [] the same as "not populated".
-- MySQL 8.0.13+ allows a function-expression column DEFAULT
-- (`DEFAULT (JSON_ARRAY())`, parens required) — verified against the real
-- mysql:8 test image this task runs integration tests on, see
-- BE-DB-SOL-012 §Kết quả thực tế for the actual test run confirming it.
ALTER TABLE automations
    ADD COLUMN actions_json JSON NOT NULL DEFAULT (JSON_ARRAY()),
    ADD COLUMN max_run_history INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN run_timeout_seconds INTEGER NOT NULL DEFAULT 0,
    -- running_run_id/running_since — TASK-BE-AUTO-011's concurrency guard.
    -- NULL running_run_id = not currently running. running_since backs the
    -- self-releasing TTL (a crash that never clears running_run_id must not
    -- lock an automation out forever) — see ExecuteAutomationChain's
    -- acquireRunLock/releaseRunLock.
    ADD COLUMN running_run_id CHAR(36),
    ADD COLUMN running_since TIMESTAMP(6) NULL;

ALTER TABLE automation_runs
    ADD COLUMN action_results_json JSON NOT NULL DEFAULT (JSON_ARRAY());
