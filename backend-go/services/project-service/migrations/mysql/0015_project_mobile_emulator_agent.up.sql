ALTER TABLE projects ADD COLUMN mobile_emulator_agent_id CHAR(36);
CREATE INDEX idx_projects_mobile_emulator_agent ON projects (mobile_emulator_agent_id);
