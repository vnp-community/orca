DROP TABLE IF EXISTS request.request_return_history;
ALTER TABLE request.requests DROP CONSTRAINT IF EXISTS requests_backlog_category;
ALTER TABLE request.requests DROP COLUMN IF EXISTS returned_category;
