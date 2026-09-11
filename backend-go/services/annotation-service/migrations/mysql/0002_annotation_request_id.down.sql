-- MySQL has no `DROP CONSTRAINT IF EXISTS` for a UNIQUE constraint (that
-- clause only covers CHECK/FOREIGN KEY constraints as of MySQL 8.0.19) —
-- a UNIQUE constraint added via ADD CONSTRAINT ... UNIQUE is implemented
-- as a plain index, so it comes off via DROP INDEX instead.
ALTER TABLE annotations DROP INDEX uq_annotations_tenant_request;
ALTER TABLE annotations DROP COLUMN request_id;
