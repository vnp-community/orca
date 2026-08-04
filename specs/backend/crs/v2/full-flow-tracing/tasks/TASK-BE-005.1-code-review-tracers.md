# TASK-BE-005.1: Thêm 5 tracer cho Code Review vào `tracers.ts`

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-005](../solutions/SOL-BE-TRACE-005-code-review.md) §2.1
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Prerequisite:** Phase 0 (TASK-BE-000)
**Status:** ✅ Done (2026-08-04) — DRIFT: the 5 key names proposed here (`codeReviewDiffFlow`/`codeReviewAnnotateFlow`/`codeReviewFeedbackFlow`/`codeReviewAiCommitFlow`/`codeReviewCreatePrFlow`) were already claimed in `tracers.ts` by a concurrent frontend task for browser-initiated `ui:codeReview.*` flows (consumed by `DiffViewer.tsx`). Per the no-rename collision rule, added backend entries under bare (no-`Flow`-suffix) keys instead — `codeReviewDiff`, `codeReviewAnnotate`, `codeReviewFeedback`, `codeReviewAiCommit`, `codeReviewCreatePr` — matching the sibling backend convention (`worktreeCreate`, `agentOrchSpawn`, `terminalCreate`). Flow name strings (`codeReview:diff`, `codeReview:annotate`, `codeReview:sendFeedback`, `codeReview:aiCommitMessage`, `codeReview:createPr`) are unchanged from the spec. `codeReviewAnnotate`/`codeReviewFeedback` declared only, not wired (verified no call sites). `typecheck:node` clean for touched files (only pre-existing unrelated `aiProviderService` unused-var warning in `git-remote.ts`).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

`Tracers` là object đã tồn tại (MODIFY case) — chạy thêm:

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

Task này chỉ thêm 5 entry mới, 2 trong số đó (`codeReviewAnnotateFlow`/`codeReviewFeedbackFlow`) không có call site nào. Fan-in lớn của `Tracers` là bình thường; chỉ dừng lại nếu risk HIGH/CRITICAL đến từ nguyên nhân khác (trùng tên entry cũ), xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 5 tracer mới cho domain Code Review trong `src/shared/trace/tracers.ts`. Hai trong số này (`codeReviewAnnotateFlow`, `codeReviewFeedbackFlow`) chỉ được khai báo trước — **KHÔNG được wire vào bất kỳ handler nào** trong task này hay bất kỳ task nào khác của CR-TRACE-005, vì RPC method tương ứng (`annotation.create`, `review.sendFeedback`) chưa tồn tại trong code (đã verify bằng grep, xem SOL-BE-TRACE-005 §1.2). Việc wire 2 tracer này thuộc về một CR/task follow-up sau khi tính năng gốc (BUG-AG-ORCH-001/005) được implement.

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm khối sau vào object `Tracers` đã tồn tại (giữ nguyên các tracer hiện có, chỉ append):

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...các tracer hiện có (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow, ...) giữ nguyên...

  // ─── CR-TRACE-005: Code Review ────────────────────────────────────────────
  /** BL-CR-01: xem diff của agent changes (local + remote) */
  codeReviewDiffFlow:      createTracer('codeReview:diff'),
  /** BL-CR-02: annotate dòng code — KHÔNG wire vào code cho tới khi
   *  annotation.create RPC method + AgentManager.injectAnnotations() tồn tại
   *  (BUG-AG-ORCH-005). Khai báo trước theo CR-TRACE-000 §4 naming convention. */
  codeReviewAnnotateFlow:  createTracer('codeReview:annotate'),
  /** BL-CR-03: gửi feedback về agent — KHÔNG wire vào code cho tới khi
   *  review.sendFeedback RPC method tồn tại (BUG-AG-ORCH-001). */
  codeReviewFeedbackFlow:  createTracer('codeReview:sendFeedback'),
  /** BL-CR-04: tạo commit message bằng AI (local + remote) */
  codeReviewAiCommitFlow:  createTracer('codeReview:aiCommitMessage'),
  /** BL-CR-05: tạo Pull Request với AI (local + remote) */
  codeReviewCreatePrFlow:  createTracer('codeReview:createPr'),
} as const
```

**Ràng buộc bắt buộc:**
- Không đổi tên hay xoá bất kỳ tracer nào đã tồn tại trong `Tracers`.
- `codeReviewAnnotateFlow` và `codeReviewFeedbackFlow` chỉ xuất hiện trong file này — không có call site (`.start(`) nào trong `git.ts`/`git-remote.ts` hay bất kỳ file nào khác.
- Không tên tracer nào trùng với tracer nội bộ đã có (`devServer:*`, `agentWs:lifecycle`, `ipc:devServerProxy`, `relay:agentCall`, `agent:rpc`) hoặc `ssh:*` (CR-TRACE-004).

## Verification

```bash
pnpm run typecheck:node
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.codeReviewDiffFlow`, `codeReviewAnnotateFlow`, `codeReviewFeedbackFlow`, `codeReviewAiCommitFlow`, `codeReviewCreatePrFlow` được export từ `src/shared/trace/tracers.ts`
- [ ] `codeReviewAnnotateFlow` và `codeReviewFeedbackFlow` không có call site nào trong codebase (verify bằng `grep -rn "codeReviewAnnotateFlow\|codeReviewFeedbackFlow" src/` — chỉ match đúng dòng khai báo trong `tracers.ts`)
- [ ] Không tracer nào trong task này trùng tên flow với tracer đã tồn tại
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
