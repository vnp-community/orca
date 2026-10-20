-- Revision jobs have no meaning without the origin, so they are removed first (like 0093 does for plan runs).
DELETE FROM request.plan_auto_jobs WHERE origin = 'revision';
ALTER TABLE request.plan_auto_jobs DROP CONSTRAINT IF EXISTS plan_auto_jobs_origin_check;
ALTER TABLE request.plan_auto_jobs
    ADD CONSTRAINT plan_auto_jobs_origin_check CHECK (origin IN ('event','retry'));
