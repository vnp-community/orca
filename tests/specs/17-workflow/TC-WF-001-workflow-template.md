# TC-WF-001 — Workflow Template Management

**BL Reference:** BL-WF-01  
**Flow Reference:** docs/flows/logic/workflow-orchestration.md  
**Priority:** P1  
**Type:** Integration  
**Actor:** Admin, Lead, User

---

## TC-WF-001-01: Tạo workflow template — Happy path

**Priority:** P1

### Preconditions
- User logged in với `create_template` permission
- Dev server `srv-1` online

### Test Data
| Field | Value |
|-------|-------|
| name | "CI/CD Pipeline" |
| scope | "team" |
| steps[0].id | "test" |
| steps[0].type | "shell" |
| steps[0].server | "project:proj-123" |
| steps[0].command | "npm test" |
| steps[1].id | "build" |
| steps[1].type | "shell" |
| steps[1].depends_on | ["test"] |

### Steps
1. `POST /api/workflows/templates` với payload YAML trên

### Expected Results
- HTTP 201, templateId returned
- DB: `INSERT orca_workflow_templates { id, name, scope, definition_json, owner_id }`
- Dependency DAG: test → build (valid, no cycle)

### Assertions
```
result = await api.post('/api/workflows/templates', templatePayload)
assert result.status === 201
assert result.body.templateId !== undefined
dbRow = db.select('orca_workflow_templates', { id: result.body.templateId })
assert dbRow.name === 'CI/CD Pipeline'
assert dbRow.scope === 'team'
```

### Error Scenarios
| Scenario | Input | Expected |
|----------|-------|----------|
| Duplicate step IDs | steps: [{id:'A'}, {id:'A'}] | 400 DUPLICATE_STEP_ID |
| Cycle in depends_on | A→B, B→A | 400 CYCLE_DETECTED |
| Invalid step type | type: 'unknown' | 400 INVALID_STEP_TYPE |
| Empty steps array | steps: [] | 400 NO_STEPS |

---

## TC-WF-001-02: Template inheritance — inject_steps after

**Priority:** P1

### Preconditions
- Parent template exists: steps [lint, implement, test]

### Steps
1. Create child template:
   ```yaml
   template_id: "<parent-id>"
   inject_steps:
     after: "implement"
     steps:
       - id: "format"
         type: "shell"
         command: "make fmt"
   ```
2. Resolve child template via `TemplateResolver.resolve(childId)`

### Expected Results
- Effective step order: **lint → implement → format → test**
- format injected immediately after implement
- Original parent unchanged

### Assertions
```
resolved = await api.get('/api/workflows/templates/' + childId + '/resolved')
stepIds = resolved.steps.map(s => s.id)
assert stepIds === ['lint', 'implement', 'format', 'test']
```

---

## TC-WF-001-03: Template inheritance — inject_steps before

**Priority:** P1

### Steps
1. Parent: steps [lint, implement, test]
2. Child:
   ```yaml
   inject_steps:
     before: "lint"
     steps:
       - id: "precheck"
         type: "shell"
         command: "make check"
   ```

### Expected Results
- Effective step order: **precheck → lint → implement → test**

---

## TC-WF-001-04: Template inheritance — override step field

**Priority:** P1

### Steps
1. Parent: `steps.implement.provider.model: "claude-opus-4-5"`
2. Child:
   ```yaml
   overrides:
     steps.implement.provider.model: "gpt-4o"
   ```

### Expected Results
- Resolved template: `implement.provider.model === "gpt-4o"` (child overrides parent)
- Other parent fields preserved

### Assertions
```
resolved = await api.get('/api/workflows/templates/' + childId + '/resolved')
implementStep = resolved.steps.find(s => s.id === 'implement')
assert implementStep.provider.model === 'gpt-4o'
```

---

## TC-WF-001-05: Template inheritance — remove_steps

**Priority:** P1

### Steps
1. Parent: steps [A, B, C]
2. Child: `remove_steps: ['B']`

### Expected Results
- Effective steps: **[A, C]**
- B not present in resolved

### Assertions
```
resolved = resolveTemplate(childId)
assert !resolved.steps.find(s => s.id === 'B')
assert resolved.steps.map(s => s.id).join(',') === 'A,C'
```

---

## TC-WF-001-06: Template visibility scopes

**Priority:** P1

### Test Matrix
| Visibility | Who can see |
|------------|------------|
| `private` | Owner only |
| `team` | Team members only |
| `company` | All company users |
| `public` | Anyone with share link |

### Steps (team scope)
1. Create template `{ visibility: 'team', teamId: 'team-engineering' }`
2. User A (team member): GET `/api/workflows/library` → template visible
3. User B (other team): GET `/api/workflows/library` → template NOT visible

### Assertions
```
loginAs(userA_engineering)
list = await api.get('/api/workflows/library')
assert list.some(t => t.id === templateId)

loginAs(userB_design)
list = await api.get('/api/workflows/library')
assert !list.some(t => t.id === templateId)
```

---

## TC-WF-001-07: Template inheritance — 3-level chain

**Priority:** P1

### Steps
1. Company template: steps [A, B, C], model: "claude-opus-4-5"
2. Team template: inherits company, override model → "gpt-4o", inject D after B
3. Personal workflow: inherits team, inject E before A

### Expected Results
- Resolved steps: **[E, A, B, D, C]**
- model: "gpt-4o" (team overrides company)

---

*TC-WF-001 — Orca v5.0 — Updated 2026-08-01*
