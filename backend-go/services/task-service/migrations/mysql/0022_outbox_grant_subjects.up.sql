-- MySQL translation of migrations/postgres/0022_outbox_grant_subjects.up.sql.
UPDATE outbox_events SET subject = CONCAT('orca.', subject) WHERE published_at IS NULL AND subject IN ('task.grant_received', 'task.grant_revoked');
