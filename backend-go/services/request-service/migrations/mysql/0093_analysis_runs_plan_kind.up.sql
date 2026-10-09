-- Plan generation runs on the same lease machinery as Solution analysis; only the kind list needs to grow.
ALTER TABLE analysis_runs MODIFY kind ENUM('solution','diagnosis','findings','answer','plan') NOT NULL;
