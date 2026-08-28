ALTER TABLE infra.dev_servers ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_infra_dev_servers_tags ON infra.dev_servers USING GIN (tags);
