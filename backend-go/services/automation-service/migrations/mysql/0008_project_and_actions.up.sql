-- BR-AT-02: automations are scoped to a project for the 20-per-project
-- cap. Additive — pre-migration rows are unscoped (project_id NULL) and
-- skip the cap (see usecase.CreateAutomation's BR-AT-02 check).
ALTER TABLE automations
  ADD COLUMN project_id CHAR(36);

CREATE INDEX idx_automations_project ON automations (tenant_id, project_id);
