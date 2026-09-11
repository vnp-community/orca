-- BR-AT-08: at most one 'running' automation_runs row per automation.
--
-- Postgres enforces this with a PARTIAL unique index
-- (CREATE UNIQUE INDEX ... WHERE status = 'running') — MySQL/InnoDB has no
-- partial-index syntax at all (unlike annotation-service's/this service's
-- own 0002's idx_automations_due, where dropping partiality only costs
-- index selectivity; here it is LOAD-BEARING: a plain full UNIQUE INDEX on
-- automation_id would allow only ONE row EVER per automation, breaking run
-- history entirely). The standard MySQL workaround is a shadow column that
-- is NULL except in the row(s) we want to constrain, then a UNIQUE INDEX
-- on that column — MySQL's UNIQUE INDEX treats NULL as "not equal to any
-- other NULL", so any number of non-running rows (running_slot = NULL)
-- coexist, while two 'running' rows for the same automation_id collide on
-- running_slot and are rejected — identical semantics to the Postgres
-- partial index.
--
-- running_slot is a PLAIN column (application-maintained by
-- internal/adapter/mysql.AutomationRunRepository.UpdateStatus), NOT a
-- GENERATED ALWAYS AS (...) STORED column as first attempted: MySQL 8's
-- InnoDB refuses "ALTER TABLE ... ADD COLUMN ... GENERATED ... STORED"
-- when the generated expression references a column (automation_id) that
-- also carries a FOREIGN KEY constraint — confirmed against the real
-- mysql:8 test image ("Error 1215: Cannot add foreign key constraint",
-- reproducible even with the FK temporarily dropped and re-added; the
-- plain-column-set-by-the-application form has no such restriction, tested
-- working against the same image). See BE-DB-SOL-012 §3.1 for the full
-- writeup and TestRepository_OneRunningPartialUniqueIndex (mysql variant)
-- for the passing proof.
ALTER TABLE automation_runs
    ADD COLUMN running_slot CHAR(36) NULL;

CREATE UNIQUE INDEX idx_automation_runs_one_running ON automation_runs (running_slot);
