# TASK-BE-017.1: Migration `0013_workflow_trace_correlation.ts` — persist `root_trace_id`

**Phase:** 3
**SOL Ref:** [SOL-BE-TRACE-017](../solutions/SOL-BE-TRACE-017-workflow-orchestration.md)
**CR Ref:** [CR-TRACE-017](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-017-workflow-orchestration.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + none (task đầu tiên của CR-TRACE-017 — thuần DB migration, không phụ thuộc tracer nào)
**Status:** ✅ Done (2026-08-04) — Migration `0013_workflow_trace_correlation.ts` created exactly as specced (`root_trace_id TEXT` nullable column on `orca_workflow_executions`, no-op `down()`), registered in `ALL_MIGRATIONS` after `migration0012PortForwardsPush`. 0013 confirmed free (verified `src/main/db/migrations/` fresh before creating). 6 new tests in `src/main/db/migrations/__tests__/0013_workflow_trace_correlation.test.ts`, all pass. DRIFT: mid-task a concurrent agent's scoped `git stash` briefly reverted this file and `migrations/index.ts` (and, separately, `src/shared/trace/tracers.ts` + `src/shared/trace/index.ts`) — recovered deterministically from the stash (`git show stash@{0}:<path>`), no data lost, no destructive git ops used (stash kept, not dropped/popped).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này tạo file MỚI hoàn toàn (`migration0013WorkflowTraceCorrelation`, `src/main/db/migrations/0013_workflow_trace_correlation.ts`) — không có symbol cũ để chạy `gitnexus_impact`. Trước khi tạo, khám phá migration liền kề để bám đúng convention/pattern (`Migration` type, cách đăng ký vào `ALL_MIGRATIONS`):

```bash
codegraph explore "ALL_MIGRATIONS"
```

Xác nhận đúng convention (số version tiếp theo, cấu trúc `up()`/`down()`) trước khi tạo file mới và thêm vào `src/main/db/migrations/index.ts`.

## Mô tả

Đây là task **migration độc lập**, tách riêng khỏi các task instrumentation code vì nó thay đổi DB schema (không phải additive tracer call thuần tuý). Thêm cột `root_trace_id TEXT` vào bảng `orca_workflow_executions` để lưu persistent `parentTraceId` (span cha `workflow:execute`), cho phép correlation sống sót qua Orca Server restart (`resumeRunningExecutions()`). Số migration tiếp theo còn trống là `0013` (0011/0012 đã dùng cho terminal-sessions/port-forwards-push).

**Quan trọng cho TASK-BE-018.x (Task Graph):** SOL-BE-TRACE-018 tái dùng đúng pattern `parentTraceId` (field nghiệp vụ) được thiết kế ở đây, nhưng **không** tự thêm cột DB mới cho Task Graph (không có yêu cầu sống sót qua restart tương tự Workflow) — mọi tham chiếu tới "migration cho `parentTraceId`" trong CR-TRACE-018 PHẢI trỏ về task này (`TASK-BE-017.1`), không tạo migration trùng lặp.

## File: `src/main/db/migrations/0013_workflow_trace_correlation.ts` [NEW]

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

## File: `src/main/db/migrations/index.ts` [MODIFY]

```typescript
// src/main/db/migrations/index.ts — thêm vào ALL_MIGRATIONS sau migration0012PortForwardsPush
import { migration0013WorkflowTraceCorrelation } from './0013_workflow_trace_correlation'

export const ALL_MIGRATIONS: readonly Migration[] = [
  // ...0001-0012 unchanged...
  migration0013WorkflowTraceCorrelation,
]
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/main/db/migrations/__tests__/0013_workflow_trace_correlation.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] File `src/main/db/migrations/0013_workflow_trace_correlation.ts` tồn tại, `version: 13`, `name: 'workflow_trace_correlation'`
- [ ] `up()` thêm cột `root_trace_id TEXT` (nullable) vào `orca_workflow_executions`, không lỗi khi cột đã null cho execution cũ
- [ ] `down()` là no-op an toàn (SQLite không hỗ trợ `DROP COLUMN` trước 3.35), có comment giải thích
- [ ] `migration0013WorkflowTraceCorrelation` được đăng ký vào `ALL_MIGRATIONS` trong `src/main/db/migrations/index.ts`, đặt SAU `migration0012PortForwardsPush`
- [ ] Không có migration nào khác trùng số `0013` trong repo
- [ ] `pnpm tsc --noEmit` pass, không lỗi mới
