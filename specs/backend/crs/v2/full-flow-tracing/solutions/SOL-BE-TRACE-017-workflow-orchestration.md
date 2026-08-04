# SOL-BE-TRACE-017: Workflow Orchestration — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**TDD Ref:** TDD-17 (Multi-Server Workflow Orchestration — [17-workflow-orchestration.md](../../../../tdd/v5/17-workflow-orchestration.md), §3 `DAGBuilder`, §4 `WorkflowOrchestrator`, §5 `TemplateResolver`); tham chiếu §F.10 HLD cross-reference trong `00-index.md` ("WorkflowOrchestrator — DAG Dispatch")
**Date:** 2026-08-02
**Status:** Proposed
**Strategy:** Additive-only — chỉ thêm tracer calls + 1 migration mới (cột `root_trace_id`) để persist correlation id qua restart; không đổi logic DAG/wave

---

## 1. Phân tích phạm vi (Backend-side only)

Đã Read toàn bộ `WorkflowOrchestrator.ts`, `DAGBuilder.ts`, `StepExecutors.ts`, `TemplateResolver.ts`, `workflow-rpc-handler.ts` — API thật khác tên hàm so với pseudo-code minh hoạ trong CR-TRACE-017 ở vài chỗ (`execute()` không phải `start()`, `buildWaves()` không phải `build()`), nhưng **toàn bộ số dòng file:line trong CR khớp chính xác với code thật** đã verify trực tiếp.

| File | Hàm/method | Dòng thực tế đã verify | Gap |
|------|-----------|--------------------------|-----|
| `src/shared/trace/tracers.ts` | thêm 4 tracer | — | ❌ Thiếu `workflowTemplateCreateFlow`/`workflowExecuteFlow`/`workflowStepFlow`/`workflowShareFlow` |
| `src/main/workflow/WorkflowOrchestrator.ts` | `execute()` (93), `runExecution()` private (228), `executeStep()` private (307), `resumeRunningExecutions()` (218), `markExecutionCompleted()` (402), `markExecutionFailed()` (411), `updateCurrentWave()` (421) | ✅ tất cả khớp CR | ❌ Chưa có tracer nào; **chưa có cơ chế persist `rootTraceId`** — `persistExecution()` (368) và bảng `orca_workflow_executions` (migration 0009) không có cột lưu trace id |
| `src/main/workflow/DAGBuilder.ts` | `buildWaves()` (32) | ✅ khớp CR | ❌ Không có step riêng (đây là hàm nội bộ, không băng qua boundary — chỉ đo qua `step('build-waves')` ở span cha, không tự tạo span) |
| `src/main/workflow/StepExecutors.ts` | `execute()` (33), `executeAgent()` relay.call (88), `executeShell()` relay.call (108), `executeNotification()` relay.call (149) | ✅ tất cả khớp CR gần như tuyệt đối | ❌ Chưa forward `traceId` vào bất kỳ `relay.call()` nào trong 3 hàm |
| `src/main/workflow/TemplateResolver.ts` | `create()` (120), `resolve()` (72) | ✅ khớp CR | ❌ Chưa có tracer cho `create()`; `resolve()` không nằm trong BL-WF-01 write path nên không cần tracer (đọc thuần, xem §2.4) |
| `src/main/workflow/workflow-rpc-handler.ts` | `workflow.execute` (90-104), `workflow.template.create` (148-162) | gần khớp CR (CR ghi 149-165, thực tế 148-162 — lệch 1 dòng do comment) | ❌ Chưa forward `traceId`; chưa trả `traceId: execution` trong response |
| `src/main/db/migrations/` | migration mới | tiếp theo `0012_port_forwards_push.ts` | ❌ Cần migration `0013` thêm cột `root_trace_id TEXT` vào `orca_workflow_executions` |

**Ngoài phạm vi (agent-side):** phần Dev Server nhận `agent.exec`/`shell.exec`/`notification.send` qua relay và thực thi thật (`src/relay/agent-*.ts`). Backend chỉ trace tới điểm `relay.call()` rời khỏi `StepExecutors`.

**BL-WF-03 (Workflow Sharing):** đã verify — `TemplateResolver.ts` (đọc toàn bộ 192 dòng) **không có** `updateVisibility()`, share-token, hay bất kỳ API `shared/:token` nào. `workflowShareFlow` là **placeholder tracer** — được khai báo trong `tracers.ts` để sẵn sàng khi tính năng sharing tồn tại, nhưng solution này không patch code nào cho nó (đúng theo CR §BL-WF-03 "không có file:function thật để trích dẫn").

---

## 2. Full Implementation

### 2.1 Thiết kế `parentTraceId` — mô hình correlation cho DAG/wave

Đây là phần kiến trúc quan trọng nhất của solution này. Bối cảnh: 1 workflow execution có N wave × M step song song, mỗi step là 1 network hop độc lập (có thể tới Dev Server khác nhau, có thể fail/timeout độc lập với các step khác trong cùng wave). CR-TRACE-000 §3.1 (`resume`) được thiết kế cho **1 hop tiếp nối id qua nhiều layer** (Browser → Main → Relay → Agent của **cùng một lời gọi**) — không phù hợp để nhóm N span độc lập (N step) dưới 1 "cha" logic, vì mỗi step vẫn cần `id` riêng để TracePanel hiển thị đúng latency/status riêng từng step.

**Quyết định thiết kế:** `parentTraceId` là **field nghiệp vụ** (nằm trong `TraceFields`, giống `projectId`/`accountId`), KHÔNG phải cơ chế `resume`/`traceId` giao thức của CR-TRACE-000 §3.1-3.2. Hai cơ chế này độc lập và bổ sung cho nhau:

| Cơ chế | Dùng để | Phạm vi |
|--------|---------|---------|
| `resume: { id }` (CR-TRACE-000 §3.1) | Giữ nguyên `span.id` khi CÙNG một hop đi qua nhiều layer (vd. Browser → RPC → Relay → Agent của 1 request) | 1 span, nhiều layer |
| `traceId` trong wire envelope (CR-TRACE-000 §3.2-3.3) | Truyền span id qua transport để layer sau `resume` đúng | Giữa 2 layer liền kề |
| `parentTraceId` (field nghiệp vụ, **mới trong CR này**) | Nhóm N span ĐỘC LẬP (N step, mỗi step 1 `id` riêng) dưới 1 execution để TracePanel filter theo execution mà không cần join DB | 1-nhiều span, không đổi `id` của span con |

```typescript
// WorkflowOrchestrator.ts — execute()
async execute(...): Promise<WorkflowExecution> {
  const id = randomUUID()
  const span = Tracers.workflowExecuteFlow.start({ executionId: id, projectId })
  // rootTraceId = span.id — PHẢI được persist vào DB (xem §2.3) để resumeRunningExecutions()
  // có thể tái tạo lại span cha với CÙNG id sau khi Orca Server restart giữa chừng execution.
  await this.persistExecution({ ..., rootTraceId: span.id })
  void this.runExecution(execution, 0, span.id)  // truyền rootTraceId xuống runExecution
  return execution
}

private async runExecution(execution: WorkflowExecution, startWave = 0, rootTraceId?: string): Promise<void> {
  // ...
  wave.map(async (step) => {
    const stepSpan = Tracers.workflowStepFlow.start({
      parentTraceId: rootTraceId,   // field nghiệp vụ — KHÔNG resume, span con có id riêng
      executionId: execution.id, stepId: step.id, stepType: step.config.type,
    })
    // ...
  })
}
```

**Bảo toàn `rootTraceId` qua restart:** khi `resumeRunningExecutions()` (dòng 218) load lại các execution có `status='running'` sau khi Orca Server khởi động lại, nó phải đọc `root_trace_id` đã persist và tái tạo span cha bằng `resume: { id: rootTraceId }` — nếu không, `workflow:execute` sẽ có `id` mới sau mỗi lần restart, làm gãy khả năng nhóm step cũ + step mới trong TracePanel dưới cùng 1 `parentTraceId`.

### 2.2 `src/shared/trace/tracers.ts` — thêm 4 tracer

```typescript
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (profile:*, aiProvider:* từ SOL-015/016)...

  // ── Workflow Orchestration (CR-TRACE-017) ────────────────────────────────────
  /** BL-WF-01: template create/inherit */
  workflowTemplateCreateFlow: createTracer('workflow:templateCreate'),
  /** BL-WF-02: span CHA — 1 per execution, sống suốt vòng đời execution */
  workflowExecuteFlow:        createTracer('workflow:execute'),
  /** BL-WF-02: span CON — 1 per step, mang field parentTraceId để group theo execution */
  workflowStepFlow:           createTracer('workflow:stepExecute'),
  /** BL-WF-03: PLACEHOLDER — chưa có implementation, xem §1 */
  workflowShareFlow:          createTracer('workflow:share'),
} as const
```

### 2.3 Migration `0013_workflow_trace_correlation.ts` — persist `root_trace_id`

```typescript
// src/main/db/migrations/0013_workflow_trace_correlation.ts
import type { Migration } from './types'

export const migration0013WorkflowTraceCorrelation: Migration = {
  version: 13,
  name: 'workflow_trace_correlation',

  async up(db) {
    // Why: rootTraceId phải sống sót qua Orca Server restart để resumeRunningExecutions()
    // tái tạo đúng span cha (CR-TRACE-000 §3.1 resume) — nếu không, TracePanel mất khả năng
    // nhóm step cũ (trước restart) với step mới (sau restart) dưới cùng 1 execution.
    await db.exec(`ALTER TABLE orca_workflow_executions ADD COLUMN root_trace_id TEXT`)
  },

  async down(db) {
    // SQLite không hỗ trợ DROP COLUMN trực tiếp trước 3.35 — no-op an toàn,
    // cột thừa không ảnh hưởng hành vi nếu rollback (theo pattern các migration khác trong repo).
  },
}
```

```typescript
// src/main/db/migrations/index.ts — thêm vào ALL_MIGRATIONS sau migration0012PortForwardsPush
import { migration0013WorkflowTraceCorrelation } from './0013_workflow_trace_correlation'

export const ALL_MIGRATIONS: readonly Migration[] = [
  // ...0001-0012 unchanged...
  migration0013WorkflowTraceCorrelation,
]
```

### 2.4 `src/main/workflow/WorkflowOrchestrator.ts` — BL-WF-02

```typescript
import { Tracers } from '../../shared/trace/tracers'

async execute(
  definition: WorkflowDefinition,
  inputs: Record<string, unknown>,
  triggeredBy: string,
  projectId?: string
): Promise<WorkflowExecution> {
  const id = randomUUID()
  const now = Date.now()

  const span = Tracers.workflowExecuteFlow.start({ executionId: id, projectId, triggeredBy })

  await this.persistExecution({ id, definition, inputs, triggeredBy, projectId, now, rootTraceId: span.id })

  const execution: WorkflowExecution = {
    id, definition, status: 'pending', inputs, currentWave: 0, triggeredBy, projectId,
    createdAt: new Date(now),
  }

  // rootTraceId = span.id truyền xuống runExecution() để mọi step span mang parentTraceId đúng
  void this.runExecution(execution, 0, span.id)

  return execution
}

async resumeRunningExecutions(): Promise<void> {
  const running = await this.listExecutions({ status: 'running' })
  for (const execution of running) {
    console.log(`[WorkflowOrchestrator] Resuming execution ${execution.id} from wave ${execution.currentWave}`)
    // Đọc lại rootTraceId đã persist — resume() giữ nguyên id cha qua restart (CR-TRACE-000 §3.1)
    const row = await this.pool.withConnection((db) =>
      db.query<{ rootTraceId: string | null }>(
        `SELECT root_trace_id as rootTraceId FROM orca_workflow_executions WHERE id = ?`, [execution.id]
      )
    )
    const rootTraceId = row[0]?.rootTraceId ?? undefined
    const span = Tracers.workflowExecuteFlow.start(
      { executionId: execution.id, projectId: execution.projectId, resumed: true },
      rootTraceId ? { id: rootTraceId } : undefined
    )
    void this.runExecution(execution, execution.currentWave, span.id)
  }
}

private async runExecution(
  execution: WorkflowExecution,
  startWave = 0,
  rootTraceId?: string   // [NEW] param — id của span cha workflow:execute, dùng làm parentTraceId cho mọi step
): Promise<void> {
  const controller = new AbortController()
  this.abortControllers.set(execution.id, controller)

  try {
    await this.markExecutionRunning(execution.id)

    const waves = this.dagBuilder.buildWaves(execution.definition.steps)
    // build-waves KHÔNG tự tạo span (không băng qua boundary, thuần in-memory tính toán DAG)
    // — chỉ step() trên span cha nếu đang có tham chiếu; ở đây span cha đã start()/persist ở
    // execute() nên ta không giữ reference trực tiếp — log qua console theo pattern hiện có,
    // KHÔNG bắt buộc phải giữ span object xuyên async boundary (tránh leak nếu execute() return sớm).

    for (let waveIndex = startWave; waveIndex < waves.length; waveIndex++) {
      if (controller.signal.aborted) return

      await this.updateCurrentWave(execution.id, waveIndex)
      const wave = waves[waveIndex]

      const results = await Promise.allSettled(
        wave.map(async (step) => {
          if (startWave > 0 || execution.status === 'running') {
            const rows = await this.pool.withConnection((db) =>
              db.query(
                `SELECT status FROM orca_workflow_step_executions WHERE execution_id = ? AND step_id = ?`,
                [execution.id, step.id]
              )
            ).catch(() => null)
            const stepRecord = rows?.[0] as { status: string } | undefined
            if (stepRecord?.status === 'completed') {
              console.log(`[WorkflowOrchestrator] Skipping already-completed step ${step.id} (resume)`)
              return { exitCode: 0, data: { skippedOnResume: true } }
            }
          }
          return this.executeStep(step, execution, controller.signal, rootTraceId)
        })
      )

      // ...phần đánh giá shouldFail giữ nguyên logic hiện có, không đổi...
      let shouldFail = false
      let firstError: string | undefined
      for (let i = 0; i < results.length; i++) {
        const result = results[i]
        if (result.status === 'rejected') {
          const step = wave[i]
          if (!step.continueOnError) { shouldFail = true; firstError = result.reason instanceof Error ? result.reason.message : String(result.reason); break }
        } else if (result.value.exitCode !== 0) {
          const step = wave[i]
          if (!step.continueOnError) { shouldFail = true; firstError = `Step ${step.id} exited with code ${result.value.exitCode}`; break }
        }
      }
      if (shouldFail) {
        await this.markExecutionFailed(execution.id, firstError ?? 'Unknown error')
        return
      }
    }

    await this.markExecutionCompleted(execution.id)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    await this.markExecutionFailed(execution.id, message).catch(() => {})
  } finally {
    this.abortControllers.delete(execution.id)
  }
}

private async executeStep(
  step: WorkflowStep,
  execution: WorkflowExecution,
  signal: AbortSignal,
  rootTraceId?: string   // [NEW] param
): Promise<StepOutput> {
  const interpolatedStep = this.interpolateStep(step, execution.inputs)
  const executor = this.stepExecutors[interpolatedStep.config.type as string]
  if (!executor) throw new Error(`UNSUPPORTED_STEP_TYPE: ${interpolatedStep.config.type}`)

  // Span CON độc lập — id riêng, KHÔNG resume — chỉ mang field parentTraceId để group.
  const stepSpan = Tracers.workflowStepFlow.start({
    parentTraceId: rootTraceId, executionId: execution.id,
    stepId: step.id, stepType: interpolatedStep.config.type as string,
  })

  try {
    await this.persistStepStart(execution.id, step.id)
    // stepExecutors[type] (StepExecutors.execute() thật) cần nhận traceId để forward vào relay.call() —
    // xem §2.5. Truyền stepSpan.id qua signature hiện có bằng cách đính vào step config tạm thời
    // KHÔNG khả thi (interpolatedStep bị JSON.stringify lại) — patch StepExecutorFn signature.
    const output = await executor(interpolatedStep, execution.inputs, signal, stepSpan.id)
    await this.persistStepComplete(execution.id, step.id, output)
    stepSpan.ok({ exitCode: output.exitCode })
    return output
  } catch (err) {
    stepSpan.fail(err, { stepId: step.id })
    throw err
  }
}
```

> **Thay đổi signature `StepExecutorFn`:** để `stepSpan.id` tới được `relay.call()` bên trong `StepExecutors`, cần mở rộng type `StepExecutorFn` (`WorkflowOrchestrator.ts:32-36`) thêm tham số thứ 4 `traceId?: string`. Đây là thay đổi tối thiểu, không đổi hành vi khi `traceId` không được truyền (optional).
>
> ```typescript
> export type StepExecutorFn = (
>   step: WorkflowStep,
>   inputs: Record<string, unknown>,
>   signal: AbortSignal,
>   traceId?: string   // [NEW]
> ) => Promise<StepOutput>
> ```

Cập nhật `execute()` cha để `span.ok()`/`span.fail()` khi execution kết thúc — patch `markExecutionCompleted()`/`markExecutionFailed()` cần giữ tham chiếu tới span cha. Vì `runExecution()` chạy `void` (fire-and-forget, không giữ Promise ở `execute()`), span cha object không thể truyền xuyên qua các hàm private hiện có mà không refactor lớn — giải pháp additive-only: dùng `Map<executionId, TraceSpan>` nội bộ trong class để tra cứu:

```typescript
export class WorkflowOrchestrator {
  private readonly abortControllers = new Map<string, AbortController>()
  private readonly rootSpans = new Map<string, TraceSpan>()   // [NEW] executionId → span cha workflow:execute

  async execute(...): Promise<WorkflowExecution> {
    const id = randomUUID()
    const span = Tracers.workflowExecuteFlow.start({ executionId: id, projectId, triggeredBy })
    this.rootSpans.set(id, span)
    // ...
  }

  private async markExecutionCompleted(executionId: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_workflow_executions SET status = 'completed', completed_at = ? WHERE id = ?`, [Date.now(), executionId])
    )
    const span = this.rootSpans.get(executionId)
    if (span) { span.ok({ status: 'completed' }); this.rootSpans.delete(executionId) }
  }

  private async markExecutionFailed(executionId: string, errorMessage: string): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(`UPDATE orca_workflow_executions SET status = 'failed', completed_at = ?, error_message = ? WHERE id = ?`, [Date.now(), errorMessage, executionId])
    )
    const span = this.rootSpans.get(executionId)
    if (span) { span.fail(errorMessage, { status: 'failed' }); this.rootSpans.delete(executionId) }
  }
}
```

> **Lưu ý cancel():** `cancel(executionId)` (dòng 199-212) UPDATE status → `'cancelled'` trực tiếp, không qua `markExecutionFailed`/`markExecutionCompleted` — cần thêm `span.fail('EXECUTION_CANCELLED')` + `rootSpans.delete()` tương tự để tránh leak entry trong `rootSpans` map.

### 2.5 `src/main/workflow/StepExecutors.ts` — forward `traceId` vào relay.call()

```typescript
async execute(
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string   // [NEW]
): Promise<StepOutput> {
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  const timeoutMs = step.timeout ?? DEFAULT_TIMEOUT_MS
  return Promise.race([
    this.executeByType(step, inputs, signal, traceId),
    new Promise<never>((_, reject) => {
      const timer = setTimeout(() => reject(new Error(`STEP_TIMEOUT: step "${step.id}" exceeded ${timeoutMs}ms`)), timeoutMs)
      signal.addEventListener('abort', () => clearTimeout(timer), { once: true })
    }),
  ])
}

private async executeByType(step: WorkflowStep, inputs: Record<string, unknown>, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const { type } = step.config
  switch (type) {
    case 'agent':        return this.executeAgent(step, signal, traceId)
    case 'shell':         return this.executeShell(step, signal, traceId)
    case 'webhook':       return this.executeWebhook(step, signal)       // không qua relay — không cần traceId
    case 'notification':  return this.executeNotification(step, signal, traceId)
    case 'condition':     return this.executeCondition(step, inputs)     // sync, không I/O
    default: throw new Error(`UNSUPPORTED_STEP_TYPE: "${String(type)}"`)
  }
}

private async executeAgent(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  const result = (await relay.call('agent.exec', {
    stepId: step.id,
    prompt: step.config['prompt'],
    worktreePath: step.config['worktreePath'],
    trustPreset: step.config['trustPreset'] ?? 'default',
    traceId,   // [NEW] — relay:agentCall (dev-server-relay-bridge.ts:21) resume theo id này
  })) as { exitCode?: number; stdout?: string; stderr?: string }
  return { exitCode: result.exitCode ?? 0, stdout: result.stdout, stderr: result.stderr }
}

private async executeShell(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  const result = (await relay.call('shell.exec', {
    script: step.config['script'], env: step.config['env'] ?? {}, traceId,   // [NEW]
  })) as { exitCode?: number; stdout?: string; stderr?: string }
  return { exitCode: result.exitCode ?? 0, stdout: result.stdout, stderr: result.stderr }
}

private async executeNotification(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
  const relay = await this.getRelay(step)
  if (signal.aborted) throw new Error('EXECUTION_CANCELLED')
  await relay.call('notification.send', {
    channel: step.config['channel'], message: step.config['message'], traceId,   // [NEW]
  })
  return { exitCode: 0 }
}
```

### 2.6 `src/main/workflow/TemplateResolver.ts` — BL-WF-01

```typescript
import { Tracers } from '../../shared/trace/tracers'

async create(params: CreateTemplateParams): Promise<string> {
  const span = Tracers.workflowTemplateCreateFlow.start({
    name: params.name, scope: params.scope ?? 'user', hasParent: !!params.parentTemplateId,
  })
  try {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_workflow_templates (id, name, definition_json, owner_id, scope, parent_template_id, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [id, params.name, JSON.stringify(params.definition), params.ownerId, params.scope ?? 'user', params.parentTemplateId ?? null, now, now]
      )
    )
    span.ok({ templateId: id })
    return id
  } catch (err) {
    span.fail(err)
    throw err
  }
}

// resolve() (dòng 72) — KHÔNG có tracer: đây là read-path gọi mỗi lần cần resolve inheritance
// chain (kể cả trong workflow.execute), không phải write path BL-WF-01. Nếu cần đo latency
// inheritance-chain-walk sâu (MAX_INHERIT_DEPTH=5), field này nên gộp vào step('resolve-parent')
// bên trong span cha của caller (workflow:execute khi resolve() được gọi từ đó), không tự tạo
// tracer riêng — tránh vi phạm "1 tracer = 1 sub-flow" khi resolve() không phải là 1 sub-flow độc lập.
```

### 2.7 `src/main/workflow/workflow-rpc-handler.ts` — forward traceId + trả về cho FE

```typescript
const ExecuteParam = z.object({
  definition: WorkflowDefinitionSchema,
  inputs: z.record(z.unknown()).optional(),
  projectId: z.string().optional(),
  traceId: z.string().optional(),   // [NEW]
})
```

```typescript
defineMethod({
  name: 'workflow.execute',
  params: ExecuteParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId ?? 'system'
    // NOTE: orchestrator.execute() hiện chưa nhận resume param — cần overload tương tự §2.5
    // của SOL-BE-TRACE-016 (thêm traceId? cuối signature) nếu muốn FE-initiated traceId resume
    // vào workflowExecuteFlow. Việc này để lại cho patch nhỏ bổ sung khi core resume API ổn định,
    // không chặn phần còn lại của solution (rootTraceId nội bộ vẫn hoạt động độc lập).
    const execution = await orchestrator.execute(
      params.definition as Parameters<typeof orchestrator.execute>[0],
      params.inputs ?? {}, userId, params.projectId,
    )
    // Trả traceId để FE có thể filter TracePanel theo execution — đọc lại rootTraceId đã persist
    const row = await pool.withConnection((db) =>
      db.query<{ rootTraceId: string | null }>(
        `SELECT root_trace_id as rootTraceId FROM orca_workflow_executions WHERE id = ?`, [execution.id]
      )
    )
    return { executionId: execution.id, status: execution.status, traceId: row[0]?.rootTraceId ?? undefined }
  },
}),
```

---

## 3. Test Plan (Vitest)

| Test file | Test case | Verify |
|-----------|-----------|--------|
| `src/main/workflow/__tests__/WorkflowOrchestrator.test.ts` | `execute() tạo span workflow:execute + persist rootTraceId vào DB` | assert `root_trace_id` column == `span.id` |
| | `runExecution() 2-wave, mỗi step trong cả 2 wave đều có field parentTraceId === rootTraceId` | assert MỌI `workflow:stepExecute` event có cùng `parentTraceId` |
| | `mỗi step có span.id RIÊNG (khác nhau giữa các step), dù cùng parentTraceId` | assert `id` unique set |
| | `resumeRunningExecutions() đọc lại root_trace_id từ DB → span cha mới có CÙNG id` | mock DB row có `root_trace_id`, assert `resume: { id }` được truyền |
| | `markExecutionCompleted() → span cha ok({ status: 'completed' }); rootSpans map được dọn` | |
| | `markExecutionFailed() → span cha fail(); rootSpans map được dọn (không leak)` | |
| `src/main/workflow/__tests__/StepExecutors.test.ts` | `executeAgent() forward traceId vào relay.call('agent.exec', { ..., traceId })` | mock relay, assert params |
| | `executeShell()/executeNotification() tương tự` | |
| | `executeWebhook()/executeCondition() KHÔNG nhận traceId param (không qua relay)` | type-check + runtime assert không lỗi khi omit |
| `src/main/workflow/__tests__/TemplateResolver.test.ts` | `create() với parentTemplateId → span field hasParent: true` | |
| | `create() không có parent → hasParent: false` | |
| `src/main/db/migrations/__tests__/0013_workflow_trace_correlation.test.ts` | `up() thêm cột root_trace_id, không lỗi khi cột đã null cho execution cũ` | |

**Test Targets:**

| Module | Target tests |
|--------|--------------|
| WorkflowOrchestrator tracing (bao gồm parentTraceId + resume) | ≥ 8 |
| StepExecutors tracing | ≥ 5 |
| TemplateResolver tracing | ≥ 2 |
| Migration 0013 | ≥ 2 |
| **Total** | **≥ 17** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.workflowExecuteFlow` bao phủ trọn vòng đời execution: `start` → (implicit `build-waves` qua DAGBuilder) → `ok`/`fail` khi `markExecutionCompleted`/`markExecutionFailed` chạy
- [ ] `Tracers.workflowStepFlow` tạo 1 span độc lập (id riêng) cho MỖI step, luôn mang field `parentTraceId` trỏ đúng về `id` của span cha `workflow:execute` tương ứng
- [ ] `parentTraceId` nhất quán trên MỌI step span trong cùng 1 execution, **kể cả sau khi `resumeRunningExecutions()` chạy lại sau restart** — verify bằng test giả lập restart (tạo execution, kill in-memory state, gọi lại `resumeRunningExecutions()`, so `parentTraceId` step mới với step cũ)
- [ ] Cột `root_trace_id` trong `orca_workflow_executions` (migration 0013) được ghi ngay khi `execute()` persist execution lần đầu, không phải patch sau
- [ ] `traceId` forward đúng vào `relay.call()` params ở cả 3 loại step dùng relay (`agent`/`shell`/`notification`) — KHÔNG forward cho `webhook`/`condition` (không qua relay)
- [ ] `Tracers.workflowTemplateCreateFlow` phân biệt template tạo mới vs kế thừa qua field `hasParent`
- [ ] `Tracers.workflowShareFlow` được khai báo trong `tracers.ts` nhưng KHÔNG có bất kỳ call site nào trong code — verify bằng `grep -r "workflowShareFlow" src/main/workflow/` chỉ match chính định nghĩa tracer
- [ ] Không thêm `span.step()` cho các UPDATE đơn dòng (`updateCurrentWave`, `persistStepStart`, `persistStepComplete`) — các UPDATE này gộp vào fields của `ok()`/`fail()` step tương ứng
- [ ] `rootSpans` map trong `WorkflowOrchestrator` không leak entry — mọi `executionId` được set phải được `delete()` ở đúng 1 trong 3 nhánh kết thúc (`completed`/`failed`/`cancelled`)
