-- BR-AT-01/BR-AT-02: an ordered multi-action chain replaces the single
-- step_type/step_config_json pair as the source of truth for new/updated
-- automations, and automations are scoped to a project for the
-- 20-per-project cap. Both columns are additive — pre-migration rows keep
-- working through the deprecated step_type/step_config_json columns
-- (domain.NewAutomation normalizes them into a one-element actions_json on
-- next write).
ALTER TABLE automation.automations
  ADD COLUMN project_id UUID,
  ADD COLUMN actions_json JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX idx_automations_project ON automation.automations (tenant_id, project_id);

-- action_results_json records each action's outcome in a multi-action
-- chain (SOL-AT-01) — output_json/error_message above still hold the last
-- action's result for back-compat with any caller reading them directly.
ALTER TABLE automation.automation_runs
  ADD COLUMN action_results_json JSONB;
