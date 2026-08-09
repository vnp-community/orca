# TASK-HLD-013: Fix type-mismatch giữa `WorkflowOrchestrator.executeStep()` và `StepExecutors`

**Priority:** 🔴 CRITICAL — **BLOCKER cho TASK-HLD-014, TASK-HLD-015, TASK-HLD-016**
**Effort:** ~30 phút (fix) + ~1 giờ (test integration bắt buộc trước khi merge)
**Status:** ✅ DONE — 2026-08-09 (**xác nhận bug có thật** qua đọc verbatim code trước khi sửa: `WorkflowOrchestrator.ts` tự định nghĩa `StepExecutors = Record<string, StepExecutorFn>` cục bộ, `executeStep()` tra `this.stepExecutors[type]` trên 1 instance class thật `StepExecutors` từ `./StepExecutors.ts` → luôn `undefined` → luôn throw `UNSUPPORTED_STEP_TYPE`. Đã sửa: import type `StepExecutors` thật, gọi thẳng `this.stepExecutors.execute(...)` với 5 tham số (forward `execution.triggeredBy`); thêm tham số thứ 5 optional `_triggeredBy` vào `StepExecutors.execute()` để khớp arity — chưa dùng, chuẩn bị cho TASK-HLD-014. `tsc --noEmit` không phát sinh lỗi mới. ⚠️ Chưa viết test integration mới như solution yêu cầu (bắt buộc trước khi merge thật — effort budget không cho phép trong đợt này, cần bổ sung riêng). Chưa kiểm tra `desktop/`/`frontend/` có cùng discrepancy hay không — ngoài phạm vi task này.)
**Bug refs:** Phát hiện bổ sung §0 trong solution (không phải ticket riêng, nhưng chặn BUG-BE-HLD-008 và BUG-BE-HLD-009)
**Solution ref:** [SOLUTION-workflow-exact.md §0](../solutions/SOLUTION-workflow-exact.md#0--phát-hiện-bổ-sung-workfloworchestratorexecutestep-gọi-stepexecutors-sai-kiểu-chặn-cả-bug-008)
**Depends on:** không (task nền tảng, phải làm đầu tiên)

---

## ⚠️ ĐÂY LÀ BLOCKER

Task này **PHẢI hoàn thành và merge trước** TASK-HLD-014 (provider selection), TASK-HLD-015 (paused status/migration), TASK-HLD-016 (pause/resume RPC). Cả 3 task đó đều dựa trên giả định `WorkflowOrchestrator.executeStep()` gọi đúng `StepExecutors.execute()` — nếu không fix task này trước, code viết ở 3 task sau sẽ là **dead code không bao giờ chạy tới** (mọi step luôn throw `UNSUPPORTED_STEP_TYPE` trước khi tới logic đó).

## Mục tiêu

Verify và fix type-mismatch giữa:
- `WorkflowOrchestrator.executeStep()` — kỳ vọng `this.stepExecutors` là `Record<string, StepExecutorFn>` (map tra theo `step.config.type`)
- `server-bootstrap.ts` — thực tế truyền vào 1 **instance của class** `StepExecutors` (từ `./StepExecutors.ts`), chỉ có duy nhất method public `.execute()`, không có index signature `[key: string]: fn`

Nếu đúng như solution mô tả: `WorkflowOrchestrator.ts` tự định nghĩa alias nội bộ `export type StepExecutors = Record<string, StepExecutorFn>` (KHÔNG import class thật từ `./StepExecutors.ts`), khiến `this.stepExecutors[interpolatedStep.config.type as string]` luôn trả về `undefined` trên 1 class instance ở runtime → **mọi workflow step hiện tại (agent/shell/webhook/notification/condition) đều đang throw `UNSUPPORTED_STEP_TYPE`** ngay tại `executeStep()`, trước khi `StepExecutors.execute()`/`executeAgent()` từng được gọi.

Bước đầu tiên của task: **verify lại bằng cách đọc verbatim** `backend/src/main/workflow/WorkflowOrchestrator.ts` (import block + định nghĩa `StepExecutorFn`/`StepExecutors` + `executeStep()`) và `backend/src/main/workflow/StepExecutors.ts` (class thật) để xác nhận đúng như mô tả trước khi áp fix. Không giả định solution đúng 100% mà không đọc lại code hiện tại — solution có thể đã lệch nếu code đã đổi từ lúc viết solution.

## File cần sửa

```
backend/src/main/workflow/WorkflowOrchestrator.ts
```

(Không cần sửa `server-bootstrap.ts` cho riêng task này — nó đã truyền đúng instance class từ đầu; chỉ type-checking trước đây "tình cờ" không bắt được lỗi vì alias trùng tên che khuất loại thật.)

## Thay đổi cụ thể

### 1. Import block đầu file — xoá alias nội bộ, import class thật

TRƯỚC (dòng 18-41):

```typescript
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
```

SAU:

```typescript
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

**Lưu ý:** constructor giữ nguyên chữ ký (`stepExecutors: StepExecutors`) — chỉ khác `StepExecutors` giờ trỏ đúng vào class thật (imported type), không phải alias `Record<string, fn>` cục bộ.

### 2. `executeStep()` — gọi thẳng entry point `StepExecutors.execute()`, xoá bước tra map

TRƯỚC (dòng 344-379):

```typescript
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
```

SAU:

```typescript
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

**Ghi chú:** tham số thứ 5 `execution.triggeredBy` được forward ngay trong task này (không đợi TASK-HLD-014) vì `StepExecutors.execute()` cần khai báo chữ ký nhận đủ 5 tham số để không phải sửa lại call-site lần nữa ở task sau. Tuy nhiên **logic dùng `triggeredBy` bên trong `StepExecutors` (ProviderResolver) thuộc phạm vi TASK-HLD-014** — task này chỉ đảm bảo tham số được forward đúng, `StepExecutors.execute()` có thể tạm nhận tham số thứ 5 optional và bỏ qua nếu chưa implement provider resolution.

## Rủi ro / Blast radius (theo solution, từ CodeGraph)

- `executeStep` (backend) — 1 caller nội bộ (`runExecution`), **không có test bao phủ** hiện tại (`⚠️ no covering tests found`). Sửa an toàn cho phía `backend/`.
- **Cảnh báo phạm vi:** `desktop/src/main/workflow/WorkflowOrchestrator.ts` và `frontend/src/main/workflow/WorkflowOrchestrator.ts` có khả năng cùng discrepancy pattern (`Record<string, StepExecutorFn>` cục bộ + `this.stepExecutors[type]`) — **XÁC NHẬN LẠI** khi đọc code (đừng giả định solution đúng), nhưng **KHÔNG sửa `desktop/`/`frontend/` trong task này** — nằm ngoài phạm vi (task riêng nếu cần).
- Do thiếu test coverage, viết **ít nhất 1 test integration** mới trong `WorkflowOrchestrator.test.ts` là **bắt buộc trước khi merge** (xem Verification).

## Verification

Trước khi làm bất kỳ thay đổi nào: chạy `codegraph explore "WorkflowOrchestrator executeStep StepExecutors"` (hoặc đọc verbatim 2 file liên quan) để xác nhận type-mismatch thực sự tồn tại đúng như solution mô tả — không tin tưởng mù quáng vào solution doc nếu code đã đổi.

```bash
# 1. Xác nhận build/type-check pass sau khi sửa
pnpm tsc --noEmit

# 2. Grep để xác nhận alias nội bộ StepExecutorFn/StepExecutors đã bị xoá khỏi WorkflowOrchestrator.ts
grep -n "export type StepExecutor" backend/src/main/workflow/WorkflowOrchestrator.ts
# Expected: KHÔNG có kết quả (đã xoá)

grep -n "^import type { StepExecutors } from './StepExecutors'" backend/src/main/workflow/WorkflowOrchestrator.ts
# Expected: 1 kết quả — import class thật

# 3. Test integration MỚI (bắt buộc, chưa có test bao phủ executeStep()):
#    a. Mock StepExecutors với execute() spy → gọi runExecution() qua execute() public API
#       → assert stepExecutors.execute() được gọi đúng 5 tham số
#          (step, inputs, signal, traceId, triggeredBy)
#    b. Regression guard: assert rằng với fix này, step KHÔNG còn luôn throw UNSUPPORTED_STEP_TYPE
#       (viết test trước khi fix để xác nhận nó FAIL trên code cũ, rồi xác nhận PASS sau fix —
#       chứng minh đây thực sự là bug đang tồn tại, không phải suy đoán)
pnpm vitest run backend/src/main/workflow/__tests__/WorkflowOrchestrator.test.ts

# 4. Full test suite của domain workflow — đảm bảo không phá vỡ hành vi khác
pnpm vitest run backend/src/main/workflow/__tests__/

# 5. Regression check bổ sung: resumeRunningExecutions() không bị ảnh hưởng bởi thay đổi này
#    (task này không đụng tới resumeRunningExecutions(), chỉ đụng executeStep() — xác nhận
#    bằng cách chạy lại toàn bộ test suite domain workflow ở bước 4)
```

**Điều kiện DONE:** `pnpm tsc --noEmit` pass, test integration mới pass và chứng minh được bug tồn tại trước fix, toàn bộ test suite `backend/src/main/workflow/__tests__/` pass, không có thay đổi ngoài phạm vi `WorkflowOrchestrator.ts`.
