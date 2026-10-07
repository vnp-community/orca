-- Down làm mất phân biệt plan/phase/epic, chỉ dùng khi rollback toàn bộ v6
UPDATE task.tasks SET task_type = 'epic' WHERE task_type IN ('plan','phase');
ALTER TABLE task.tasks DROP CONSTRAINT tasks_task_type_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_task_type_check CHECK (task_type IN ('task','bug','feature','epic'));
