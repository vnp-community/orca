-- How often the tasks.md sync or the archive step of a change has failed in a row; the sweep stops at the limit and leaves the
-- change as failed, which never changes the Request.
ALTER TABLE request.openspec_changes ADD COLUMN tasks_sync_attempts INT NOT NULL DEFAULT 0;
