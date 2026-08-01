# Luồng Dữ liệu — Workflow Orchestration

**Domain:** Workflow Orchestration  
**Nghiệp vụ:** BL-WF-01 → BL-WF-03  
**Kiến trúc tham chiếu:** HLD v1 — Workflow Orchestrator (C3.11b), C4.9, ADR-009, F36

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Developer/Lead Browser | UI | Workflow builder, execution monitor |
| Orca Web Server | Backend | /api/workflows REST + SSE events |
| TemplateRegistry | Business Logic | CRUD workflow templates |
| TemplateResolver | Business Logic | Inheritance chain merge |
| WorkflowOrchestrator | Business Logic | DAG build + topological sort + execute |
| StepExecutors | Business Logic | agent, shell, action, webhook, parallel, condition |
| WorkflowServerResolver | Business Logic | project:/server:/fleet:tag: → devServerId |
| Server Database | Persistence | orca_workflow_templates, _executions, _step_executions |
| Dev Server (relay) | Remote | Thực thi agent/shell steps |

---

## BL-WF-01 — Workflow Template Management (Create/Inherit/Share)

```
Admin/Lead/User
    │
    ▼
[Browser] Workflow Library → "New Template"
    Input YAML/JSON:
    {
      name: "Feature Implementation",
      scope: "team",
      steps: [
        { id: "plan", type: "agent", provider: "anthropic",
          target: "project:vnp-blc", prompt: "Analyze and plan: {{task}}" },
        { id: "implement", type: "agent", dependsOn: ["plan"],
          target: "project:vnp-blc", prompt: "Implement based on plan" },
        { id: "test", type: "shell", dependsOn: ["implement"],
          target: "project:vnp-blc", command: "npm test" },
        { id: "review", type: "agent", dependsOn: ["test"],
          prompt: "Review implementation and suggest improvements" }
      ]
    }
    │ POST /api/workflows/templates
    ▼
[TemplateRegistry.create()]
    ├─ requireAuth() guard
    ├─ Validate: step IDs unique, dependsOn valid, no cycles
    │   (DAG cycle detection: Kahn's algorithm)
    ├─ INSERT orca_workflow_templates { id, name, scope, definition_json, owner_id }  ← DB
    └─ Return { templateId }

INHERITANCE:
    {
      name: "My Custom Feature Flow",
      templateId: "<parent-id>",     // inherits from parent
      overrides: { "steps.implement.prompt": "Also add OpenAPI docs" },
      inject_steps: [{ position: "after", after_step: "implement",
                       step: { id: "format", type: "shell", command: "make fmt" } }],
      remove_steps: ["review"]
    }
    ├─ TemplateResolver.resolve(childId):
    │   parent = loadTemplate(parentId)
    │   resolved = deepMerge(parent, child.overrides)
    │   resolved.steps = applyInjectionsAndRemovals(resolved.steps, child)
    └─ INSERT orca_workflow_templates (resolved) ← DB

Luồng:
User → POST /api/workflows/templates → TemplateRegistry
      → cycle detection → Server DB (INSERT template)

Inheritance:
User creates child → TemplateResolver.resolve() → DB (load parent)
                  → deepMerge + inject/remove steps → DB (INSERT child)
```

---

## BL-WF-02 — Multi-Server Workflow Execution

```
Developer/Lead
    │
    ▼
[Browser] Workflow Library → chọn template → "Execute"
    Input: { templateId, params: { task: "Add OAuth login" } }
    │ POST /api/workflows/execute
    ▼
[WorkflowOrchestrator.execute()]
    │
    ├─ Load template: TemplateResolver.resolve(templateId)
    ├─ Resolve server targets:
    │   WorkflowServerResolver:
    │   "project:vnp-blc"     → lookup projectId → devServerId: "dev-01"
    │   "server:dev-02"       → direct mapping
    │   "fleet:tag:backend"   → all servers với tag='backend'
    │
    ├─ Build DAG + topological sort:
    │   [plan] → [implement] → [test] → [review]
    │                          ↑ [format] (injected)
    │
    ├─ INSERT orca_workflow_executions { id, templateId, status:'running', params }  ← DB
    │
    ├─ Execute waves (parallel within wave, sequential across waves):
    │
    │   WAVE 1: [plan] (no deps)
    │   ├─ StepExecutor.agent:
    │   │   relay.call('agent.spawn', { cmd: 'claude', env, cwd, prompt: resolved_prompt })
    │   │   → Dev Server → PTY → agent runs
    │   │   → stream output events → SSE to browser
    │   │   INSERT orca_step_executions { stepId, status:'running' }  ← DB
    │   │   Wait for agent:complete → stepOutput
    │   │   UPDATE step_executions SET status='done', output  ← DB
    │   │
    │   WAVE 2: [implement] (depends on plan)
    │   ├─ Inject plan output as context
    │   ├─ relay.call('agent.spawn', ...)
    │   │
    │   WAVE 3: [test] || [format] (parallel - both depend on implement)
    │   ├─ StepExecutor.shell: relay.call('shell.exec', { command: 'npm test' })
    │   ├─ StepExecutor.shell: relay.call('shell.exec', { command: 'make fmt' })
    │   │
    │   WAVE 4: [review] (depends on test + format)
    │
    ├─ UPDATE orca_workflow_executions SET status='completed'  ← DB
    └─ emit: workflow:completed { executionId, summary }

Luồng:
User → POST /execute → WorkflowOrchestrator
     → TemplateResolver (load + resolve)
     → WorkflowServerResolver (target → devServerId)
     → Server DB (INSERT execution)
     → WAVE-by-WAVE: relay.call(step.type) → Dev Server
     → Server DB (INSERT/UPDATE step records per wave)
     → SSE events → Browser (real-time progress)
```

---

## BL-WF-03 — Workflow Sharing & Library Discovery

```
Owner (User/Lead)
    │
    ▼
[Browser] My Workflows → template → "Share" → change visibility
    │ PATCH /api/workflows/templates/:id { visibility: 'team' | 'company' | 'public' }
    ▼
[TemplateRegistry.updateVisibility()]
    ├─ Check ownership (must be owner or admin)
    ├─ IF company scope: require admin approval (optional)
    ├─ UPDATE orca_workflow_templates SET visibility=?  ← DB
    └─ IF public: generate share link token
        INSERT workflow_share_links { token, templateId, expiresAt }

DISCOVERY:
    GET /api/workflows/library?scope=company&tag=review
    ├─ SELECT templates WHERE visibility IN ('team', 'company', 'public')
    │   AND scope matches user's dept/company
    └─ Return paginated template list

IMPORT from share link:
    GET /api/workflows/shared/:token
    ├─ SELECT template WHERE share_token=? AND NOT expired
    ├─ User can "Fork" (copy to personal)
    └─ INSERT orca_workflow_templates (copy, scope='user')  ← DB

Luồng:
Owner → PATCH /api/templates/:id → TemplateRegistry → DB (UPDATE visibility)
User → GET /api/workflows/library → DB (SELECT by scope) → list
User → GET /api/workflows/shared/:token → DB → import/fork
```

---

## Sơ đồ tổng quan — Workflow Orchestration

```
┌──────────────────┐  HTTP/SSE  ┌────────────────────────────────────────┐
│  Browser         │◄──────────►│  Orca Web Server                       │
│  Workflow builder│            │  TemplateRegistry (CRUD)               │
│  Execution monitor│           │  TemplateResolver (inheritance)        │
│  Library         │            │  WorkflowOrchestrator (DAG engine)     │
└──────────────────┘            │  WorkflowServerResolver (routing)      │
                                └──────────┬─────────────────────────────┘
                                           │
                                ┌──────────▼─────────────────────────────┐
                                │  Server Database                        │
                                │  orca_workflow_templates               │
                                │  orca_workflow_executions              │
                                │  orca_step_executions                  │
                                └──────────┬─────────────────────────────┘
                                           │ relay.call (per step)
                           ┌──────────────┬┴───────────────────┐
                           │              │                     │
                  ┌────────▼──┐  ┌────────▼──┐       ┌────────▼──┐
                  │ Dev Server│  │ Dev Server│       │ Dev Server│
                  │  01       │  │  02       │       │  03       │
                  │  agent/   │  │  shell/   │       │  agent/   │
                  │  shell    │  │  webhook  │       │  shell    │
                  └───────────┘  └───────────┘       └───────────┘

Execution model:
- DAG topological sort → wave-based parallel
- Each wave: dispatch steps (parallel) → collect outputs
- State persisted: resumable after Orca restart
```
