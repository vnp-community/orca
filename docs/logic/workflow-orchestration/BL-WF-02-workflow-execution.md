# BL-WF-02 — Multi-Server Workflow Execution

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-WF-02 |
| **Tên** | Multi-Server Workflow Execution |
| **Domain** | Workflow Orchestration |
| **Actor** | Developer, Lead, System (scheduler) |
| **Priority** | P1 |

---

## Mô tả

Orchestrator chạy workflow: build DAG từ steps + depends_on, dispatch song song các step độc lập, collect outputs, và stream trạng thái realtime về UI. Mỗi step có thể chạy trên server và AI provider khác nhau.

---

## Execution Lifecycle

```
1. User run workflow → nhập input variables ({{feature_description}}...)
       │
2. WorkflowOrchestrator.start(workflowId, userId, inputs)
       │
3. resolveTemplate(workflowId)   ← merge parent template nếu có inheritance
       │
4. DAGBuilder.build(steps)
       → Validate: no cycles in depends_on
       → Build adjacency graph
       → Topological sort → execution waves
       
       Wave 0 (no deps): [lint-check]
       Wave 1 (after wave 0): [backend-impl, docs-gen]  ← parallel
       Wave 2 (after wave 1): [run-tests]
       Wave 3: [create-pr]
       │
5. INSERT orca_workflow_executions (id, workflow_id, user_id, status='running', inputs)
   INSERT orca_step_executions per step (status='pending')
       │
6. Execute Wave 0:
       │
       ├── For each step in wave:
       │   ├── ServerResolver.resolve(step.server, ctx)
       │   │     → devServerId (from project binding / direct / fleet tag)
       │   │
       │   ├── ProviderResolver.resolve(step.provider, devServerId, ctx)
       │   │     → AIProviderAccount (priority cascade)
       │   │
       │   ├── StepExecutor.execute(step, devServerId, providerAccount, resolvedInputs)
       │   │     → agent: ProfileAwareAgentSpawner.spawn(...)
       │   │     → shell: relay.call('shell.exec', {cmd})
       │   │     → action: ActionExecutor.run(action, params)
       │   │     → webhook: fetch(url, body)
       │   │
       │   ├── Stream step output → WebSocket event → UI
       │   │   { type: 'step.output', executionId, stepId, line: '...' }
       │   │
       │   └── On step complete:
       │       UPDATE orca_step_executions SET status='completed', output=?
       │       Emit { type: 'step.completed', stepId, outputs }
       │
7. Execute subsequent waves (after all deps complete)
       │
8. All waves done → UPDATE orca_workflow_executions SET status='completed'
   Emit { type: 'execution.completed', executionId, summary }
```

---

## Server Resolution

```typescript
async function resolveServer(serverSpec: string, ctx: ExecutionContext): Promise<string> {
  // "project:<projectId>" → project.devServerId
  if (serverSpec.startsWith('project:')) {
    const project = await ProjectService.get(serverSpec.slice(8))
    return project.devServerId
  }

  // "server:<serverId>" → direct
  if (serverSpec.startsWith('server:')) return serverSpec.slice(7)

  // "fleet:tag:<tag>" → load balance across servers with tag
  if (serverSpec.startsWith('fleet:tag:')) {
    const tag = serverSpec.slice(10)
    const servers = await FleetHealthMonitor.getHealthyByTag(tag)
    if (servers.length === 0) throw new Error(`No healthy server with tag '${tag}'`)
    return servers[Math.floor(Math.random() * servers.length)].id  // simple round-robin
  }

  // Inherit from workflow context (project's server)
  return ctx.defaultDevServerId
}
```

---

## Parallel Execution (parallel step type)

```typescript
case 'parallel': {
  const subResults = await Promise.allSettled(
    step.steps.map(subStep =>
      executeStep(subStep, ctx)
    )
  )
  // Collect all outputs, fail workflow if any sub-step fails (configurable)
  const failed = subResults.filter(r => r.status === 'rejected')
  if (failed.length > 0 && !step.allowPartialFailure) {
    throw new WorkflowError(`Parallel step failed: ${failed.length} sub-steps failed`)
  }
  return mergeOutputs(subResults)
}
```

---

## Variable Interpolation

```typescript
// {{feature_description}} → từ inputs
// {{outputs.backend-impl.api_endpoint}} → từ step output
// {{project.name}}, {{user.email}}, {{now()}}

function interpolate(template: string, ctx: InterpolationContext): string {
  return template.replace(/\{\{([^}]+)\}\}/g, (_, expr) => {
    if (expr.startsWith('outputs.')) {
      const [, stepId, ...path] = expr.split('.')
      return getDeep(ctx.outputs[stepId], path.join('.')) ?? ''
    }
    if (expr.startsWith('project.')) return getDeep(ctx.project, expr.slice(8))
    if (expr.startsWith('user.')) return getDeep(ctx.user, expr.slice(5))
    if (expr === 'now()') return new Date().toISOString()
    return ctx.inputs[expr] ?? `{{${expr}}}`  // passthrough if not found
  })
}
```

---

## Resumability

```
Orca Server restart mid-execution:
    │
    ├── On startup: scan orca_workflow_executions WHERE status='running'
    │
    ├── For each: load step states, rebuild DAG
    │   Steps 'completed' → skip (use saved outputs)
    │   Steps 'running' → treat as interrupted → retry from scratch
    │   Steps 'pending' → continue execution
    │
    └── Resume from correct wave
```

---

## Tiêu chí chấp nhận

- [ ] DAG build + cycle detection
- [ ] Topological sort → wave-based parallel execution
- [ ] Server resolution: project / direct / fleet:tag
- [ ] Provider resolution (delegate to BL-AIP-02)
- [ ] Step types: agent, shell, action, webhook, parallel, condition
- [ ] Variable interpolation: inputs + outputs + project/user context
- [ ] Real-time stream: step.output, step.completed, execution.completed events
- [ ] State persistence (orca_workflow_executions + orca_step_executions)
- [ ] Resumability sau Orca Server restart
- [ ] parallel type: allSettled + allowPartialFailure option
- [ ] On step fail: default stop workflow + persist state
