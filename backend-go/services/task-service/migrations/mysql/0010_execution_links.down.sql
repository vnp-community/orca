ALTER TABLE tasks
  DROP FOREIGN KEY fk_tasks_active_execution_link,
  DROP COLUMN active_execution_link_id;

DROP TABLE IF EXISTS execution_links;
