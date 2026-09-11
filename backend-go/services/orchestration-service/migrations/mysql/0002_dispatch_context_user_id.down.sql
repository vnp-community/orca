DROP INDEX idx_dispatch_contexts_user ON dispatch_contexts;
ALTER TABLE dispatch_contexts
  DROP COLUMN user_id;
