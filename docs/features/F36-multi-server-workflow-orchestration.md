# F36 — Multi-Server Workflow Orchestration

| Trường | Giá trị |
|--------|---------|
| **ID** | F36 |
| **Tên** | Multi-Server Workflow Orchestration |
| **Ưu tiên** | P1 |
| **Trạng thái** | 🚧 Phát triển |
| **Phiên bản** | v5.0+ |
| **ADR References** | ADR-009 |
| **HLD References** | C2.15, C3.11b |

---

## Mô tả

Workflows (automations) trong Orca có thể span **nhiều dev servers**, dùng **nhiều AI providers** khác nhau, và được tổ chức theo hệ thống **template kế thừa 3 tầng** (Company → Team → Personal). Workflows có thể được **chia sẻ** giữa cá nhân, publish lên team library, hoặc set làm company standard.

---

## Vấn đề cần giải quyết

- Workflow hiện tại chỉ chạy single-server, single-agent — không hỗ trợ parallel execution
- Không có cơ chế tái sử dụng: mỗi user phải tự tạo workflow tương tự từ đầu
- Team không có "standard workflow" — mỗi người làm theo cách riêng
- Không thể mix providers: step 1 dùng Claude, step 2 dùng GPT-4o, step 3 dùng Ollama (local)

---

## Kiến trúc Workflow

### Workflow Definition (YAML/JSON)

```yaml
# Ví dụ: Full-stack feature workflow
id: "feat-full-stack-v2"
name: "Full-Stack Feature Development"
description: "Tạo feature end-to-end: backend → frontend → tests"
version: "2.1"
template_id: "company:standard-feature-dev"  # kế thừa từ template
author: "nguyen.van.a@company.com"
visibility: "team"                            # private | team | company | public
tags: ["backend", "frontend", "testing"]

steps:
  - id: "backend-impl"
    name: "Implement Backend API"
    type: "agent"
    server: "project:vnp-blc-backend"         # route tới dev server của project
    provider:
      account: "server:anthropic-default"     # dùng Anthropic account của server đó
      model: "claude-opus-4-5"
    worktree: "current"
    prompt: |
      Implement the REST API endpoint for: {{feature_description}}
      Follow the existing patterns in src/routes/
    on_complete: "next"

  - id: "frontend-impl"
    name: "Implement Frontend Component"
    type: "agent"
    server: "project:vnp-blc-frontend"        # KHÁC server! → frontend server
    provider:
      account: "server:openai-default"        # OpenAI account trên frontend server
      model: "gpt-4o"
    depends_on: ["backend-impl"]              # chờ backend xong
    prompt: |
      Create React component for: {{feature_description}}
      Backend API: {{outputs.backend-impl.api_endpoint}}

  - id: "run-tests"
    name: "Run Test Suite"
    type: "shell"
    server: "project:vnp-blc-backend"
    command: "npm test -- --coverage"
    depends_on: ["backend-impl", "frontend-impl"]

  - id: "create-pr"
    name: "Create Pull Request"
    type: "action"
    action: "github.createPR"
    depends_on: ["run-tests"]
    params:
      title: "feat: {{feature_description}}"
      base: "main"
      draft: false
```

### Step Types

| Type | Mô tả | Runs on |
|------|-------|---------|
| `agent` | Spawn AI agent (Claude, GPT, Gemini...) | Dev Server (PTY) |
| `shell` | Chạy shell command | Dev Server |
| `action` | Gọi Orca built-in action (git, github, jira...) | Orca Server / relay |
| `webhook` | Gọi external HTTP endpoint | Orca Server |
| `parallel` | Chạy nhiều steps đồng thời | Distributed |
| `condition` | Branch logic dựa trên output của step trước | Orca Server |

---

## Template Inheritance System

### 3-tầng Template Hierarchy

```
Company Templates (company-wide standards)
    └── Team Templates (kế thừa + override company)
           └── Personal Workflows (kế thừa + override team)
```

**Inheritance cơ chế:**

```yaml
# Company template: "standard-feature-dev"
steps:
  - id: "lint-check"
    type: "shell"
    command: "npm run lint"
  - id: "implement"
    type: "agent"
    provider: { model: "claude-opus-4-5" }   # company default
  - id: "test"
    type: "shell"
    command: "npm test"

# Team template: kế thừa từ company, override model
template_id: "company:standard-feature-dev"
overrides:
  steps.implement.provider.model: "gpt-4o"   # team dùng GPT thay Claude
  steps.implement.prompt: "{{base}} Use team coding style guide."
inject_steps:
  after: "implement"
  steps:
    - id: "team-review"
      type: "action"
      action: "slack.notify"

# Personal: kế thừa từ team, thêm bước cá nhân
template_id: "team:backend-team:standard-feature-dev"
inject_steps:
  before: "lint-check"
  steps:
    - id: "my-precheck"
      type: "shell"
      command: "make check"
```

### Template Registry

```typescript
interface WorkflowTemplate {
  id: string
  scope: 'company' | 'team' | 'personal'
  teamId?: string
  ownerId?: string
  name: string
  description?: string
  templateYaml: string             // YAML definition
  parentTemplateId?: string        // inheritance
  visibility: 'private' | 'team' | 'company' | 'public'
  version: string
  tags: string[]
  usageCount: number
  rating?: number
  createdAt: number
  updatedAt: number
}
```

---

## Workflow Sharing

### Sharing Modes

| Mode | Mô tả |
|------|-------|
| `private` | Chỉ owner thấy |
| `team` | Tất cả member cùng team thấy |
| `company` | Tất cả user trong org thấy |
| `public` | Share link — ai có link đều dùng được |

### Workflow Library UI

```
Workflows → Library
┌──────────────────────────────────────────────────────┐
│ 🔍 Search templates...     [Filter: All / Company /  │
│                             Team / Trending]          │
│                                                      │
│ ⭐ Company Standards (3)                              │
│  ├── Standard Feature Dev  [★4.8] [used: 142]        │
│  │   Backend → Frontend → Tests → PR                 │
│  │   [Use] [Preview] [Clone]                         │
│  ├── Hotfix Workflow       [★4.5] [used: 67]         │
│  └── Code Review Workflow  [★4.2] [used: 38]         │
│                                                      │
│ 👥 Team Templates (Backend Team) (5)                 │
│  ├── API Development       [★4.7] [used: 89]         │
│  └── Database Migration    [★4.0] [used: 22]         │
│                                                      │
│ 👤 My Workflows (8)                                  │
│  ├── Quick Debug           [private] [Edit]          │
│  └── ... [+ Create New]                              │
└──────────────────────────────────────────────────────┘
```

---

## Multi-Server Execution Engine

### Execution Model

```
WorkflowOrchestrator (runs on Orca Server)
    │
    ├── Build DAG (Directed Acyclic Graph) từ steps + depends_on
    │
    ├── Topological sort → execution order
    │
    ├── For each step (parallel where possible):
    │   ├── Resolve server: "project:vnp-blc" → devServerId → relay
    │   ├── Resolve provider: "server:anthropic-default" → account → credentials on server
    │   ├── Execute step trên đúng server
    │   └── Collect outputs → pass to dependent steps via {{outputs.<stepId>.*}}
    │
    ├── WorkflowStateStore: persist state (resumable on Orca restart)
    │
    └── Stream real-time events → WebSocket → UI
```

### Parallel Execution

```yaml
steps:
  - id: "parallel-block"
    type: "parallel"
    steps:
      - id: "backend"
        server: "project:api-server"
        provider: { account: "server:anthropic-default" }
        type: "agent"
        prompt: "Implement API..."
      - id: "docs"
        server: "project:docs-server"
        provider: { account: "server:openai-default" }
        type: "agent"
        prompt: "Write docs..."
    # backend và docs chạy ĐỒNG THỜI trên 2 server khác nhau
```

### State & Resumability

```typescript
interface WorkflowExecution {
  id: string
  workflowId: string
  userId: string
  status: 'queued' | 'running' | 'paused' | 'completed' | 'failed'
  stepStates: Record<string, StepState>
  inputs: Record<string, string>    // {{feature_description}} = '...'
  outputs: Record<string, any>      // outputs per step
  startedAt: number
  completedAt?: number
  error?: string
}

interface StepState {
  stepId: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
  serverId?: string
  providerId?: string
  startedAt?: number
  completedAt?: number
  output?: any
  logStreamId?: string             // stream ID cho PTY output
}
```

---

## Provider Selection trong Workflow

```typescript
// Syntax trong workflow YAML:
server: "project:vnp-blc"           // lấy devServer từ project binding
server: "server:server-alpha"        // hardcode server ID
server: "fleet:tag:backend"          // chọn server có tag 'backend' (load balance)

provider: "server:anthropic-default" // default Anthropic account trên server đó
provider: "project:vnp-blc:openai"   // project-scope OpenAI account
provider: "user:personal-ollama"     // user's personal Ollama
provider: { model: "claude-opus-4-5" }  // auto-pick account có model này
```

---

## Workflow Execution UI

```
Workflow Running: "Full-Stack Feature Dev"
┌───────────────────────────────────────────────────┐
│ Status: ● RUNNING  Duration: 4m 32s               │
│                                                   │
│ ✅ lint-check    dev-alpha   0:12s                 │
│ ✅ backend-impl  dev-alpha   2:45s  [Claude]       │
│    └── output: api_endpoint=/api/v2/features      │
│ ● frontend-impl  dev-beta    1:30s  [GPT-4o]       │
│    └── [Live terminal output stream...]           │
│ ⏸ run-tests     waiting on: frontend-impl         │
│ ⏸ create-pr     waiting on: run-tests             │
│                                                   │
│ [Pause] [Cancel]  Progress: 2/4 steps             │
└───────────────────────────────────────────────────┘
```

---

## Tiêu chí chấp nhận

- [ ] Workflow YAML/JSON schema validation
- [ ] Step types: agent, shell, action, webhook, parallel, condition
- [ ] `server:` resolution: project binding, direct ID, fleet tag
- [ ] `provider:` resolution: server/project/user scope, model-based auto-select
- [ ] DAG builder + topological sort cho depends_on
- [ ] Parallel step execution (concurrent across multiple servers)
- [ ] Template inheritance: override steps, inject_steps
- [ ] Template registry: company / team / personal scopes
- [ ] Sharing: private / team / company / public (share link)
- [ ] WorkflowExecution state persistence (resumable)
- [ ] Real-time step status stream → UI (WebSocket events)
- [ ] Workflow Library UI với search + filter
- [ ] Workflow Run UI với live log per step
- [ ] `{{outputs.<stepId>.*}}` variable passing between steps
- [ ] Input variables: `{{feature_description}}` → prompt user on run

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Workflow types | `src/shared/workflow-types.ts` |
| Workflow orchestrator | `src/main/workflow/workflow-orchestrator.ts` |
| DAG builder | `src/main/workflow/dag-builder.ts` |
| Step executor (agent) | `src/main/workflow/executors/agent-step-executor.ts` |
| Step executor (shell) | `src/main/workflow/executors/shell-step-executor.ts` |
| Step executor (action) | `src/main/workflow/executors/action-step-executor.ts` |
| Step executor (parallel) | `src/main/workflow/executors/parallel-step-executor.ts` |
| Template registry | `src/main/workflow/template-registry.ts` |
| Template resolver | `src/main/workflow/template-resolver.ts` |
| Server resolver | `src/main/workflow/server-resolver.ts` |
| Provider resolver | `src/main/workflow/workflow-provider-resolver.ts` |
| Execution store | `src/main/workflow/execution-store.ts` |
| DB migration | `src/main/db/migrations/0009_workflows.ts` |
| RPC methods | `src/main/runtime/rpc/methods/workflows.ts` |
| Workflow Library UI | `src/renderer/src/components/workflow/WorkflowLibrary.tsx` |
| Workflow Run UI | `src/renderer/src/components/workflow/WorkflowRunPanel.tsx` |
| Workflow Editor | `src/renderer/src/components/workflow/WorkflowEditor.tsx` |

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Workflow start (init DAG + dispatch) | < 1s |
| Step-to-step handoff latency | < 200ms |
| Max parallel steps | 20 concurrent |
| Template resolve (with inheritance) | < 50ms |
| Workflow state persistence | < 100ms per checkpoint |
