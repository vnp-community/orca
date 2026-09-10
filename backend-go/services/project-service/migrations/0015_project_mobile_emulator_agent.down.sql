DROP INDEX IF EXISTS idx_projects_mobile_emulator_agent;

ALTER TABLE project.projects
    DROP COLUMN mobile_emulator_agent_id;
