DROP TABLE review_drawing_profiles;
ALTER TABLE outcome_review_runs DROP INDEX idx_outcome_review_runs_reason, DROP INDEX uq_outcome_review_runs_client, DROP COLUMN created_by, DROP COLUMN client_key, DROP COLUMN reason;
ALTER TABLE outcome_reviews DROP COLUMN approval_note, DROP COLUMN approved_at, DROP COLUMN approved_by;
DROP TABLE outcome_review_decisions;
