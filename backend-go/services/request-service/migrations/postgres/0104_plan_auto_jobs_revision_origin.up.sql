-- CR-REQ-045: a change request on an existing Plan is committed by the same worker as the automatic Plan, as a job whose
-- origin is 'revision'. Only the list of accepted origins grows.
ALTER TABLE request.plan_auto_jobs DROP CONSTRAINT IF EXISTS plan_auto_jobs_origin_check;
ALTER TABLE request.plan_auto_jobs
    ADD CONSTRAINT plan_auto_jobs_origin_check CHECK (origin IN ('event','retry','revision'));
