# SOLUTION: Workflow Provider Selection & Pause/Resume — Fix BUG-BE-HLD-008 + BUG-BE-HLD-009

**Domain:** workflow-orchestration (per-step AI provider override + user-triggered pause/resume)
**TDD Reference:** TDD-17 (Workflow Orchestration) §2 (`WorkflowStep.providerSpec`), §4 (`resumeRunningExecutions`), §6 (RPC methods)
**Bugs fixed:** [BUG-BE-HLD-008](../BUG-BE-HLD-008-workflow-provider-selection-not-implemented.md), [BUG-BE-HLD-009](../BUG-BE-HLD-009-workflow-pause-resume-not-implemented.md)
**Files cần thay đổi:**
- `backend/src/main/workflow/WorkflowTypes.ts`
- `backend/src/main/workflow/StepExecutors.ts`
- `backend/src/main/workflow/WorkflowOrchestrator.ts`
- `backend/src/main/workflow/TemplateResolver.ts`
- `backend/src/main/workflow/workflow-rpc-handler.ts`
- `backend/src/main/db/migrations/0014_workflow_pause_state.ts` (NEW)
- `backend/src/main/db/migrations/index.ts`
- `backend/src/main/server-bootstrap.ts`

**Không đụng tới:** `backend/src/main/db/migrations/0009_workflows.ts`, `0013_workflow_trace_correlation.ts` (đã chạy production — mọi thay đổi schema đi qua migration mới `0014`). Solution này không lặp lại nội dung của `specs/backend/bugs/workflow-orchestration/solutions/SOLUTION-workflow-orchestration.md` (domain đó xử lý `server:<devServerId>` dispatch, condition-injection, resume-orphan-step — 3 vấn đề khác, đã fix). Ở đây tập trung thuần vào (1) chọn AI provider theo từng step và (2) pause/resume do user chủ động kích hoạt.

---

## Tổng quan phụ thuộc

```
BUG-BE-HLD-008 (provider selection)          BUG-BE-HLD-009 (pause/resume)
    │                                             │
    ├── WorkflowTypes.ts (provider field)         ├── WorkflowTypes.ts ('paused' status)
    ├── workflow-rpc-handler.ts (zod schema)       ├── migration 0014 (paused_at column)
    ├── StepExecutors.ts (resolve + relay.call)    ├── WorkflowOrchestrator.pause/resumeFromPause
    ├── TemplateResolver.ts (merge provider field) ├── workflow-rpc-handler.ts (2 RPC methods mới)
    └── server-bootstrap.ts (wiring)               └── (không đụng resumeRunningExecutions — giữ nguyên)

Phát hiện phụ trong lúc đọc code thật (chặn cả 2 bug nếu không sửa — xem §0 dưới đây):
    WorkflowOrchestrator.executeStep() gọi StepExecutors theo kiểu SAI
```

**Thứ tự áp dụng:** `§0 (wiring fix bắt buộc) → BUG-BE-HLD-008 → BUG-BE-HLD-009`. §0 không phải là bug riêng trong 2 ticket, nhưng nếu không sửa thì StepExecutors.execute() (nơi BUG-BE-HLD-008 gắn logic provider) **không bao giờ được gọi** — xem giải thích bên dưới.

---

## §0 — Phát hiện bổ sung: `WorkflowOrchestrator.executeStep()` gọi `stepExecutors` sai kiểu (chặn cả BUG-008)

### Root cause

Đọc verbatim `backend/src/main/workflow/WorkflowOrchestrator.ts`:

```typescript
// dòng 32-41 — type alias NỘI BỘ của WorkflowOrchestrator.ts, KHÔNG import từ StepExecutors.ts
export type StepExecutorFn = (
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string
) => Promise<StepOutput>

export type StepExecutors = Record<string, StepExecutorFn>   // ⚠️ map theo step type, không phải class

// dòng 84-89 — constructor nhận tham số đúng theo alias Record<string, StepExecutorFn> ở trên
constructor(
  private readonly pool: IConnectionPool,
  private readonly dagBuilder: DAGBuilder,
  private readonly stepExecutors: StepExecutors,   // = Record<string, StepExecutorFn>
  private readonly router: ProjectServerRouter
) {}

// dòng 344-355 — executeStep() coi stepExecutors như 1 map index theo step type
const executor = this.stepExecutors[interpolatedStep.config.type as string]
if (!executor) {
  throw new Error(`UNSUPPORTED_STEP_TYPE: ${interpolatedStep.config.type}`)
}
```

`WorkflowOrchestrator.ts` **không hề `import` class `StepExecutors` từ `./StepExecutors.ts`** (đã xác nhận qua import block đầu file). Nó tự định nghĩa alias cục bộ trùng tên `StepExecutors = Record<string, StepExecutorFn>` — một map `{ agent: fn, shell: fn, ... }`.

Nhưng `backend/src/main/workflow/StepExecutors.ts` thực tế là **1 class** với đúng 1 entry point public:

```typescript
export class StepExecutors {
  constructor(private readonly router: ProjectServerRouter) {}
  async execute(step, inputs, signal, traceId?): Promise<StepOutput> { ... }  // dispatch nội bộ qua executeByType()
  private async executeByType(...) { switch(type) { case 'agent': return this.executeAgent(...); ... } }
}
```

Và `server-bootstrap.ts` (dòng 456) khởi tạo đúng class này rồi truyền thẳng vào `WorkflowOrchestrator`:

```typescript
const stepExecutors = new StepExecutors(_projectRouter)                              // instance của CLASS
const workflowOrchestrator = new WorkflowOrchestrator(pool, dagBuilder, stepExecutors, _projectRouter)
```

Một instance class (không có index signature `[key: string]: fn`) được gán vào tham số kiểu `Record<string, StepExecutorFn>`. Ở runtime, `this.stepExecutors['agent']` trên 1 class instance → `undefined` → mọi step (agent/shell/webhook/notification/condition) đều throw `UNSUPPORTED_STEP_TYPE` ngay tại `executeStep()`, **trước khi** `StepExecutors.execute()`/`executeAgent()` từng được gọi. Đây là lý do sâu xa khiến "workflow step execution" không hoạt động bất kể có sửa `StepExecutors.executeAgent()` hay không — nếu chỉ làm theo đúng 3 bước đề xuất của BUG-BE-HLD-008 mà bỏ qua chỗ này, code provider-resolution mới viết sẽ là dead code không bao giờ chạy tới.

### Fix — executeStep() gọi thẳng `StepExecutors.execute()`, xoá alias `Record` sai

```typescript
// backend/src/main/workflow/WorkflowOrchestrator.ts

// TRƯỚC (dòng 18-41)
import { randomUUID } from 'node:crypto'
import { Tracers } from '../../shared/trace/tracers'
import type { TraceSpan } from '../../shared/trace'
import type { IConnectionPool } from '../db/pool'
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { DAGBuilder } from './DAGBuilder'
import type {
  WorkflowDefinition,
  WorkflowExecution,
  WorkflowStep,
  StepOutput,
  ListExecutionsFilter,
} from './WorkflowTypes'

// ── Step executors type ───────────────────────────────────────────────────────

export type StepExecutorFn = (
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string
) => Promise<StepOutput>

export type StepExecutors = Record<string, StepExecutorFn>

// SAU
import { randomUUID } from 'node:crypto'
import { Tracers } from '../../shared/trace/tracers'
import type { TraceSpan } from '../../shared/trace'
import type { IConnectionPool } from '../db/pool'
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { DAGBuilder } from './DAGBuilder'
// FIX §0: import CLASS thật thay vì tự định nghĩa alias Record<string, fn> trùng tên —
// WorkflowOrchestrator trước đây không bao giờ gọi trúng StepExecutors.execute() thật sự
// (xem SOLUTION-workflow-exact.md §0), khiến mọi step luôn throw UNSUPPORTED_STEP_TYPE.
import type { StepExecutors } from './StepExecutors'
import type {
  WorkflowDefinition,
  WorkflowExecution,
  WorkflowStep,
  StepOutput,
  ListExecutionsFilter,
} from './WorkflowTypes'

// (xoá hẳn export type StepExecutorFn / export type StepExecutors nội bộ — không còn dùng)
```

```typescript
// executeStep() — TRƯỚC (dòng 344-379)
private async executeStep(
  step: WorkflowStep,
  execution: WorkflowExecution,
  signal: AbortSignal,
  rootTraceId?: string
): Promise<StepOutput> {
  const interpolatedStep = this.interpolateStep(step, execution.inputs)
  const executor = this.stepExecutors[interpolatedStep.config.type as string]

  if (!executor) {
    throw new Error(`UNSUPPORTED_STEP_TYPE: ${interpolatedStep.config.type}`)
  }

  const stepSpan = Tracers.workflowStepFlow.start({
    parentTraceId: rootTraceId,
    executionId: execution.id,
    stepId: step.id,
    stepType: interpolatedStep.config.type as string,
  })

  try {
    await this.persistStepStart(execution.id, step.id)
    const output = await executor(interpolatedStep, execution.inputs, signal, stepSpan.id)
    await this.persistStepComplete(execution.id, step.id, output)
    stepSpan.ok({ exitCode: output.exitCode })
    return output
  } catch (err) {
    stepSpan.fail(err, { stepId: step.id })
    throw err
  }
}

// SAU
private async executeStep(
  step: WorkflowStep,
  execution: WorkflowExecution,
  signal: AbortSignal,
  rootTraceId?: string
): Promise<StepOutput> {
  const interpolatedStep = this.interpolateStep(step, execution.inputs)

  // FIX §0: gọi thẳng entry point duy nhất của class StepExecutors — nó tự dispatch theo
  // step.config.type nội bộ (executeByType) và tự throw UNSUPPORTED_STEP_TYPE nếu cần,
  // nên không còn cần bước tra map + kiểm tra !executor ở đây.
  const stepSpan = Tracers.workflowStepFlow.start({
    parentTraceId: rootTraceId,
    executionId: execution.id,
    stepId: step.id,
    stepType: interpolatedStep.config.type as string,
  })

  try {
    await this.persistStepStart(execution.id, step.id)
    const output = await this.stepExecutors.execute(
      interpolatedStep,
      execution.inputs,
      signal,
      stepSpan.id,
      execution.triggeredBy // [NEW BUG-BE-HLD-008] forward — dùng để ProviderResolver áp user-scope priority
    )
    await this.persistStepComplete(execution.id, step.id, output)
    stepSpan.ok({ exitCode: output.exitCode })
    return output
  } catch (err) {
    stepSpan.fail(err, { stepId: step.id })
    throw err
  }
}
```

Constructor giữ nguyên chữ ký (`stepExecutors: StepExecutors`) — chỉ khác là `StepExecutors` giờ trỏ đúng vào class thật, nên không cần sửa `server-bootstrap.ts` ở điểm này (đã truyền đúng instance class từ đầu, chỉ là type-checking trước đây "tình cờ" không bắt được lỗi vì alias trùng tên che khuất — xem ghi chú rủi ro ở cuối §0).

### Rủi ro / Blast radius (từ CodeGraph)

- `executeStep` (backend) — 1 caller nội bộ (`runExecution`), không có test bao phủ (`⚠️ no covering tests found` theo CodeGraph). Sửa an toàn cho phía `backend/`.
- **Cảnh báo:** `desktop/src/main/workflow/WorkflowOrchestrator.ts` và `frontend/src/main/workflow/WorkflowOrchestrator.ts` có **y hệt** discrepancy này (cùng pattern `Record<string, StepExecutorFn>` cục bộ + cùng `this.stepExecutors[type]`). Solution này chỉ sửa `backend/` theo đúng phạm vi 2 ticket — nếu `desktop/`/`frontend/` cũng cần chạy workflow step thật, áp cùng fix ở đó là một task riêng, không nằm trong BUG-BE-HLD-008/009.
- Do thiếu test coverage, thêm ít nhất 1 test integration (`WorkflowOrchestrator.test.ts`) khẳng định `stepExecutors.execute()` được gọi với đúng 5 tham số (bao gồm `triggeredBy`) là bắt buộc trước khi merge — xem §Verification Plan.

---

## BUG-BE-HLD-008 — Chọn AI provider theo từng workflow step

**Mức độ:** 🟠 HIGH
**Root cause:** `WorkflowStepConfig` không có field `provider`; `StepExecutors.executeAgent()` không nhận `ProviderResolver`/`AIProviderService`; `server-bootstrap.ts` khởi tạo 2 domain (`AIProviderService`/`ProviderResolver` ở bước 11, `WorkflowOrchestrator`/`StepExecutors` ở bước 12) hoàn toàn độc lập, không cross-reference.

### Bước 1 — `WorkflowTypes.ts`: thêm field `provider` vào `WorkflowStepConfig`

```typescript
// backend/src/main/workflow/WorkflowTypes.ts

// TRƯỚC (dòng 18-27)
/** Per-step configuration — type-specific fields are opaque to the DAG engine */
export interface WorkflowStepConfig {
  type: WorkflowStepType
  // agent:        { prompt: string; worktreePath: string; trustPreset?: string }
  // shell:        { script: string; env?: Record<string, string> }
  // webhook:      { url: string; method?: string; body?: unknown }
  // notification: { channel: string; message: string }
  // condition:    { expression: string }
  [key: string]: unknown
}

// SAU
/**
 * Explicit AI provider override for a single 'agent' step (BUG-BE-HLD-008).
 * `accountId` must reference a row in orca_ai_provider_accounts belonging to the
 * dev server the step's serverSpec resolves to — StepExecutors validates this at
 * dispatch time (WORKFLOW_STEP_PROVIDER_NOT_FOUND if the account doesn't exist).
 */
export interface WorkflowStepProviderConfig {
  accountId: string
  /** Overrides the account's configured model for this step only. */
  model?: string
}

/** Per-step configuration — type-specific fields are opaque to the DAG engine */
export interface WorkflowStepConfig {
  type: WorkflowStepType
  // agent:        { prompt: string; worktreePath: string; trustPreset?: string; provider?: WorkflowStepProviderConfig }
  // shell:        { script: string; env?: Record<string, string> }
  // webhook:      { url: string; method?: string; body?: unknown }
  // notification: { channel: string; message: string }
  // condition:    { expression: string }
  /** 'agent' steps only — see WorkflowStepProviderConfig. Absent = use project default (ProviderResolver priority chain). */
  provider?: WorkflowStepProviderConfig
  [key: string]: unknown
}
```

> **Ghi chú alignment với TDD-17:** TDD-17 §2 định nghĩa `WorkflowStep.providerSpec?: string` (top-level, giá trị là accountId hoặc chuỗi `'resolve:server'`) — thiết kế này **chưa từng được implement** ở bất kỳ file thật nào (`grep providerSpec` trong `backend/src/main/workflow/*.ts` = 0 kết quả). Ticket BUG-BE-HLD-008 và `docs/features/F36-multi-server-workflow-orchestration.md` (nguồn thực tế người dùng thấy) đều mô tả cú pháp `provider: { account, model }` **lồng trong step config**, không phải field rời ở top-level `WorkflowStep`. Solution này theo đúng cú pháp ticket yêu cầu (khớp F36 doc + đã có tiền lệ `WorkflowStepConfig` opaque bag chứa mọi field type-specific) thay vì TDD §2 — vì đó là gap cụ thể được báo cáo. Nếu muốn đồng bộ tuyệt đối với TDD sau này, đó là 1 rename riêng (`gitnexus rename`), không phải bug-fix.

### Bước 2 — `workflow-rpc-handler.ts`: mở rộng zod schema cho `provider`

```typescript
// backend/src/main/workflow/workflow-rpc-handler.ts

// TRƯỚC (dòng 27-29)
const WorkflowStepConfigSchema = z.object({
  type: z.enum(['agent', 'shell', 'webhook', 'notification', 'condition']),
}).catchall(z.unknown())

// SAU
const WorkflowStepProviderSchema = z.object({
  accountId: z.string().min(1),
  model: z.string().optional(),
})

const WorkflowStepConfigSchema = z.object({
  type: z.enum(['agent', 'shell', 'webhook', 'notification', 'condition']),
  provider: WorkflowStepProviderSchema.optional(), // [NEW BUG-BE-HLD-008]
}).catchall(z.unknown())
```

Không cần thay đổi `WorkflowStepSchema`/`WorkflowDefinitionSchema`/`ExecuteParam` — chúng đã lồng `config: WorkflowStepConfigSchema` nên field mới tự động được validate xuyên suốt `workflow.execute` và `workflow.template.create`.

### Bước 3 — `StepExecutors.ts`: nhận `ProviderResolver` + `AIProviderService`, resolve trước khi `relay.call('agent.exec', …)`

```typescript
// backend/src/main/workflow/StepExecutors.ts

// TRƯỚC (dòng 17-23)
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { WorkflowStep, StepOutput } from './WorkflowTypes'

const DEFAULT_TIMEOUT_MS = 30 * 60_000 // 30 minutes

export class StepExecutors {
  constructor(private readonly router: ProjectServerRouter) {}

// SAU
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { ProviderResolver } from '../ai-providers/ProviderResolver'      // [NEW BUG-BE-HLD-008]
import type { AIProviderService } from '../ai-providers/AIProviderService'    // [NEW BUG-BE-HLD-008]
import type { WorkflowStep, StepOutput, WorkflowStepProviderConfig } from './WorkflowTypes'

const DEFAULT_TIMEOUT_MS = 30 * 60_000 // 30 minutes

export class StepExecutors {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly providerResolver: ProviderResolver,     // [NEW BUG-BE-HLD-008]
    private readonly aiProviderService: AIProviderService     // [NEW BUG-BE-HLD-008]
  ) {}
```

`execute()` / `executeByType()` cần forward `triggeredBy` (userId của người kích hoạt workflow, đã forward từ `WorkflowOrchestrator.executeStep()` ở §0) xuống `executeAgent()` — chỉ agent step cần nó, các step type khác giữ nguyên chữ ký:

```typescript
// TRƯỚC (dòng 33-82)
async execute(
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string
): Promise<StepOutput> {
  if (signal.aborted) {
    throw new Error('EXECUTION_CANCELLED')
  }
  const timeoutMs = step.timeout ?? DEFAULT_TIMEOUT_MS
  return Promise.race([
    this.executeByType(step, inputs, signal, traceId),
    new Promise<never>((_, reject) => {
      const timer = setTimeout(
        () => reject(new Error(`STEP_TIMEOUT: step "${step.id}" exceeded ${timeoutMs}ms`)),
        timeoutMs
      )
      signal.addEventListener('abort', () => clearTimeout(timer), { once: true })
    }),
  ])
}

private async executeByType(
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string
): Promise<StepOutput> {
  const { type } = step.config
  switch (type) {
    case 'agent':
      return this.executeAgent(step, signal, traceId)
    case 'shell':
      return this.executeShell(step, signal, traceId)
    case 'webhook':
      return this.executeWebhook(step, signal)
    case 'notification':
      return this.executeNotification(step, signal, traceId)
    case 'condition':
      return this.executeCondition(step, inputs)
    default:
      throw new Error(`UNSUPPORTED_STEP_TYPE: "${String(type)}"`)
  }
}

// SAU
async execute(
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string,
  triggeredBy?: string // [NEW BUG-BE-HLD-008] execution.triggeredBy — chỉ agent step dùng, để ProviderResolver áp user-scope priority
): Promise<StepOutput> {
  if (signal.aborted) {
    throw new Error('EXECUTION_CANCELLED')
  }
  const timeoutMs = step.timeout ?? DEFAULT_TIMEOUT_MS
  return Promise.race([
    this.executeByType(step, inputs, signal, traceId, triggeredBy),
    new Promise<never>((_, reject) => {
      const timer = setTimeout(
        () => reject(new Error(`STEP_TIMEOUT: step "${step.id}" exceeded ${timeoutMs}ms`)),
        timeoutMs
      )
      signal.addEventListener('abort', () => clearTimeout(timer), { once: true })
    }),
  ])
}

private async executeByType(
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string,
  triggeredBy?: string
): Promise<StepOutput> {
  const { type } = step.config
  switch (type) {
    case 'agent':
      return this.executeAgent(step, signal, traceId, triggeredBy)
    case 'shell':
      return this.executeShell(step, signal, traceId)
    case 'webhook':
      return this.executeWebhook(step, signal)
    case 'notification':
      return this.executeNotification(step, signal, traceId)
    case 'condition':
      return this.executeCondition(step, inputs)
    default:
      throw new Error(`UNSUPPORTED_STEP_TYPE: "${String(type)}"`)
  }
}
```

`executeAgent()` — logic chính của bug fix:

```typescript
// TRƯỚC (dòng 86-103)
private async executeAgent(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')

  const result = (await relay.call('agent.exec', {
    stepId: step.id,
    prompt: step.config['prompt'],
    worktreePath: step.config['worktreePath'],
    trustPreset: step.config['trustPreset'] ?? 'default',
    traceId,
  })) as { exitCode?: number; stdout?: string; stderr?: string }

  return {
    exitCode: result.exitCode ?? 0,
    stdout: result.stdout,
    stderr: result.stderr,
  }
}

// SAU
private async executeAgent(
  step: WorkflowStep,
  signal: AbortSignal,
  traceId?: string,
  triggeredBy?: string
): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')

  // [NEW BUG-BE-HLD-008] Resolve per-step provider BEFORE dispatch — F36 doc's core
  // use-case (Claude ở bước 1, GPT-4o ở bước 2 trong CÙNG 1 workflow).
  const resolved = await this.resolveAgentProvider(step, triggeredBy)

  const result = (await relay.call('agent.exec', {
    stepId: step.id,
    prompt: step.config['prompt'],
    worktreePath: step.config['worktreePath'],
    trustPreset: step.config['trustPreset'] ?? 'default',
    traceId,
    // Omit entirely (not even as `undefined` keys) when no override AND no scope match —
    // dev server's agent.exec handler then falls back to its own pre-fix default account,
    // preserving current behavior for workflows that never pin a provider.
    ...(resolved ? { accountId: resolved.accountId, model: resolved.model } : {}),
  })) as { exitCode?: number; stdout?: string; stderr?: string }

  return {
    exitCode: result.exitCode ?? 0,
    stdout: result.stdout,
    stderr: result.stderr,
  }
}

/**
 * Resolve the AI provider account an 'agent' step should use.
 *
 * Priority:
 * 1. step.config.provider.accountId — explicit pin, validated + must be 'active'.
 * 2. ProviderResolver.resolve() fallback — user > project > server scope, same
 *    priority chain already used by every other AI-consuming feature (TDD-16).
 * 3. undefined — no scope match (or serverSpec is 'server:<id>', not yet
 *    resolvable to a devServerId here — same SERVER_SPEC_NOT_SUPPORTED gap as
 *    getRelay() below) → let the dev server apply its own configured default.
 *
 * @throws Error('WORKFLOW_STEP_PROVIDER_NOT_FOUND')  step.config.provider.accountId doesn't exist
 * @throws Error('WORKFLOW_STEP_PROVIDER_INACTIVE')   the pinned account isn't 'active'
 */
private async resolveAgentProvider(
  step: WorkflowStep,
  triggeredBy: string | undefined
): Promise<{ accountId: string; model?: string } | undefined> {
  const providerCfg = step.config['provider'] as WorkflowStepProviderConfig | undefined

  // Case 1: explicit per-step pin — trust verbatim, no priority resolution.
  if (providerCfg?.accountId) {
    const account = await this.aiProviderService.getAccount(providerCfg.accountId)
    if (!account) {
      throw new Error(
        `WORKFLOW_STEP_PROVIDER_NOT_FOUND: step "${step.id}" references unknown provider account "${providerCfg.accountId}"`
      )
    }
    if (account.status !== 'active') {
      throw new Error(
        `WORKFLOW_STEP_PROVIDER_INACTIVE: step "${step.id}" provider account "${providerCfg.accountId}" is not active (status: "${account.status}")`
      )
    }
    return { accountId: account.id, model: providerCfg.model ?? account.model }
  }

  // Case 2: no override — fall back to ProviderResolver's priority chain, scoped to
  // this step's project. Only 'project:<id>' serverSpecs can resolve a devServerId today.
  const [specType, specId] = step.serverSpec.split(':')
  if (specType !== 'project' || !specId) {
    return undefined
  }

  const project = await this.router.getProject(specId)
  if (!project) return undefined

  try {
    const account = await this.providerResolver.resolve({
      devServerId: project.devServerId,
      projectId: specId,
      userId: triggeredBy ?? '__workflow_system__',
      modelHint: providerCfg?.model,
    })
    return { accountId: account.id, model: providerCfg?.model ?? account.model }
  } catch (err) {
    // NO_PROVIDER_AVAILABLE → step still runs, dev server applies its own default —
    // matches pre-fix "always default" behavior for workflows that never pin a provider.
    if (err instanceof Error && err.message.startsWith('NO_PROVIDER_AVAILABLE')) return undefined
    throw err
  }
}
```

`executeShell`/`executeNotification` giữ nguyên chữ ký (không đổi — chỉ `executeAgent` cần `triggeredBy`).

### Bước 4 — `TemplateResolver.ts`: merge logic giữ `provider` khi template con không tự đặt

Đọc verbatim `mergeDefinitions()` hiện tại: nó merge **cả step** theo `stepMap.set(step.id, step)` — leaf ghi đè toàn bộ object step (bao gồm `config`) của root có cùng `id`. Nếu template con chỉ muốn đổi `prompt` mà không lặp lại `provider` đã pin ở template cha, `provider` sẽ **bị mất** (vì override là whole-step, không phải field-level).

```typescript
// backend/src/main/workflow/TemplateResolver.ts

// TRƯỚC (dòng 186-205)
private mergeDefinitions(chain: WorkflowDefinition[]): WorkflowDefinition {
  const stepMap = new Map<string, WorkflowStep>()
  let mergedInputs: Record<string, unknown> = {}

  for (const definition of chain) {
    if (definition.inputs) {
      mergedInputs = { ...mergedInputs, ...definition.inputs }
    }
    for (const step of definition.steps) {
      stepMap.set(step.id, step)
    }
  }

  return {
    steps: [...stepMap.values()],
    inputs: Object.keys(mergedInputs).length > 0 ? mergedInputs : undefined,
  }
}

// SAU
private mergeDefinitions(chain: WorkflowDefinition[]): WorkflowDefinition {
  const stepMap = new Map<string, WorkflowStep>()
  let mergedInputs: Record<string, unknown> = {}

  for (const definition of chain) {
    if (definition.inputs) {
      mergedInputs = { ...mergedInputs, ...definition.inputs }
    }
    for (const step of definition.steps) {
      const existing = stepMap.get(step.id)
      // FIX BUG-BE-HLD-008 (item 3): whole-step override (leaf wins) stays the default —
      // EXCEPT config.provider. JSON has no way to express "explicitly cleared" vs "simply
      // omitted", so a child template step that omits `provider` is read as "inherit the
      // ancestor's pin" rather than "silently drop it" — a child narrowing just the prompt
      // should not accidentally revert to the project-default provider.
      const mergedStep: WorkflowStep =
        existing && step.config['provider'] === undefined && existing.config['provider'] !== undefined
          ? { ...step, config: { ...step.config, provider: existing.config['provider'] } }
          : step
      stepMap.set(step.id, mergedStep)
    }
  }

  return {
    steps: [...stepMap.values()],
    inputs: Object.keys(mergedInputs).length > 0 ? mergedInputs : undefined,
  }
}
```

### Bước 5 — `server-bootstrap.ts`: cross-reference `providerResolver`/`aiProviderService` vào `StepExecutors`

`aiProviderService` và `providerResolver` đã được tạo ở **bước 11** (dòng 429-434), **trước** bước 12 nơi `StepExecutors` được khởi tạo — cùng 1 function `initializeOrcaServices`, cùng scope closure, không cần reorder:

```typescript
// backend/src/main/server-bootstrap.ts

// TRƯỚC (dòng 448-464, bước 12)
// 12. WorkflowOrchestrator + TemplateResolver + StepExecutors [v5.0 TDD-17]
const { DAGBuilder } = await import('./workflow/DAGBuilder')
const { WorkflowOrchestrator } = await import('./workflow/WorkflowOrchestrator')
const { StepExecutors } = await import('./workflow/StepExecutors')
const { TemplateResolver } = await import('./workflow/TemplateResolver')
const { createWorkflowMethods } = await import('./workflow/workflow-rpc-handler')
const dagBuilder = new DAGBuilder()
// Note: _projectRouter from step 10 is used here — it is in scope
const stepExecutors = new StepExecutors(_projectRouter)
const templateResolver = new TemplateResolver(pool)
const workflowOrchestrator = new WorkflowOrchestrator(pool, dagBuilder, stepExecutors, _projectRouter)
await workflowOrchestrator.resumeRunningExecutions().catch(err =>
  console.warn('[ServerBootstrap] resumeRunningExecutions (non-fatal):', (err as Error).message)
)
rpcServer.addMethods(createWorkflowMethods(workflowOrchestrator, templateResolver, pool))
console.log('[ServerBootstrap] ✅ WorkflowOrchestrator initialized (v5.0)')

// SAU
// 12. WorkflowOrchestrator + TemplateResolver + StepExecutors [v5.0 TDD-17]
const { DAGBuilder } = await import('./workflow/DAGBuilder')
const { WorkflowOrchestrator } = await import('./workflow/WorkflowOrchestrator')
const { StepExecutors } = await import('./workflow/StepExecutors')
const { TemplateResolver } = await import('./workflow/TemplateResolver')
const { createWorkflowMethods } = await import('./workflow/workflow-rpc-handler')
const dagBuilder = new DAGBuilder()
// Note: _projectRouter from step 10, aiProviderService/providerResolver from step 11 —
// FIX BUG-BE-HLD-008: cross-reference the AI Provider domain into the Workflow domain so
// agent steps can pin a provider (F36's "mix Claude/GPT-4o across steps" use-case).
const stepExecutors = new StepExecutors(_projectRouter, providerResolver, aiProviderService)
const templateResolver = new TemplateResolver(pool)
const workflowOrchestrator = new WorkflowOrchestrator(pool, dagBuilder, stepExecutors, _projectRouter)
await workflowOrchestrator.resumeRunningExecutions().catch(err =>
  console.warn('[ServerBootstrap] resumeRunningExecutions (non-fatal):', (err as Error).message)
)
rpcServer.addMethods(createWorkflowMethods(workflowOrchestrator, templateResolver, pool))
console.log('[ServerBootstrap] ✅ WorkflowOrchestrator initialized (v5.0)')
```

Không cần thay đổi nào khác ở `server-bootstrap.ts` cho BUG-008 — thứ tự bước 11 → 12 vốn đã đúng (AI Provider domain init trước Workflow domain), chỉ thiếu truyền tham số.

---

## BUG-BE-HLD-009 — Workflow pause/resume (user-triggered)

**Mức độ:** 🟠 HIGH
**Root cause:** `WorkflowStatus` thiếu `'paused'`; không có `WorkflowOrchestrator.pause()`; `resumeRunningExecutions()` là crash-recovery nội bộ (chạy 1 lần lúc bootstrap cho mọi execution `status='running'`), không phải API user-triggered; RPC namespace thiếu `workflow.pause`/`workflow.resume`.

### Bước 1 — `WorkflowTypes.ts`: thêm `'paused'` vào `WorkflowStatus` + `pausedAt` vào `WorkflowExecution`

```typescript
// backend/src/main/workflow/WorkflowTypes.ts

// TRƯỚC (dòng 16)
export type WorkflowStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'

// SAU
export type WorkflowStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
```

```typescript
// TRƯỚC (dòng 51-65)
export interface WorkflowExecution {
  id: string
  definition: WorkflowDefinition
  status: WorkflowStatus
  inputs: Record<string, unknown>
  currentWave: number
  triggeredBy: string
  projectId?: string
  startedAt?: Date
  completedAt?: Date
  errorMessage?: string
  createdAt: Date
}

// SAU
export interface WorkflowExecution {
  id: string
  definition: WorkflowDefinition
  status: WorkflowStatus
  inputs: Record<string, unknown>
  currentWave: number
  triggeredBy: string
  projectId?: string
  startedAt?: Date
  completedAt?: Date
  pausedAt?: Date // [NEW BUG-BE-HLD-009] set on pause(), cleared on resumeFromPause()
  errorMessage?: string
  createdAt: Date
}
```

`'paused'` không phải trạng thái kết thúc (terminal) — không có `pausedCompletedAt`/tương đương; execution vẫn có thể tiếp tục (`resumeFromPause`) hoặc bị huỷ hẳn (`cancel()`, vẫn hợp lệ từ `paused`).

### Bước 2 — Migration mới `0014_workflow_pause_state.ts`: thêm cột `paused_at`

`status` là cột `TEXT NOT NULL DEFAULT 'pending'` **không có CHECK constraint** (xác nhận từ `0009_workflows.ts`) — giá trị `'paused'` tự lưu được mà không cần đổi schema. Cột duy nhất thực sự cần thêm là `paused_at` (audit trail + hiển thị "paused since" ở UI), theo đúng pattern `ALTER TABLE ... ADD COLUMN` của `0013_workflow_trace_correlation.ts`:

```typescript
// backend/src/main/db/migrations/0014_workflow_pause_state.ts (NEW)

/**
 * Migration 0014 — Workflow Pause State
 *
 * FIX BUG-BE-HLD-009: user-triggered pause/resume needs a timestamp for
 * "paused since" in the Workflow Execution UI and for audit — `status='paused'`
 * itself needs no schema change (orca_workflow_executions.status is an
 * unconstrained TEXT column, see 0009_workflows.ts).
 *
 * @module db/migrations/0014_workflow_pause_state
 */

import type { Migration } from './types'

export const migration0014WorkflowPauseState: Migration = {
  version: 14,
  name: 'workflow_pause_state',

  async up(db) {
    // Why: nullable — most rows never pause. Cleared back to NULL by
    // WorkflowOrchestrator.resumeFromPause() so it always reflects "currently paused since".
    await db.exec(`ALTER TABLE orca_workflow_executions ADD COLUMN paused_at INTEGER`)
  },

  async down(_db) {
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // theo đúng pattern 0013_workflow_trace_correlation.ts.
  },
}
```

Đăng ký vào registry:

```typescript
// backend/src/main/db/migrations/index.ts

// TRƯỚC
import { migration0013WorkflowTraceCorrelation } from './0013_workflow_trace_correlation'
import type { Migration } from './types'

export const ALL_MIGRATIONS: readonly Migration[] = [
  ...
  migration0013WorkflowTraceCorrelation,
]

// SAU
import { migration0013WorkflowTraceCorrelation } from './0013_workflow_trace_correlation'
import { migration0014WorkflowPauseState } from './0014_workflow_pause_state'
import type { Migration } from './types'

export const ALL_MIGRATIONS: readonly Migration[] = [
  ...
  // v5.1 — Workflow Trace Correlation (CR-TRACE-017 §3.1 — parentTraceId resume-after-restart)
  migration0013WorkflowTraceCorrelation,
  // v5.1 — Workflow Pause State (BUG-BE-HLD-009 — user-triggered pause/resume)
  migration0014WorkflowPauseState,
]
```

### Bước 3 — `WorkflowOrchestrator.pause(executionId)` / `resumeFromPause(executionId)`

Thiết kế:
- **`pause()`**: chỉ đánh dấu 1 `Set<string>` nội bộ (`pauseRequests`) — KHÔNG abort `AbortController` (khác `cancel()`). Wave đang chạy dở được để chạy hết (các step đã dispatch không bị cắt giữa chừng); `runExecution()`'s vòng lặp wave kiểm tra flag này ở **đầu mỗi vòng lặp, trước khi dispatch wave kế tiếp** — nếu có pause request, dừng lại, ghi `status='paused'`, `current_wave` giữ nguyên giá trị đã lưu của wave vừa hoàn thành (đúng yêu cầu "giữ nguyên state DB hiện tại, không phải rollback").
- **`resumeFromPause()`**: validate execution đang ở `status='paused'`, đọc lại `root_trace_id` đã persist (giống hệt cách `resumeRunningExecutions()` làm — cho trường hợp Orca Server restart trong lúc paused, khi `rootSpans` in-memory đã mất), rồi gọi `runExecution(execution, execution.currentWave, rootTraceId)` — **runExecution() tự chuyển `status: paused → running`** qua `markExecutionRunning()` đã có sẵn ở đầu hàm, không cần thêm bước riêng.
- **Khác biệt rõ ràng với `resumeRunningExecutions()`:** hàm đó là crash-recovery nội bộ, chạy **1 lần lúc bootstrap**, quét toàn bộ execution có `status='running'` (bị ngắt giữa chừng do server restart) và tự resume tất cả — không nhận `executionId`, không có access-control, không gọi được qua RPC. `resumeFromPause()` là **user-triggered**, nhận `executionId` cụ thể, chỉ áp dụng cho execution đang `status='paused'` (không đụng đến execution `'running'` — đó là việc của `resumeRunningExecutions()`), và có access-control ở RPC layer (chỉ `triggeredBy` user).

```typescript
// backend/src/main/workflow/WorkflowOrchestrator.ts

// Thêm field mới trong class (cạnh `rootSpans`, dòng ~82)
export class WorkflowOrchestrator {
  private readonly abortControllers = new Map<string, AbortController>()
  private readonly rootSpans = new Map<string, TraceSpan>()
  // [NEW BUG-BE-HLD-009] executionIds có pause request đang chờ — kiểm tra ở đầu mỗi
  // vòng lặp wave trong runExecution(), KHÔNG abort AbortController (khác cancel()).
  private readonly pauseRequests = new Set<string>()

  constructor(
    private readonly pool: IConnectionPool,
    private readonly dagBuilder: DAGBuilder,
    private readonly stepExecutors: StepExecutors,
    private readonly router: ProjectServerRouter
  ) {}

  // ── Public API ────────────────────────────────────────────────────────────

  /**
   * User-triggered pause. Does NOT abort in-flight steps — the current wave (if
   * any is running) is allowed to finish; only the NEXT wave's dispatch is withheld.
   * Idempotent-safe against double-pause via the status guard below.
   *
   * @throws Error('EXECUTION_NOT_FOUND')          executionId doesn't exist
   * @throws Error('WORKFLOW_PAUSE_INVALID_STATE')  execution isn't currently 'running'
   */
  async pause(executionId: string): Promise<void> {
    const execution = await this.getExecution(executionId)
    if (!execution) throw new Error(`EXECUTION_NOT_FOUND: ${executionId}`)
    if (execution.status !== 'running') {
      throw new Error(
        `WORKFLOW_PAUSE_INVALID_STATE: cannot pause execution "${executionId}" in status "${execution.status}" (expected "running")`
      )
    }
    this.pauseRequests.add(executionId)
    console.log(`[WorkflowOrchestrator] Pause requested for execution ${executionId} — will stop before next wave`)
  }

  /**
   * User-triggered resume of a PAUSED execution — distinct from resumeRunningExecutions()
   * (internal crash-recovery, bootstrap-only, scans ALL status='running' executions).
   * This method targets exactly one execution and only accepts status='paused'.
   *
   * @throws Error('EXECUTION_NOT_FOUND')           executionId doesn't exist
   * @throws Error('WORKFLOW_RESUME_INVALID_STATE')  execution isn't currently 'paused'
   */
  async resumeFromPause(executionId: string): Promise<void> {
    const execution = await this.getExecution(executionId)
    if (!execution) throw new Error(`EXECUTION_NOT_FOUND: ${executionId}`)
    if (execution.status !== 'paused') {
      throw new Error(
        `WORKFLOW_RESUME_INVALID_STATE: cannot resume execution "${executionId}" from status "${execution.status}" (expected "paused")`
      )
    }

    console.log(`[WorkflowOrchestrator] User-resuming paused execution ${executionId} from wave ${execution.currentWave}`)

    // Re-read root_trace_id like resumeRunningExecutions() does — rootSpans is in-memory
    // only, so a server restart WHILE paused (paused executions are intentionally excluded
    // from resumeRunningExecutions()'s bootstrap scan) would otherwise lose the parent span id.
    let span = this.rootSpans.get(executionId)
    if (!span) {
      const rows = await this.pool.withConnection((db) =>
        db.query(`SELECT root_trace_id as rootTraceId FROM orca_workflow_executions WHERE id = ?`, [executionId])
      )
      const rootTraceId = (rows[0] as { rootTraceId: string | null } | undefined)?.rootTraceId ?? undefined
      span = Tracers.workflowExecuteFlow.start(
        { executionId, projectId: execution.projectId, resumedFromPause: true },
        rootTraceId ? { id: rootTraceId } : undefined
      )
      this.rootSpans.set(executionId, span)
    }

    await this.clearPausedAt(executionId)
    // runExecution() calls markExecutionRunning() internally — transitions status paused → running.
    void this.runExecution(execution, execution.currentWave, span.id)
  }

  /**
   * Cancel a running execution by aborting its AbortController.
   */
  async cancel(executionId: string): Promise<void> {
    const controller = this.abortControllers.get(executionId)
    if (controller) {
      controller.abort()
      this.abortControllers.delete(executionId)
    }
    this.pauseRequests.delete(executionId) // [NEW BUG-BE-HLD-009] cancel wins over a pending pause request
    await this.pool.withConnection((db) =>
      db.query(
        `UPDATE orca_workflow_executions SET status = 'cancelled', completed_at = ? WHERE id = ?`,
        [Date.now(), executionId]
      )
    )
    const span = this.rootSpans.get(executionId)
    if (span) {
      span.fail('EXECUTION_CANCELLED', { status: 'cancelled' })
      this.rootSpans.delete(executionId)
    }
  }
```

Wave loop trong `runExecution()` — kiểm tra pause **trước** `updateCurrentWave`/dispatch của wave kế tiếp:

```typescript
// TRƯỚC (đoạn trong runExecution(), dòng 274-280)
for (let waveIndex = startWave; waveIndex < waves.length; waveIndex++) {
  if (controller.signal.aborted) {
    return
  }

  await this.updateCurrentWave(execution.id, waveIndex)
  const wave = waves[waveIndex]

// SAU
for (let waveIndex = startWave; waveIndex < waves.length; waveIndex++) {
  if (controller.signal.aborted) {
    return
  }

  // [NEW BUG-BE-HLD-009] Check pause request BEFORE dispatching the next wave. The
  // previous wave's steps (if any) have already been awaited by this point in the loop —
  // they finish normally; only this wave's dispatch is withheld. current_wave stays at
  // the last value written by updateCurrentWave() (the last COMPLETED wave), matching
  // "giữ nguyên state DB hiện tại, không phải rollback".
  if (this.pauseRequests.has(execution.id)) {
    this.pauseRequests.delete(execution.id)
    await this.markExecutionPaused(execution.id)
    return
  }

  await this.updateCurrentWave(execution.id, waveIndex)
  const wave = waves[waveIndex]
```

DB helpers mới (cạnh `markExecutionFailed`/`markExecutionCompleted`):

```typescript
// [NEW BUG-BE-HLD-009]
private async markExecutionPaused(executionId: string): Promise<void> {
  await this.pool.withConnection((db) =>
    db.query(
      `UPDATE orca_workflow_executions SET status = 'paused', paused_at = ? WHERE id = ?`,
      [Date.now(), executionId]
    )
  )
  // 'paused' is NOT a terminal status (unlike completed/failed/cancelled) — the root span
  // stays open in rootSpans; TracePanel keeps grouping steps under it after resume.
}

private async clearPausedAt(executionId: string): Promise<void> {
  await this.pool.withConnection((db) =>
    db.query(`UPDATE orca_workflow_executions SET paused_at = NULL WHERE id = ?`, [executionId])
  )
}
```

Cập nhật `ExecutionRow`/`rowToExecution`/2 câu `SELECT` (trong `getExecution()` và `listExecutions()`) để đọc `paused_at`:

```typescript
// TRƯỚC (dòng 45-73)
interface ExecutionRow {
  id: string
  definitionSnapshot: string
  status: string
  inputsJson: string
  currentWave: number
  triggeredBy: string
  projectId: string | null
  startedAt: number | null
  completedAt: number | null
  errorMessage: string | null
  createdAt: number
}

function rowToExecution(r: ExecutionRow): WorkflowExecution {
  return {
    id: r.id,
    definition: JSON.parse(r.definitionSnapshot) as WorkflowDefinition,
    status: r.status as WorkflowExecution['status'],
    inputs: JSON.parse(r.inputsJson) as Record<string, unknown>,
    currentWave: r.currentWave,
    triggeredBy: r.triggeredBy,
    projectId: r.projectId ?? undefined,
    startedAt: r.startedAt ? new Date(r.startedAt) : undefined,
    completedAt: r.completedAt ? new Date(r.completedAt) : undefined,
    errorMessage: r.errorMessage ?? undefined,
    createdAt: new Date(r.createdAt),
  }
}

// SAU
interface ExecutionRow {
  id: string
  definitionSnapshot: string
  status: string
  inputsJson: string
  currentWave: number
  triggeredBy: string
  projectId: string | null
  startedAt: number | null
  completedAt: number | null
  pausedAt: number | null // [NEW BUG-BE-HLD-009]
  errorMessage: string | null
  createdAt: number
}

function rowToExecution(r: ExecutionRow): WorkflowExecution {
  return {
    id: r.id,
    definition: JSON.parse(r.definitionSnapshot) as WorkflowDefinition,
    status: r.status as WorkflowExecution['status'],
    inputs: JSON.parse(r.inputsJson) as Record<string, unknown>,
    currentWave: r.currentWave,
    triggeredBy: r.triggeredBy,
    projectId: r.projectId ?? undefined,
    startedAt: r.startedAt ? new Date(r.startedAt) : undefined,
    completedAt: r.completedAt ? new Date(r.completedAt) : undefined,
    pausedAt: r.pausedAt ? new Date(r.pausedAt) : undefined, // [NEW BUG-BE-HLD-009]
    errorMessage: r.errorMessage ?? undefined,
    createdAt: new Date(r.createdAt),
  }
}
```

```typescript
// getExecution() — thêm "paused_at as pausedAt" vào SELECT (dòng 147-158)
db.query<ExecutionRow>(
  `SELECT id,
          definition_snapshot as definitionSnapshot,
          status,
          inputs_json         as inputsJson,
          current_wave        as currentWave,
          triggered_by        as triggeredBy,
          project_id          as projectId,
          started_at          as startedAt,
          completed_at        as completedAt,
          paused_at           as pausedAt,
          error_message       as errorMessage,
          created_at          as createdAt
   FROM orca_workflow_executions WHERE id = ?`,
  [executionId]
)

// listExecutions() — cùng thay đổi cho SELECT (dòng 189-201)
const sql = `
  SELECT id,
         definition_snapshot as definitionSnapshot,
         status,
         inputs_json         as inputsJson,
         current_wave        as currentWave,
         triggered_by        as triggeredBy,
         project_id          as projectId,
         started_at          as startedAt,
         completed_at        as completedAt,
         paused_at           as pausedAt,
         error_message       as errorMessage,
         created_at          as createdAt
  FROM orca_workflow_executions ${where}
  ORDER BY created_at DESC LIMIT ?`
```

`resumeRunningExecutions()` **giữ nguyên 100%** — không sửa gì (nó chỉ query `status='running'`, execution `'paused'` không nằm trong tập kết quả, đúng ý đồ: server restart không tự resume 1 execution mà user chủ động pause).

### Bước 4 — `workflow-rpc-handler.ts`: 2 RPC method mới `workflow.pause` / `workflow.resume`

```typescript
// backend/src/main/workflow/workflow-rpc-handler.ts

// Header comment — TRƯỚC
/**
 * Workflow RPC Methods (TDD-17)
 *
 * Factory function — inject orchestrator and templateResolver at bootstrap.
 * 7 RPC methods:
 *   workflow.execute, workflow.getExecution, workflow.listExecutions,
 *   workflow.cancel, workflow.template.create, workflow.template.list,
 *   workflow.template.resolve
 *
 * Access control:
 * - All ops require authentication (userId from ctx.userId)
 * - workflow.cancel: only the triggeredBy user (or admin)
 * - workflow.template.create: any authenticated user
 */

// Header comment — SAU
/**
 * Workflow RPC Methods (TDD-17)
 *
 * Factory function — inject orchestrator and templateResolver at bootstrap.
 * 9 RPC methods:
 *   workflow.execute, workflow.getExecution, workflow.listExecutions,
 *   workflow.cancel, workflow.pause, workflow.resume,
 *   workflow.template.create, workflow.template.list, workflow.template.resolve
 *
 * Access control:
 * - All ops require authentication (userId from ctx.userId)
 * - workflow.cancel / workflow.pause / workflow.resume: only the triggeredBy user (or admin)
 * - workflow.template.create: any authenticated user
 *
 * [BUG-BE-HLD-009] workflow.resume calls orchestrator.resumeFromPause() — a SINGLE-execution,
 * user-triggered resume, NOT orchestrator.resumeRunningExecutions() (internal crash-recovery,
 * called once at server bootstrap for every status='running' execution, no RPC exposure).
 */
```

```typescript
// ListExecutionsParam — TRƯỚC (dòng 57-62)
const ListExecutionsParam = z.object({
  projectId: z.string().optional(),
  triggeredBy: z.string().optional(),
  status: z.enum(['pending', 'running', 'completed', 'failed', 'cancelled']).optional(),
  limit: z.number().int().positive().max(500).optional(),
})

// SAU
const ListExecutionsParam = z.object({
  projectId: z.string().optional(),
  triggeredBy: z.string().optional(),
  status: z.enum(['pending', 'running', 'paused', 'completed', 'failed', 'cancelled']).optional(), // [NEW BUG-BE-HLD-009]
  limit: z.number().int().positive().max(500).optional(),
})
```

```typescript
// Thêm 2 param schema cạnh CancelParam (dòng 64-66)
const PauseParam = z.object({
  executionId: z.string().min(1),
})

const ResumeParam = z.object({
  executionId: z.string().min(1),
})
```

```typescript
// Thêm 2 method mới ngay sau 'workflow.cancel' (sau dòng 162), TRƯỚC 'workflow.template.create'
    // ── workflow.pause ─────────────────────────────────────────────────────

    defineMethod({
      name: 'workflow.pause',
      params: PauseParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        // Access control: same rule as workflow.cancel — only the triggering user may pause
        const execution = await orchestrator.getExecution(params.executionId)
        if (!execution) throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)
        if (execution.triggeredBy !== userId) {
          throw new Error('WORKFLOW_PAUSE_DENIED: only the triggering user can pause this execution')
        }
        await orchestrator.pause(params.executionId)
        return { paused: true }
      },
    }),

    // ── workflow.resume ────────────────────────────────────────────────────

    defineMethod({
      name: 'workflow.resume',
      params: ResumeParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        const execution = await orchestrator.getExecution(params.executionId)
        if (!execution) throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)
        if (execution.triggeredBy !== userId) {
          throw new Error('WORKFLOW_RESUME_DENIED: only the triggering user can resume this execution')
        }
        // [BUG-BE-HLD-009] resumeFromPause(), KHÔNG PHẢI resumeRunningExecutions() — xem header comment
        await orchestrator.resumeFromPause(params.executionId)
        return { resumed: true }
      },
    }),
```

`workflow.cancel` không cần đổi — `cancel()` vẫn hợp lệ gọi trên execution `status='paused'` (huỷ hẳn thay vì tiếp tục), và §Bước 3 đã thêm `this.pauseRequests.delete(executionId)` vào `cancel()` để dọn pending pause-request nếu có.

`server-bootstrap.ts` **không cần sửa thêm gì** cho BUG-009 — `createWorkflowMethods(workflowOrchestrator, templateResolver, pool)` ở bước 12 (đã sửa ở BUG-008 Bước 5) tự động đăng ký 2 method mới vì chúng nằm trong cùng mảng trả về của factory function.

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `backend/src/main/workflow/WorkflowOrchestrator.ts` | Sửa `executeStep()` gọi đúng `StepExecutors.execute()`; xoá alias `StepExecutorFn`/`StepExecutors` sai; forward `triggeredBy` | §0 (chặn BUG-008) |
| `backend/src/main/workflow/WorkflowOrchestrator.ts` | Thêm `pause()`, `resumeFromPause()`, `pauseRequests`, `markExecutionPaused()`, `clearPausedAt()`, wave-boundary check, `pausedAt` trong `ExecutionRow`/SELECT | BUG-BE-HLD-009 |
| `backend/src/main/workflow/WorkflowTypes.ts` | Thêm `WorkflowStepProviderConfig`, `WorkflowStepConfig.provider` | BUG-BE-HLD-008 |
| `backend/src/main/workflow/WorkflowTypes.ts` | Thêm `'paused'` vào `WorkflowStatus`, `pausedAt` vào `WorkflowExecution` | BUG-BE-HLD-009 |
| `backend/src/main/workflow/StepExecutors.ts` | Constructor nhận `ProviderResolver` + `AIProviderService`; `executeAgent()` resolve provider; `resolveAgentProvider()` mới; forward `triggeredBy` qua `execute()`/`executeByType()` | BUG-BE-HLD-008 |
| `backend/src/main/workflow/TemplateResolver.ts` | `mergeDefinitions()` giữ `config.provider` từ ancestor khi leaf step không tự đặt | BUG-BE-HLD-008 |
| `backend/src/main/workflow/workflow-rpc-handler.ts` | `WorkflowStepConfigSchema` thêm `provider` (zod) | BUG-BE-HLD-008 |
| `backend/src/main/workflow/workflow-rpc-handler.ts` | 2 RPC method `workflow.pause`/`workflow.resume`; `status` enum thêm `'paused'` | BUG-BE-HLD-009 |
| `backend/src/main/db/migrations/0014_workflow_pause_state.ts` | NEW — cột `paused_at` | BUG-BE-HLD-009 |
| `backend/src/main/db/migrations/index.ts` | Đăng ký `migration0014WorkflowPauseState` | BUG-BE-HLD-009 |
| `backend/src/main/server-bootstrap.ts` | `new StepExecutors(_projectRouter, providerResolver, aiProviderService)` | BUG-BE-HLD-008 |

---

## Verification Plan

```bash
pnpm vitest run backend/src/main/workflow/__tests__/

# §0 wiring fix (viết test mới — chưa có test bao phủ executeStep()):
# 1. Mock StepExecutors với execute() spy → gọi runExecution() qua execute() public API
#    → assert stepExecutors.execute() được gọi đúng 5 tham số (step, inputs, signal, traceId, triggeredBy)
# 2. Trước fix: assert step luôn throw UNSUPPORTED_STEP_TYPE (regression guard nếu ai revert §0)

# BUG-BE-HLD-008 — Provider selection:
# 1. Step có config.provider={accountId:'acc-claude'} → assert relay.call('agent.exec', {..., accountId:'acc-claude'})
# 2. Step KHÔNG có provider, project có 1 account user-scope active → assert accountId của account đó được dùng
# 3. Step provider.accountId không tồn tại → assert throw WORKFLOW_STEP_PROVIDER_NOT_FOUND
# 4. Step provider.accountId tồn tại nhưng status != 'active' → assert throw WORKFLOW_STEP_PROVIDER_INACTIVE
# 5. Workflow 2 bước: bước 1 provider=Claude account, bước 2 provider=GPT-4o account (khác scope/model)
#    → assert 2 relay.call khác nhau đúng account/model — đúng use-case chính của F36
# 6. TemplateResolver: template cha step id='build' có provider=X; template con override cùng id chỉ đổi prompt,
#    không set provider → assert merged step vẫn giữ provider=X
# 7. Template con TỰ đặt provider=Y cho step 'build' → assert override thắng (Y, không phải X)

# BUG-BE-HLD-009 — Pause/Resume:
# 1. Execute workflow 3 wave → gọi pause() giữa wave 1 đang chạy → assert wave 1 chạy hết (steps complete
#    bình thường), wave 2 KHÔNG dispatch, status cuối cùng = 'paused', current_wave = 1 (wave đã hoàn thành)
# 2. pause() trên execution status='pending'/'completed' → assert throw WORKFLOW_PAUSE_INVALID_STATE
# 3. resumeFromPause() trên execution 'paused' → assert status → 'running' → tiếp tục đúng từ current_wave,
#    KHÔNG re-run các step đã completed ở wave trước (dùng lại guard TASK-WF-002 đã có sẵn)
# 4. resumeFromPause() trên execution KHÔNG phải 'paused' (vd 'running') → assert throw WORKFLOW_RESUME_INVALID_STATE
# 5. paused_at set đúng lúc pause(), clear về NULL đúng lúc resumeFromPause()
# 6. cancel() trên execution đang 'paused' → assert vẫn chuyển 'cancelled' bình thường (không bị kẹt)
# 7. RPC access control: workflow.pause/workflow.resume bởi user khác triggeredBy → assert throw *_DENIED
# 8. resumeRunningExecutions() (bootstrap) KHÔNG động vào execution 'paused' — chỉ quét 'running'
#    (regression guard: đảm bảo 2 cơ chế resume không giẫm lên nhau)
```
