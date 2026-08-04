# TASK-BE-005.3: Instrument REMOTE git RPC handlers (`git-remote.ts`) với tracing + forward `traceId`

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-005](../solutions/SOL-BE-TRACE-005-code-review.md) §2.3
**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-005.1
**Status:** ✅ Done (2026-08-04) — DRIFT: `ProjectWorktreeParam` already had `traceId: OptionalString` (added by TASK-BE-001.1, using the shared `OptionalString` schema helper from `../schemas`, not a literal new `z.string().optional()`) — no schema change needed, contrary to the doc's assumption that the field didn't exist yet. `Tracers` import was already present in `git-remote.ts` from TASK-BE-001.3. Used the renamed tracer keys from TASK-BE-005.1 (`Tracers.codeReviewDiff`/`codeReviewAiCommit`/`codeReviewCreatePr`). Implemented the no-outer-catch version of `git.generateCommitMessage` per the doc's "Sửa lỗi double-fail" guidance (only 2 business-outcome `span.fail()` calls, no double-fail). `pnpm run typecheck:node` clean for `git-remote.ts` (only pre-existing unrelated `aiProviderService` unused-var warning, not introduced by this task).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "git.diff"
codegraph explore "git.generateCommitMessage"
codegraph explore "git.pr.create"
```

Cả 3 là RPC handler REMOTE đã tồn tại trong `src/main/runtime/rpc/methods/git-remote.ts` (MODIFY case). Chạy:

```
gitnexus_impact({ target: "git.pr.create", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa; chú ý field `traceId` mới thêm vào `ProjectWorktreeParam` (schema dùng chung) không phá vỡ handler khác đang dùng schema này. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc 3 RPC handler REMOTE trong `src/main/runtime/rpc/methods/git-remote.ts` (`git.diff`, `git.generateCommitMessage`, `git.pr.create`) bằng tracer tương ứng, đồng thời thêm field `traceId: z.string().optional()` vào Zod schema `ProjectWorktreeParam` để cho phép resume span từ Browser (CR-TRACE-000 §3.2), và forward `traceId: span.id` vào mọi `relay.call()` theo CR-TRACE-000 §3.3. `RpcContext`/`RpcRequest` hiện không có field per-request `traceId` cố định, nên cách additive-only rẻ nhất là đọc trực tiếp từ `params.traceId` thay vì sửa `dispatcher.ts`/`core.ts`.

## File: `src/main/runtime/rpc/methods/git-remote.ts` [MODIFY]

```typescript
// src/main/runtime/rpc/methods/git-remote.ts
import { Tracers } from '../../../../shared/trace/tracers'

// ── Common schema thêm traceId optional ───────────────────────────────────────
const ProjectWorktreeParam = z.object({
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  traceId: z.string().optional(),   // [NEW] CR-TRACE-000 §3.2 — resume span từ Browser
})

// ── git.diff (REMOTE) ──────────────────────────────────────────────────────────
defineMethod({
  name: 'git.diff',
  params: ProjectWorktreeParam.extend({
    staged: z.boolean().optional(),
    files: z.array(z.string()).optional(),
  }),
  handler: async (params, ctx) => {
    const span = Tracers.codeReviewDiffFlow.start(
      { worktreeId: params.worktreePath, mode: 'remote' },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
      span.step('routeRelay', { projectId: params.projectId })
      const args = ['diff']
      if (params.staged) args.push('--staged')
      if (params.files?.length) args.push('--', ...params.files)
      const result = await relay.call('git.exec', {
        cwd: params.worktreePath, args, traceId: span.id,   // forward theo CR-TRACE-000 §3.3
      }) as GitExecResult
      span.ok({ mode: 'remote', fileCount: result.stdout ? result.stdout.split('\n').length : 0 })
      return result
    } catch (err) {
      span.fail(err, { mode: 'remote' })
      throw err
    }
  },
}),

// ── git.generateCommitMessage (REMOTE) ────────────────────────────────────────
// Dùng bản KHÔNG có catch bọc ngoài — cả 2 nhánh lỗi nghiệp vụ tự span.fail()+throw
// bên trong; catch ngoài dư thừa sẽ gây double-fail. Xem ghi chú "Sửa lỗi double-fail"
// bên dưới.
defineMethod({
  name: 'git.generateCommitMessage',
  params: ProjectWorktreeParam.extend({
    devServerId: z.string().min(1),
    userId: z.string().optional(),
    modelHint: z.string().optional(),
  }),
  handler: async (params, ctx) => {
    const userId = params.userId ?? ctx.userId ?? ''
    const span = Tracers.codeReviewAiCommitFlow.start(
      { worktreeId: params.worktreePath, mode: 'remote' },
      params.traceId ? { id: params.traceId } : undefined
    )
    const relay = await router.getRelayForProject(params.projectId, userId)

    const diffResult = await relay.call('git.exec', {
      cwd: params.worktreePath, args: ['diff', '--staged', '--no-color'], traceId: span.id,
    }) as GitExecResult
    span.step('diffStaged', { mode: 'remote', hasChanges: diffResult.stdout.trim().length > 0 })

    if (!diffResult.stdout.trim()) {
      span.fail('GIT_NO_STAGED_CHANGES', { mode: 'remote' })
      throw new Error('GIT_NO_STAGED_CHANGES')
    }

    const prompt = [
      'You are a git commit message generator. Given the following diff, write a concise commit message.',
      'Format: <type>(<scope>): <subject> (max 72 chars on first line)',
      'Optionally add body paragraphs after blank line.',
      '', 'Diff:', diffResult.stdout.slice(0, 8000),
    ].join('\n')
    span.step('aiComplete', { mode: 'remote', promptChars: prompt.length })
    const aiResult = await relay.call('ai.complete', { prompt, format: 'text', traceId: span.id }) as
      { content?: string; text?: string }
    const message = (aiResult.content ?? aiResult.text ?? '').trim()
    if (!message) {
      span.fail('GIT_AI_EMPTY_RESPONSE', { mode: 'remote' })
      throw new Error('GIT_AI_EMPTY_RESPONSE')
    }

    span.ok({ mode: 'remote', messageChars: message.length })
    return { message }
  }
}),

// ── git.pr.create (REMOTE) ─────────────────────────────────────────────────────
defineMethod({
  name: 'git.pr.create',
  params: ProjectWorktreeParam.extend({
    title: z.string().min(1),
    body: z.string().optional(),
    base: z.string().min(1),
    draft: z.boolean().optional(),
    head: z.string().optional(),
  }),
  handler: async (params, ctx) => {
    const span = Tracers.codeReviewCreatePrFlow.start(
      { worktreeId: params.worktreePath, mode: 'remote' },
      params.traceId ? { id: params.traceId } : undefined
    )
    try {
      const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')

      const ghArgs = ['pr', 'create', '--title', params.title, '--base', params.base]
      if (params.body) ghArgs.push('--body', params.body)
      if (params.draft) ghArgs.push('--draft')
      if (params.head) ghArgs.push('--head', params.head)

      span.step('ghExec', { mode: 'remote', base: params.base })
      const result = await relay.call('shell.exec', {
        cwd: params.worktreePath, cmd: 'gh', args: ghArgs, traceId: span.id,
      }) as { stdout?: string; stderr?: string; exitCode?: number }

      const prUrl = (result.stdout ?? '').trim()
      const exitCode = result.exitCode ?? 0
      if (exitCode !== 0) {
        span.fail(result.stderr ?? 'gh pr create failed', { mode: 'remote', exitCode })
      } else {
        span.ok({ mode: 'remote', prUrl, exitCode })
      }
      return { url: prUrl, exitCode }
    } catch (err) {
      span.fail(err, { mode: 'remote' })
      throw err
    }
  },
}),
```

**Ghi chú quan trọng — `relay.call()` transport error trong `git.generateCommitMessage`:** nếu `relay.call()` tự ném lỗi network/timeout (không phải 1 trong 2 lỗi nghiệp vụ `GIT_NO_STAGED_CHANGES`/`GIT_AI_EMPTY_RESPONSE`), lỗi đó sẽ propagate mà không có `span.fail()` bổ sung nếu chỉ dùng bản không có `try/catch` ở trên. Đây là gap nhỏ được chấp nhận theo SOL-BE-TRACE-005 §2.3 vì `relay:agentCall` (transport layer) đã tự log fail của chính nó. Nếu muốn bọc đầy đủ transport error, wrap toàn bộ thân hàm trong `try/catch` và gọi `span.fail(err, { mode: 'remote', phase: 'transport' })` trước khi `throw err` — áp dụng cách này khi implement thật để tránh silent span (span mở `start()` nhưng không bao giờ `ok()`/`fail()` nếu `relay.call()` reject ngoài 2 nhánh nghiệp vụ đã biết).

**Ràng buộc bắt buộc:**
- Không thêm `try/catch` bọc ngoài dư thừa cho `git.generateCommitMessage` — dùng đúng bản không có catch ngoài ở trên (tránh double-fail: gọi `span.fail()` 2 lần cho cùng 1 outcome).
- `traceId` phải được forward đúng field ở cả 3 method: `params.traceId` → `relay.call({..., traceId: span.id})`.
- Không đưa token/credential/API key nào vào `TraceFields` của 3 tracer này (diff content, prompt, PR body có thể chứa dữ liệu nhạy cảm của người dùng nhưng không phải secret hệ thống — không đưa `prompt` đầy đủ vào field, chỉ `promptChars`).

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

- [ ] `Tracers.codeReviewDiffFlow` phân biệt `mode: 'local'` vs `'remote'` trong mọi event, kể cả khi fail
- [ ] `Tracers.codeReviewAiCommitFlow` có `step('diffStaged')` và `step('aiComplete')` tách biệt ở nhánh remote
- [ ] `Tracers.codeReviewCreatePrFlow` ghi được `exitCode` của `gh` CLI khi `relay.call('shell.exec', ...)` trả về `exitCode !== 0`, gọi `fail()` thay vì chỉ dựa vào exception
- [ ] `traceId` được forward đúng field (`params.traceId` → `relay.call({..., traceId: span.id})`) ở cả 3 RPC method remote (`git.diff`, `git.generateCommitMessage`, `git.pr.create`)
- [ ] `git.generateCommitMessage` không có double-fail (chỉ 2 điểm nghiệp vụ gọi `span.fail()`, không có catch ngoài dư thừa)
- [ ] Không có giá trị token/credential/API key nào trong `TraceFields` của 3 tracer trên
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
