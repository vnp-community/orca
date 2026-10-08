-- The reason CHECK was declared inline and unnamed, so find its generated name first.
SELECT cc.constraint_name INTO @old_check
FROM information_schema.check_constraints cc
JOIN information_schema.table_constraints tc
  ON tc.constraint_schema = cc.constraint_schema AND tc.constraint_name = cc.constraint_name
WHERE tc.table_schema = DATABASE() AND tc.table_name = 'request_links' AND tc.constraint_type = 'CHECK'
  AND cc.check_clause LIKE '%relates_to%' LIMIT 1;
SET @drop_sql = CONCAT('ALTER TABLE request_links DROP CHECK ', @old_check);
PREPARE drop_stmt FROM @drop_sql;
EXECUTE drop_stmt;
DEALLOCATE PREPARE drop_stmt;

ALTER TABLE request_links ADD CONSTRAINT request_links_reason_check CHECK (reason IN
    ('relates_to','blocks','is_blocked_by','duplicates','spawned_by_spike','spawned_by_question','followup_hotfix','escalation')),
    ADD COLUMN created_by CHAR(36) NULL,
    ADD COLUMN created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6);
