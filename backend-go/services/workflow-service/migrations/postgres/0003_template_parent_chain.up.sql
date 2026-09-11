-- Adds the parent-chain column workflow-service.md §4/§5/§6 always
-- specified for template inheritance (company -> team -> personal) but
-- this scaffold's initial narrowed schema deferred — see
-- internal/domain/template.go's pre-2026-08-17 doc comment ("Template
-- inheritance resolution ... is out of scope for this scaffold's narrowed
-- data model"). Added once ResolveTemplate/ListTemplates were actually
-- implemented (not before), per the execution plan's own deferral
-- condition for this item.
ALTER TABLE workflow.templates ADD COLUMN parent_template_id UUID REFERENCES workflow.templates(id) ON DELETE SET NULL;

-- Walked by ResolveTemplate's recursive CTE (workflow-service.md §6: depth
-- capped at 5, WITH RECURSIVE chain AS (...) — see
-- internal/adapter/postgres/repository.go's ResolveChain).
CREATE INDEX idx_workflow_templates_parent ON workflow.templates (parent_template_id);
