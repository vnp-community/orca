DROP TABLE request.review_drawing_profiles;
DROP INDEX request.idx_outcome_review_runs_reason;
DROP INDEX request.uq_outcome_review_runs_client;
ALTER TABLE request.outcome_review_runs DROP COLUMN created_by, DROP COLUMN client_key, DROP COLUMN reason;
ALTER TABLE request.outcome_reviews DROP COLUMN approval_note, DROP COLUMN approved_at, DROP COLUMN approved_by;
DROP TABLE request.outcome_review_decisions;
