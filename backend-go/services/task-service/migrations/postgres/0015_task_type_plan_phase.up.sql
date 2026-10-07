ALTER TABLE task.tasks DROP CONSTRAINT tasks_task_type_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_task_type_check CHECK (task_type IN ('task','bug','feature','epic','plan','phase'));
