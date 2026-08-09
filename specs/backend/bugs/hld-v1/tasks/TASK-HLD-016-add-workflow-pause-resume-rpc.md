# TASK-HLD-016: Thêm RPC method `workflow.pause` / `workflow.resume`

**Priority:** 🟠 HIGH
**Effort:** ~2 giờ (bao gồm test)
**Status:** ✅ DONE — 2026-08-09 (header JSDoc cập nhật 9 method; `'paused'` thêm vào `ListExecutionsParam` enum; 2 param schema mới; 2 RPC method `workflow.pause`/`workflow.resume` chèn đúng vị trí (sau cancel, trước template.create), gọi đúng `resumeFromPause()` — đã ghi rõ trong comment không nhầm với `resumeRunningExecutions()`. `tsc --noEmit` chỉ còn 2 lỗi `z.record()` pre-existing (đã xác nhận từ trước). Nhóm Workflow (013→016, toàn bộ chuỗi blocker) hoàn tất.)
**Bug refs:** BUG-BE-HLD-009 (phần 2 — RPC exposure; phần 1 orchestrator + schema nằm ở TASK-HLD-015)
**Solution ref:** [SOLUTION-workflow-exact.md — BUG-BE-HLD-009 Bước 4](../solutions/SOLUTION-workflow-exact.md#bước-4--workflow-rpc-handlerts-2-rpc-method-mới-workflowpause--workflowresume)
**Depends on:** **TASK-HLD-015 (phải merge trước)** — cần `WorkflowOrchestrator.pause()`/`resumeFromPause()` và `WorkflowStatus` đã có `'paused'` tồn tại trước khi wire RPC gọi tới chúng. Gián tiếp phụ thuộc **TASK-HLD-013** (blocker gốc, đã được TASK-HLD-015 kế thừa).

---

## Mục tiêu

Expose `WorkflowOrchestrator.pause()` và `resumeFromPause()` (đã implement ở TASK-HLD-015) ra ngoài qua 2 RPC method mới `workflow.pause` / `workflow.resume`, với access-control chỉ cho phép `triggeredBy` user thực hiện.

**Lưu ý quan trọng:** `workflow.resume` gọi `orchestrator.resumeFromPause()` — một resume **đơn lẻ, do user chủ động kích hoạt**, KHÔNG PHẢI `orchestrator.resumeRunningExecutions()` (crash-recovery nội bộ, chạy 1 lần lúc bootstrap cho mọi execution `status='running'`, không có RPC exposure). Đừng nhầm 2 hàm này.

## File cần sửa

```
backend/src/main/workflow/workflow-rpc-handler.ts
```

(`server-bootstrap.ts` **không cần sửa thêm** — `createWorkflowMethods(workflowOrchestrator, templateResolver, pool)` đã đăng ký tự động mọi method trong mảng trả về của factory function, bao gồm 2 method mới.)

## Thay đổi cụ thể

### 1. Header comment — cập nhật danh sách method + access control

TRƯỚC:

```typescript
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
```

SAU:

```typescript
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

### 2. `ListExecutionsParam` — thêm `'paused'` vào enum `status`

TRƯỚC (dòng 57-62):

```typescript
const ListExecutionsParam = z.object({
  projectId: z.string().optional(),
  triggeredBy: z.string().optional(),
  status: z.enum(['pending', 'running', 'completed', 'failed', 'cancelled']).optional(),
  limit: z.number().int().positive().max(500).optional(),
})
```

SAU:

```typescript
const ListExecutionsParam = z.object({
  projectId: z.string().optional(),
  triggeredBy: z.string().optional(),
  status: z.enum(['pending', 'running', 'paused', 'completed', 'failed', 'cancelled']).optional(), // [NEW BUG-BE-HLD-009]
  limit: z.number().int().positive().max(500).optional(),
})
```

### 3. Thêm 2 param schema cạnh `CancelParam` (dòng 64-66)

```typescript
const PauseParam = z.object({
  executionId: z.string().min(1),
})

const ResumeParam = z.object({
  executionId: z.string().min(1),
})
```

### 4. Thêm 2 RPC method mới, ngay sau `'workflow.cancel'` (sau dòng 162), TRƯỚC `'workflow.template.create'`

```typescript
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

`workflow.cancel` không cần đổi — `cancel()` vẫn hợp lệ gọi trên execution `status='paused'` (huỷ hẳn thay vì tiếp tục), và TASK-HLD-015 đã thêm `this.pauseRequests.delete(executionId)` vào `cancel()` để dọn pending pause-request nếu có.

**Trước khi code, verify verbatim** vị trí thật của `defineMethod`/`CancelParam` trong `workflow-rpc-handler.ts` hiện tại (số dòng có thể lệch nếu file đã đổi từ lúc viết solution) — đảm bảo 2 method mới chèn đúng thứ tự logic (sau `workflow.cancel`, trước `workflow.template.create`) để giữ nhóm method theo domain (execution lifecycle trước, template sau).

## Verification

Trước khi bắt đầu: xác nhận TASK-HLD-015 đã merge (`WorkflowOrchestrator.pause()`/`resumeFromPause()` tồn tại và có test pass).

```bash
pnpm tsc --noEmit
pnpm vitest run backend/src/main/workflow/__tests__/

# 1. workflow.pause gọi bởi đúng triggeredBy user, execution đang 'running'
#    → assert orchestrator.pause() được gọi, response { paused: true }
# 2. workflow.pause gọi bởi user KHÁC triggeredBy → assert throw WORKFLOW_PAUSE_DENIED
#    (orchestrator.pause() KHÔNG được gọi — access-control chặn trước khi tới orchestrator)
# 3. workflow.pause với executionId không tồn tại → assert throw EXECUTION_NOT_FOUND
# 4. workflow.resume gọi bởi đúng triggeredBy user, execution đang 'paused'
#    → assert orchestrator.resumeFromPause() được gọi (KHÔNG PHẢI resumeRunningExecutions()),
#       response { resumed: true }
# 5. workflow.resume gọi bởi user KHÁC triggeredBy → assert throw WORKFLOW_RESUME_DENIED
# 6. workflow.listExecutions với status='paused' → assert filter hoạt động đúng
#    (zod schema chấp nhận 'paused', SQL WHERE lọc đúng)

# Regression check bắt buộc:
# 7. workflow.cancel vẫn hoạt động bình thường trên execution status='paused'
#    (không bị 2 method mới ảnh hưởng)
# 8. Xác nhận workflow.resume KHÔNG gọi nhầm resumeRunningExecutions() — grep code:
grep -n "resumeFromPause\|resumeRunningExecutions" backend/src/main/workflow/workflow-rpc-handler.ts
# Expected: chỉ 'resumeFromPause' xuất hiện trong handler của workflow.resume;
# 'resumeRunningExecutions' không được gọi trực tiếp từ RPC layer ở đây

# 9. Full regression suite domain workflow
pnpm vitest run backend/src/main/workflow/__tests__/
```

**Điều kiện DONE:** `pnpm tsc --noEmit` pass, toàn bộ 9 test case pass, `workflow.resume` xác nhận gọi đúng `resumeFromPause()` (không phải `resumeRunningExecutions()`), `pnpm vitest run backend/src/main/workflow/__tests__/` pass không regression, header comment JSDoc phản ánh đúng 9 method hiện có.
