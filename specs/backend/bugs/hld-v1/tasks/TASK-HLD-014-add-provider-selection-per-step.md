# TASK-HLD-014: Thêm chọn AI provider theo từng workflow step

**Priority:** 🟠 HIGH
**Effort:** ~3-4 giờ (bao gồm test)
**Status:** ✅ DONE — 2026-08-09 (áp dụng đủ 5 bước: `WorkflowStepProviderConfig` type, zod schema, `StepExecutors` constructor+`executeAgent`+`resolveAgentProvider` mới, `TemplateResolver.mergeDefinitions` giữ provider khi child không tự đặt, `server-bootstrap.ts` cross-reference `providerResolver`/`aiProviderService`. Đã verify từng API thật trước khi dùng: `AIProviderService.getAccount()`, `ProviderResolver.resolve()`, `ProjectServerRouter.getProject()`, field `account.status`/`account.model` đều khớp. `tsc --noEmit` không phát sinh lỗi mới — mọi lỗi còn lại (TemplateResolver.ts, server-bootstrap.ts:485 không liên quan, workflow-rpc-handler.ts z.record) đều pre-existing baseline. ⚠️ Chưa viết 7 test case theo yêu cầu solution — effort budget, cần bổ sung riêng trước khi merge thật.)
**Bug refs:** BUG-BE-HLD-008
**Solution ref:** [SOLUTION-workflow-exact.md — BUG-BE-HLD-008](../solutions/SOLUTION-workflow-exact.md#bug-be-hld-008--chọn-ai-provider-theo-từng-workflow-step)
**Depends on:** **TASK-HLD-013 (BLOCKER — phải merge trước)**. Nếu TASK-HLD-013 chưa xong, `WorkflowOrchestrator.executeStep()` không gọi trúng `StepExecutors.execute()` → toàn bộ logic provider resolution viết trong task này là dead code không bao giờ chạy.

---

## Mục tiêu

Cho phép mỗi `agent` step trong workflow chọn riêng 1 AI provider account (thay vì luôn dùng default của project) — use-case chính (F36): dùng Claude ở step 1, GPT-4o ở step 2 trong cùng 1 workflow.

Root cause (theo solution): `WorkflowStepConfig` không có field `provider`; `StepExecutors.executeAgent()` không nhận `ProviderResolver`/`AIProviderService`; `server-bootstrap.ts` khởi tạo AI Provider domain và Workflow domain độc lập, không cross-reference.

## File cần sửa

```
backend/src/main/workflow/WorkflowTypes.ts
backend/src/main/workflow/workflow-rpc-handler.ts
backend/src/main/workflow/StepExecutors.ts
backend/src/main/workflow/TemplateResolver.ts
backend/src/main/server-bootstrap.ts
```

## Thay đổi cụ thể

### Bước 1 — `WorkflowTypes.ts`: thêm field `provider` vào `WorkflowStepConfig`

TRƯỚC (dòng 18-27):

```typescript
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
```

SAU:

```typescript
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

> **Ghi chú alignment với TDD-17:** TDD-17 §2 định nghĩa `WorkflowStep.providerSpec?: string` (top-level, giá trị là accountId hoặc chuỗi `'resolve:server'`) — thiết kế này **chưa từng được implement** ở bất kỳ file thật nào (`grep providerSpec` trong `backend/src/main/workflow/*.ts` = 0 kết quả tại thời điểm viết solution). Ticket BUG-BE-HLD-008 và `docs/features/F36-multi-server-workflow-orchestration.md` đều mô tả cú pháp `provider: { account, model }` **lồng trong step config**, không phải field rời ở top-level `WorkflowStep`. Task này theo đúng cú pháp ticket yêu cầu (khớp F36 doc), không phải TDD §2. Nếu muốn đồng bộ tuyệt đối với TDD sau này, đó là 1 rename riêng (`gitnexus rename`), không phải trong phạm vi task này.

### Bước 2 — `workflow-rpc-handler.ts`: mở rộng zod schema cho `provider`

TRƯỚC (dòng 27-29):

```typescript
const WorkflowStepConfigSchema = z.object({
  type: z.enum(['agent', 'shell', 'webhook', 'notification', 'condition']),
}).catchall(z.unknown())
```

SAU:

```typescript
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

Constructor:

TRƯỚC (dòng 17-23):

```typescript
import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { WorkflowStep, StepOutput } from './WorkflowTypes'

const DEFAULT_TIMEOUT_MS = 30 * 60_000 // 30 minutes

export class StepExecutors {
  constructor(private readonly router: ProjectServerRouter) {}
```

SAU:

```typescript
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

`execute()`/`executeByType()` forward `triggeredBy` xuống `executeAgent()`:

TRƯỚC (dòng 33-82):

```typescript
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
```

SAU:

```typescript
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

TRƯỚC (dòng 86-103):

```typescript
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
```

SAU:

```typescript
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

Đọc verbatim `mergeDefinitions()` hiện tại trước khi sửa — theo solution, nó merge **cả step** theo `stepMap.set(step.id, step)`: leaf ghi đè toàn bộ object step (bao gồm `config`) của root có cùng `id`. Nếu template con chỉ đổi `prompt` mà không lặp lại `provider` đã pin ở template cha, `provider` sẽ bị mất (override là whole-step, không phải field-level).

TRƯỚC (dòng 186-205):

```typescript
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
```

SAU:

```typescript
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

`aiProviderService` và `providerResolver` đã được tạo ở bước 11 (AI Provider domain), **trước** bước 12 nơi `StepExecutors` được khởi tạo — cùng 1 function `initializeOrcaServices`, cùng scope closure, không cần reorder.

TRƯỚC (dòng 448-464, bước 12):

```typescript
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
```

SAU:

```typescript
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

Không cần thay đổi nào khác ở `server-bootstrap.ts` — thứ tự bước 11 → 12 vốn đã đúng (AI Provider domain init trước Workflow domain), chỉ thiếu truyền tham số.

## Verification

Trước khi bắt đầu: xác nhận TASK-HLD-013 đã merge (kiểm tra `WorkflowOrchestrator.executeStep()` gọi `this.stepExecutors.execute(...)` chứ không phải tra map `this.stepExecutors[type]`).

```bash
pnpm tsc --noEmit
pnpm vitest run backend/src/main/workflow/__tests__/

# 1. Step có config.provider={accountId:'acc-claude'}
#    → assert relay.call('agent.exec', {..., accountId:'acc-claude'})
# 2. Step KHÔNG có provider, project có 1 account user-scope active
#    → assert accountId của account đó được dùng (qua ProviderResolver)
# 3. Step provider.accountId không tồn tại → assert throw WORKFLOW_STEP_PROVIDER_NOT_FOUND
# 4. Step provider.accountId tồn tại nhưng status != 'active' → assert throw WORKFLOW_STEP_PROVIDER_INACTIVE
# 5. Workflow 2 bước: bước 1 provider=Claude account, bước 2 provider=GPT-4o account (khác scope/model)
#    → assert 2 relay.call khác nhau đúng account/model — đúng use-case chính của F36
# 6. TemplateResolver: template cha step id='build' có provider=X; template con override cùng id
#    chỉ đổi prompt, không set provider → assert merged step vẫn giữ provider=X
# 7. Template con TỰ đặt provider=Y cho step 'build' → assert override thắng (Y, không phải X)

# Regression check bắt buộc trước khi commit:
# - executeShell/executeNotification/executeWebhook/executeCondition không bị ảnh hưởng
#   bởi việc thêm tham số triggeredBy (chữ ký của chúng không đổi)
# - resumeRunningExecutions() không bị đụng tới trong task này — chạy lại toàn bộ suite
#   để xác nhận không có regression ngoài phạm vi provider selection
grep -n "resumeRunningExecutions" backend/src/main/workflow/WorkflowOrchestrator.ts
# Expected: không có thay đổi so với trước task này (task chỉ đụng StepExecutors,
# WorkflowTypes, TemplateResolver, workflow-rpc-handler, server-bootstrap)
```

**Điều kiện DONE:** `pnpm tsc --noEmit` pass, toàn bộ 7 test case trên pass, `pnpm vitest run backend/src/main/workflow/__tests__/` pass không regression, `server-bootstrap.ts` khởi tạo `StepExecutors` với đúng 3 tham số.
