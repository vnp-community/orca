CREATE TABLE infra.browser_profiles (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  dev_server_id  UUID NOT NULL REFERENCES infra.dev_servers(id),
  name           TEXT NOT NULL,
  source_browser TEXT,               -- e.g. "chrome", "firefox" — set by profileImportFromBrowser
  is_default     BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_browser_profiles_tenant_dev_server ON infra.browser_profiles (tenant_id, dev_server_id);
