ALTER TABLE tasks DROP CHECK tasks_task_type_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_task_type_check CHECK (task_type IN ('task','bug','feature','epic','plan','phase'));
