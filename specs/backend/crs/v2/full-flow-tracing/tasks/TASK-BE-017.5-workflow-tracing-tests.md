# TASK-BE-017.5: Test Vitest cho toàn bộ instrumentation CR-TRACE-017 (bao gồm `parentTraceId` + resume qua restart)

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-017](../solutions/SOL-BE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-017.1, TASK-BE-017.2, TASK-BE-017.3, TASK-BE-017.4
**Status:** ✅ Done (2026-08-04) — Delivered 35 new tests total (target ≥17): `WorkflowOrchestrator.test.ts` +10 (target ≥8, incl. resume-after-restart same-id assertion, unique step-span ids, and rootSpans no-leak across all 3 terminal transitions completed/failed/cancelled), `StepExecutors.test.ts` +6 new file (target ≥5), `TemplateResolver.test.ts` +3 (target ≥2), migration `0013` test +6 new file (target ≥2). All pass: `pnpm test --run src/main/workflow src/main/db/migrations` → 123 passed / 3 pre-existing unrelated failures (in `more-migrations.test.ts`, a stale hardcoded "10 migrations" assertion that already didn't match HEAD's 12 migrations before this task — confirmed via `git status`/`git show HEAD` untouched by this task). `pnpm tsc --noEmit` — no new errors across the CR-TRACE-017 file set.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này chỉ viết test file (bao gồm test cho migration `0013`) — không sửa symbol sản xuất nào, nên KHÔNG cần `gitnexus_impact`. Khám phá lại các symbol đã instrument ở TASK-BE-017.1 → 017.4 trước khi viết test:

```bash
codegraph explore "WorkflowOrchestrator.execute"
codegraph explore "StepExecutors.execute"
codegraph explore "TemplateResolver.create"
codegraph explore "migration0013WorkflowTraceCorrelation"
```

## Mô tả

Viết test Vitest cho migration `0013` và 3 module đã instrument (`WorkflowOrchestrator`, `StepExecutors`, `TemplateResolver`). Tổng mục tiêu ≥ 17 test case. Test quan trọng nhất: verify `parentTraceId` nhất quán trên mọi step span trong cùng 1 execution, **kể cả sau khi `resumeRunningExecutions()` chạy lại sau restart**.

## Test Plan (từ SOL-BE-TRACE-017 §3)

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

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/workflow/__tests__/WorkflowOrchestrator.test.ts
pnpm test --run src/main/workflow/__tests__/StepExecutors.test.ts
pnpm test --run src/main/workflow/__tests__/TemplateResolver.test.ts
pnpm test --run src/main/db/migrations/__tests__/0013_workflow_trace_correlation.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] Tổng ≥ 17 test case mới trên 4 file test liệt kê ở bảng trên, tất cả pass
- [ ] Test xác nhận `parentTraceId` nhất quán trên MỌI step span trong cùng 1 execution, kể cả sau khi `resumeRunningExecutions()` chạy lại sau restart (giả lập: tạo execution, kill in-memory state, gọi lại `resumeRunningExecutions()`, so `parentTraceId` step mới với step cũ)
- [ ] Test xác nhận mỗi step có `span.id` RIÊNG (khác nhau) dù cùng `parentTraceId`
- [ ] Test xác nhận `rootSpans` map không leak entry ở cả 3 nhánh kết thúc (`completed`/`failed`/`cancelled`)
- [ ] `gitnexus_detect_changes()` xác nhận thay đổi của CR-TRACE-017 chỉ giới hạn đúng các file: `tracers.ts`, `WorkflowOrchestrator.ts`, `StepExecutors.ts`, `TemplateResolver.ts`, `workflow-rpc-handler.ts`, `0013_workflow_trace_correlation.ts`, `migrations/index.ts`
