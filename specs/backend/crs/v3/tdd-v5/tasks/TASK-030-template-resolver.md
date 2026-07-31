# TASK-030: TemplateResolver

**Phase:** 5 — Workflow Orchestration  
**Solution ref:** [SOL-V5-004](../solutions/SOL-V5-004-workflow-orchestration.md) §6  
**Prerequisite:** TASK-028 (WorkflowTypes)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/workflow/TemplateResolver.ts`

Template inheritance chain resolver. MAX_INHERIT_DEPTH = 5 to prevent infinite loops.

```typescript
const MAX_INHERIT_DEPTH = 5

export class TemplateResolver {
  constructor(private readonly pool: IConnectionPool) {}

  async resolve(templateId: string): Promise<WorkflowDefinition> {
    // 1. Load template chain (follow parent_template_id)
    // 2. Merge from root → leaf (leaf overrides root)
    // 3. Throw if depth > MAX_INHERIT_DEPTH
    // 4. Return merged WorkflowDefinition
  }

  async create(params: { name: string; definition: WorkflowDefinition; ownerId: string; scope?: string; parentTemplateId?: string }): Promise<string>
  async list(scope: string, ownerId?: string): Promise<unknown[]>
}
```

## Acceptance Criteria

- [x] `TemplateResolver` class export
- [x] Inheritance chain resolved correctly
- [x] MAX_INHERIT_DEPTH=5 enforced
- [x] Không TypeScript errors
