# TASK-BE-017.2: Đăng ký 4 tracer `workflow:*` + thiết kế `parentTraceId` trong `WorkflowOrchestrator.ts` (BL-WF-02)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-017](../solutions/SOL-BE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-017.1 (cần cột `root_trace_id` để persist)
**Status:** ✅ Done (2026-08-04) — 4 tracers added additively to `tracers.ts` (`workflowTemplateCreateFlow`/`workflowExecuteFlow`/`workflowStepFlow`/`workflowShareFlow`, no rename of any existing entry). `WorkflowOrchestrator.ts` implemented per spec: `rootSpans` map, `execute()` creates the parent span and persists `rootTraceId` on first insert, `resumeRunningExecutions()` re-reads `root_trace_id` and resumes the same span id, `runExecution()`/`executeStep()` thread `rootTraceId` down to a per-step `workflowStepFlow` span carrying `parentTraceId`, `StepExecutorFn` gained the 4th `traceId?` param, and `markExecutionCompleted`/`markExecutionFailed`/`cancel()` all close+delete the `rootSpans` entry (3 terminal transitions, no leak). DRIFT vs the doc's code sample: real `IDatabase.query()` has no generic type parameter (`query(sql, params?): Promise<Record<string,unknown>[]>`), so DB reads use plain `db.query(...)` + an `as` cast instead of `db.query<T>(...)` (matches the pre-existing in-file convention, e.g. the wave-resume SELECT). `pnpm tsc --noEmit` and `WorkflowOrchestrator.test.ts` (28/28, 10 new tracing tests) pass; remaining tsc errors in this file/area are pre-existing (`db.query<T>` generic misuse elsewhere in the file, unused `router` field) and unrelated to this task.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "WorkflowOrchestrator.execute"
codegraph explore "WorkflowOrchestrator.runExecution"
codegraph explore "WorkflowOrchestrator.executeStep"
codegraph explore "WorkflowOrchestrator.resumeRunningExecutions"
```

Cả 4 là method đã tồn tại (MODIFY case) — đây là phần kiến trúc quan trọng nhất của toàn bộ backend tracing (thiết kế `parentTraceId`). Chạy:

```
gitnexus_impact({ target: "WorkflowOrchestrator.execute", direction: "upstream" })
gitnexus_impact({ target: "WorkflowOrchestrator.executeStep", direction: "upstream" })
```

(Phần đăng ký 4 tracer mới vào `Tracers` là additive-only — chỉ cần `codegraph explore "Tracers"`.) Đọc kỹ báo cáo blast radius — đặc biệt `StepExecutorFn` type mở rộng thêm tham số thứ 4 ảnh hưởng tới mọi implementation trong `StepExecutors.ts` (TASK-BE-017.3). Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Đây là phần kiến trúc quan trọng nhất của toàn bộ backend tracing (xem `00-index.md` §4). Khai báo 4 tracer `workflow:*`, sau đó implement mô hình **`parentTraceId`** — một field NGHIỆP VỤ (nằm trong `TraceFields`, giống `projectId`/`accountId`), KHÔNG PHẢI cơ chế `resume`/`traceId` giao thức của CR-TRACE-000 §3.1. Lý do: 1 workflow execution có N wave × M step song song, mỗi step là 1 network hop độc lập cần `id` riêng để TracePanel hiển thị đúng latency/status riêng — không thể dùng `resume` (vốn chỉ giữ nguyên `id` xuyên nhiều layer của CÙNG 1 hop) để nhóm N span độc lập.

| Cơ chế | Dùng để | Phạm vi |
|--------|---------|---------|
| `resume: { id }` (CR-TRACE-000 §3.1) | Giữ nguyên `span.id` khi CÙNG một hop đi qua nhiều layer | 1 span, nhiều layer |
| `traceId` trong wire envelope (CR-TRACE-000 §3.2-3.3) | Truyền span id qua transport để layer sau `resume` đúng | Giữa 2 layer liền kề |
| `parentTraceId` (field nghiệp vụ, **mới trong CR-TRACE-017**) | Nhóm N span ĐỘC LẬP (N step, mỗi step 1 `id` riêng) dưới 1 execution để TracePanel filter theo execution mà không cần join DB | 1-nhiều span, không đổi `id` của span con |

`rootTraceId` (= `id` của span `workflow:execute`) PHẢI được persist vào cột `root_trace_id` (TASK-BE-017.1) ngay khi `execute()` persist execution lần đầu — để `resumeRunningExecutions()` có thể tái tạo lại span cha bằng CÙNG `id` (qua `resume: { id }`) sau khi Orca Server restart giữa chừng execution, tránh gãy khả năng nhóm step cũ + step mới dưới cùng 1 `parentTraceId` trên TracePanel.

## File: `src/shared/trace/tracers.ts` [MODIFY]

```typescript
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (profile:*, aiProvider:* từ TASK-BE-015.1/016.1)...

  // ── Workflow Orchestration (CR-TRACE-017) ────────────────────────────────────
  /** BL-WF-01: template create/inherit */
  workflowTemplateCreateFlow: createTracer('workflow:templateCreate'),
  /** BL-WF-02: span CHA — 1 per execution, sống suốt vòng đời execution */
  workflowExecuteFlow:        createTracer('workflow:execute'),
  /** BL-WF-02: span CON — 1 per step, mang field parentTraceId để group theo execution */
  workflowStepFlow:           createTracer('workflow:stepExecute'),
  /** BL-WF-03: PLACEHOLDER — chưa có implementation, TemplateResolver.ts không có
   *  updateVisibility()/share-token/shared route nào trong code hiện tại. Khai báo tên
   *  tracer để sẵn sàng khi tính năng sharing tồn tại, KHÔNG viết call site nào cho nó. */
  workflowShareFlow:          createTracer('workflow:share'),
} as const
```

## File: `src/main/workflow/WorkflowOrchestrator.ts` [MODIFY]

**1. Thêm field `rootSpans` map + patch `execute()`:**

```typescript
import { Tracers } from '../../shared/trace/tracers'
import type { TraceSpan } from '../../shared/trace'

export class WorkflowOrchestrator {
  private readonly abortControllers = new Map<string, AbortController>()
  private readonly rootSpans = new Map<string, TraceSpan>()   // [NEW] executionId → span cha workflow:execute

  async execute(
    definition: WorkflowDefinition,
    inputs: Record<string, unknown>,
    triggeredBy: string,
    projectId?: string
  ): Promise<WorkflowExecution> {
    const id = randomUUID()
    const now = Date.now()

    const span = Tracers.workflowExecuteFlow.start({ executionId: id, projectId, triggeredBy })
    this.rootSpans.set(id, span)

    await this.persistExecution({ id, definition, inputs, triggeredBy, projectId, now, rootTraceId: span.id })

    const execution: WorkflowExecution = {
      id, definition, status: 'pending', inputs, currentWave: 0, triggeredBy, projectId,
      createdAt: new Date(now),
    }

    // rootTraceId = span.id truyền xuống runExecution() để mọi step span mang parentTraceId đúng
    void this.runExecution(execution, 0, span.id)

    return execution
  }
```

**2. `resumeRunningExecutions()` — đọc lại `root_trace_id` từ DB, tái tạo span cha bằng `resume`:**

```typescript
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
```

**3. `runExecution()` nhận thêm param `rootTraceId` (dùng làm `parentTraceId` của mọi step span):**

```typescript
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
      // — không bắt buộc phải giữ span object xuyên async boundary (tránh leak nếu execute() return sớm).

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
```

**4. `executeStep()` nhận thêm param `rootTraceId`, tạo span CON độc lập mang `parentTraceId`:**

```typescript
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
      // stepExecutors[type] (StepExecutors.execute() thật, TASK-BE-017.3) cần nhận traceId để
      // forward vào relay.call() — patch StepExecutorFn signature bên dưới.
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

**5. Mở rộng `StepExecutorFn` (type định nghĩa tại `WorkflowOrchestrator.ts:32-36`) thêm tham số thứ 4:**

```typescript
export type StepExecutorFn = (
  step: WorkflowStep,
  inputs: Record<string, unknown>,
  signal: AbortSignal,
  traceId?: string   // [NEW]
) => Promise<StepOutput>
```

**6. `markExecutionCompleted()`/`markExecutionFailed()` — đóng span cha, dọn `rootSpans` map:**

```typescript
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
```

> **Lưu ý `cancel()`:** `cancel(executionId)` (dòng 199-212) UPDATE status → `'cancelled'` trực tiếp, không qua `markExecutionFailed`/`markExecutionCompleted` — cần thêm `span.fail('EXECUTION_CANCELLED')` + `rootSpans.delete()` tương tự để tránh leak entry trong `rootSpans` map. Đây là nhánh kết thúc thứ 3 (cùng với `completed`/`failed`) mà mọi `executionId` được `set()` phải được `delete()` đúng 1 lần.

## Verification

```bash
pnpm tsc --noEmit
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] 4 tracer `workflowTemplateCreateFlow`/`workflowExecuteFlow`/`workflowStepFlow`/`workflowShareFlow` tồn tại trong `tracers.ts` với đúng flow name
- [ ] `Tracers.workflowExecuteFlow` bao phủ trọn vòng đời execution: `start` → (implicit `build-waves` qua DAGBuilder, không tự tạo span) → `ok`/`fail` khi `markExecutionCompleted`/`markExecutionFailed` chạy
- [ ] `Tracers.workflowStepFlow` tạo 1 span độc lập (id riêng) cho MỖI step, luôn mang field `parentTraceId` trỏ đúng về `id` của span cha `workflow:execute` tương ứng
- [ ] Cột `root_trace_id` (migration TASK-BE-017.1) được ghi ngay khi `execute()` persist execution lần đầu, không phải patch sau
- [ ] `resumeRunningExecutions()` đọc lại `root_trace_id` từ DB → span cha mới tái tạo với CÙNG `id` qua `resume: { id: rootTraceId }`
- [ ] `rootSpans` map không leak entry — mọi `executionId` được `set()` phải được `delete()` ở đúng 1 trong 3 nhánh kết thúc (`completed`/`failed`/`cancelled`)
- [ ] `StepExecutorFn` type có thêm tham số thứ 4 `traceId?: string`, optional, không đổi hành vi khi không truyền
- [ ] Không thêm `span.step()` cho các UPDATE đơn dòng (`updateCurrentWave`, `persistStepStart`, `persistStepComplete`) — gộp vào fields của `ok()`/`fail()` step tương ứng
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
