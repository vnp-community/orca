-- SOL-PW-04: task_number/pr_url columns for referencing tasks as "#TG-42"
-- and recording their write-back PR — task-service.md §6 already named
-- adapter/eventbus/ "task.created/task.completed/... via outbox" as the
-- intended shape; the outbox table itself already exists
-- (0005_outbox.up.sql, TG-03-07's grant-mutation audit events) and is
-- reused here for task.* domain events too, so this migration only adds
-- the two new columns plus the sequence/index task_number needs.
-- worktree_id already exists (0003_task_fields_and_comments.up.sql).
ALTER TABLE task.tasks ADD COLUMN task_number BIGINT;
ALTER TABLE task.tasks ADD COLUMN pr_url TEXT;

-- One sequence per project would require dynamic sequence creation;
-- instead use a single global sequence and enforce project-scoped
-- uniqueness via a composite unique index — task_number's *value* need
-- not be contiguous per project, only unique-per-project and monotonic,
-- which nextval() on one shared sequence still guarantees.
CREATE SEQUENCE task.task_number_seq;
CREATE UNIQUE INDEX idx_tasks_project_task_number
    ON task.tasks (project_id, task_number)
    WHERE task_number IS NOT NULL;
