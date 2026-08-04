# TASK-BE-005.2: Instrument LOCAL git RPC handlers (`git.ts`) với tracing

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-005](../solutions/SOL-BE-TRACE-005-code-review.md) §2.2
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-005.1
**Status:** ✅ Done (2026-08-04) — Implemented using the renamed tracer keys from TASK-BE-005.1 (`Tracers.codeReviewDiff`/`codeReviewAiCommit`/`codeReviewCreatePr`, not the `*Flow` names in this doc's code sample — see TASK-BE-005.1 status for the collision rationale). Current `git.ts` handler shapes matched the doc closely (post-001.x state); `git.diff`/`git.generateCommitMessage`/`git.generatePullRequestFields` wrapped per spec, no `relay.call()` present in `git.ts`. `pnpm run typecheck:node` clean for `git.ts` (only pre-existing unrelated error elsewhere in the repo from concurrent agents).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "git.diff"
codegraph explore "git.generateCommitMessage"
codegraph explore "git.generatePullRequestFields"
```

Cả 3 là RPC handler LOCAL đã tồn tại trong `src/main/runtime/rpc/methods/git.ts` (MODIFY case). Chạy:

```
gitnexus_impact({ target: "git.diff", direction: "upstream" })
```

`git.diff` đặc biệt cần chú ý — nó dùng chung cho nhiều mục đích (code review, compare), impact có thể lan rộng hơn 2 handler còn lại. Báo cáo blast radius trước khi sửa; nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc 3 RPC handler LOCAL trong `src/main/runtime/rpc/methods/git.ts` (`git.diff`, `git.generateCommitMessage`, `git.generatePullRequestFields`) bằng tracer tương ứng (`codeReviewDiffFlow`, `codeReviewAiCommitFlow`, `codeReviewCreatePrFlow`), giữ nguyên hành vi throw hiện tại (không nuốt lỗi) bằng pattern `try { ... } catch { span.fail(); throw }`. Đây là code chạy trong Electron Main process hoặc per-user process ở Web mode — **không băng qua `relay.call()`**, khác với nhánh REMOTE (xử lý ở TASK-BE-005.3).

Lưu ý: `git.pr.create` **không tồn tại** trong `git.ts` (LOCAL) — path local dùng `git.generatePullRequestFields` (chỉ generate title/description bằng AI); bước `ghExec` thật của local path nằm ở `GITHUB_METHODS` (`src/main/runtime/rpc/methods/github.ts`, tên method chính xác chưa xác định) — **ngoài phạm vi task này**, `codeReviewCreatePrFlow` ở đây chỉ bọc bước `aiGenerate`.

## File: `src/main/runtime/rpc/methods/git.ts` [MODIFY]

Thêm import và bọc 3 handler sau (giữ nguyên `defineMethod`, `params`, logic nghiệp vụ bên trong — chỉ thêm tracer):

```typescript
// src/main/runtime/rpc/methods/git.ts
import { Tracers } from '../../../../shared/trace/tracers'

// ── git.diff (LOCAL) ──────────────────────────────────────────────────────────
defineMethod({
  name: 'git.diff',
  params: GitDiff,
  handler: async (params, { runtime }) => {
    const span = Tracers.codeReviewDiffFlow.start({
      worktreeId: params.worktree, mode: 'local', staged: params.staged ?? false
    })
    try {
      const result = await runtime.getRuntimeGitDiff(
        params.worktree,
        params.filePath,
        params.staged,
        params.compareAgainstHead
      )
      span.ok({ mode: 'local' })
      return result
    } catch (err) {
      span.fail(err, { mode: 'local' })
      throw err
    }
  }
}),

// ── git.generateCommitMessage (LOCAL) ─────────────────────────────────────────
defineMethod({
  name: 'git.generateCommitMessage',
  params: GitGenerateCommitMessage,
  handler: async (params, { runtime }) => {
    const span = Tracers.codeReviewAiCommitFlow.start({ worktreeId: params.worktree, mode: 'local' })
    try {
      const override = buildCommitMessageGenerationOverride(params)
      span.step('diffStaged', { mode: 'local' })
      const result = override === undefined
        ? await runtime.generateRuntimeCommitMessage(params.worktree)
        : await runtime.generateRuntimeCommitMessage(params.worktree, override)
      if (!result.success) {
        span.fail(result.error, { mode: 'local' })
        return result
      }
      span.ok({ mode: 'local', messageChars: result.message?.length ?? 0 })
      return result
    } catch (err) {
      span.fail(err, { mode: 'local' })
      throw err
    }
  }
}),

// ── git.generatePullRequestFields (LOCAL — phần AI của BL-CR-05) ─────────────
defineMethod({
  name: 'git.generatePullRequestFields',
  params: GitGeneratePullRequestFields,
  handler: async (params, { runtime }) => {
    const span = Tracers.codeReviewCreatePrFlow.start({ worktreeId: params.worktree, mode: 'local' })
    try {
      const input = {
        base: params.base, title: params.title, body: params.body,
        draft: params.draft, provider: params.provider, useTemplate: params.useTemplate
      }
      const override = buildCommitMessageGenerationOverride(params)
      span.step('aiGenerate', { mode: 'local' })
      const result = override === undefined
        ? await runtime.generateRuntimePullRequestFields(params.worktree, input)
        : await runtime.generateRuntimePullRequestFields(params.worktree, input, override)
      span.ok({ mode: 'local' })
      return result
    } catch (err) {
      span.fail(err, { mode: 'local' })
      throw err
    }
  }
}),
```

**Ràng buộc bắt buộc:**
- Không sửa logic nghiệp vụ bên trong (`runtime.getRuntimeGitDiff`, `runtime.generateRuntimeCommitMessage`, `runtime.generateRuntimePullRequestFields`, `buildCommitMessageGenerationOverride`) — chỉ thêm tracer calls bao quanh.
- `git.ts` (LOCAL) không được gọi `relay.call()` ở bất kỳ đâu trong các handler đã patch — đảm bảo đúng ranh giới Backend-only vs Backend→Relay.
- Không đưa bất kỳ giá trị token/credential/API key nào (nếu handler có truy cập) vào `TraceFields` của `codeReviewDiffFlow`/`codeReviewAiCommitFlow`/`codeReviewCreatePrFlow` — chỉ các field nghiệp vụ như `worktreeId`, `mode`, `staged`, `messageChars`.

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

- [ ] `Tracers.codeReviewDiffFlow` bọc `git.diff` với `mode: 'local'` trong mọi event (`start`/`ok`/`fail`)
- [ ] `Tracers.codeReviewAiCommitFlow` bọc `git.generateCommitMessage` với `step('diffStaged')` trước khi gọi `runtime.generateRuntimeCommitMessage`
- [ ] `Tracers.codeReviewCreatePrFlow` bọc `git.generatePullRequestFields` với `step('aiGenerate')`
- [ ] Lỗi từ `runtime.*` vẫn propagate nguyên vẹn (không bị nuốt bởi tracer instrumentation) — `span.fail()` gọi trước `throw err`
- [ ] `git.ts` không gọi `relay.call()` ở bất kỳ đâu
- [ ] Không có giá trị token/credential/API key nào trong `TraceFields` của 3 tracer trên
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
