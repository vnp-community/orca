# ADR-009 — Workflow Orchestration via DAG with Topological Sort

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-009 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C2.15 (Workflow Orchestrator), C3.11b |
| **Code Ref** | `src/main/runtime/orchestration/` (hiện có), cần mở rộng |
| **Feature Ref** | F36 |

---

## Bối cảnh

Orca cần chạy automation workflows phức tạp:
- Steps phụ thuộc vào output của steps trước
- Steps có thể chạy song song nếu không có dependency
- Steps chạy trên **nhiều dev servers khác nhau**
- Workflow phải resumable sau khi Orca restart

Codebase hiện tại có `src/main/runtime/orchestration/` nhưng chủ yếu cho local orchestration. Cần mở rộng thành multi-server DAG engine.

---

## Quyết định

### Workflow Definition (YAML/JSON)

```typescript
interface WorkflowDefinition {
  id: string
  name: string
  version: number
  templateId?: string            // parent template for inheritance
  steps: WorkflowStep[]
  inputs?: Record<string, InputSchema>
}

interface WorkflowStep {
  id: string
  type: 'agent' | 'shell' | 'action' | 'webhook' | 'parallel' | 'condition'
  name: string
  dependsOn?: string[]           // step IDs this step waits for
  serverSpec: string             // 'project:<id>' | 'server:<id>' | 'fleet:tag:<tag>'
  providerSpec?: string          // AI provider spec
  config: Record<string, unknown>
  timeout?: number               // ms, default 30min
}
```

### DAG Builder

```typescript
class DAGBuilder {
  build(steps: WorkflowStep[]): DAGNode[] {
    // 1. Create adjacency list from dependsOn
    // 2. Kahn's algorithm: topological sort
    // 3. Detect cycles → throw WorkflowCycleError
    // 4. Assign wave numbers (parallel groups)
    // Returns: nodes sorted by wave [wave0: [A,B], wave1: [C], wave2: [D,E]]
  }
}
```

### Wave-based Parallel Execution

```typescript
class WorkflowOrchestrator {
  async execute(execution: WorkflowExecution): Promise<void> {
    const waves = DAGBuilder.build(execution.definition.steps)

    for (const wave of waves) {
      // Run all steps in wave concurrently
      const results = await Promise.allSettled(
        wave.map(step => this.executeStep(step, execution))
      )
      // If any step fails → check continueOnError, else abort
    }
  }

  private async executeStep(step: WorkflowStep, execution: WorkflowExecution): Promise<StepOutput> {
    // 1. Resolve server: serverSpec → devServerId
    // 2. Interpolate variables: {{inputs.*}}, {{outputs.<stepId>.*}}, {{project.*}}
    // 3. Dispatch to step executor based on type
    // 4. Stream output via WebSocket to clients
    // 5. Persist result to orca_step_executions
  }
}
```

### State Persistence (Resumability)

```typescript
// orca_workflow_executions table
interface WorkflowExecution {
  id: string
  definitionSnapshot: WorkflowDefinition  // snapshot at start time
  status: 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
  inputs: Record<string, unknown>
  currentWave: number
  startedAt: Date
  completedAt?: Date
}

// orca_step_executions table
interface StepExecution {
  id: string
  executionId: string
  stepId: string
  status: StepStatus
  output?: StepOutput
  startedAt: Date
  completedAt?: Date
  devServerId: string         // which server ran this
  accountId?: string          // which AI provider was used
}
```

**Resume logic:** On Orca restart, load `status=running` executions → resume từ `currentWave` (không re-run completed steps).

### Server Resolution

```typescript
function resolveServer(serverSpec: string, context: ExecutionContext): string {
  if (serverSpec.startsWith('project:')) {
    return ProjectService.get(serverSpec.slice(8)).devServerId
  }
  if (serverSpec.startsWith('server:')) {
    return serverSpec.slice(7)
  }
  if (serverSpec.startsWith('fleet:tag:')) {
    return FleetHealthMonitor.getHealthyServerByTag(serverSpec.slice(10))
  }
  throw new Error(`Unknown server spec: ${serverSpec}`)
}
```

### Template Inheritance

```typescript
class TemplateResolver {
  resolve(templateId: string, overrides: Partial<WorkflowDefinition>): WorkflowDefinition {
    const parent = TemplateRegistry.get(templateId)
    // Apply overrides → inject_steps → remove_steps
    // Cascade: parent.templateId → grandparent → ...
    // Max depth: 5 (prevent infinite chains)
  }
}
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **DAG + topological sort + wave execution** ✅ | Classic approach; proven in CI/CD (GitHub Actions, Argo); easy to visualize |
| State machine (XState) | Tốt cho linear flows; khó model parallel DAG |
| Queue-based (BullMQ) | External Redis dependency; overkill cho embedded use |
| Airflow | Python; external service; too heavy |
| Linear execution (sequential only) | Không parallel; slow |

---

## Hậu quả

**Tích cực:**
- Wave execution: steps không dependency chạy song song → fast
- Resumable: không rerun completed steps sau restart
- Server-agnostic: mỗi step có thể chạy trên server khác nhau
- Template inheritance: company templates → team override → personal

**Tiêu cực:**
- `definitionSnapshot` trong DB → large JSONB field (cần index carefully)
- DAG validation phải chặt (cycles, missing steps, invalid serverSpec)
- Multi-server: cần relay connections đến nhiều servers cùng lúc
- Output interpolation (`{{outputs.step1.result}}`) cần parser cẩn thận

---

## Liên quan codebase hiện tại

`src/main/runtime/orchestration/` có `orchestration.ts` (local only). Cần:
- Mở rộng thành multi-server
- Thêm DAGBuilder
- Thêm TemplateRegistry + TemplateResolver
- Migration 0009

---

## Trạng thái Implementation

⚠️ Partial (local orchestration có)  
❌ Multi-server DAG chưa implement  
🎯 DAGBuilder với topological sort  
🎯 WorkflowOrchestrator (wave execution)  
🎯 TemplateRegistry + Resolver  
🎯 Migration 0009
