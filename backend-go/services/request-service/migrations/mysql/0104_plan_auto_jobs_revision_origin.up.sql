-- CR-REQ-045: a change request on an existing Plan is committed by the same worker as the automatic Plan, as a job whose
-- origin is 'revision'. Only the list of accepted origins grows.
ALTER TABLE plan_auto_jobs DROP CHECK plan_auto_jobs_origin;
ALTER TABLE plan_auto_jobs ADD CONSTRAINT plan_auto_jobs_origin CHECK (origin IN ('event','retry','revision'));
