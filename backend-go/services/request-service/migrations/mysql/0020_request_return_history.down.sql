DROP TABLE IF EXISTS request_return_history;
ALTER TABLE requests DROP CHECK requests_backlog_category;
ALTER TABLE requests DROP CHECK requests_returned_category_values;
ALTER TABLE requests DROP COLUMN returned_category;
