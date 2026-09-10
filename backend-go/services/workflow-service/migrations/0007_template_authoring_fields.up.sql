ALTER TABLE workflow.templates
  ADD COLUMN description TEXT,
  ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN owner_id UUID,                      -- backfilled below, then NOT NULL in a follow-up migration
  ADD COLUMN usage_count INT NOT NULL DEFAULT 0,
  ADD COLUMN overrides JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN inject_steps JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN remove_steps JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN cloned_from_template_id UUID REFERENCES workflow.templates(id) ON DELETE SET NULL;

-- Backfill: no owner history exists for templates created before this
-- migration — the only safe backfill value is each row's own tenant,
-- treated as an implicit "system"-owned template. Real ownership for
-- pre-existing rows must be reconciled by a data-fix script per tenant
-- (out of scope here) — this migration only expands the shape.
UPDATE workflow.templates SET owner_id = tenant_id WHERE owner_id IS NULL;

CREATE INDEX idx_workflow_templates_tags ON workflow.templates USING GIN (tags);
CREATE INDEX idx_workflow_templates_owner ON workflow.templates(tenant_id, owner_id);
