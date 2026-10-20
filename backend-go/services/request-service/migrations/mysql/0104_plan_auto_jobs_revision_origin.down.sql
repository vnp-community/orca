-- Revision jobs have no meaning without the origin, so they are removed first (like 0093 does for plan runs).
DELETE FROM plan_auto_jobs WHERE origin = 'revision';
ALTER TABLE plan_auto_jobs DROP CHECK plan_auto_jobs_origin;
ALTER TABLE plan_auto_jobs ADD CONSTRAINT plan_auto_jobs_origin CHECK (origin IN ('event','retry'));
