-- ActiveExecutionID makes a staleness check possible for
-- ReportTaskExecutionResult (TASK-TG-04-05): a retried/duplicate callback,
-- or a callback for a coordinator_run this task was re-dispatched away
-- from, must be ignored, not an error — at-least-once consumer
-- idempotence per 05-data-architecture.md's outbox-consumer note.
ALTER TABLE task.tasks ADD COLUMN active_execution_id TEXT;
