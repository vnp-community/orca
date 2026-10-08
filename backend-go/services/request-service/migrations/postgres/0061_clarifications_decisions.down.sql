-- Only for rolling back the whole v6 request flow: Clarification and Decision data is dropped.
DROP TABLE request.decision_history;
DROP TABLE request.decisions;
DROP TABLE request.clarification_assignees;
DROP TABLE request.clarification_questions;
DROP TABLE request.clarifications;

-- Requests parked in awaiting_information go back to the backlog (stage task, category missing_info) before the CHECK shrinks.
-- The owner is subject to FORCE RLS, so lift it for this statement only.
ALTER TABLE request.requests NO FORCE ROW LEVEL SECURITY;
UPDATE request.requests
SET status = 'request_backlog', returned_from_stage = 'task', returned_category = 'missing_info', return_reason = 'rollback_awaiting_information'
WHERE status = 'awaiting_information';
ALTER TABLE request.requests FORCE ROW LEVEL SECURITY;

ALTER TABLE request.requests DROP CONSTRAINT requests_status_check;
ALTER TABLE request.requests ADD CONSTRAINT requests_status_check CHECK (status IN (
    'new','classifying','awaiting_type_confirmation','analyzing','awaiting_analysis_approval','planning',
    'awaiting_plan_approval','executing','completed','request_backlog','cancelled'));
