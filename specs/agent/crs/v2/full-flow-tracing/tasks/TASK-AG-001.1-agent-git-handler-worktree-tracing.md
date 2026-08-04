# TASK-AG-001.1: Add worktree:create/delete tracer spans to agent-git-handler.ts

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-001](../solutions/SOL-AG-TRACE-001-worktree-management.md)
**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**Precondition:** Phase 0 (shared `Tracer.start(fields?, resume?)` API)
**Estimated time:** 2h
**Status:** ✅ Done (2026-08-03) — implemented exactly as specced, no concurrent drift found in `agent-git-handler.ts`. `pnpm run typecheck:node` clean for this file; pre-existing unrelated `AgentConfig` type error in the test file (missing `orcaHttpUrl`/`apiSecret`) confirmed present before this change too.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

Task này sửa nhiều symbol trên 2 file — chạy `codegraph explore` cho từng symbol trước khi sửa:

```bash
codegraph explore "Tracers"
codegraph explore "handleGitWorktreeAdd"
codegraph explore "handleGitWorktreeRemove"
codegraph explore "handleGitExec"
```

Cả 4 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis cho từng symbol:

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
gitnexus_impact({ target: "handleGitWorktreeAdd", direction: "upstream" })
gitnexus_impact({ target: "handleGitWorktreeRemove", direction: "upstream" })
gitnexus_impact({ target: "handleGitExec", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

`src/relay/agent-git-handler.ts` xử lý 2 RPC method thật của Agent WS JSON-RPC: `git.worktree.add` (`handleGitWorktreeAdd`) và `git.worktree.remove` (`handleGitWorktreeRemove`). Cả hai hiện KHÔNG có tracer domain nào — chỉ được bọc bởi tracer hạ tầng chung `agent:rpc` ở tầng dispatch. `handleGitExec()` (dùng chung bởi cả hai qua `spawn('git', ...)`) đã có tracer `agent:git` (`gitTracer`) nhưng KHÔNG resume `id` từ `params._trace`.

**Lưu ý module path:** KHÔNG sửa `src/relay/git-handler.ts` (1498 dòng, module khác dùng cho relay daemon thường) — chỉ `agent-git-handler.ts`. `git-handler.ts` chỉ được tái sử dụng qua dynamic import 2 helper thuần (`parseWorktreePorcelain`, `validateWorktreePath`), không đổi.

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm 2 entry mới vào object `Tracers` (idempotent — nếu backend-side solution đã merge trước, coi là no-op, không tạo trùng):

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

  // ─── CR-TRACE-001: Worktree Management ─────────────────────────────────────
  /** BL-WT-01 — create worktree (RPC handler + Agent git.worktree.add) */
  worktreeCreate: createTracer('worktree:create'),
  /** BL-WT-03 — delete worktree (RPC handler + Agent git.worktree.remove) */
  worktreeDelete: createTracer('worktree:delete'),
} as const
```

## File: `src/relay/agent-git-handler.ts` [MODIFY]

### Imports (thêm vào phần import hiện có)

```typescript
import { createTracer } from '../shared/trace'
import { Tracers } from '../shared/trace/tracers'          // [NEW]
```

`const gitTracer = createTracer('agent:git')` đã tồn tại — giữ nguyên.

### Resume helper (mới)

```typescript
// ─── Trace propagation helper ───────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested tại params._trace.id (CR-TRACE-000 §3.3),
// KHÔNG dùng field phẳng params.traceId — tránh đụng field `id` JSON-RPC 2.0 chuẩn.
function resumeFrom(params: Record<string, unknown>): { id: string } | undefined {   // [NEW]
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}
```

### `handleGitExec()` — resume tracer hạ tầng `agent:git`

Sửa DÒNG khởi tạo `span` duy nhất (giữ nguyên toàn bộ phần còn lại của hàm):

```typescript
export async function handleGitExec(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const rawArgs = Array.isArray(params.args) ? params.args.map(String) : []
  const cwd     = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const timeout = Math.min(typeof params.timeout === 'number' ? params.timeout : 30_000, 60_000)
  const argsStr = rawArgs.join(' ').slice(0, 80)
  const span    = gitTracer.start({ method: 'git.exec', cmd: argsStr, cwd }, resumeFrom(params))  // [MODIFIED]

  try {
    validateGitArgs(rawArgs)
  } catch (err: unknown) {
    if (err instanceof GitValidationError) {
      span.fail(`validation: ${err.message}`, { cmd: argsStr })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: err.message } }
    }
    throw err
  }

  return new Promise<object>((resolve) => {
    const child = spawn('git', rawArgs, {
      cwd,
      env:   config.toolEnv,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,
    })

    const stdout: string[] = []
    const stderr: string[] = []

    const timer = setTimeout(() => {
      child.kill('SIGTERM')
      span.fail(`timeout after ${timeout}ms`, { cmd: argsStr })
      resolve({
        jsonrpc: '2.0', id,
        error: { code: AgentErrorCode.ServerError, message: `git.exec timeout after ${timeout}ms` },
      })
    }, timeout)

    child.stdout?.on('data', (chunk: Buffer) => stdout.push(chunk.toString()))
    child.stderr?.on('data', (chunk: Buffer) => stderr.push(chunk.toString()))

    child.on('close', (code) => {
      clearTimeout(timer)
      const exitCode = code ?? 0
      log.info(`git.exec: ${rawArgs.join(' ')} → exitCode=${exitCode}`)
      const outLen = stdout.join('').length
      if (exitCode === 0) {
        span.ok({ cmd: argsStr, exitCode, outLen })
      } else {
        span.fail(`git exit ${exitCode}`, { cmd: argsStr, exitCode, outLen })
      }
      resolve({
        jsonrpc: '2.0', id,
        result: { stdout: stdout.join(''), stderr: stderr.join(''), exitCode },
      })
    })

    child.on('error', (err) => {
      clearTimeout(timer)
      span.fail(err, { cmd: argsStr })
      resolve({ jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: err.message } })
    })

    child.stdin?.end()
  })
}
```

### `handleGitWorktreeAdd()` — domain span `worktree:create`

```typescript
export async function handleGitWorktreeAdd(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const worktreePath = typeof params.path   === 'string' ? params.path.trim()   : ''
  const branch       = typeof params.branch === 'string' ? params.branch.trim() : ''
  const createBranch = params.createBranch === true
  const cwd          = typeof params.cwd    === 'string' ? params.cwd           : config.workDir

  const span = Tracers.worktreeCreate.start(                                    // [NEW]
    { path: worktreePath, branch, cwd },
    resumeFrom(params)
  )

  if (!worktreePath || !branch) {
    span.fail('missing required params', { path: worktreePath, branch })         // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required params: path, branch' } }
  }
  if (SHELL_METACHARACTERS.test(worktreePath) || SHELL_METACHARACTERS.test(branch)) {
    span.fail('unsafe characters in params', { path: worktreePath, branch })      // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in worktree params' } }
  }

  // WT-Issue-1: Security validation — prevent path traversal
  try {
    const { validateWorktreePath } = await import('./git-handler')
    span.step('validate-path', { path: worktreePath })                          // [NEW]
    validateWorktreePath(['worktree', 'add', worktreePath], cwd)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg, { path: worktreePath })                                       // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: msg } }
  }

  const args = createBranch
    ? ['worktree', 'add', '-b', branch, worktreePath]
    : ['worktree', 'add', worktreePath, branch]

  span.step('git-worktree-add-exec', { branch })                                 // [NEW]
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15_000, _trace: { id: span.id } },         // [MODIFIED] — nối id xuống agent:git
    config,
    log
  )
  if (result && typeof result === 'object' && 'error' in result) {
    span.fail((result as { error: { message: string } }).error.message, { path: worktreePath })  // [NEW]
  } else {
    span.ok({ path: worktreePath, branch })                                      // [NEW]
  }
  return result
}
```

### `handleGitWorktreeRemove()` — domain span `worktree:delete`

```typescript
export async function handleGitWorktreeRemove(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const path  = typeof params.path  === 'string' ? params.path.trim() : ''
  const force = params.force === true

  const span = Tracers.worktreeDelete.start({ path, force }, resumeFrom(params))  // [NEW]

  if (!path) {
    span.fail('missing required param: path')                                    // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' } }
  }
  if (SHELL_METACHARACTERS.test(path)) {
    span.fail('unsafe characters in path', { path })                             // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in path' } }
  }

  const args = ['worktree', 'remove', path]
  if (force) args.push('--force')

  span.step('git-worktree-remove-exec', { force })                               // [NEW]
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15_000, _trace: { id: span.id } },         // [MODIFIED]
    config,
    log
  )
  if (result && typeof result === 'object' && 'error' in result) {
    span.fail((result as { error: { message: string } }).error.message, { path })  // [NEW]
  } else {
    span.ok({ path, force })                                                      // [NEW]
  }
  return result
}
```

**BL-WT-04/BL-WT-05 (compare/merge):** không có case RPC riêng ở Agent — dùng chung `git.exec`. Khi backend sau này gọi `relay.call('git.exec', { ..., _trace: { id: span.id } })`, `handleGitExec()` đã resume sẵn — KHÔNG cần sửa gì thêm phía agent. Không thêm code cho 2 sub-flow này trong task này.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep -E "agent-git-handler|tracers" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `Tracers.worktreeCreate`/`worktreeDelete` thêm vào `src/shared/trace/tracers.ts` đúng tên `worktree:create`/`worktree:delete`
- [ ] `handleGitWorktreeAdd()` phát span `worktree:create`: `ok({path, branch})` khi thành công, `fail()` khi validate lỗi (missing params, unsafe chars, path traversal)
- [ ] `handleGitWorktreeRemove()` phát span `worktree:delete`: `ok({path, force})` khi thành công, `fail()` khi lỗi
- [ ] Cả hai handler resume `id` từ `params._trace.id` khi có mặt, tự sinh `id` mới khi không có (backward-compatible)
- [ ] Khi gọi xuống `handleGitExec`, `id` của span `agent:git` bên trong TRÙNG với `id` của span `worktree:create`/`worktree:delete` bên ngoài (forward qua `_trace: { id: span.id }`)
- [ ] `handleGitExec()` resume từ `params._trace.id` — dùng chung được cho BL-WT-04/05 tương lai không cần sửa lại
- [ ] KHÔNG có `span.step()` nào cho việc parse output `git worktree list --porcelain` (biến đổi in-memory thuần tuý, theo CR-TRACE-000 §5)
- [ ] KHÔNG sửa `src/relay/git-handler.ts` (module khác — chỉ tái sử dụng qua dynamic import)
- [ ] `pnpm run typecheck:node` pass không lỗi liên quan 2 file trên
