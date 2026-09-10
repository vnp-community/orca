ALTER TABLE workflow.templates
  ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private'
    CHECK (visibility IN ('private','team','company','public')),
  ADD COLUMN share_token TEXT UNIQUE,           -- NULL until visibility reaches 'public'
  ADD COLUMN rating_sum   INT NOT NULL DEFAULT 0,
  ADD COLUMN rating_count INT NOT NULL DEFAULT 0; -- average computed at read time, not stored

CREATE INDEX idx_workflow_templates_visibility ON workflow.templates(tenant_id, visibility);
CREATE UNIQUE INDEX idx_workflow_templates_share_token ON workflow.templates(share_token) WHERE share_token IS NOT NULL;
-- Trending sort needs both usage_count (0007) and rating; a composite
-- index keeps ListTemplates(sort=trending) an index-only scan.
CREATE INDEX idx_workflow_templates_trending ON workflow.templates(tenant_id, visibility, usage_count DESC, rating_sum DESC);
-- Full-text search backing TASK-WF-03-07's `query` filter.
CREATE INDEX idx_workflow_templates_fts ON workflow.templates USING GIN (to_tsvector('english', name || ' ' || coalesce(description,'')));

CREATE TABLE workflow.ratings (
  template_id UUID NOT NULL REFERENCES workflow.templates(id) ON DELETE CASCADE,
  user_id UUID NOT NULL,
  stars SMALLINT NOT NULL CHECK (stars BETWEEN 1 AND 5),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (template_id, user_id)
);

-- RLS via template_id join — ratings carries no tenant_id column of its
-- own; the application layer scopes every query by joining to
-- workflow.templates (see internal/adapter/postgres), and this policy is
-- the secondary defense-in-depth backstop for that join, same idiom as
-- 0004_step_executions' execution_id join.
ALTER TABLE workflow.ratings ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON workflow.ratings
    USING (EXISTS (
        SELECT 1 FROM workflow.templates t
        WHERE t.id = ratings.template_id
          AND t.tenant_id = current_setting('app.tenant_id', true)::uuid
    ));

-- approvals mirrors orchestration-service.md §5's decision_gates table
-- shape deliberately (id/tenant_id/.../status CHECK/resolved_at) — same "a
-- row gating a state transition until a human resolves it" idiom.
CREATE TABLE workflow.approvals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL,
  template_id UUID NOT NULL REFERENCES workflow.templates(id) ON DELETE CASCADE,
  requested_by UUID NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  resolved_by UUID,
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_workflow_approvals_pending ON workflow.approvals(tenant_id, status) WHERE status = 'pending';
CREATE UNIQUE INDEX idx_workflow_approvals_one_pending_per_template ON workflow.approvals(template_id) WHERE status = 'pending';

-- approvals DOES carry its own tenant_id (unlike ratings) — direct policy,
-- matching workflow.templates'/workflow.executions' own pattern.
ALTER TABLE workflow.approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON workflow.approvals
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
