CREATE TABLE browser_profiles (
  id             CHAR(36) PRIMARY KEY,
  tenant_id      CHAR(36) NOT NULL,
  dev_server_id  CHAR(36) NOT NULL REFERENCES dev_servers(id),
  name           TEXT NOT NULL,
  source_browser TEXT,               -- e.g. "chrome", "firefox" — set by profileImportFromBrowser
  is_default     BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE INDEX idx_browser_profiles_tenant_dev_server ON browser_profiles (tenant_id, dev_server_id);
