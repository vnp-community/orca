# SOL-BE-TRACE-005: Code Review — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-005](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-005-code-review.md)
**TDD Ref:** TDD-08 (Agent Orchestration — chỉ tham chiếu cho phần agent-side annotation/feedback, KHÔNG có RPC method liên quan trực tiếp git.ts/git-remote.ts), TDD-20 (Remote Git UI — git-handler relay, `git.generateCommitMessage`, `git.pr.create`)
**Date:** 2026-08-02
**Status:** Proposed
**Strategy:** Additive-only — chỉ thêm tracer calls vào handler đã tồn tại, không đổi business logic; không viết instrumentation cho code chưa tồn tại

---

## 1. Phân tích phạm vi (Backend-side only)

### 1.1 Bối cảnh

Code Review (BL-CR-01→05) có 2 code path song song mà CR-TRACE-005 đã xác minh bằng grep thực tế:

- **LOCAL**: `src/main/runtime/rpc/methods/git.ts` — handler mỏng, forward thẳng vào `OrcaRuntimeService` (`src/main/runtime/orca-runtime-git.ts`). Đây là code chạy trong Electron Main process hoặc trong per-user process ở Web mode; **không băng qua `relay.call()`**.
- **REMOTE**: `src/main/runtime/rpc/methods/git-remote.ts` — handler forward qua `ProjectServerRouter.getRelayForProject()` rồi `relay.call('git.exec' | 'ai.complete' | 'shell.exec', ...)`. Đây là code Backend/Gateway gọi ra Dev Server.

Cả hai file đều nằm trong `src/main/` — thuộc phạm vi Backend/Gateway của solution này. **Không sửa** `src/relay/git-handler.ts` (agent-side thực thi `git.exec` thật trên Dev Server, TDD-20 §2) — đó là trách nhiệm của solution phía Agent.

### 1.2 Gap Analysis

| Sub-flow | Backend file:function (verified) | Hiện trạng | Việc cần làm (Backend) | Trong/ngoài phạm vi |
|----------|-----------------------------------|-----------|------------------------|----------------------|
| BL-CR-01 Diff (local) | `src/main/runtime/rpc/methods/git.ts:157` (`git.diff` → `runtime.getRuntimeGitDiff()`) | Không tracer | Thêm `codeReviewDiffFlow` bọc handler, `mode: 'local'` | Trong phạm vi |
| BL-CR-01 Diff (remote) | `src/main/runtime/rpc/methods/git-remote.ts:100-113` (`git.diff` → `relay.call('git.exec', ...)`) | Không tracer | Thêm `codeReviewDiffFlow` bọc handler + `step('routeRelay')` + forward `traceId` vào `relay.call()` params | Trong phạm vi |
| BL-CR-02 Annotate | RPC method `annotation.create` **không tồn tại** trong `src/main/runtime/rpc/methods/*` (verified: không có route nào tên này); `AnnotationDiffService.ts` (`src/main/code-review/`) chỉ có `getDiffAnnotations()` đọc, không có method tạo | Chưa implement (BUG-AG-ORCH-005) | Khai báo `codeReviewAnnotateFlow` trong `tracers.ts`, **KHÔNG wire vào code** | Tracer khai báo trước; wiring ngoài phạm vi cho tới khi feature tồn tại |
| BL-CR-03 Send Feedback | RPC method `review.sendFeedback` **không tồn tại**; nhánh remote thực tế nếu có sẽ gọi `agent.sendInput` — nhưng đó là JSON-RPC dispatch phía `src/relay/agent-session.ts:111` (AGENT side, không phải Backend RPC method) | Chưa implement | Khai báo `codeReviewFeedbackFlow` trong `tracers.ts`, **KHÔNG wire vào code** | Tracer khai báo trước; điểm gọi `agent.sendInput` thuộc phạm vi solution phía Agent |
| BL-CR-04 AI Commit Message (local) | `src/main/runtime/rpc/methods/git.ts:263` (`git.generateCommitMessage`) → `runtime.generateRuntimeCommitMessage()` (`src/main/runtime/orca-runtime-git.ts:576`) | Không tracer | Thêm `codeReviewAiCommitFlow`, `mode: 'local'` | Trong phạm vi |
| BL-CR-04 AI Commit Message (remote) | `src/main/runtime/rpc/methods/git-remote.ts:352-389` (`git.generateCommitMessage` → 2 `relay.call()` tuần tự: `git.exec` diff-staged rồi `ai.complete`) | Không tracer | Thêm `codeReviewAiCommitFlow` với `step('diffStaged')` + `step('aiComplete')` tách biệt | Trong phạm vi |
| BL-CR-05 Tạo PR (local) | `runtime.generateRuntimePullRequestFields()` (`orca-runtime-git.ts:663`) + `createGitHubPullRequest()` (`src/main/github/client.ts:1833`) | Không tracer | Thêm `codeReviewCreatePrFlow`, `mode: 'local'` | Trong phạm vi |
| BL-CR-05 Tạo PR (remote) | `src/main/runtime/rpc/methods/git-remote.ts:393-425` (`git.pr.create` → `relay.call('shell.exec', {cmd: 'gh', ...})`) | Không tracer | Thêm `codeReviewCreatePrFlow`, `step('ghExec')`, ghi `exitCode` | Trong phạm vi |

### 1.3 Phát hiện bổ sung khi verify code (ngoài những gì CR-TRACE-005 đã nêu)

Đọc `src/main/runtime/orca-runtime-git.ts:309-331` (`getRuntimeGitDiff`) cho thấy nhánh "LOCAL" của `git.ts` thực ra **tự phân nhánh thêm lần nữa** bên trong `OrcaRuntimeService`:

```typescript
// src/main/runtime/orca-runtime-git.ts:309
async getRuntimeGitDiff(worktreeSelector, filePath, staged, compareAgainstHead) {
  const target = await this.host.resolveRuntimeGitTarget(worktreeSelector)
  const provider = target.connectionId ? getSshGitProvider(target.connectionId) : null
  if (target.connectionId) {
    return provider.getDiff(...)   // ← SSH-connection-based execution (khác hẳn ProjectServerRouter/relay.call)
  }
  return getDiff(...)              // ← true local child_process.execFile
}
```

Đây là **path thứ ba** (SSH-connection cũ, `SshGitProvider`) nằm trong nhánh mà CR-TRACE-005 gọi chung là "local". CR-TRACE-005 không yêu cầu tách field riêng cho path này (acceptance criteria chỉ đòi `mode: 'local'|'remote'`), nên solution này giữ đúng phạm vi: `mode: 'local'` bao phủ cả `SshGitProvider` lẫn true local exec — không thêm `step()` phân biệt case này vì không thuộc acceptance criteria của CR-TRACE-005 và việc thêm sẽ vượt phạm vi "additive-only". Ghi chú lại đây để solution phía Backend follow-up (nếu có CR riêng cho `SshGitProvider`) không bỏ sót.

### 1.4 Ngoài phạm vi solution này (Agent-side — companion solution xử lý)

- `src/relay/git-handler.ts` (`git.exec`/`git.execStream` thực thi trên Dev Server, TDD-20 §2)
- `src/relay/agent-rpc-dispatch.ts:488`, `src/relay/agent-spawner.ts:479`, `src/relay/agent-session.ts:111` (`agent.sendInput` — nếu BL-CR-03 remote được implement trong tương lai)
- `src/relay/preflight-handler.ts`, `src/main/github/client.ts` (external HTTPS call, không nhận `traceId`, không cần instrument theo CR-TRACE-000 §3.3 vì là 3rd-party API)

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts` — thêm 5 tracer

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  /** Browser → RPC → IPC → Relay → Agent: directory browse */
  browseDirFlow: createTracer('devServer:browseDir'),
  /** Browser → RPC → IPC → Relay → Agent: mkdir */
  mkdirFlow:     createTracer('devServer:mkdir'),
  /** Browser → RPC → IPC → Relay → Agent: rmdir */
  rmdirFlow:     createTracer('devServer:rmdir'),
  /** Agent WebSocket lifecycle (connect / disconnect) */
  agentWsFlow:   createTracer('agentWs:lifecycle'),
  /** IPC proxy call from user-process to main-process */
  ipcProxyFlow:  createTracer('ipc:devServerProxy'),

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

### 2.2 `src/main/runtime/rpc/methods/git.ts` — LOCAL path

`defineMethod` handler hiện tại chỉ forward `(params, { runtime }) => runtime.xxx(...)` — không có `try/catch` riêng, lỗi được `RpcDispatcher` bắt và map thành `RpcFailure` (`src/main/runtime/rpc/dispatcher.ts`). Instrumentation phải giữ nguyên hành vi throw (không nuốt lỗi), nên bọc bằng `try { ... } catch { span.fail(); throw }`.

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

> Ghi chú: `git.pr.create` **không tồn tại trong `git.ts` (LOCAL)** — path local dùng `git.generatePullRequestFields` (chỉ generate title/description bằng AI) rồi renderer gọi thẳng `github.pr.create`/GitHub client riêng (`GITHUB_METHODS`, xem TDD-04 §7). `codeReviewCreatePrFlow` cho nhánh local vì vậy chỉ bọc bước `aiGenerate`; bước `ghExec` thật của local path nằm ở `GITHUB_METHODS` (`src/main/runtime/rpc/methods/github.ts`, chưa xác định tên method chính xác — cần điều tra thêm khi triển khai nếu muốn nối full span local `createPr`).

### 2.3 `src/main/runtime/rpc/methods/git-remote.ts` — REMOTE path

Params schema hiện tại (`ProjectWorktreeParam` v.v.) không có field `traceId`. Theo CR-TRACE-000 §3.3 hàng "WebSocket RPC", `traceId` nên là sibling field của request envelope; nhưng `RpcContext` (`src/main/runtime/rpc/core.ts:42`) hiện không có field nào mang theo per-request `traceId`, và `RpcRequest` (`core.ts:32`) cũng chỉ có `{id, authToken, method, params}` cố định. Additive-only cách rẻ nhất (không sửa `dispatcher.ts`/`core.ts`): thêm `traceId: z.string().optional()` vào từng Zod schema liên quan, đọc trực tiếp từ `params.traceId` — đúng như CR-TRACE-005 mục 5.1 dự phòng ("params.traceId tuỳ nơi RPC context được inject").

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
    try {
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
    } catch (err) {
      // fail() đã được gọi ở 2 nhánh throw phía trên; nhánh lỗi bất ngờ khác (relay.call ném)
      // vẫn cần fail() ở đây để không bỏ sót — TraceSpan không cấm gọi fail() nếu ok()/fail()
      // trước đó đã emit; theo thiết kế hiện tại của TraceSpan (`src/shared/trace/index.ts`)
      // mỗi lời gọi step/ok/fail đều emit độc lập, nên guard tránh double-fail bằng cờ cục bộ:
      throw err
    }
  },
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

**Sửa lỗi double-fail tiềm ẩn ở `git.generateCommitMessage`**: bản trên có 1 `catch` bọc ngoài không cần thiết vì cả 2 nhánh lỗi nghiệp vụ đã tự `span.fail()` + `throw` bên trong `try`. Cách đúng — bỏ catch ngoài, để lỗi tự propagate (v.v `RpcDispatcher` map exception thành `RpcFailure`), chỉ giữ `span.fail()` tại đúng 2 điểm nghiệp vụ:

```typescript
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

  const prompt = [/* ...same as above... */].join('\n')
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
```

Nếu `relay.call()` tự ném lỗi network/timeout (không phải 1 trong 2 lỗi nghiệp vụ trên), lỗi đó KHÔNG được `span.fail()` — đây là gap nhỏ chấp nhận được vì `relay:agentCall` (transport layer) đã tự log fail của chính nó; `codeReviewAiCommitFlow` ở business layer chỉ cần `ok()`/`fail()` cho outcome nghiệp vụ. Muốn bọc đầy đủ transport error, wrap toàn bộ thân hàm trong `try/catch` và gọi `span.fail(err, { mode: 'remote', phase: 'transport' })` trước khi `throw err` — khuyến nghị áp dụng khi implement thật để tránh silent span (span mở `start()` nhưng không bao giờ `ok()`/`fail()` nếu relay.call() reject).

### 2.4 Khai báo tracer cho BL-CR-02/03 — KHÔNG wire

Theo mục 1.2, `codeReviewAnnotateFlow`/`codeReviewFeedbackFlow` chỉ tồn tại trong `tracers.ts` (mục 2.1). Không có patch nào khác trong solution này gắn 2 tracer này vào code, vì:

- `annotation.create` RPC method không tồn tại → không có handler nào để patch
- `review.sendFeedback` RPC method không tồn tại → tương tự
- Khi 2 RPC method này được xây (theo dõi BUG-AG-ORCH-001/005), một CR/solution follow-up sẽ patch chúng bằng đúng pattern ở mục 2.3 (`start()` ở đầu handler, `step()` cho boundary calls, `ok()`/`fail()` ở cuối)

---

## 3. Test Plan (Vitest)

### 3.1 File test mới

```
src/main/runtime/rpc/methods/__tests__/git-remote-tracing.test.ts
src/main/runtime/rpc/methods/__tests__/git-local-tracing.test.ts
```

### 3.2 Test cases

**`git-remote-tracing.test.ts`**
- `git.diff (remote)`: gọi handler với mock `router.getRelayForProject` + mock `relay.call` trả `{stdout, stderr, exitCode:0}` → assert `Tracers.codeReviewDiffFlow` nhận đúng 1 `start()` với `mode:'remote'`, 1 `step('routeRelay')`, 1 `ok()` với `fileCount`
- `git.diff (remote) — relay.call throws`: assert `span.fail()` được gọi với `mode:'remote'` trước khi exception propagate
- `git.diff (remote) — traceId forwarded`: gọi handler với `params.traceId = 'abc123'` → assert `span.id === 'abc123'` (resume, không random mới)
- `git.diff (remote) — relay.call receives traceId`: assert `relay.call` được gọi với `params.traceId === span.id`
- `git.generateCommitMessage (remote) — happy path`: assert 2 `step()` riêng biệt (`diffStaged`, `aiComplete`) theo đúng thứ tự, mỗi step có field đúng theo bảng mục 1.2
- `git.generateCommitMessage (remote) — GIT_NO_STAGED_CHANGES`: mock diff rỗng → assert `span.fail('GIT_NO_STAGED_CHANGES', ...)` được gọi TRƯỚC `ai.complete` (không gọi AI khi không có staged diff)
- `git.generateCommitMessage (remote) — GIT_AI_EMPTY_RESPONSE`: mock AI trả rỗng → assert `span.fail('GIT_AI_EMPTY_RESPONSE', ...)`
- `git.pr.create (remote) — success`: mock `shell.exec` trả `exitCode:0` → assert `span.ok()` với `prUrl`, `exitCode:0`
- `git.pr.create (remote) — gh CLI fails`: mock `shell.exec` trả `exitCode:1` → assert `span.fail()` được gọi với `exitCode:1` (KHÔNG chỉ dựa vào exception vì `relay.call('shell.exec')` không tự throw khi exitCode != 0)

**`git-local-tracing.test.ts`**
- `git.diff (local)`: mock `runtime.getRuntimeGitDiff` → assert `Tracers.codeReviewDiffFlow.start()` với `mode:'local'`, `ok()` cùng `mode`
- `git.diff (local) — runtime throws`: assert `span.fail(err, {mode:'local'})` trước khi re-throw
- `git.generateCommitMessage (local) — success.true`: mock `runtime.generateRuntimeCommitMessage` trả `{success:true, message:'...'}` → assert `ok()` với `messageChars`
- `git.generateCommitMessage (local) — success.false`: mock trả `{success:false, error:'...'}` → assert `fail()` được gọi, KHÔNG throw (vì handler return result thay vì throw trong nhánh này)

**Xác nhận không phá tracer hiện có (regression)**
- `tracers.test.ts` (nếu tồn tại, hoặc thêm mới): `Tracers.codeReviewAnnotateFlow` và `Tracers.codeReviewFeedbackFlow` tồn tại (được export) nhưng KHÔNG có bất kỳ call site nào trong `git.ts`/`git-remote.ts` (grep-based assertion hoặc review thủ công trong CI checklist — không cần test runtime)

### 3.3 Test Targets

| Test file | Target số test |
|-----------|---------------|
| `git-remote-tracing.test.ts` | ≥ 9 |
| `git-local-tracing.test.ts` | ≥ 4 |
| **Total** | **≥ 13** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.codeReviewDiffFlow` phân biệt `mode: 'local'` vs `'remote'` trong mọi event (`start`/`step`/`ok`/`fail`), kể cả khi fail
- [ ] `Tracers.codeReviewAiCommitFlow` có `step('diffStaged')` và `step('aiComplete')` tách biệt ở nhánh remote để đo latency Git diff vs AI provider riêng
- [ ] `Tracers.codeReviewCreatePrFlow` ghi được `exitCode` của `gh` CLI khi `relay.call('shell.exec', ...)` trả về `exitCode !== 0`, gọi `fail()` thay vì chỉ dựa vào exception
- [ ] `traceId` được forward đúng field (`params.traceId` → `relay.call({..., traceId: span.id})`) ở cả 3 RPC method remote (`git.diff`, `git.generateCommitMessage`, `git.pr.create`); verify bằng `ORCA_TRACE=1` thấy cùng `id` ở cả `codeReview:*` và `relay:agentCall`
- [ ] `Tracers.codeReviewAnnotateFlow` và `Tracers.codeReviewFeedbackFlow` được khai báo trong `tracers.ts` nhưng không có call site nào trong `git.ts`/`git-remote.ts` — verify bằng review code, không trace path ảo
- [ ] Không tracer nào trong solution này trùng tên với tracer nội bộ đã có (`devServer:*`, `agentWs:lifecycle`, `ipc:devServerProxy`, `relay:agentCall`, `agent:rpc`) hoặc với `ssh:*` (CR-TRACE-004)
- [ ] `git.ts` (LOCAL) không gọi `relay.call()` ở bất kỳ đâu trong các handler được patch — đảm bảo phân biệt đúng ranh giới Backend-only vs Backend→Relay
- [ ] File `src/relay/git-handler.ts`, `agent-rpc-dispatch.ts`, `agent-spawner.ts`, `agent-session.ts` KHÔNG bị sửa bởi solution này (thuộc phạm vi solution phía Agent)
