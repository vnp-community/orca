-- Mirrors postgres/0006_client_state_and_workspace_sessions.up.sql — 5
-- opaque per-user JSON columns on user_profiles plus a new
-- user_workspace_sessions table. TEXT (not JSON) both dialects here, same
-- as the Postgres variant: these columns are opaque frontend JSON that
-- tenant-service never queries into, no JSON operators to lose.
ALTER TABLE user_profiles
  ADD COLUMN keybindings_json                TEXT NULL,
  ADD COLUMN ui_local_state_json             TEXT NULL,
  ADD COLUMN saved_runtime_environments_json TEXT NULL,
  ADD COLUMN client_settings_json            TEXT NULL,
  ADD COLUMN accounts_dev_server_json        TEXT NULL;

-- host_id: TEXT -> VARCHAR(255) — this column is part of the composite
-- PRIMARY KEY below, and InnoDB requires an explicit key length for a TEXT
-- column used in a PRIMARY KEY/UNIQUE index (same reasoning as
-- 0004_company_email_domains's email_domain). host_id's own doc comment
-- ("'local'/environmentId; '' = default host") already describes a short
-- identifier, never free text, so this is a safe narrowing.
CREATE TABLE user_workspace_sessions (
  user_id      CHAR(36) NOT NULL,       -- logical FK -> auth.users (different DB)
  company_id   CHAR(36) NOT NULL REFERENCES companies(id),
  host_id      VARCHAR(255) NOT NULL,
  session_json TEXT NOT NULL,
  updated_at   TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

  PRIMARY KEY (user_id, host_id)
);
CREATE INDEX idx_user_workspace_sessions_company ON user_workspace_sessions (company_id);

-- RLS dropped for this dialect — see 0001_init.up.sql's file-level comment;
-- must not be skipped for a new table just because it's additive, same rule
-- the Postgres migration's own comment states.
