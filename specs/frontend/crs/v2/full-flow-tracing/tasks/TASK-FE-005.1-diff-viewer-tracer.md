# TASK-FE-005.1: Đăng ký tracer code review + instrument `DiffViewer.tsx` (BL-CR-01, đã mount)

**Phase:** 2
**SOL Ref:** [SOL-FE-TRACE-005 §1.1, §1.2, §2.1, §2.2](../solutions/SOL-FE-TRACE-005-code-review.md)
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001)
**Status:** ✅ Done (2026-08-03) — Checked `DiffViewer.tsx` first per top-level instructions for concurrent user work: no `Tracers` import or prior instrumentation present (file was at its original pre-tracing baseline), so implemented the task doc's plan directly. Added 5 `ui:codeReview.*` tracers to `tracers.ts` (`codeReviewDiffFlow` used here; `Annotate/Feedback/AiCommit/CreatePr` reserved for TASK-FE-005.2/005.3), no naming collision with the existing bare `codeReview:*` backend entries. Instrumented the `useEffect` diff-load in `DiffViewer.tsx` (staged: single RPC + ok/fail; unstaged: `step('parallelFetch')` + 2 parallel RPCs sharing one `traceId`). Confirmed `git.getDiff` RPC method mismatch vs backend `git.diff` per task note — NOT fixed, out of scope. gitnexus_impact LOW risk (1 direct caller, code-review-panel.tsx). Fixed 1 pre-existing exact-match test broken by `traceId`; added 4 new tracing tests. `pnpm tsc --noEmit` clean, 9/9 tests pass.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "DiffViewer"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "DiffViewer", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục — lưu ý `DiffViewer` được dùng chung bởi cả `GitPanel` (đã mount) và `CodeReviewPanel` (chưa mount), nên khả năng có caller thực từ `GitPanel` khiến risk có thể không phải LOW.

## Mô tả

**Quan trọng — 2 cây component song song:** `GitPanel` (`components/workspace/git/*`, **ĐÃ mount** tại `WorkspaceLayout.tsx`) và `CodeReviewPanel` (`components/code-review/*`, **CHƯA mount** ở đâu). Cả hai đều dùng chung `DiffViewer.tsx` (`components/workspace/git/DiffViewer.tsx`) để hiển thị diff — đây là điểm instrument BL-CR-01 duy nhất, tự động bao phủ cả hai cây.

**Lưu ý bắt buộc:** RPC method `git.getDiff` gọi từ đây **không tồn tại trong backend** (chỉ có `git.diff`, viết khác). Khi CR này ship, `span.fail()` sẽ tự động lộ ra lỗi "method not found" mỗi lần user click 1 file — đây chính là giá trị thực tế của tracing: biến bug âm thầm (component fail silent với message chung chung) thành tín hiệu rõ trong TracePanel. **Task này KHÔNG tự sửa tên RPC method** (ngoài phạm vi tracing, thuộc companion backend/UI-bugfix CR).

## File: `src/shared/trace/tracers.ts` [MODIFY, additive]

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  codeReviewDiffFlow:      createTracer('ui:codeReview.diff'),           // BL-CR-01
  codeReviewAnnotateFlow:  createTracer('ui:codeReview.annotate'),       // BL-CR-02 — dùng ở TASK-FE-005.3
  codeReviewFeedbackFlow:  createTracer('ui:codeReview.sendFeedback'),   // BL-CR-03 — dùng ở TASK-FE-005.2
  codeReviewAiCommitFlow:  createTracer('ui:codeReview.aiCommitMessage'),// BL-CR-04 — dùng ở TASK-FE-005.2/005.3
  codeReviewCreatePrFlow:  createTracer('ui:codeReview.createPr'),       // BL-CR-05 — dùng ở TASK-FE-005.3
} as const
```

> N.B. prefix `ui:`: bắt buộc theo convention chung (xem các task Phase 1, `00-index.md` mục 1) — 5 tracer trên dùng prefix `ui:` nhất quán với toàn bộ 10 CR. Companion backend solution (SOL-BE-TRACE-005) dùng tên KHÔNG prefix (`codeReview:diff|annotate|sendFeedback|aiCommitMessage|createPr`) — hai bộ tracer độc lập, chỉ liên kết qua `traceId`/`resume`.

## File: `src/renderer/src/components/workspace/git/DiffViewer.tsx` [MODIFY]

```typescript
import { Tracers } from '../../../../../shared/trace/tracers'

useEffect(() => {
  if (!project || !filePath) return
  setIsLoading(true)
  setError(null)

  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  const span = Tracers.codeReviewDiffFlow.start({ filePath, staged, mode: target.kind })

  if (staged) {
    callRuntimeRpc<string>(target, 'git.getDiff', { projectId: project.id, path: filePath, staged: true, traceId: span.id })
      .then(diff => {
        const idx = diff.indexOf('@@')
        setOriginal(idx >= 0 ? diff.slice(0, idx) : '')
        setModified(diff)
        span.ok({ staged: true })
      })
      .catch(err => {
        setError(err?.message ?? 'Failed to load diff')
        span.fail(err, { staged: true })
      })
      .finally(() => setIsLoading(false))
  } else {
    span.step('parallelFetch', { staged: false })
    Promise.all([
      callRuntimeRpc<string>(target, 'git.getDiff', { projectId: project.id, path: filePath, staged: false, side: 'original', traceId: span.id }).catch(() => ''),
      callRuntimeRpc<{ content: string }>(target, 'fs.readFile', { projectId: project.id, path: filePath, encoding: 'utf-8', traceId: span.id }).then(r => r.content).catch(() => ''),
    ])
      .then(([original, modified]) => {
        setOriginal(original)
        setModified(modified)
        span.ok({ staged: false })
      })
      .catch(err => {
        setError(err?.message ?? 'Failed to load diff')
        span.fail(err, { staged: false })
      })
      .finally(() => setIsLoading(false))
  }
}, [filePath, worktreePath, project, staged])
```

> `span.step('parallelFetch', ...)` đánh dấu điểm rẽ nhánh quan trọng (staged vs unstaged dùng chiến lược fetch khác nhau hẳn — 1 call vs 2 call song song).

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/components/workspace/git/__tests__/DiffViewer.test.tsx
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] 5 tracer `codeReviewDiffFlow/AnnotateFlow/FeedbackFlow/AiCommitFlow/CreatePrFlow` thêm vào `tracers.ts` đúng tên `ui:codeReview.diff|annotate|sendFeedback|aiCommitMessage|createPr`
- [ ] `DiffViewer.tsx` (dùng chung cho cả `GitPanel` đã mount và `CodeReviewPanel` chưa mount) phát `span.step('parallelFetch')` khi `staged=false`, phân biệt `staged` trong `ok()`/`fail()`
- [ ] `staged=true` → 1 call `git.getDiff` với `traceId`; `staged=false` → 2 call song song, cả hai đều có `traceId: span.id` giống nhau
- [ ] Không method RPC nào bị đổi tên trong quá trình thêm tracing — mismatch `git.getDiff` vs backend `git.diff` được ghi nhận nhưng KHÔNG sửa
- [ ] Test suite đạt ≥ 4 test case mới: `staged=true` → `ok()`; `staged=false` → `step('parallelFetch')` trước `Promise.all`; `traceId` trong cả 2 lời gọi khi `staged=false`; reject → `fail()` với field `staged` đúng nhánh
