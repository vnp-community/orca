# SOL-AG-TRACE-001: Worktree Management — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-001](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-001-worktree-management.md)
**TDD Ref:** TDD-AG-10 (Git Handler Extension v5.0), TDD-AG-07 (JSON-RPC Dispatch)
**File(s):**
- `src/shared/trace/tracers.ts` [MODIFY]
- `src/relay/agent-git-handler.ts` [MODIFY]
**Mức độ:** 🟡 Trung bình
**Thời gian ước tính:** 3h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

CR-TRACE-001 mô tả 5 sub-flow (BL-WT-01 → 05). Sau khi đối chiếu với `src/relay/*.ts` thật (không phải `git-handler.ts` như TDD-AG-10 gợi ý ban đầu — xem ghi chú bên dưới), chỉ **2/5 sub-flow** có call site thật ở phía Agent:

| Sub-flow | Có call site Agent-side? | File thật |
|----------|---------------------------|-----------|
| BL-WT-01 (tạo worktree) | ✅ Có | `src/relay/agent-git-handler.ts` — `handleGitWorktreeAdd()` |
| BL-WT-03 (xoá worktree) | ✅ Có | `src/relay/agent-git-handler.ts` — `handleGitWorktreeRemove()` |
| BL-WT-02 (fan-out) | ❌ Không | Không có RPC method Orca-Server nào gọi xuống Agent (CR-TRACE-001 §1 mục 5) |
| BL-WT-04 (compare) | ⚠️ Gián tiếp | Chỉ đi qua `git.exec` chung, không có case riêng |
| BL-WT-05 (merge) | ⚠️ Gián tiếp | Chỉ đi qua `git.exec` chung, không có case riêng |

**Ghi chú doc/code drift phát hiện thêm (ngoài những gì CR-TRACE-001 đã ghi):** TDD-AG-10 §"v2.1 Integration Note" giả định module tên `src/relay/git-handler.ts`, nhưng RPC dispatcher (`agent-rpc-dispatch.ts:207-215, 341-360`) thực tế `import(...)` từ **`./agent-git-handler`**, không phải `./git-handler`. `src/relay/git-handler.ts` (1498 dòng) là module khác — dùng cho relay daemon thường (không phải Agent RPC), và chỉ được `agent-git-handler.ts` tái sử dụng 2 helper thuần (`parseWorktreePorcelain`, `validateWorktreePath`) qua dynamic import nội bộ (`agent-git-handler.ts:324, 358`). Toàn bộ code trong solution này chỉ sửa `agent-git-handler.ts`.

**Trong phạm vi (agent-side):**
- `src/relay/agent-git-handler.ts` — `handleGitWorktreeAdd()`, `handleGitWorktreeRemove()`, `handleGitExec()` (dùng chung bởi cả hai, và bởi BL-WT-04/05 khi được implement).
- `src/shared/trace/tracers.ts` — thêm entry `worktreeCreate`/`worktreeDelete` (file này isomorphic, dùng chung với backend; xem ghi chú idempotent ở mục 3).

**Ngoài phạm vi (thuộc solution set phía Orca Server/backend, không đụng ở đây):**
- `src/main/runtime/rpc/methods/worktree.ts` (`worktree.create`, `worktree.rm`)
- `src/main/runtime/rpc/methods/git-remote.ts` (`git.worktree.add/remove` RPC methods, `git.diff`)
- `src/main/runtime/orca-runtime.ts` (`createManagedWorktree`, `removeManagedWorktree`)
- `src/main/dev-server/dev-server-relay-bridge.ts` (`relayCallTracer`/`relay:agentCall` resume)
- `worktree-schemas.ts` (`traceId?: string` field bổ sung vào params schema)

**Điều kiện tiên quyết (không lặp lại code ở đây):** Solution này giả định `Tracer.start(fields?, resume?: { id: string })` đã được triển khai theo CR-TRACE-000 §3.1 (thay đổi lõi trong `src/shared/trace/index.ts`, dùng chung toàn hệ thống, không thuộc riêng CR-TRACE-001). Nếu chưa ship, mọi lệnh gọi `.start(fields, resume)` bên dưới sẽ lỗi kiểu (`start()` hiện tại chỉ nhận 1 tham số).

## 2. Gap hiện tại

| Vị trí | Hiện trạng | Gap |
|--------|-----------|-----|
| `agent-rpc-dispatch.ts` case `'git.worktree.add'` (L341-349) / `'git.worktree.remove'` (L352-360) / `'git.exec'` (L207-215) | Chỉ được bọc bởi tracer hạ tầng chung `agent:rpc` (`rpcTracer`, `dispatch()` L120-150) — đã có field extraction theo method group (`extractTraceFields()` L58-118, nhánh `git.*` ở L79-86 lấy `repo/cmd/branch/worktree`) | Không có domain span `worktree:create`/`worktree:delete`; `rpcTracer.start()` (L128) không nhận `resume` — span `id` luôn random mới (GAP-1 CR-TRACE-000) |
| `agent-git-handler.ts` — `handleGitExec()` (L98-167) | Có tracer hạ tầng riêng `agent:git` (`gitTracer`, khởi tạo L27, dùng L108) — log `cmd`/`cwd`, `ok()`/`fail()` theo exit code | Không resume từ `params`; không phân biệt được lệnh này phục vụ worktree hay domain nghiệp vụ nào khác (được gọi lại từ nhiều nơi: `handleGitWorktreeAdd`, `handleGitWorktreeRemove`, và tương lai là compare/merge) |
| `agent-git-handler.ts` — `handleGitWorktreeAdd()` (L338-370) / `handleGitWorktreeRemove()` (L374-394) | **Không có tracer nào** | Không có span `worktree:create`/`worktree:delete` — khi tạo/xoá worktree chậm hoặc lỗi (path traversal reject, git exit non-zero), không tách được bước validate vs. bước git exec |
| `src/shared/trace/tracers.ts` | Chỉ có `browseDirFlow`, `mkdirFlow`, `rmdirFlow`, `agentWsFlow`, `ipcProxyFlow` | Thiếu `worktreeCreate`, `worktreeDelete` theo tên `worktree:create`/`worktree:delete` (CR-TRACE-001 §3) |
| Toàn bộ `src/relay/` | Không có nơi nào đọc `params._trace` | Không có cách nhận `traceId` lan truyền từ Orca Server qua Agent WS JSON-RPC 2.0 (CR-TRACE-000 §3.3) |

## 3. Full Implementation

### 3.1 `src/shared/trace/tracers.ts` — thêm entry mới

File này isomorphic, dùng chung giữa Orca Server và Agent (đã được `agent-rpc-dispatch.ts`, `agent-git-handler.ts`, `agent-spawner.ts` import `createTracer` từ `../shared/trace`). Nếu solution phía backend (worktree.ts/git-remote.ts) merge trước, đoạn này đã tồn tại — coi thay đổi dưới đây là idempotent, không phải xung đột.

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

### 3.2 `src/relay/agent-git-handler.ts` — resume helper + domain spans

```typescript
// src/relay/agent-git-handler.ts
// (imports hiện có giữ nguyên, thêm 2 dòng sau)
import { createTracer } from '../shared/trace'
import { Tracers } from '../shared/trace/tracers'          // [NEW]

const gitTracer = createTracer('agent:git')

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

`handleGitExec()` — resume tracer hạ tầng `agent:git` (áp dụng cho MỌI caller của `git.exec`, kể cả BL-WT-04/05 tương lai khi chưa có domain span riêng — đảm bảo `id` vẫn nối tiếp xuyên process theo GAP-1, dù chưa gắn tên `worktree:*`):

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

`handleGitWorktreeAdd()` — thêm domain span `worktree:create` bọc ngoài (validate + delegate), tách biệt với `agent:git` span bên trong `handleGitExec` (2 tracer khác tên, cùng `id` nếu resume trùng nhau — xem field `parentSpan` không cần thiết vì cùng request, không phải fan-out):

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

`handleGitWorktreeRemove()` — tương tự, dùng `Tracers.worktreeDelete`:

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

> **BL-WT-04/BL-WT-05:** chưa có case RPC riêng ở Agent (dùng chung `git.exec`) — khi Orca Server implement `worktree.compare`/`worktree.merge` và gọi `relay.call('git.exec', { ..., traceId })` theo mẫu CR-TRACE-001 §4, chỉ cần Orca Server gửi `_trace: { id: span.id }` (không phải field phẳng `traceId`, theo phân tích ở mục 1) — phía Agent **không cần sửa gì thêm**, vì `handleGitExec()` đã resume `agent:git` từ `resumeFrom(params)` ở trên.

## 4. Test Plan (Vitest)

File test đã tồn tại: `src/relay/__tests__/agent-git-handler.test.ts` — thêm các case sau (dùng `registerTraceSink` từ `../../shared/trace` để bắt `TraceEvent[]` phát ra, theo pattern test isomorphic đã mô tả ở TDD-AG-01 §7):

```typescript
// src/relay/__tests__/agent-git-handler.test.ts (thêm)
import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import { handleGitWorktreeAdd, handleGitWorktreeRemove } from '../agent-git-handler'

describe('agent-git-handler — worktree tracing', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
  })
  afterEach(() => unregister())

  it('handleGitWorktreeAdd emits worktree:create span with ok() on success', async () => {
    // ... mock spawn('git', ['worktree','add',...]) → exit 0 ...
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'feature/x' }, config, log)
    const okEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'ok')
    expect(okEvent).toBeDefined()
  })

  it('handleGitWorktreeAdd emits fail() when path/branch missing', async () => {
    await handleGitWorktreeAdd(1, {}, config, log)
    const failEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'fail')
    expect(failEvent).toBeDefined()
  })

  it('resumes span id from params._trace.id when present', async () => {
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'x', _trace: { id: 'abc123' } }, config, log)
    const startEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'start')
    expect(startEvent?.id).toBe('abc123')
  })

  it('generates a new span id when params._trace is absent (backward-compat)', async () => {
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'x' }, config, log)
    const startEvent = events.find(e => e.flow === 'worktree:create' && e.level === 'start')
    expect(startEvent?.id).toBeDefined()
    expect(startEvent?.id).not.toBe('abc123')
  })

  it('handleGitWorktreeRemove emits worktree:delete span, forwards id to nested agent:git span', async () => {
    await handleGitWorktreeRemove(1, { path: '/tmp/wt1', _trace: { id: 'xyz789' } }, config, log)
    const deleteStart = events.find(e => e.flow === 'worktree:delete' && e.level === 'start')
    const gitStart     = events.find(e => e.flow === 'agent:git' && e.level === 'start')
    expect(deleteStart?.id).toBe('xyz789')
    expect(gitStart?.id).toBe('xyz789')   // nối tiếp id xuống agent:git qua _trace forward
  })
})
```

## 5. Acceptance Criteria

- [ ] `Tracers.worktreeCreate`/`worktreeDelete` được thêm vào `src/shared/trace/tracers.ts` đúng tên `worktree:create`/`worktree:delete`
- [ ] `handleGitWorktreeAdd()` phát span `worktree:create` với `ok()` chứa `{ path, branch }` khi thành công, `fail()` khi validate lỗi (missing params, unsafe chars, path traversal)
- [ ] `handleGitWorktreeRemove()` phát span `worktree:delete` tương tự với `{ path, force }`
- [ ] Cả hai handler resume `id` từ `params._trace.id` khi có mặt (Agent WS JSON-RPC 2.0 convention, CR-TRACE-000 §3.3), và tự sinh `id` mới khi không có (không phá backward compatibility với caller cũ chưa gửi `_trace`)
- [ ] Khi `handleGitWorktreeAdd`/`Remove` gọi xuống `handleGitExec`, `id` của span `agent:git` bên trong trùng với `id` của span `worktree:create`/`worktree:delete` bên ngoài (forward qua `_trace: { id: span.id }`)
- [ ] `handleGitExec()` (dùng chung, kể cả cho BL-WT-04/05 tương lai) resume từ `params._trace.id` — không cần sửa lại khi BL-WT-04/05 được implement phía Orca Server
- [ ] Không có `span.step()` nào thêm cho việc parse output `git worktree list --porcelain` (biến đổi in-memory thuần tuý, theo CR-TRACE-000 §5)
- [ ] `src/relay/__tests__/agent-git-handler.test.ts` có đủ test cases ở mục 4, pass với `vitest run`
