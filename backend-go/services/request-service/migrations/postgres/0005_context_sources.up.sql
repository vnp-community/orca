-- Postgres, schema request. RLS đúng mẫu SOL-001 mục 2.D (FORCE + NULLIF + policy tenant_isolation).

CREATE TABLE request.context_sources (
  id UUID PRIMARY KEY, 
  tenant_id UUID NOT NULL,
  source_key TEXT NOT NULL CHECK (source_key ~ '^[a-z][a-z0-9_]{1,63}$'),
  kind TEXT NOT NULL CHECK (kind IN ('request_origin','conventions','decisions','specs','service_catalog','code_graph',
    'git_history','contracts','schema','dependencies','ci_config','ci_results','coverage','observability','incidents',
    'feature_flags','security_scan','policy','ownership','history','dev_server_profile','external_knowledge')),
  transport TEXT NOT NULL CHECK (transport IN ('internal','mcp')),
  adapter TEXT, 
  server_ref UUID,
  scopes JSONB NOT NULL DEFAULT '[]', 
  trust TEXT NOT NULL CHECK (trust IN ('high','medium','low')),
  ttl_seconds INT NOT NULL DEFAULT 300 CHECK (ttl_seconds >= 0), 
  max_bytes INT NOT NULL DEFAULT 65536 CHECK (max_bytes BETWEEN 1024 AND 1048576),
  redaction JSONB NOT NULL DEFAULT '{"profiles":["secrets"]}', 
  rate_limit_per_minute INT NOT NULL DEFAULT 60 CHECK (rate_limit_per_minute > 0),
  enabled_for JSONB NOT NULL, 
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','disabled')),
  owner_id UUID NOT NULL, 
  version BIGINT NOT NULL DEFAULT 1, 
  created_by UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), 
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, source_key),
  CONSTRAINT context_sources_transport_shape CHECK ((transport = 'internal' AND adapter IS NOT NULL AND server_ref IS NULL)
                                               OR (transport = 'mcp' AND server_ref IS NOT NULL AND adapter IS NULL))
);

CREATE TABLE request.context_packs (
  id UUID PRIMARY KEY, 
  tenant_id UUID NOT NULL, 
  request_id UUID NOT NULL REFERENCES request.requests(id),
  stage TEXT NOT NULL CHECK (stage IN ('classify','solution','plan','task','execute','risk')),
  cp_version TEXT NOT NULL, 
  input_digest CHAR(64) NOT NULL, 
  digest CHAR(64) NOT NULL,
  budget_tokens INT NOT NULL, 
  used_tokens INT NOT NULL, 
  items JSONB NOT NULL, 
  missing JSONB NOT NULL,
  body TEXT NOT NULL, 
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX context_packs_latest ON request.context_packs (tenant_id, request_id, stage, created_at DESC);
CREATE INDEX context_packs_input ON request.context_packs (tenant_id, request_id, stage, input_digest);

CREATE TABLE request.evidence (
  id UUID PRIMARY KEY, 
  tenant_id UUID NOT NULL, 
  request_id UUID NOT NULL, 
  context_pack_id UUID NOT NULL REFERENCES request.context_packs(id),
  seq INT NOT NULL, 
  source_id TEXT NOT NULL, 
  ref TEXT NOT NULL, 
  title TEXT NOT NULL DEFAULT '',
  retrieved_at TIMESTAMPTZ NOT NULL, 
  freshness TEXT NOT NULL CHECK (freshness IN ('fresh','stale','unknown')),
  trust TEXT NOT NULL CHECK (trust IN ('high','medium','low')), 
  digest CHAR(64) NOT NULL, 
  size INT NOT NULL CHECK (size >= 0),
  excerpt TEXT NOT NULL DEFAULT '' CHECK (octet_length(excerpt) <= 4096), 
  used_by JSONB, 
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, request_id, seq)
);

ALTER TABLE request.context_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.context_sources FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.context_sources
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE request.context_packs ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.context_packs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.context_packs
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);

ALTER TABLE request.evidence ENABLE ROW LEVEL SECURITY;
ALTER TABLE request.evidence FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON request.evidence
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid);
