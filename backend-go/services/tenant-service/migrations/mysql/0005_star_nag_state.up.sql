-- Mirrors postgres/0005_star_nag_state.up.sql. UUID -> CHAR(36),
-- JSONB -> JSON (nullable, no default — active_prompt is NULL exactly when
-- no prompt is currently displayed, same semantics both dialects),
-- TIMESTAMPTZ -> TIMESTAMP(6).
CREATE TABLE star_nag_state (
  user_id                          CHAR(36) PRIMARY KEY,      -- logical FK -> auth.users, 1:1 like user_profiles.user_id
  company_id                       CHAR(36) NOT NULL REFERENCES companies(id),
  baseline_agents                  BIGINT,
  app_version                      TEXT,
  next_threshold                   BIGINT NOT NULL DEFAULT 35,
  completed                        BOOLEAN NOT NULL DEFAULT false,
  deferred_until                   TIMESTAMP(6) NULL,
  agent_value_moment_app_version   TEXT,
  active_prompt                    JSON,
  updated_at                       TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);
CREATE INDEX idx_star_nag_state_company ON star_nag_state(company_id);
