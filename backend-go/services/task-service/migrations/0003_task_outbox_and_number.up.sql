-- SOL-PW-04: task_number/worktree_id/pr_url columns + the transactional
-- outbox this service was missing entirely (task-service.md §6 already
-- named adapter/eventbus/ "task.created/task.completed/... via outbox" as
-- the intended shape; this is the schema half of actually building it).
ALTER TABLE task.tasks ADD COLUMN task_number BIGINT;
ALTER TABLE task.tasks ADD COLUMN worktree_id UUID;
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

-- Transactional outbox table — same shape as usage.outbox_events
-- (usage-service/migrations/0002_outbox.up.sql). task.tasks writes and
-- this table's INSERT happen in the same Postgres transaction
-- (internal/adapter/postgres.Repository.Update); common/outbox.Relay
-- polls unpublished rows and publishes them to NATS JetStream.
CREATE TABLE task.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_task_outbox_events_unpublished
    ON task.outbox_events (created_at)
    WHERE published_at IS NULL;
