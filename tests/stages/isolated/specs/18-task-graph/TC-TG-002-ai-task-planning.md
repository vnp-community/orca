# TC-TG-002 — AI-Assisted Task Planning & Decomposition

**BL Reference:** BL-TG-02  
**Flow Reference:** docs/flows/logic/task-graph.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Developer, Lead

---

## TC-TG-002-01: AI decompose task — Generate subtasks với dependency graph

**Priority:** P0

### Preconditions
- Task: `{ title: 'Implement JWT authentication', type: 'story', description: '...' }`
- AI provider mocked (returns predefined JSON)

### Test Data (mocked AI response)
```json
{
  "subtasks": [
    { "title": "Implement JWT token generation", "type": "subtask", "estimatedHours": 3, "promptTemplate": "..." },
    { "title": "Implement token validation middleware", "type": "subtask", "estimatedHours": 2 },
    { "title": "Add refresh token logic", "type": "subtask", "estimatedHours": 4, "depends_on": ["Implement JWT token generation"] }
  ],
  "totalEstimate": 9,
  "criticalPath": ["Implement JWT token generation", "Add refresh token logic"]
}
```

### Steps
1. `POST /api/tasks/:id/ai-plan`
2. Verify AI receives correct prompt
3. Verify response parsed and stored

### Expected Results
- DB: `orca_tasks.ai_plan_json` = raw AI response
- API returns plan for user review (NOT auto-applied)
- Subtasks in preview with checkboxes

### Assertions
```
result = await api.post('/api/tasks/' + taskId + '/ai-plan')
assert result.subtasks.length === 3
assert result.subtasks[0].title === 'Implement JWT token generation'
assert typeof result.subtasks[0].estimatedHours === 'number'
assert result.totalEstimate === 9

// Check AI prompt included task context
aiPrompt = capturedAICall.prompt
assert aiPrompt.includes('Implement JWT authentication')
assert aiPrompt.includes(project.name)
```

---

## TC-TG-002-02: Apply AI plan — Insert accepted subtasks + edges

**Priority:** P0

### Preconditions
- AI plan generated (TC-TG-002-01)
- User accepts subtasks 1 and 3 (rejects subtask 2)

### Steps
1. `POST /api/tasks/:id/ai-plan/apply { acceptedIndices: [0, 2] }`

### Expected Results
- 2 subtasks inserted: `orca_tasks` (parentId = taskId)
- Dependency edge: subtask-0 → subtask-2 (via `depends_on`)
- `task.aiPlanJson` updated
- Rejected subtask 2 NOT inserted

### Assertions
```
await api.post('/api/tasks/' + taskId + '/ai-plan/apply', { acceptedIndices: [0, 2] })
subtasks = db.tasks.find({ parentId: taskId })
assert subtasks.length === 2
assert subtasks[0].title === 'Implement JWT token generation'
assert subtasks[1].title === 'Add refresh token logic'

edge = db.taskEdges.find({ from_task_id: subtasks[1].id, to_task_id: subtasks[0].id })
assert edge !== undefined  // dependency: refresh → token gen

assert !subtasks.find(s => s.title === 'Implement token validation middleware')
```

---

## TC-TG-002-03: Apply plan triggers DAG cycle check

**Priority:** P0

### Preconditions
- Task T already has subtask-X
- AI plan suggests: subtask-Y depends on T (would create cycle: T → subtask-Y → T)

### Steps
1. `POST /api/tasks/:id/ai-plan/apply { acceptedIndices: [bad-subtask-index] }`

### Expected Results
- Error: `{ code: 'CYCLE_DETECTED' }` during applyPlan
- No subtasks inserted (rollback)

---

## TC-TG-002-04: AI generate agent prompt từ task context

**Priority:** P0

### Preconditions
- Task: `{ title: 'Implement bcrypt hashing', description: '...', aiContext: 'Use 12 rounds' }`
- Project: `{ name: 'vnp-blc-backend', techStack: 'Node.js/TypeScript' }`

### Steps
1. `POST /api/tasks/:id/generate-prompt`

### Expected Results
- AI call với: title + description + aiContext + project tech stack
- Generated prompt includes:
  - File path recommendations
  - Function signature guidance
  - Test file references
- Prompt stored in `task.promptTemplate` (editable by user)

### Assertions
```
result = await api.post('/api/tasks/' + taskId + '/generate-prompt')
assert result.prompt.length > 50
assert result.prompt.includes('bcrypt')
assert result.prompt.toLowerCase().includes('typescript')

// Saveable
await api.patch('/api/tasks/' + taskId, { promptTemplate: result.prompt })
task = await api.get('/api/tasks/' + taskId)
assert task.promptTemplate === result.prompt
```

---

## TC-TG-002-05: Critical path calculation

**Priority:** P1

### Steps
1. Task graph với estimates:
   - A(2h) → B(3h) → D(1h)
   - A(2h) → C(5h) → D(1h)
2. `GET /api/tasks/:rootId/critical-path`

### Expected Results
- Critical path: A → C → D (total: 8h)
- Non-critical: A → B → D (total: 6h)
- B is NOT on critical path

### Assertions
```
result = await api.get('/api/tasks/' + A.id + '/critical-path')
assert result.criticalPath.map(t => t.id).join(',') === [A.id, C.id, D.id].join(',')
assert result.totalDuration === 8
assert !result.criticalPath.find(t => t.id === B.id)
```

---

## TC-TG-002-06: Tech stack detection from dev server

**Priority:** P1

### Preconditions
- Dev server has `package.json` (Node.js project)
- Mock `relay.call('fs.readFile', { path: '.../package.json' })` returns Node.js deps

### Steps
1. `POST /api/tasks/:id/ai-plan` (triggers context collection)

### Expected Results
- AI prompt includes tech stack: "Node.js / TypeScript / Express"
- Tech stack derived from package.json analysis

### Assertions
```
capturedAICall = spy('relay.call', 'ai.complete')
await api.post('/api/tasks/' + taskId + '/ai-plan')
assert capturedAICall.prompt.includes('Node.js')
assert capturedAICall.prompt.includes('TypeScript')
```

---

## TC-TG-002-07: AI decompose — AI provider timeout

**Priority:** P1

### Preconditions
- AI provider mock: timeout after 30s

### Steps
1. `POST /api/tasks/:id/ai-plan` (AI doesn't respond)

### Expected Results
- Error: `{ code: 'AI_TIMEOUT', timeoutMs: 30000 }`
- No partial subtasks inserted
- DB: `task.aiPlanJson` unchanged

### Assertions
```
mockAI.timeout(30000)
result = await api.post('/api/tasks/' + taskId + '/ai-plan').catch(e => e)
assert result.code === 'AI_TIMEOUT'
assert db.tasks.count({ parentId: taskId }) === 0  // no partial subtasks
```

---

## TC-TG-002-08: AI decompose — Invalid JSON response → raw display

**Priority:** P1

### Steps
1. AI responds with non-JSON: `"Sorry, I can't help with that."`
2. `POST /api/tasks/:id/ai-plan`

### Expected Results
- Error: `{ code: 'AI_INVALID_RESPONSE', raw: '...' }`
- Raw AI response shown to user for manual parsing
- No subtasks inserted
- Retry option available

---

*TC-TG-002 — Orca v5.0 — Updated 2026-08-01*
