-- JSONB -> JSON, `'{}'::jsonb` default -> `(JSON_OBJECT())` expression
-- default (MySQL 8.0.13+ allows function-expression defaults on JSON
-- columns; a literal string default is not accepted for JSON).
ALTER TABLE worktrees
    ADD COLUMN metadata JSON NOT NULL DEFAULT (JSON_OBJECT());
