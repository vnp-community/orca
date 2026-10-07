CREATE TABLE context_sources (
  id CHAR(36) PRIMARY KEY, 
  tenant_id CHAR(36) NOT NULL,
  source_key VARCHAR(64) NOT NULL CHECK (source_key REGEXP '^[a-z][a-z0-9_]{1,63}$'),
  kind VARCHAR(24) NOT NULL CHECK (kind IN ('request_origin','conventions','decisions','specs','service_catalog','code_graph',
    'git_history','contracts','schema','dependencies','ci_config','ci_results','coverage','observability','incidents',
    'feature_flags','security_scan','policy','ownership','history','dev_server_profile','external_knowledge')),
  transport VARCHAR(24) NOT NULL CHECK (transport IN ('internal','mcp')),
  adapter VARCHAR(64), 
  server_ref CHAR(36),
  scopes JSON NOT NULL, 
  trust VARCHAR(8) NOT NULL CHECK (trust IN ('high','medium','low')),
  ttl_seconds INT NOT NULL DEFAULT 300 CHECK (ttl_seconds >= 0), 
  max_bytes INT NOT NULL DEFAULT 65536 CHECK (max_bytes BETWEEN 1024 AND 1048576),
  redaction JSON NOT NULL, 
  rate_limit_per_minute INT NOT NULL DEFAULT 60 CHECK (rate_limit_per_minute > 0),
  enabled_for JSON NOT NULL, 
  status VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','disabled')),
  owner_id CHAR(36) NOT NULL, 
  version BIGINT NOT NULL DEFAULT 1, 
  created_by CHAR(36) NOT NULL,
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), 
  updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE (tenant_id, source_key),
  CONSTRAINT context_sources_transport_shape CHECK ((transport = 'internal' AND adapter IS NOT NULL AND server_ref IS NULL)
                                               OR (transport = 'mcp' AND server_ref IS NOT NULL AND adapter IS NULL))
);

CREATE TABLE context_packs (
  id CHAR(36) PRIMARY KEY, 
  tenant_id CHAR(36) NOT NULL, 
  request_id CHAR(36) NOT NULL REFERENCES requests(id),
  stage VARCHAR(16) NOT NULL CHECK (stage IN ('classify','solution','plan','task','execute','risk')),
  cp_version VARCHAR(32) NOT NULL, 
  input_digest CHAR(64) NOT NULL, 
  digest CHAR(64) NOT NULL,
  budget_tokens INT NOT NULL, 
  used_tokens INT NOT NULL, 
  items JSON NOT NULL, 
  missing JSON NOT NULL,
  body MEDIUMTEXT NOT NULL, 
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
);

CREATE INDEX context_packs_latest ON context_packs (tenant_id, request_id, stage, created_at DESC);
CREATE INDEX context_packs_input ON context_packs (tenant_id, request_id, stage, input_digest);

CREATE TABLE evidence (
  id CHAR(36) PRIMARY KEY, 
  tenant_id CHAR(36) NOT NULL, 
  request_id CHAR(36) NOT NULL, 
  context_pack_id CHAR(36) NOT NULL REFERENCES context_packs(id),
  seq INT NOT NULL, 
  source_id CHAR(36) NOT NULL, 
  ref VARCHAR(255) NOT NULL, 
  title TEXT NOT NULL,
  retrieved_at TIMESTAMP(6) NOT NULL, 
  freshness VARCHAR(16) NOT NULL CHECK (freshness IN ('fresh','stale','unknown')),
  trust VARCHAR(8) NOT NULL CHECK (trust IN ('high','medium','low')), 
  digest CHAR(64) NOT NULL, 
  size INT NOT NULL CHECK (size >= 0),
  excerpt TEXT NOT NULL, 
  used_by JSON, 
  created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE (tenant_id, request_id, seq)
);
