# TC-WF-002 — Multi-Server Workflow Execution

**BL Reference:** BL-WF-02  
**Flow Reference:** docs/flows/logic/workflow-orchestration.md  
**Priority:** P1  
**Type:** Integration  
**Actor:** Developer, Lead, System

---

## TC-WF-002-01: DAG execution — Mixed dependency waves

**Priority:** P1

### Steps
1. Workflow với steps:
   - A (no deps)
   - B (no deps)
   - C (depends_on: A)
   - D (depends_on: A, B)
2. `POST /api/workflows/execute { templateId, params: {} }`

### Expected Results
- Wave 1: [A, B] execute in parallel
- Wave 2 (after A done): [C]
- Wave 3 (after A and B done): [D]
- Timeline: A||B → C → D

### Assertions
```
execution = await api.post('/api/workflows/execute', { templateId })
events = collectSSEEvents(execution.id)

wave1 = events.filter(e => e.type === 'step.started' && e.wave === 0).map(e => e.stepId)
assert wave1.sort() deepEqual ['A', 'B']

// C starts only after A completes
aComplete = events.find(e => e.type === 'step.completed' && e.stepId === 'A')
cStart = events.find(e => e.type === 'step.started' && e.stepId === 'C')
assert cStart.timestamp >= aComplete.timestamp

// D starts after both A and B complete
bComplete = events.find(e => e.type === 'step.completed' && e.stepId === 'B')
dStart = events.find(e => e.type === 'step.started' && e.stepId === 'D')
assert dStart.timestamp >= Math.max(aComplete.timestamp, bComplete.timestamp)
```

---

## TC-WF-002-02: Multi-server dispatch — Correct server routing

**Priority:** P1

### Steps
1. Step 'test': `server: 'project:proj-frontend'` → resolves to dev-server-A
2. Step 'deploy': `server: 'server:srv-prod-1'` → resolves to dev-server-B
3. Execute workflow

### Expected Results
- 'test' step executes on dev-server-A (proj-frontend's devServerId)
- 'deploy' step executes on dev-server-B (direct ID)
- No cross-contamination

### Assertions
```
execution = await api.post('/api/workflows/execute', { templateId })
stepExecs = db.select('orca_step_executions', { execution_id: execution.id })
testStep = stepExecs.find(s => s.step_id === 'test')
deployStep = stepExecs.find(s => s.step_id === 'deploy')
assert testStep.server_id === devServerA.id
assert deployStep.server_id === 'srv-prod-1'
```

---

## TC-WF-002-03: fleet:tag: server resolution — Load balance

**Priority:** P1

### Steps
1. Step: `server: 'fleet:tag:backend'`
2. Healthy servers with tag 'backend': [srv-1, srv-2, srv-3]
3. Execute same workflow 10 times

### Expected Results
- Each execution dispatched to one of [srv-1, srv-2, srv-3]
- No execution on unhealthy servers
- Distribution roughly even (load balance)

### Assertions
```
// Mock: FleetHealthMonitor returns [srv-1, srv-2] healthy, srv-3 unhealthy
serverIds = []
for (let i = 0; i < 10; i++) {
  exec = await api.post('/api/workflows/execute', { templateId })
  stepExec = db.select('orca_step_executions WHERE execution_id=?', exec.id)
  serverIds.push(stepExec.server_id)
}
assert serverIds.every(id => ['srv-1', 'srv-2'].includes(id))
assert !serverIds.includes('srv-3')  // unhealthy excluded
```

---

## TC-WF-002-04: Variable interpolation — inputs + outputs + system

**Priority:** P1

### Preconditions
- Workflow has input variable `feature_description`
- Step 'backend' outputs `api_endpoint`

### Steps
1. Execute: `{ params: { feature_description: 'Add OAuth login' } }`
2. Step 'backend' prompt: `"Build: {{feature_description}}"`
3. Step 'frontend' prompt: `"Use API: {{outputs.backend.api_endpoint}}, User: {{user.email}}, At: {{now()}}"`

### Expected Results
- backend prompt: `"Build: Add OAuth login"`
- frontend prompt: `"Use API: /api/v2/oauth, User: dev@company.com, At: 2026-08-01T..."`
- Unknown `{{xyz}}` → passthrough as `{{xyz}}`

### Assertions
```
captured = spyRelayCall('agent.spawn')
backendCall = captured.find(c => c.args.stepId === 'backend')
assert backendCall.args.prompt.includes('Add OAuth login')

frontendCall = captured.find(c => c.args.stepId === 'frontend')
assert frontendCall.args.prompt.includes('/api/v2/oauth')
assert frontendCall.args.prompt.includes('dev@company.com')
assert frontendCall.args.prompt.match(/\d{4}-\d{2}-\d{2}T/)  // ISO date
```

---

## TC-WF-002-05: Workflow resumability — Orca restart mid-execution

**Priority:** P1

### Preconditions
- Workflow: steps [A, B, C, D, E]
- A completed, B in progress, C-E pending

### Steps
1. Start workflow
2. Mark step A as completed, step B as running (in DB)
3. Simulate Orca Server restart
4. On startup: scan `orca_workflow_executions WHERE status='running'`
5. Resume

### Expected Results
- Step A: skip (use saved output)
- Step B: treated as interrupted → retry from scratch
- Steps C-E: continue from wave B onwards
- No duplicate execution of A

### Assertions
```
// Pre-restart state
db.update('orca_step_executions', { status: 'completed' }, { step_id: 'A' })
db.update('orca_step_executions', { status: 'running' }, { step_id: 'B' })

// Simulate restart
await orca.restart()
await waitFor(() => execution.status === 'completed')

// A was NOT re-executed (no new spawn call for A)
spawnCalls = capturedRelaySpawnCalls()
assert spawnCalls.filter(c => c.stepId === 'A').length === 0  // no re-run
assert spawnCalls.filter(c => c.stepId === 'B').length === 1  // retried once
```

---

## TC-WF-002-06: Parallel step type — allowPartialFailure=false

**Priority:** P1

### Steps
1. Step type='parallel':
   ```yaml
   - id: "parallel-block"
     type: "parallel"
     allowPartialFailure: false
     steps:
       - id: "sub-a"
         type: "shell"
         command: "exit 0"
       - id: "sub-b"
         type: "shell"
         command: "exit 1"  # fails
   ```
2. Execute workflow

### Expected Results
- sub-a: success
- sub-b: failed (exit code 1)
- allowPartialFailure=false → workflow STOPPED
- `orca_workflow_executions.status = 'failed'`

---

## TC-WF-002-07: Parallel step type — allowPartialFailure=true

**Priority:** P1

### Steps
1. Same as TC-WF-002-06 but `allowPartialFailure: true`

### Expected Results
- sub-b fails → WARNING logged
- Workflow continues to next wave
- `orca_workflow_executions.status = 'completed'` (with warning)

---

## TC-WF-002-08: Condition step — Branch on output

**Priority:** P1

### Steps
1. Step 'test': `type: 'shell', command: 'npm test'`
2. Step 'condition': 
   ```yaml
   type: condition
   if: "{{outputs.test.exitCode}} === 0"
   then: deploy
   else: notify-fail
   ```

### Expected Results
- If test exits 0: 'deploy' runs, 'notify-fail' skipped
- If test exits non-0: 'notify-fail' runs, 'deploy' skipped

---

## TC-WF-002-09: Real-time event stream

**Priority:** P1

### Steps
1. Execute workflow
2. Subscribe to SSE stream: `GET /api/workflows/executions/:id/stream`

### Expected Results
Events received in order:
- `{ type: 'execution.started', executionId }`
- `{ type: 'step.started', stepId: 'A', status: 'running', wave: 0 }`
- `{ type: 'step.output', stepId: 'A', line: '...' }`  (0..N)
- `{ type: 'step.completed', stepId: 'A', outputs: {...} }`
- ... (per step)
- `{ type: 'execution.completed', executionId, summary }`

### Assertions
```
events = []
sseClient.on('message', e => events.push(JSON.parse(e.data)))
await api.post('/api/workflows/execute', { templateId })
await waitFor(() => events.some(e => e.type === 'execution.completed'))

assert events[0].type === 'execution.started'
lastEvent = events[events.length - 1]
assert lastEvent.type === 'execution.completed'
// Every step has started + completed pair
stepIds = new Set(events.filter(e => e.type === 'step.started').map(e => e.stepId))
stepIds.forEach(id => {
  assert events.some(e => e.type === 'step.completed' && e.stepId === id)
})
```

---

## TC-WF-002-10: Step fail → Workflow stopped + state persisted

**Priority:** P1

### Steps
1. Step 'B' fails (agent returns non-zero / shell error)
2. Dependent steps C, D pending

### Expected Results
- Workflow status: `failed`
- Step B: `status: 'failed'`
- Steps C, D: `status: 'pending'` (not started)
- State persisted in DB (resumable if needed)

### Assertions
```
// Simulate step B failure
mockRelayCall('shell.exec', { stepId: 'B' }, { exitCode: 1, error: 'command failed' })

execution = await api.post('/api/workflows/execute', { templateId })
await waitFor(() => execution.status === 'failed' || 'completed')

exec = db.select('orca_workflow_executions', { id: execution.id })
assert exec.status === 'failed'

stepB = db.select('orca_step_executions', { step_id: 'B', execution_id: exec.id })
assert stepB.status === 'failed'

stepC = db.select('orca_step_executions', { step_id: 'C', execution_id: exec.id })
assert stepC.status === 'pending'
```

---

*TC-WF-002 — Orca v5.0 — Updated 2026-08-01*
