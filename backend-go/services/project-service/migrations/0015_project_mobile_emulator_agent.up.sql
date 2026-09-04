-- CR-DS-009 §3.2 (docs/crs/v2/dev-server/CR-DS-009-mobile-emulator-agent-separation.md)
-- mobile_emulator_agent_id is a parallel, independent binding to
-- dev_server_id: which infra-fleet-service DevServer (kind =
-- AGENT_KIND_MOBILE_EMULATOR) this project's Mobile Emulator pane routes
-- emulator.* control to. Logical FK, not a real FK — project-service has no
-- visibility into infra-fleet-service's schema, same as dev_server_id
-- above it (migrations/0001_init.up.sql).
ALTER TABLE project.projects
    ADD COLUMN mobile_emulator_agent_id UUID;

CREATE INDEX idx_projects_mobile_emulator_agent ON project.projects (mobile_emulator_agent_id);
