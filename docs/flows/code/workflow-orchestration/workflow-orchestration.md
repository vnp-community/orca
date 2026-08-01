# Workflow Orchestration Flow — F36 Multi-Server Workflow Orchestration

> **Scope**: Luồng tạo template workflow → build DAG → thực thi song song theo waves → relay đến Dev Servers
>
> **Key files**:
> - [`src/main/workflow/workflow-types.ts`](../../src/main/workflow/workflow-types.ts) — WorkflowDefinition, StepDef, WorkflowExecution
> - [`src/main/workflow/dag-builder.ts`](../../src/main/workflow/dag-builder.ts) — DAGBuilder: build + topological sort
> - [`src/main/workflow/workflow-orchestrator.ts`](../../src/main/workflow/workflow-orchestrator.ts) — WorkflowOrchestrator: wave execution
> - [`src/main/workflow/template-resolver.ts`](../../src/main/workflow/template-resolver.ts) — TemplateResolver: inheritance chain merge
> - [`src/main/workflow/execution-store.ts`](../../src/main/workflow/execution-store.ts) — Resume-after-restart persistence
> - **Feature**: [F36 Multi-Server Workflow Orchestration](../features/F36-multi-server-workflow-orchestration.md)
> - **Business Logic**: [BL-WF-01](../logic/workflow-orchestration/BL-WF-01-workflow-template.md), [BL-WF-02](../logic/workflow-orchestration/BL-WF-02-workflow-execution.md), [BL-WF-03](../logic/workflow-orchestration/BL-WF-03-workflow-sharing.md)

---

## 1. Tổng quan — Workflow Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Workflow Template (định nghĩa tĩnh)                        │
│  scope: company | team | personal                           │
│  steps: [{ id, type, server, dependsOn, ... }]             │
│  Inheritance: company-base ← team-override ← personal      │
└───────────────────────────────┬─────────────────────────────┘
                                │ WorkflowOrchestrator.start()
                                ▼
┌─────────────────────────────────────────────────────────────┐
│  WorkflowExecution (runtime state)                          │
│  → DAGBuilder: topological sort → execution waves           │
│  → Wave 1: [step-A, step-B] (parallel, no deps)            │
│  → Wave 2: [step-C] (depends on A+B)                       │
│  → Wave 3: [step-D] (depends on C)                         │
│  State persisted in DB (resumable after Orca restart)       │
└───────────────────────────────┬─────────────────────────────┘
                                │
                    ┌───────────▼───────────┐
                    │  StepExecutors        │
                    │  agent | shell        │
                    │  action | webhook     │
                    │  parallel | condition │
                    └───────────┬───────────┘
                                │ relay RPC
                    ┌───────────▼───────────┐
                    │  Dev Servers (relay)  │
                    │  svr-01 / svr-02 / …  │
                    └───────────────────────┘
```

---

## 2. Workflow Template Structure

```typescript
// src/main/workflow/workflow-types.ts

interface WorkflowDefinition {
  id:        string
  name:      string
  scope:     'company' | 'team' | 'personal'
  teamId?:   string    // nếu scope='team'
  extends?:  string    // inherit từ template khác

  // Steps
  steps: StepDef[]

  // Template metadata
  description?: string
  tags:         string[]
  version:      number
}

interface StepDef {
  id:        string
  name:      string
  type:      StepType
  dependsOn: string[]  // other step IDs → DAG edges
  server?:   ServerRef // WHERE to run

  // Type-specific config
  agentConfig?:    AgentStepConfig
  shellConfig?:    ShellStepConfig
  actionConfig?:   ActionStepConfig
  webhookConfig?:  WebhookStepConfig
  parallelConfig?: ParallelStepConfig
  conditionConfig?: ConditionStepConfig
}

type StepType = 'agent' | 'shell' | 'action' | 'webhook' | 'parallel' | 'condition'

// Server targeting (where to run the step)
type ServerRef =
  | { type: 'project'; projectId: string }     // use project's bound dev server
  | { type: 'server'; devServerId: string }    // specific server
  | { type: 'fleet'; tag: string }             // any server with tag (round-robin)
```

---

## 3. Template Inheritance & Resolution

### 3.1 Inheritance Chain

```
company-base template:
  steps: [lint, test, build, deploy]

team-backend overrides company-base:
  extends: 'company-base'
  inject: { after: 'test', step: integration-test }
  override: { 'deploy': { server: { type: 'server', devServerId: 'prod-01' } } }
  remove: ['lint']  # backend team doesn't use lint step

personal override:
  extends: 'team-backend'
  override: { 'build': { shellConfig: { command: 'npm run build:dev' } } }
```

### 3.2 TemplateResolver.resolve()

```typescript
// src/main/workflow/template-resolver.ts
async resolve(templateId: string): Promise<ResolvedWorkflowDefinition> {
  const template = await TemplateRegistry.get(templateId)

  if (!template.extends) return applyOverrides(template, [])

  // Recursive resolution of inheritance chain
  const base = await this.resolve(template.extends)

  let steps = [...base.steps]

  // 1. Remove steps
  steps = steps.filter(s => !template.remove?.includes(s.id))

  // 2. Override step configs
  for (const [stepId, overrides] of Object.entries(template.override ?? {})) {
    const idx = steps.findIndex(s => s.id === stepId)
    if (idx >= 0) steps[idx] = deepMerge(steps[idx], overrides)
  }

  // 3. Inject new steps (after/before positioning)
  for (const inject of template.inject ?? []) {
    const afterIdx = steps.findIndex(s => s.id === inject.after)
    steps.splice(afterIdx + 1, 0, inject.step)
  }

  return { ...base, ...template, steps }
}
```

---

## 4. Execution Flow

### 4.1 DAG Build + Wave Partition

```typescript
// src/main/workflow/dag-builder.ts
class DAGBuilder {
  build(steps: StepDef[]): ExecutionDAG {
    // 1. Validate: no cycles (DFS)
    if (hasCycle(steps)) throw new Error('Workflow has circular dependency')

    // 2. Topological sort (Kahn's algorithm)
    const sorted = topologicalSort(steps)

    // 3. Partition into execution waves
    // Wave: set of steps where ALL dependencies are in previous waves
    const waves: StepDef[][] = []
    const done = new Set<string>()

    while (sorted.length > 0) {
      const wave = sorted.filter(s =>
        s.dependsOn.every(depId => done.has(depId))
      )
      if (wave.length === 0) throw new Error('DAG build failed: no progress')
      waves.push(wave)
      wave.forEach(s => {
        done.add(s.id)
        sorted.splice(sorted.indexOf(s), 1)
      })
    }

    return { steps: sorted, waves }
  }
}
```

### 4.2 WorkflowOrchestrator.start()

```
Lead trigger workflow "CI/CD Pipeline"
    │
    ▼ RPC: workflow.start({ templateId, projectId, inputs })
    │
    ├── TemplateResolver.resolve(templateId)
    │   → ResolvedWorkflowDefinition { steps: [lint, test, build, deploy] }
    │
    ├── DAGBuilder.build(steps)
    │   → ExecutionDAG { waves: [[lint, test], [build], [deploy]] }
    │
    ├── INSERT orca_workflow_executions (id, templateId, status='running', dag)
    │
    ├── WorkflowOrchestrator.executeDAG(execution)
    │   │
    │   ├── Wave 1: [lint, test] → PARALLEL
    │   │   ├── executeStep(lint)
    │   │   │   → WorkflowServerResolver.resolve(lint.server, projectId)
    │   │   │   → devServerId = 'svr-01'
    │   │   │   → relay = RelayConnectionPool.getOrConnect('svr-01')
    │   │   │   → StepExecutor.execute(lint, relay)
    │   │   │   → INSERT orca_step_executions (stepId, status='running')
    │   │   │
    │   │   └── executeStep(test)  [concurrent]
    │   │       → same flow, may use different server
    │   │
    │   │ [Wait for Wave 1 complete]
    │   │
    │   ├── Wave 2: [build]
    │   │   (depends on lint + test both done)
    │   │
    │   └── Wave 3: [deploy]
    │       (depends on build done)
    │
    └── UPDATE orca_workflow_executions status='completed'
        → Stream events to browser (workflow.step.update)
```

---

## 5. StepExecutors

### 5.1 Agent Step

```typescript
// AgentStepConfig: spawn AI agent on dev server
interface AgentStepConfig {
  provider:   string   // 'anthropic' | 'openai' | ...
  model?:     string   // override profile model
  prompt:     string   // can reference ${inputs.xxx}
  worktreePath?: string
  maxTokens?: number
}

async executeAgentStep(step: StepDef, relay: DevServerRelayBridge): Promise<StepResult> {
  const profile = await ProfileResolver.resolve(executionContext.userId)
  const apiKey = await ProviderResolver.resolve(...)

  const sessionId = await relay.call('pty.spawn', {
    binary: resolveAgentBinary(step.agentConfig!.provider),
    args: buildAgentArgs(profile, step.agentConfig),
    env: { ANTHROPIC_API_KEY: apiKey, ...profile.shell.envVars },
    cwd: step.agentConfig!.worktreePath,
  })

  return streamAndWait(relay, sessionId)
}
```

### 5.2 Shell Step

```typescript
interface ShellStepConfig {
  command: string   // e.g., 'npm run test:integration'
  cwd?:   string
  timeout?: number  // seconds
  env?:   Record<string, string>
}

async executeShellStep(step: StepDef, relay): Promise<StepResult> {
  return relay.callStream('shell.exec', {
    command: step.shellConfig!.command,
    cwd: step.shellConfig!.cwd,
    timeout: step.shellConfig!.timeout,
  })
}
```

### 5.3 Condition Step

```typescript
interface ConditionStepConfig {
  expression: string   // e.g., "${steps.test.exitCode} === 0"
  ifTrue:     string[] // step IDs to enable
  ifFalse:    string[] // step IDs to skip
}

async executeConditionStep(step: StepDef, context): Promise<StepResult> {
  const result = evalExpression(step.conditionConfig!.expression, context)
  if (result) {
    context.enableSteps(step.conditionConfig!.ifTrue)
  } else {
    context.skipSteps(step.conditionConfig!.ifFalse)
  }
  return { status: 'done', output: { conditionResult: result } }
}
```

---

## 6. Server Resolution

```typescript
// src/main/workflow/workflow-server-resolver.ts
async resolve(serverRef: ServerRef, projectId: string): Promise<string> {
  switch (serverRef.type) {
    case 'project':
      const project = await ProjectService.get(serverRef.projectId ?? projectId)
      return project.devServerId

    case 'server':
      return serverRef.devServerId

    case 'fleet':
      // Any healthy server with the tag (round-robin)
      const servers = await FleetHealthMonitor.getHealthyByTag(serverRef.tag)
      if (servers.length === 0) throw new Error(`No healthy server with tag: ${serverRef.tag}`)
      return servers[Math.floor(Math.random() * servers.length)].id
  }
}
```

---

## 7. Resumability — Restart Recovery

```typescript
// Khi Orca Server restart, các workflow đang chạy cần resume

async resumeOnBoot(): Promise<void> {
  const running = await db.query(
    "SELECT * FROM orca_workflow_executions WHERE status IN ('running', 'paused')"
  )

  for (const execution of running) {
    // Tìm steps chưa complete
    const pendingSteps = await db.query(
      "SELECT * FROM orca_step_executions WHERE execution_id=? AND status NOT IN ('done','failed','skipped')",
      [execution.id]
    )

    // Resumesfrom the last completed wave
    await this.executeDAG(execution, { resumeFrom: pendingSteps })
  }
}
```

---

## 8. DB Schema (Migration 0009)

```sql
CREATE TABLE orca_workflow_templates (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  scope       TEXT DEFAULT 'personal',  -- company | team | personal
  team_id     TEXT REFERENCES orca_departments(id),
  extends_id  TEXT REFERENCES orca_workflow_templates(id),
  definition  TEXT NOT NULL,            -- JSON: WorkflowDefinition
  created_by  TEXT REFERENCES orca_users(id),
  version     INTEGER DEFAULT 1,
  created_at  INTEGER,
  updated_at  INTEGER
);
CREATE INDEX idx_workflow_templates_scope ON orca_workflow_templates(scope, team_id);

CREATE TABLE orca_workflow_executions (
  id            TEXT PRIMARY KEY,
  template_id   TEXT REFERENCES orca_workflow_templates(id),
  project_id    TEXT REFERENCES orca_projects(id),
  triggered_by  TEXT REFERENCES orca_users(id),
  status        TEXT DEFAULT 'running', -- running | paused | completed | failed | cancelled
  dag_snapshot  TEXT,                   -- JSON: resolved DAG at execution time
  inputs        TEXT DEFAULT '{}',      -- user-provided inputs
  outputs       TEXT DEFAULT '{}',      -- final outputs
  started_at    INTEGER,
  completed_at  INTEGER
);

CREATE TABLE orca_step_executions (
  id            TEXT PRIMARY KEY,
  execution_id  TEXT REFERENCES orca_workflow_executions(id) ON DELETE CASCADE,
  step_id       TEXT NOT NULL,
  status        TEXT DEFAULT 'pending', -- pending | running | done | failed | skipped
  dev_server_id TEXT,
  stdout        TEXT,
  stderr        TEXT,
  exit_code     INTEGER,
  started_at    INTEGER,
  completed_at  INTEGER
);
CREATE INDEX idx_step_exec_execution ON orca_step_executions(execution_id);
```

---

## 9. RPC Methods — workflow.*

```typescript
'workflow.templates.list'     // (scope?, projectId?) → WorkflowDefinition[]
'workflow.templates.get'      // (templateId) → ResolvedWorkflowDefinition
'workflow.templates.create'   // (def) → WorkflowDefinition
'workflow.templates.update'   // (templateId, fields)
'workflow.templates.delete'   // (templateId)
'workflow.start'              // (templateId, projectId, inputs) → { executionId }
'workflow.stop'               // (executionId) — cancel in-flight
'workflow.pause'              // (executionId) — pause between waves
'workflow.resume'             // (executionId) — continue after pause
'workflow.getExecution'       // (executionId) → WorkflowExecution
'workflow.getActiveExecutions'// (projectId?) → WorkflowExecution[]
'workflow.getHistory'         // (projectId?, templateId?) → WorkflowExecution[]
'workflow.subscribe'          // (executionId) → stream { stepId, status, output }
```

---

## 10. Cross-References

| Resource | Mô tả |
|---|---|
| [task-agent-execution.md](./task-agent-execution.md) | Task có thể trigger workflow |
| [project-workspace-switch.md](./project-workspace-switch.md) | workspace.getActiveWorkflows trong switchProject |
| [relay-management.md](./relay-management.md) | Relay channel cho mỗi step |
| [ai-provider-credential.md](./ai-provider-credential.md) | API key cho agent steps |
| **HLD C2 Container 15** | Workflow Orchestrator |
| **HLD C4.9** | Workflow module detail |
| **F36 Workflow Orchestration** | Feature spec |
| **BL-WF-01** | Workflow template business logic |
| **BL-WF-02** | Workflow execution business logic |
| **BL-WF-03** | Workflow sharing business logic |
