DELETE FROM analysis_runs WHERE kind = 'plan';
ALTER TABLE analysis_runs MODIFY kind ENUM('solution','diagnosis','findings','answer') NOT NULL;
