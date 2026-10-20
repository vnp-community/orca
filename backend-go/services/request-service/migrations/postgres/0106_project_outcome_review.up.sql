-- Outcome Review (CR-OR-001): the project's own choice. Empty means "no choice saved": the deployment default applies.
ALTER TABLE request.project_engine_settings
    ADD COLUMN outcome_review TEXT NOT NULL DEFAULT '' CHECK (outcome_review IN ('','off','manual'));
