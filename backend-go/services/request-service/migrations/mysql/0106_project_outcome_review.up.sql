-- Outcome Review (CR-OR-001): the project's own choice. Empty means "no choice saved": the deployment default applies.
ALTER TABLE project_engine_settings
    ADD COLUMN outcome_review VARCHAR(10) NOT NULL DEFAULT '' CHECK (outcome_review IN ('','off','manual'));
