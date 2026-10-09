DELETE FROM request.analysis_runs WHERE kind = 'plan';
ALTER TABLE request.analysis_runs DROP CONSTRAINT IF EXISTS analysis_runs_kind_check;
ALTER TABLE request.analysis_runs
    ADD CONSTRAINT analysis_runs_kind_check CHECK (kind IN ('solution','diagnosis','findings','answer'));
