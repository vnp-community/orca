# CR-TRACE-005 — Code Review Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-005 |
| **Tên** | Code Review — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P1 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/code-review.md`, `src/main/runtime/rpc/methods/git.ts`, `src/main/runtime/rpc/methods/git-remote.ts`, `src/main/runtime/orca-runtime-git.ts`, `src/main/code-review/AnnotationDiffService.ts`, `src/relay/agent-rpc-dispatch.ts`, `src/relay/agent-spawner.ts`, `src/relay/agent-session.ts`, `src/main/project/ProjectServerRouter.ts` |

---

## 1. Vấn đề

Luồng Code Review (BL-CR-01→05) có **hai code path song song cho mọi thao tác** — LOCAL (`child_process.execFile` / `runtime.*` trên máy Electron) và REMOTE (`relay.call('git.exec', ...)` qua Dev Server) — như chính flow doc ghi nhận trong bảng "Khác biệt HLD vs Implementation". Không có tracing nào hiện tại phân biệt được hai path này khi troubleshoot:

- **BL-CR-01 (Diff)**: `git.diff` tồn tại ở CẢ `src/main/runtime/rpc/methods/git.ts:157` (local, gọi `runtime.getRuntimeGitDiff()`) và `src/main/runtime/rpc/methods/git-remote.ts:101` (remote, `router.getRelayForProject()` → `relay.call('git.exec', {args: ['diff', ...]})`). Khi diff load chậm, không biết là do Git CLI local chậm, hay do `ProjectServerRouter.getRelayForProject()` phải chờ relay reconnect, hay do bản thân `relay.call()` timeout.
- **BL-CR-02/03 (Annotate & Send Feedback)**: Theo flow doc, `AgentManager.injectAnnotations()` **chưa được implement** (đánh dấu BUG-AG-ORCH-005) và nhánh remote `relay.call('agent.sendInput')` cho annotation cũng được đánh dấu MISSING (BUG-AG-ORCH-001) tại thời điểm viết flow doc — nhưng `agent.sendInput` THỰC TẾ đã tồn tại cho mục đích khác (`src/relay/agent-rpc-dispatch.ts:488`, `src/relay/agent-spawner.ts:479`, `src/relay/agent-session.ts:111`). Việc thiếu tracer khiến không thể xác nhận khi nào code path annotation→agent thực sự chạy qua đâu, phục vụ cả việc verify khi 2 bug trên được fix.
- **BL-CR-04 (AI Commit Message)**: nhánh REMOTE (`git-remote.ts:353`, `git.generateCommitMessage`) là một chuỗi 2 relay call tuần tự: `relay.call('git.exec', diff --staged)` rồi `relay.call('ai.complete', {prompt})` — nếu chậm, không biết là git diff chậm hay AI provider chậm. Nhánh LOCAL (`git.ts:263` → `runtime.generateRuntimeCommitMessage()`) là một luồng hoàn toàn khác (PTY-based) — cần tracer riêng biệt để so sánh latency.
- **BL-CR-05 (Tạo PR)**: `git-remote.ts:394` (`git.pr.create`) gọi `relay.call('shell.exec', {cmd: 'gh', args: [...]})` — một external `gh` CLI call trên remote host, cộng thêm bước AI generate title/description (qua `runtime.generateRuntimePullRequestFields()` ở local, hoặc luồng tương tự generateCommitMessage ở remote) trước khi gọi GitHub REST. Không có breakdown thời gian giữa "AI generate" và "gh CLI exec" và "GitHub API network".

## 2. Thành phần & Transport liên quan

| Thành phần | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------|-------|-----------|-------------------------------|
| Renderer/Browser (DiffViewer, PR Form) | UI | WebSocket RPC | Browser tạo `traceId` (khi có tracer riêng), gửi kèm RPC call |
| `src/main/runtime/rpc/methods/git.ts` (LOCAL) | Business Logic | IPC/RPC nội bộ | Điểm bắt đầu span nếu không có `traceId` từ Renderer |
| `src/main/runtime/rpc/methods/git-remote.ts` (REMOTE, web mode) | Business Logic | WebSocket RPC (Browser↔Orca Server) | Đọc `params.traceId` nếu Renderer gửi kèm |
| `ProjectServerRouter.getRelayForProject()` | Routing | in-process | Không băng qua network — không cần `step()` riêng trừ khi có validation chậm (theo mục 5 CR-TRACE-000) |
| `relay.call('git.exec' \| 'ai.complete' \| 'shell.exec' \| 'agent.sendInput', ...)` | Remote Execution | `relay.call()` (Orca Server ↔ Dev Server) | Field `traceId` trong params envelope — resume bằng `relayCallTracer` (`relay:agentCall`) đã có sẵn |
| GitHub REST API (`createGitHubPullRequest`, `addPRReviewComment`, `mergePR` — `src/main/github/client.ts`) | External | HTTPS | Không có transport propagation row riêng trong CR-TRACE-000 — external 3rd-party API, không nhận `traceId` |
| SQLite (annotations) | Persistence | in-process | Không `step()` riêng — gộp vào `ok()` theo mục 5 |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  codeReviewDiffFlow:      createTracer('codeReview:diff'),
  codeReviewAnnotateFlow:  createTracer('codeReview:annotate'),
  codeReviewFeedbackFlow:  createTracer('codeReview:sendFeedback'),
  codeReviewAiCommitFlow:  createTracer('codeReview:aiCommitMessage'),
  codeReviewCreatePrFlow:  createTracer('codeReview:createPr'),
}
```

Ghi chú đặt tên: dùng `aiCommitMessage`/`createPr` thay vì `aiReview`/`comment`/`merge` nêu trong ví dụ CR-TRACE-000 §4 vì BL-CR-01→05 thực tế của flow doc này là diff/annotate/feedback/commit-message/PR-create — chưa có sub-flow "AI review" hay "merge" riêng biệt trong `code-review.md` hiện tại (những thao tác đó thuộc `project-integration.md` BL-PI-04, xem CR-TRACE-006).

## 4. Instrumentation theo từng sub-flow

### BL-CR-01 — Xem Diff của Agent Changes

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `worktreeId`, `mode: 'local'\|'remote'` | `src/main/runtime/rpc/methods/git.ts:157` (local) hoặc `src/main/runtime/rpc/methods/git-remote.ts:101` (remote) |
| Route tới relay (chỉ remote) | `step('routeRelay')` | `projectId` | `src/main/project/ProjectServerRouter.ts` (`getRelayForProject()`) |
| Gọi Git CLI | `step('gitExec')` | `mode`, `staged` | local: `src/main/runtime/orca-runtime-git.ts` (`getRuntimeGitDiff`, chưa xác định số dòng chính xác) / remote: `relay.call('git.exec', ...)` trong `git-remote.ts:106-112` |
| Hoàn tất | `ok` | `fileCount` | cùng handler |
| Lỗi | `fail` | `mode` | cùng handler |

```typescript
// src/main/runtime/rpc/methods/git-remote.ts — git.diff (remote path)
handler: async (params, ctx) => {
  const span = Tracers.codeReviewDiffFlow.start({
    worktreeId: params.worktreePath, mode: 'remote'
  }, ctx.traceId ? { id: ctx.traceId } : undefined)
  try {
    const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
    span.step('routeRelay', { projectId: params.projectId })
    const args = ['diff']
    if (params.staged) args.push('--staged')
    const result = await relay.call('git.exec', { cwd: params.worktreePath, args }) as GitExecResult
    span.ok({ fileCount: result.stdout.split('\n').length })
    return result
  } catch (err) {
    span.fail(err, { mode: 'remote' })
    throw err
  }
}
```

### BL-CR-02 — Annotate Dòng Code trong Diff

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `worktreeId`, `file`, `line` | chưa xác định file cụ thể — cần điều tra thêm khi triển khai (không tìm thấy RPC method `annotation.create`; `AnnotationDiffService.ts` hiện chỉ có `getDiffAnnotations()` đọc, không có method tạo) |
| Lưu annotation | `step('persist')` | `annotationId` | `src/main/code-review/AnnotationDiffService.ts:39` (class), method tạo annotation chưa tồn tại — cần thêm khi BUG liên quan được fix |
| Gửi cho agent (nếu có) | `step('sendToAgent')` | `mode: 'local'\|'remote'` | chưa xác định — `AgentManager.injectAnnotations()` theo flow doc chưa implement (BUG-AG-ORCH-005); nếu remote path dùng `agent.sendInput`, xem `src/relay/agent-rpc-dispatch.ts:488` |
| Hoàn tất | `ok` | `annotationId` | như trên |

```typescript
// Khi annotation.create RPC method được thêm (hiện chưa tồn tại) — mẫu instrumentation dự kiến
handler: async (params, ctx) => {
  const span = Tracers.codeReviewAnnotateFlow.start({
    worktreeId: params.worktreeId, file: params.file, line: params.line
  })
  const annotation = await annotationDiffService.create(params) // method chưa tồn tại
  span.step('persist', { annotationId: annotation.id })
  span.ok({ annotationId: annotation.id })
  return annotation
}
```

> Vì `annotation.create` và `AgentManager.injectAnnotations()` chưa được implement (theo ghi chú BUG-AG-ORCH-005 trong chính flow doc), CR này chỉ định nghĩa **tracer name và điểm gắn dự kiến** — việc instrument thật sự phải đi kèm khi 2 bug đó được fix, không nên trace một code path chưa tồn tại.

### BL-CR-03 — Gửi Feedback về Agent

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `sessionId`, `mode` | chưa xác định file cụ thể — RPC method `review.sendFeedback` không tìm thấy trong codebase hiện tại, cần điều tra khi triển khai |
| Ghi vào PTY (local) | `step('ptyWrite')` | `mode: 'local'` | chưa xác định — cần tìm PTY write API thực tế khi implement |
| Gửi qua relay (remote) | `step('agentSendInput')` | `mode: 'remote'`, `ptyId` | `src/relay/agent-session.ts:111` (`'agent.sendInput'` call site) |
| Hoàn tất | `ok` | `mode` | như trên |

```typescript
// Mẫu instrumentation cho nhánh remote — dựa trên agent.sendInput đã tồn tại
const span = Tracers.codeReviewFeedbackFlow.start({ sessionId, mode: 'remote' })
span.step('agentSendInput', { ptyId })
try {
  await relay.call('agent.sendInput', { ptyId, data: feedbackMessage })
  span.ok({ mode: 'remote' })
} catch (err) {
  span.fail(err, { mode: 'remote' })
}
```

### BL-CR-04 — Tạo Commit Message bằng AI

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `worktreeId`, `mode` | local: `src/main/runtime/rpc/methods/git.ts:263` (`git.generateCommitMessage`) / remote: `src/main/runtime/rpc/methods/git-remote.ts:353` |
| Lấy staged diff | `step('diffStaged')` | `mode` | remote: `git-remote.ts:364` (`relay.call('git.exec', {args: ['diff','--staged',...]})`) / local: `src/main/runtime/orca-runtime-git.ts:576` (`generateRuntimeCommitMessage()`) |
| Gọi AI generate | `step('aiComplete')` | `mode`, `promptChars` | remote: `git-remote.ts:383` (`relay.call('ai.complete', {prompt})`) / local: trong `generateRuntimeCommitMessage()` (PTY-based, xem `src/main/text-generation/commit-message-text-generation.ts:867`) |
| Hoàn tất | `ok` | `messageChars` | cùng handler |
| Lỗi (vd. `GIT_NO_STAGED_CHANGES`, `GIT_AI_EMPTY_RESPONSE`) | `fail` | `mode`, `errorCode` | `git-remote.ts:370,385` |

```typescript
// src/main/runtime/rpc/methods/git-remote.ts — git.generateCommitMessage (remote path)
handler: async (params, ctx) => {
  const span = Tracers.codeReviewAiCommitFlow.start({ worktreeId: params.worktreePath, mode: 'remote' })
  const relay = await router.getRelayForProject(params.projectId, params.userId ?? ctx.userId ?? '')

  const diffResult = await relay.call('git.exec', {
    cwd: params.worktreePath, args: ['diff', '--staged', '--no-color']
  }) as GitExecResult
  span.step('diffStaged', { mode: 'remote', hasChanges: diffResult.stdout.trim().length > 0 })
  if (!diffResult.stdout.trim()) {
    span.fail('GIT_NO_STAGED_CHANGES', { mode: 'remote' })
    throw new Error('GIT_NO_STAGED_CHANGES')
  }

  span.step('aiComplete', { mode: 'remote', promptChars: diffResult.stdout.length })
  const aiResult = await relay.call('ai.complete', { prompt: /* ... */ '', format: 'text' }) as { content?: string }
  const message = (aiResult.content ?? '').trim()
  if (!message) { span.fail('GIT_AI_EMPTY_RESPONSE', { mode: 'remote' }); throw new Error('GIT_AI_EMPTY_RESPONSE') }

  span.ok({ messageChars: message.length })
  return { message }
}
```

### BL-CR-05 — Tạo Pull Request với AI

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu | `start` | `worktreeId`, `mode` | `src/main/runtime/rpc/methods/git-remote.ts:394` (`git.pr.create`, remote) / local path qua `runtime.generateRuntimePullRequestFields()` (`src/main/runtime/orca-runtime-git.ts:663`) + `createGitHubPullRequest()` (`src/main/github/client.ts:1833`) |
| Push branch | `step('push')` | `mode`, `branch` | remote: chưa thấy `git.push` gọi trực tiếp trong `git.pr.create` handler hiện tại (`git-remote.ts:394-425`) — flow doc mô tả push là bước riêng, cần xác nhận thứ tự thực tế khi triển khai |
| AI generate title/description | `step('aiGenerate')` | `mode` | local: `orca-runtime-git.ts:663` (`generateRuntimePullRequestFields()`) |
| Gọi `gh` CLI / GitHub REST | `step('ghExec')` | `mode`, `base` | remote: `git-remote.ts:416` (`relay.call('shell.exec', {cmd: 'gh', args: ghArgs})`) / local: `src/main/github/client.ts:1833` (`createGitHubPullRequest()`) |
| Hoàn tất | `ok` | `prUrl` | `git-remote.ts:422-423` |
| Lỗi | `fail` | `mode`, `exitCode` | như trên |

```typescript
// src/main/runtime/rpc/methods/git-remote.ts — git.pr.create (remote path)
handler: async (params, ctx) => {
  const span = Tracers.codeReviewCreatePrFlow.start({ worktreeId: params.worktreePath, mode: 'remote' })
  const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')

  const ghArgs = ['pr', 'create', '--title', params.title, '--base', params.base]
  if (params.body) ghArgs.push('--body', params.body)

  span.step('ghExec', { mode: 'remote', base: params.base })
  try {
    const result = await relay.call('shell.exec', { cwd: params.worktreePath, cmd: 'gh', args: ghArgs })
      as { stdout?: string; exitCode?: number }
    const prUrl = (result.stdout ?? '').trim()
    span.ok({ prUrl, exitCode: result.exitCode ?? 0 })
    return { url: prUrl, exitCode: result.exitCode ?? 0 }
  } catch (err) {
    span.fail(err, { mode: 'remote' })
    throw err
  }
}
```

## 5. Lan truyền traceId qua transport của flow này

Áp dụng CR-TRACE-000 §3.2/§3.3 cụ thể cho Code Review:

1. **Browser/Renderer → RPC method (`git.ts`/`git-remote.ts`)**: theo hàng "WebSocket RPC" của CR-TRACE-000 §3.3, `traceId` là sibling field cạnh `method`/`params`. Handler đọc `ctx.traceId` (hoặc `params.traceId` tuỳ nơi RPC context được inject) và truyền vào `resume: { id: traceId }` khi gọi `Tracers.codeReview*Flow.start()`.
2. **RPC method (remote) → `relay.call()`**: mọi lệnh `relay.call('git.exec' | 'ai.complete' | 'shell.exec' | 'agent.sendInput', params)` trong `git-remote.ts` phải đính `traceId: span.id` vào `params` truyền cho `relay.call()`, theo đúng mẫu ở CR-TRACE-000 §3.2 (`relay.call('git.worktree.add', { ...gitParams, traceId: span.id })`). Việc resume ở phía relay dùng lại `relayCallTracer` (`relay:agentCall`) đã tồn tại trong `dev-server-relay-bridge.ts` — CR này không cần định nghĩa tracer relay mới, chỉ cần đảm bảo `git-remote.ts` forward đúng field.
3. **`agent.sendInput` (BL-CR-02/03 remote nhánh)**: nếu route qua Agent WS JSON-RPC 2.0 (`src/relay/agent-rpc-dispatch.ts:488`), theo CR-TRACE-000 §3.3 hàng "Agent WS JSON-RPC 2.0", `traceId` phải nằm trong `params._trace.id`, KHÔNG dùng field `id` sẵn có của JSON-RPC (dùng để match request/response).
4. **GitHub REST API (BL-CR-05)**: không có hàng propagation riêng trong CR-TRACE-000 §3.3 vì đây là external 3rd-party API — không gửi `traceId` ra ngoài, span `codeReviewCreatePrFlow` kết thúc ở Main/Orca Server trước khi gọi HTTP, ghi lại `prUrl`/latency trong `ok()`.

## Acceptance Criteria

- [ ] `Tracers.codeReviewDiffFlow` phân biệt được `mode: 'local'` vs `'remote'` trong mọi event, kể cả khi fail
- [ ] `Tracers.codeReviewAiCommitFlow` có `step('diffStaged')` và `step('aiComplete')` tách biệt để đo latency riêng của Git diff vs AI provider
- [ ] `Tracers.codeReviewCreatePrFlow` ghi được `exitCode` của `gh` CLI khi `relay.call('shell.exec', ...)` fail
- [ ] `traceId` được forward đúng field khi `git-remote.ts` gọi `relay.call()` (verify bằng cách bật `ORCA_TRACE=1` và xác nhận cùng `id` xuất hiện ở cả RPC layer và `relay:agentCall` layer)
- [ ] `Tracers.codeReviewAnnotateFlow` và `Tracers.codeReviewFeedbackFlow` được khai báo trong `tracers.ts` nhưng KHÔNG gắn vào code chưa tồn tại (`annotation.create`, `review.sendFeedback`, `AgentManager.injectAnnotations()`) cho đến khi BUG-AG-ORCH-001/005 được fix — tránh trace path ảo
- [ ] Không tracer nào trong CR này trùng tên với các tracer nội bộ đã có hoặc với `ssh:*` (CR-TRACE-004)
- [ ] `ORCA_TRACE=1` cho thấy đầy đủ 3 tracer đã implement được (`diff`, `aiCommitMessage`, `createPr`) khi thực hiện end-to-end trên một worktree REMOTE
