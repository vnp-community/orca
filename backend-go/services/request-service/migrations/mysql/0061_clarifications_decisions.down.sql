-- Only for rolling back the whole v6 request flow: Clarification and Decision data is dropped.
DROP TABLE decision_history;
DROP TABLE decisions;
DROP TABLE clarification_assignees;
DROP TABLE clarification_questions;
DROP TABLE clarifications;

-- Requests parked in awaiting_information go back to the backlog (stage task, category missing_info) before the CHECK shrinks.
UPDATE requests
SET status = 'request_backlog', returned_from_stage = 'task', returned_category = 'missing_info', return_reason = 'rollback_awaiting_information'
WHERE status = 'awaiting_information';

ALTER TABLE requests DROP CHECK requests_status_check;
ALTER TABLE requests ADD CONSTRAINT requests_status_check CHECK (status IN (
    'new','classifying','awaiting_type_confirmation','analyzing','awaiting_analysis_approval','planning',
    'awaiting_plan_approval','executing','completed','request_backlog','cancelled'));
