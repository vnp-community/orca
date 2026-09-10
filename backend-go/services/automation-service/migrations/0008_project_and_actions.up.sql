-- BR-AT-02: automations are scoped to a project for the 20-per-project
-- cap. Additive — pre-migration rows are unscoped (project_id NULL) and
-- skip the cap (see usecase.CreateAutomation's BR-AT-02 check). The
-- action-chain columns (actions_json/action_results_json/etc.) this
-- migration originally also added were a duplicate of 0003_action_chain's
-- own actions_json/action_results_json columns (BR-AT-01 and CR-AUTO-002
-- independently reached the same action-chain design) — dropped from here
-- to avoid a double ADD COLUMN; see 0003_action_chain.up.sql instead.
ALTER TABLE automation.automations
  ADD COLUMN project_id UUID;

CREATE INDEX idx_automations_project ON automation.automations (tenant_id, project_id);
