-- Plan generation runs on the same lease machinery as Solution analysis; only the kind list needs to grow.
ALTER TABLE request.analysis_runs DROP CONSTRAINT IF EXISTS analysis_runs_kind_check;
ALTER TABLE request.analysis_runs
    ADD CONSTRAINT analysis_runs_kind_check CHECK (kind IN ('solution','diagnosis','findings','answer','plan'));
