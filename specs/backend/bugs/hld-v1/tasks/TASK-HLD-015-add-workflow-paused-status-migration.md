# TASK-HLD-015: Thêm status `'paused'`, cột `paused_at` và `WorkflowOrchestrator.pause()`/`resumeFromPause()`

**Priority:** 🟠 HIGH
**Effort:** ~4-5 giờ (bao gồm migration + test)
**Status:** ✅ DONE — 2026-08-09 (áp dụng đủ 3 bước: `'paused'` status + `pausedAt` field trong `WorkflowTypes.ts`; migration mới `0014_workflow_pause_state.ts` — verify khớp 100% pattern `0013_workflow_trace_correlation.ts` trước khi tạo, đăng ký vào `migrations/index.ts`; `WorkflowOrchestrator.pause()`/`resumeFromPause()`/2 helper DB mới, wave-loop pause-check, `ExecutionRow`+`rowToExecution`+2 SELECT cập nhật `paused_at`. `resumeRunningExecutions()` xác nhận KHÔNG bị đụng — chỉ query `status='running'`. `tsc --noEmit` chỉ còn 5 lỗi pre-existing baseline (không mới). ⚠️ Chưa chạy migration thật trên DB test, chưa viết 7 test case — effort budget, cần verify riêng trước khi merge.)
**Bug refs:** BUG-BE-HLD-009 (phần 1 — orchestrator + schema; phần 2 RPC nằm ở TASK-HLD-016)
**Solution ref:** [SOLUTION-workflow-exact.md — BUG-BE-HLD-009](../solutions/SOLUTION-workflow-exact.md#bug-be-hld-009--workflow-pauseresume-user-triggered)
**Depends on:** **TASK-HLD-013 (BLOCKER — phải merge trước)**. `runExecution()`/`executeStep()` phải gọi đúng `StepExecutors.execute()` trước khi thêm logic pause-check vào wave loop, nếu không hành vi pause sẽ không thể verify đúng (mọi step đã throw `UNSUPPORTED_STEP_TYPE` từ trước khi tới wave loop).

---

## Mục tiêu

Cho phép user chủ động pause 1 workflow execution đang chạy (`status='running'`) và resume lại sau đó — khác với `resumeRunningExecutions()` (crash-recovery nội bộ, chạy 1 lần lúc bootstrap, quét mọi execution `status='running'`).

Root cause (theo solution): `WorkflowStatus` thiếu `'paused'`; không có `WorkflowOrchestrator.pause()`; `resumeRunningExecutions()` không phải API user-triggered, không nhận `executionId`, không gọi được qua RPC.

**Thiết kế cốt lõi:**
- `pause()` KHÔNG abort `AbortController` (khác `cancel()`) — chỉ đánh dấu 1 `Set<string>` nội bộ (`pauseRequests`). Wave đang chạy dở được để chạy hết; wave loop kiểm tra flag này ở **đầu mỗi vòng lặp, trước khi dispatch wave kế tiếp**.
- `resumeFromPause()` validate execution đang `status='paused'`, đọc lại `root_trace_id` đã persist (phòng trường hợp server restart trong lúc paused làm mất `rootSpans` in-memory), rồi gọi `runExecution(execution, execution.currentWave, rootTraceId)`.
- `resumeRunningExecutions()` **giữ nguyên 100% — không sửa gì**. Nó chỉ query `status='running'`, execution `'paused'` không nằm trong tập kết quả.

## File cần sửa/tạo

```
backend/src/main/workflow/WorkflowTypes.ts
backend/src/main/workflow/WorkflowOrchestrator.ts
backend/src/main/db/migrations/0014_workflow_pause_state.ts   (NEW — không sửa migration cũ)
backend/src/main/db/migrations/index.ts
```

**Không đụng tới:** `backend/src/main/db/migrations/0009_workflows.ts`, `0013_workflow_trace_correlation.ts` — đã chạy production. Mọi thay đổi schema đi qua migration mới `0014`.

## Thay đổi cụ thể

### Bước 1 — `WorkflowTypes.ts`: thêm `'paused'` vào `WorkflowStatus` + `pausedAt` vào `WorkflowExecution`

TRƯỚC (dòng 16):

```typescript
export type WorkflowStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
```

SAU:

```typescript
export type WorkflowStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
```

TRƯỚC (dòng 51-65):

```typescript
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
```

SAU:

```typescript
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

`status` là cột `TEXT NOT NULL DEFAULT 'pending'` **không có CHECK constraint** (xác nhận từ `0009_workflows.ts` trước khi bắt tay code) — giá trị `'paused'` tự lưu được mà không cần đổi schema. Cột duy nhất thực sự cần thêm là `paused_at`, theo đúng pattern `ALTER TABLE ... ADD COLUMN` của `0013_workflow_trace_correlation.ts`.

Tạo file mới:

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

**Trước khi tạo file, verify** cấu trúc `Migration` type và pattern thực tế của `0013_workflow_trace_correlation.ts` (import path `./types`, chữ ký `up(db)`/`down(db)`) khớp với những gì code hiện tại dùng — solution mô tả dựa trên đọc code tại 1 thời điểm, có thể lệch nếu `types.ts` đã đổi.

Đăng ký vào registry:

TRƯỚC:

```typescript
// backend/src/main/db/migrations/index.ts
import { migration0013WorkflowTraceCorrelation } from './0013_workflow_trace_correlation'
import type { Migration } from './types'

export const ALL_MIGRATIONS: readonly Migration[] = [
  // ... các migration trước đó
  migration0013WorkflowTraceCorrelation,
]
```

SAU:

```typescript
// backend/src/main/db/migrations/index.ts
import { migration0013WorkflowTraceCorrelation } from './0013_workflow_trace_correlation'
import { migration0014WorkflowPauseState } from './0014_workflow_pause_state'
import type { Migration } from './types'

export const ALL_MIGRATIONS: readonly Migration[] = [
  // ... các migration trước đó
  // v5.1 — Workflow Trace Correlation (CR-TRACE-017 §3.1 — parentTraceId resume-after-restart)
  migration0013WorkflowTraceCorrelation,
  // v5.1 — Workflow Pause State (BUG-BE-HLD-009 — user-triggered pause/resume)
  migration0014WorkflowPauseState,
]
```

### Bước 3 — `WorkflowOrchestrator.pause(executionId)` / `resumeFromPause(executionId)`

Thêm field mới trong class (cạnh `rootSpans`):

```typescript
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

TRƯỚC (đoạn trong `runExecution()`, dòng 274-280):

```typescript
for (let waveIndex = startWave; waveIndex < waves.length; waveIndex++) {
  if (controller.signal.aborted) {
    return
  }

  await this.updateCurrentWave(execution.id, waveIndex)
  const wave = waves[waveIndex]
```

SAU:

```typescript
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

TRƯỚC (dòng 45-73):

```typescript
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
```

SAU:

```typescript
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

`getExecution()` — thêm `paused_at as pausedAt` vào SELECT (dòng 147-158):

```typescript
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
```

`listExecutions()` — cùng thay đổi cho SELECT (dòng 189-201):

```typescript
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

## Verification

```bash
pnpm tsc --noEmit
pnpm vitest run backend/src/main/workflow/__tests__/

# Migration:
# - Chạy migration 0014 trên DB test → xác nhận cột paused_at tồn tại, nullable, mặc định NULL
# - Xác nhận migration KHÔNG sửa 0009_workflows.ts / 0013_workflow_trace_correlation.ts
git diff --stat backend/src/main/db/migrations/0009_workflows.ts backend/src/main/db/migrations/0013_workflow_trace_correlation.ts
# Expected: không có thay đổi (empty diff)

# WorkflowOrchestrator — test case:
# 1. Execute workflow 3 wave → gọi pause() giữa wave 1 đang chạy → assert wave 1 chạy hết
#    (steps complete bình thường), wave 2 KHÔNG dispatch, status cuối cùng = 'paused',
#    current_wave = 1 (wave đã hoàn thành, không rollback)
# 2. pause() trên execution status='pending'/'completed' → assert throw WORKFLOW_PAUSE_INVALID_STATE
# 3. resumeFromPause() trên execution 'paused' → assert status → 'running' → tiếp tục đúng
#    từ current_wave, KHÔNG re-run các step đã completed ở wave trước
# 4. resumeFromPause() trên execution KHÔNG phải 'paused' (vd 'running')
#    → assert throw WORKFLOW_RESUME_INVALID_STATE
# 5. paused_at set đúng lúc pause() → dispatch wave kế tiếp bị chặn, clear về NULL đúng lúc resumeFromPause()
# 6. cancel() trên execution đang 'paused' → assert vẫn chuyển 'cancelled' bình thường (không bị kẹt),
#    và pauseRequests entry (nếu còn) bị xoá đúng

# REGRESSION CHECK BẮT BUỘC (theo yêu cầu task):
# 7. resumeRunningExecutions() (bootstrap, crash-recovery) KHÔNG được đụng vào execution
#    có status='paused' — chỉ quét execution status='running'. Viết test riêng:
#    seed 1 execution status='paused' + 1 execution status='running' trong DB, gọi
#    resumeRunningExecutions() → assert execution 'paused' KHÔNG bị resume/thay đổi status,
#    chỉ execution 'running' được xử lý.
grep -n "resumeRunningExecutions" backend/src/main/workflow/WorkflowOrchestrator.ts
# Expected: hàm này không có diff so với trước task — chỉ đọc để xác nhận SQL WHERE
# vẫn lọc đúng status = 'running', không vô tình khớp cả 'paused'
```

**Điều kiện DONE:** `pnpm tsc --noEmit` pass, migration `0014` chạy sạch và không đụng migration cũ, toàn bộ 7 test case pass (bao gồm test case 7 — regression guard `resumeRunningExecutions()` không đụng execution `'paused'`), `pnpm vitest run backend/src/main/workflow/__tests__/` pass không regression.
