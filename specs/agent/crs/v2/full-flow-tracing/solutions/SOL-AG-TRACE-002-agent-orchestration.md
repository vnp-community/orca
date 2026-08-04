# SOL-AG-TRACE-002: Agent Orchestration — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**TDD Ref:** TDD-AG-12 (ProfileAware Agent Spawner — AI Agent CLI Host), TDD-AG-07 (JSON-RPC Dispatch), TDD-AG-06 (Tool Handlers)
**File(s):**
- `src/shared/trace/tracers.ts` [MODIFY]
- `src/relay/agent-rpc-dispatch.ts` [MODIFY]
- `src/relay/agent-spawner.ts` [MODIFY]
**Mức độ:** 🔴 Phức tạp
**Thời gian ước tính:** 5h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

Đây là luồng CORE phía Agent theo phân công — nhưng thực tế đọc code cho thấy **có 2 con đường "spawn agent" khác nhau, không phải 1**, và CR-TRACE-002 chỉ xác nhận rõ ràng 1 trong 2:

| Con đường | RPC method | Handler thật | Cơ chế | CR-TRACE-002 xác nhận? |
|-----------|-----------|---------------|--------|--------------------------|
| A | `agent.exec` | `agent-rpc-dispatch.ts` case inline (L502-557) | `node:child_process.spawn`, capture stdout/stderr/exitCode, KHÔNG PTY | ✅ Có — đây là target thật của `ProfileAwareAgentSpawner.spawn()` (Orca Server) theo file:line `ProfileAwareAgentSpawner.ts:115-121` mà CR-TRACE-002 đã verify |
| B | `agent.spawn` | `agent-spawner.ts` → `handleAgentSpawn()` (L247-416) | `node-pty.spawn`, PTY tương tác, state machine (`SubAgentSpawner`), hỗ trợ `resumeId` (`--resume <sessionId>`) | ⚠️ Không trực tiếp — nhưng đây chính là "AI Agent CLI Host" mà TDD-AG-12 mô tả, và nằm trong "Tác động" của CR-TRACE-002 (liệt kê `src/relay/agent-spawner.ts`) |

Cả hai đều là call site "spawn agent" hợp lệ phía Agent nên solution này instrument **cả hai**, đúng theo chỉ đạo nhiệm vụ ("instrument the agent-side handling of `agent.exec`/`agent.spawn` dispatch... and instrument its spawn/kill/status lifecycle"). Đây là một phát hiện doc/code drift bổ sung so với CR-TRACE-002 (vốn chỉ nói `agent.exec`), cần lưu ý khi review.

**Trong phạm vi (agent-side):**
- `agent-rpc-dispatch.ts` — `dispatch()` (L120-150, tracer hạ tầng `agent:rpc` dùng chung mọi method) và case `'agent.exec'` (L502-557).
- `agent-spawner.ts` — `handleAgentSpawn()` (BL-AG-01/03), `handleAgentKill()` (BL-AG-02), `handleAgentSendInput()` (BL-AG-02, chỉ nhánh Ctrl+C).
- `src/shared/trace/tracers.ts` — thêm 5 entry `agentOrch:*`.

**Ngoài phạm vi (backend/gateway, solution set khác):**
- `src/main/project/ProfileAwareAgentSpawner.ts` (`spawn()` — nơi phát span `agentOrch:spawn` gốc phía Orca Server)
- `src/main/project/ProjectServerRouter.ts`, `src/main/profile/ProfileResolver.ts`
- `src/main/dev-server/agent-ws-server.ts` (đã có `agentWs:lifecycle`, không đổi ở đây)
- `src/main/dev-server/dev-server-relay-bridge.ts` (`relayCallTracer`/`relay:agentCall` resume)

**Điều kiện tiên quyết:** giống SOL-AG-TRACE-001 mục 1 — giả định `Tracer.start(fields?, resume?)` (CR-TRACE-000 §3.1) đã ship trong `src/shared/trace/index.ts`.

### Quyết định thiết kế quan trọng: KHÔNG trace mọi lệnh `agent.sendInput`

`agent.sendInput` phục vụ **2 mục đích khác tần suất hoàn toàn khác nhau** (comment thật tại `agent-rpc-dispatch.ts:486-487`: "Used for graceful stop (Ctrl+C = '\x03') and interactive input"):
1. Gửi `\x03` (Ctrl+C) để dừng agent — tần suất thấp, đúng là BL-AG-02 (dừng agent), **đáng trace**.
2. Forward từng ký tự người dùng gõ vào PTY tương tác — tần suất cao (mỗi keystroke), **không đáng trace** theo nguyên tắc CR-TRACE-000 §5 (tương tự lý do BL-TM-04/BL-AG-05 không trace mỗi frame).

Solution này chỉ mở span `agentOrch:stop` khi `data === '\x03'`, các trường hợp khác đi qua không tạo span — đây là một quyết định không được CR-TRACE-002 nêu rõ (CR chỉ nói chung "gửi `agent.sendInput` (Ctrl+C)"), suy ra trực tiếp từ code thật và áp dụng nguyên tắc §5.

## 2. Gap hiện tại

| Vị trí | Hiện trạng | Gap |
|--------|-----------|-----|
| `agent-rpc-dispatch.ts` `dispatch()` (L126-150) | `rpcTracer.start({ method, id, ...ctxFields })` (L128) — `ctxFields` cho nhóm `agent.*` (L110-115) chỉ lấy `sessionId`/`cmd`\|`command`, **không lấy `binary`** dù `agent.exec`'s params thật dùng `p.binary` (L506) | Bug nhỏ do thiếu observability: field extraction cho `agent.*` không khớp với params thật của `agent.exec` — đúng loại lỗi mà CR-TRACE-002 §1 chỉ ra đã từng xảy ra ở `ProfileAwareAgentSpawner.ts:108-109` (field mapping `command/workdir` → `binary/args/cwd`). `rpcTracer.start()` cũng không nhận `resume` (GAP-1) |
| `agent-rpc-dispatch.ts` case `'agent.exec'` (L502-557) | Chỉ có `log.info()` (L551) khi xong; không có domain span | Không có `agentOrch:spawn` span — không tách được thời gian resolve binary/spawn subprocess vs. thời gian thực thi lệnh |
| `agent-spawner.ts` `handleAgentSpawn()` (L247-416) | Có tracer hạ tầng `agent:spawn` (`spawnerTracer`, khởi tạo L29, dùng L275/297/370/391/393/409) — đã có `step('pty-running')`, `ok()`/`fail()` theo exit code | Không resume từ `params._trace`; tên tracer là `agent:spawn` (hạ tầng), không phải `agentOrch:spawn` (domain) theo namespace CR-TRACE-000 §4; không phân biệt spawn mới (BL-AG-01) vs. resume session (BL-AG-03) dù field `req.resumeId` đã tồn tại thật (L54, L268) |
| `agent-spawner.ts` `handleAgentKill()` (L420-453) | `spawnerTracer.start()` (L430), `ok()`/`fail()` | Không resume; không phải `agentOrch:stop` |
| `agent-spawner.ts` `handleAgentSendInput()` (L459-486) | **Không có tracer nào** | Thiếu hoàn toàn — nhưng KHÔNG nên thêm tracer cho mọi lời gọi (xem quyết định thiết kế ở mục 1) |
| `src/shared/trace/tracers.ts` | Chưa có | Thiếu `agentOrchSpawn/Stop/Resume/Switch/StatusPoll` |
| Toàn bộ `src/relay/` | Không có | Không có `params._trace` extraction (giống SOL-AG-TRACE-001) |

## 3. Full Implementation

### 3.1 `src/shared/trace/tracers.ts` — thêm entry mới

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow)...
  // ...worktreeCreate, worktreeDelete từ SOL-AG-TRACE-001 nếu đã merge...

  // ─── CR-TRACE-002: Agent Orchestration ──────────────────────────────────────
  /** BL-AG-01 — spawn AI agent (agent.exec / agent.spawn) */
  agentOrchSpawn:      createTracer('agentOrch:spawn'),
  /** BL-AG-02 — stop agent (agent.kill / agent.sendInput Ctrl+C) */
  agentOrchStop:       createTracer('agentOrch:stop'),
  /** BL-AG-03 — resume agent session (agent.spawn với resumeId) */
  agentOrchResume:     createTracer('agentOrch:resume'),
  /** BL-AG-04 — switch account/provider (chưa có call site thật, đặt tên trước) */
  agentOrchSwitch:     createTracer('agentOrch:switch'),
  /** BL-AG-05 — polling loop rời rạc (KHÔNG dùng cho agent.output stream, xem CR-TRACE-002 §4) */
  agentOrchStatusPoll: createTracer('agentOrch:statusPoll'),
} as const
```

### 3.2 `src/relay/agent-rpc-dispatch.ts` — resume tracer hạ tầng `agent:rpc` (fix chung cho MỌI method)

Đây là điểm đòn bẩy cao nhất: sửa MỘT chỗ trong `dispatch()` để mọi RPC method (kể cả `git.*` ở SOL-AG-TRACE-001, `pty.*` ở SOL-AG-TRACE-003) đều resume `id` đúng khi Orca Server gửi `_trace.id`, không cần sửa từng case riêng lẻ.

```typescript
// src/relay/agent-rpc-dispatch.ts

// ─── Trace field extraction ───────────────────────────────────────────────
// (giữ nguyên extractTraceFields, chỉ SỬA nhánh agent.* để khớp params thật)
function extractTraceFields(method: string, params: Record<string, unknown>): TraceFields {
  const p = params
  const str = (v: unknown) => (typeof v === 'string' ? v : undefined)
  // ...num, truncPath, truncCmd giữ nguyên...

  // ... các nhánh fs./git./github./gitlab./ai.provider./tools/call giữ nguyên ...

  if (method.startsWith('agent.')) {
    return {
      session: str(p['sessionId'] ?? p['taskId']),      // [MODIFIED] agent.exec/agent.spawn dùng taskId, không phải sessionId
      binary:  str(p['binary'] ?? p['model'] ?? p['modelId']),  // [NEW] khớp params thật của agent.exec (p.binary) và agent.spawn (p.model)
      cmd:     truncCmd(p['cmd'] ?? p['command']),
    }
  }

  return {}
}

// ─── Trace resume extraction ────────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested tại params._trace.id (CR-TRACE-000 §3.3).
function extractResume(params: Record<string, unknown>): { id: string } | undefined {  // [NEW]
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}

export function createRpcDispatcher(
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger
): RpcDispatcher {
  return {
    async dispatch(ws: WebSocket, state: WireState, rpc: JsonRpcRequest): Promise<void> {
      const ctxFields = extractTraceFields(rpc.method, rpc.params ?? {})
      const span = rpcTracer.start(
        { method: rpc.method, id: String(rpc.id ?? 'notify'), ...ctxFields },
        extractResume(rpc.params ?? {})                                            // [MODIFIED]
      )
      let response: JsonRpcResponse
      try {
        response = await route(rpc, tools, config, log, ws, state)
        if ('error' in response) {
          span.fail(response.error.message, { method: rpc.method, code: response.error.code })
        } else {
          span.ok({ method: rpc.method })
        }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        log.error(`RPC dispatch unhandled error method=${rpc.method}: ${msg}`)
        span.fail(msg, { method: rpc.method, phase: 'dispatch' })
        response = makeError(rpc.id, AgentErrorCode.ServerError, `Internal error: ${msg}`)
      }

      if (ws.readyState === 1 /* WebSocket.OPEN */) {
        ws.send(encodeDataFrame(state, JSON.stringify(response)))
      }
    },
  }
}
```

Case `'agent.exec'` — thêm domain span `agentOrch:spawn` (đây là target thật của BL-AG-01, xem mục 1):

```typescript
    // ── v5.0: agent.exec ─────────────────────────────────────────────────────
    // TG-001/BL-AG-01: Non-interactive subprocess execution — target thật của
    // ProfileAwareAgentSpawner.spawn() (Orca Server) theo relay.call('agent.exec', ...).
    case 'agent.exec': {
      const p       = rpc.params ?? {}
      const binary  = typeof p.binary === 'string' ? p.binary : ''
      const span = Tracers.agentOrchSpawn.start(                                    // [NEW]
        { binary, taskId: typeof p.taskId === 'string' ? p.taskId : undefined },
        extractResume(p)
      )
      try {
        const { spawn } = await import('node:child_process')
        const args    = Array.isArray(p.args) ? (p.args as unknown[]).map(String) : []
        const cwd     = typeof p.cwd      === 'string' ? p.cwd                    : config.workDir
        const stdin   = typeof p.stdin    === 'string' ? p.stdin                  : null
        const extraEnv = (p.env && typeof p.env === 'object' && !Array.isArray(p.env))
          ? p.env as Record<string, string>
          : {}
        const timeoutMs = typeof p.timeoutMs === 'number'
          ? Math.min(Math.max(p.timeoutMs, 1_000), 5 * 60_000)
          : 300_000

        if (!binary) {
          span.fail('binary is required')                                          // [NEW]
          return makeError(rpc.id, AgentErrorCode.InvalidParams, 'agent.exec: binary is required')
        }

        span.step('subprocess-spawn', { binary, cwd })                             // [NEW]
        const result = await new Promise<{
          stdout: string; stderr: string; exitCode: number | null; timedOut: boolean
        }>((resolve) => {
          let stdout = '', stderr = '', timedOut = false, settled = false
          const spawnEnv = { ...process.env, ...extraEnv } as NodeJS.ProcessEnv
          const child = spawn(binary, args, { cwd, env: spawnEnv, stdio: ['pipe', 'pipe', 'pipe'] })

          const finish = (r: typeof result): void => {
            if (settled) return
            settled = true
            clearTimeout(timer)
            resolve(r)
          }
          const timer = setTimeout(() => {
            timedOut = true
            try { child.kill('SIGKILL') } catch { /* ignore */ }
            finish({ stdout, stderr, exitCode: null, timedOut })
          }, timeoutMs)

          child.stdout?.on('data', (d: Buffer) => { stdout += d.toString('utf8') })
          child.stderr?.on('data', (d: Buffer) => { stderr += d.toString('utf8') })
          child.on('error', (err) => {
            finish({ stdout, stderr: err.message, exitCode: null, timedOut })
          })
          child.on('close', (code) => { finish({ stdout, stderr, exitCode: code, timedOut }) })

          if (stdin !== null) child.stdin?.end(stdin)
          else child.stdin?.end()
        })

        log.info(`agent.exec: binary=${binary} exitCode=${result.exitCode} timedOut=${result.timedOut}`)
        if (result.timedOut) {
          span.fail(`timeout after ${timeoutMs}ms`, { binary })                     // [NEW]
        } else if (result.exitCode !== 0) {
          span.fail(`exit code ${result.exitCode}`, { binary, exitCode: result.exitCode ?? -1 })  // [NEW]
        } else {
          span.ok({ binary, exitCode: result.exitCode ?? 0 })                       // [NEW]
        }
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        span.fail(err, { binary })                                                  // [NEW]
        return makeError(rpc.id, AgentErrorCode.ServerError, `agent.exec failed: ${msg}`)
      }
    }
```

Cần thêm import ở đầu file:

```typescript
import { Tracers } from '../shared/trace/tracers'   // [NEW]
```

### 3.3 `src/relay/agent-spawner.ts` — `agentOrch:spawn`/`agentOrch:resume`/`agentOrch:stop`

```typescript
// src/relay/agent-spawner.ts
import { createTracer } from '../shared/trace'
import { Tracers } from '../shared/trace/tracers'          // [NEW]

const spawnerTracer = createTracer('agent:spawn')

function extractResume(params: Record<string, unknown>): { id: string } | undefined {   // [NEW]
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}
```

`handleAgentSpawn()` — chọn `agentOrchResume` khi `req.resumeId` có mặt (BL-AG-03), ngược lại `agentOrchSpawn` (BL-AG-01). Giữ nguyên `spawnerTracer` hạ tầng (`agent:spawn`) song song — 2 tracer khác granularity, giống pattern `agent:rpc` + `agent:git` đã có trong codebase:

```typescript
export async function handleAgentSpawn(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
  ws:     WebSocket,
  _state: WireState,
): Promise<Record<string, unknown>> {
  const wireState = createWireState()

  const modelId = typeof params.model   === 'string' ? params.model
                : typeof params.modelId === 'string' ? params.modelId
                : ''

  const req: AgentSpawnRequest = {
    taskId:        typeof params.taskId      === 'string' ? params.taskId      : '',
    userId:        typeof params.userId      === 'string' ? params.userId      : '',
    modelId,
    accountId:     typeof params.accountId   === 'string' ? params.accountId   : '',
    cwd:           typeof params.cwd         === 'string' ? params.cwd         : undefined,
    resumeId:      typeof params.resumeId    === 'string' ? params.resumeId    : undefined,
    worktreePath:  typeof params.worktreePath === 'string' ? params.worktreePath : undefined,
    branchName:    typeof params.branchName   === 'string' ? params.branchName   : undefined,
  }
  const resolvedApiKey = typeof params.resolvedApiKey === 'string' ? params.resolvedApiKey : undefined

  const span = spawnerTracer.start({ method: 'agent.spawn', taskId: req.taskId, modelId: req.modelId })

  // [NEW] BL-AG-01 vs BL-AG-03: cùng code path, phân biệt bằng req.resumeId (đã tồn tại thật — ORCH-009)
  const orchTracer = req.resumeId ? Tracers.agentOrchResume : Tracers.agentOrchSpawn
  const orchSpan = orchTracer.start(
    { taskId: req.taskId, modelId: req.modelId, resumeId: req.resumeId },
    extractResume(params)
  )

  const missing: string[] = []
  if (!req.modelId)  missing.push('model')
  if (!req.taskId)   missing.push('taskId')
  if (!req.userId)   missing.push('userId')
  if (!req.cwd)      missing.push('cwd')

  if (missing.length > 0) {
    span.fail(`missing ${missing.join(',')}`, { taskId: req.taskId, modelId: req.modelId })
    orchSpan.fail(`missing ${missing.join(',')}`, { taskId: req.taskId })          // [NEW]
    const errResp = {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: `Missing required fields: ${missing.join(', ')}` },
    }
    try { ws.send(encodeDataFrame(wireState, JSON.stringify(errResp))) } catch { /* WS may be closed */ }
    return errResp
  }

  const specResolved = resolveAgentSpec(req.modelId)
  if (!specResolved) {
    span.fail('unknown model', { modelId: req.modelId })
    orchSpan.fail('unknown model', { modelId: req.modelId })                       // [NEW]
    const errResp = {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: `Unknown model: ${req.modelId}` },
    }
    try { ws.send(encodeDataFrame(wireState, JSON.stringify(errResp))) } catch { /* WS may be closed */ }
    return errResp
  }

  const spawner = new SubAgentSpawner()

  try {
    spawner.transition('spawning')

    const spec = specResolved
    orchSpan.step('resolve-credential', { accountId: req.accountId || '(none)' })  // [NEW] — KHÔNG log giá trị key, chỉ accountId
    const envBase = await buildAgentEnv(req, spec, config, resolvedApiKey ?? null, log)
    const env: Record<string, string> = {
      ...envBase,
      ...(req.worktreePath ? { ORCA_WORKTREE_PATH: req.worktreePath } : {}),
      ...(req.branchName   ? { ORCA_WORKTREE_BRANCH: req.branchName  } : {}),
    }

    const ptyId = `pty-${req.userId}-${req.taskId}-${Date.now()}`
    const args = buildAgentArgs(spec, req)

    const { existsSync: fsExistsSync } = await import('node:fs')
    const { join: pathJoin } = await import('node:path')
    const toolPathDirs = (config.toolPath ?? '').split(':').filter(Boolean)
    const binaryExists = process.platform === 'win32'
      ? true
      : toolPathDirs.some((dir) => fsExistsSync(pathJoin(dir, spec.binary)))
        || !toolPathDirs.length

    if (!binaryExists) {
      throw new Error(
        `Agent binary '${spec.binary}' not found in toolPath '${config.toolPath ?? '(empty)'}'. ` +
        `Install it or set toolPath to the directory containing '${spec.binary}'.`
      )
    }

    orchSpan.step('node-pty-spawn', { binary: spec.binary, ptyId })                // [NEW]
    let nodePty: typeof nodePtyTypes
    try {
      nodePty = await import('node-pty')
    } catch {
      throw new Error(
        `node-pty is not installed on this dev server. ` +
        `Run: npm install node-pty  (in ~/orca-agent/)  to enable PTY-based agent spawning.`
      )
    }

    const pty = nodePty.spawn(spec.binary, args, {
      name: 'xterm-256color',
      cols: 220, rows: 50,
      cwd:  req.cwd ?? config.workDir,
      env,
    })

    PTY_REGISTRY.set(ptyId, { pty, taskId: req.taskId, userId: req.userId })
    spawner.transition('running')
    span.step('pty-running', { ptyId, modelId: req.modelId })

    log.info(`agent.spawn: ptyId=${ptyId} model=${req.modelId}`)

    // [NEW] BL-AG-05: state-transition trên span ĐANG MỞ, KHÔNG tạo span mới mỗi frame
    let firstOutputReported = false
    pty.onData((data) => {
      if (!firstOutputReported) {
        firstOutputReported = true
        orchSpan.step('first-output', { ptyId })                                   // [NEW] — chỉ 1 lần, không phải mỗi chunk
      }
      const notification = JSON.stringify({
        jsonrpc: '2.0',
        method: 'agent.output',
        params: { ptyId, data: Buffer.from(data).toString('base64') },
      })
      ws.send(encodeDataFrame(wireState, notification))
    })

    pty.onExit(({ exitCode }) => {
      PTY_REGISTRY.delete(ptyId)
      spawner.transition('stopping')
      spawner.transition('stopped')
      if (exitCode === 0) {
        span.ok({ ptyId, exitCode })
        orchSpan.ok({ ptyId, exitCode })                                           // [NEW]
      } else {
        span.fail(`exit code ${exitCode}`, { ptyId, exitCode })
        orchSpan.fail(`exit code ${exitCode}`, { ptyId, exitCode })                // [NEW]
      }
      const notification = JSON.stringify({
        jsonrpc: '2.0',
        method: 'agent.exited',
        params: { ptyId, exitCode },
      })
      ws.send(encodeDataFrame(wireState, notification))
      log.info(`agent.spawn: ptyId=${ptyId} exited code=${exitCode}`)
    })

    return { jsonrpc: '2.0', id, result: { ok: true, ptyId } }

  } catch (err: unknown) {
    spawner.transition('error')
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { taskId: req.taskId, modelId: req.modelId })
    orchSpan.fail(err, { taskId: req.taskId, modelId: req.modelId })               // [NEW]
    log.error(`agent.spawn: error ${msg}`)
    const errWireState = createWireState()
    const errResp = { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
    ws.send(encodeDataFrame(errWireState, JSON.stringify(errResp)))
    return errResp
  }
}
```

`handleAgentKill()` — thêm `agentOrch:stop`:

```typescript
export async function handleAgentKill(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const rawSignal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'
  const signal: 'SIGTERM' | 'SIGKILL' = rawSignal === 'SIGKILL' ? 'SIGKILL' : 'SIGTERM'
  const span  = spawnerTracer.start({ method: 'agent.kill', ptyId: ptyId || '(empty)', signal })
  const orchSpan = Tracers.agentOrchStop.start({ ptyId: ptyId || '(empty)', signal, via: 'agent.kill' }, extractResume(params))  // [NEW]

  if (!ptyId) {
    span.fail('missing ptyId', { method: 'agent.kill' })
    orchSpan.fail('missing ptyId')                                                 // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    span.ok({ ptyId, note: 'already dead' })
    orchSpan.ok({ ptyId, note: 'already dead' })                                   // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true, note: 'pty not found (already dead)' } }
  }

  if (process.platform === 'win32') {
    entry.pty.kill()
  } else {
    entry.pty.kill(signal)
  }
  PTY_REGISTRY.delete(ptyId)
  span.ok({ ptyId, signal })
  orchSpan.ok({ ptyId, signal })                                                   // [NEW]
  log.info(`agent.kill: ptyId=${ptyId} ${signal} sent`)
  return { jsonrpc: '2.0', id, result: { ok: true } }
}
```

`handleAgentSendInput()` — CHỈ mở span khi `data === '\x03'` (Ctrl+C, xem quyết định thiết kế mục 1):

```typescript
export async function handleAgentSendInput(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  const data  = typeof params.data  === 'string' ? params.data  : ''
  const isGracefulStop = data === '\x03'                                           // [NEW]
  const orchSpan = isGracefulStop                                                  // [NEW]
    ? Tracers.agentOrchStop.start({ ptyId, via: 'agent.sendInput' }, extractResume(params))
    : undefined

  if (!ptyId) {
    orchSpan?.fail('missing ptyId')                                                // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing ptyId' } }
  }

  const entry = PTY_REGISTRY.get(ptyId)
  if (!entry) {
    orchSpan?.fail('pty not found', { ptyId })                                     // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.write(data)
    log.info(`agent.sendInput: ptyId=${ptyId} bytes=${data.length}`)
    orchSpan?.ok({ ptyId })                                                        // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`agent.sendInput failed: ${msg}`)
    orchSpan?.fail(err, { ptyId })                                                 // [NEW]
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

> **BL-AG-04 (switch account):** không có call site agent-side riêng — là chuỗi tuần tự `agent.kill` (dùng `agentOrchStop` ở trên) rồi `agent.spawn`/`agent.exec` mới (dùng `agentOrchSpawn` ở trên), điều phối hoàn toàn từ Orca Server. Nếu Orca Server gửi field `parentTraceId` trong `params` khi switch, forward nó vào field của span (`orchSpan.start({..., parentTraceId: params.parentTraceId})`) — không cần logic mới phía Agent ngoài việc đọc field này qua (đã có sẵn cơ chế field pass-through ở trên).

## 4. Test Plan (Vitest)

File test đã tồn tại: `src/relay/__tests__/agent-rpc-dispatch.test.ts`, `src/relay/__tests__/agent-spawner.test.ts` — thêm:

```typescript
// src/relay/__tests__/agent-rpc-dispatch.test.ts (thêm)
describe('dispatch() — trace resume', () => {
  it('resumes agent:rpc span id from params._trace.id', async () => { /* ... */ })
  it('generates a new span id when params._trace is absent', async () => { /* ... */ })
})

describe("case 'agent.exec' — agentOrch:spawn", () => {
  it('emits agentOrch:spawn span with ok() containing exitCode on success', async () => { /* ... */ })
  it('emits fail() when binary is missing', async () => { /* ... */ })
  it('emits fail() with timeout field when subprocess times out', async () => { /* ... */ })
})
```

```typescript
// src/relay/__tests__/agent-spawner.test.ts (thêm)
import { registerTraceSink, type TraceEvent } from '../../shared/trace'

describe('handleAgentSpawn — agentOrch tracing', () => {
  it('emits agentOrch:spawn when resumeId is absent (BL-AG-01)', async () => { /* ... */ })
  it('emits agentOrch:resume instead of agentOrch:spawn when resumeId is present (BL-AG-03)', async () => { /* ... */ })
  it('does not emit a new span per pty.onData frame (BL-AG-05) — only one "first-output" step', async () => {
    // simulate pty.onData firing 50 times → assert only 1 step event with label 'first-output'
  })
  it('resumes span id from params._trace.id', async () => { /* ... */ })
})

describe('handleAgentKill — agentOrch:stop', () => {
  it('emits agentOrch:stop span with ok() when pty found and killed', async () => { /* ... */ })
  it('emits ok() with note=already dead when ptyId not in registry', async () => { /* ... */ })
})

describe('handleAgentSendInput — agentOrch:stop (Ctrl+C only)', () => {
  it('emits agentOrch:stop span when data === "\\x03"', async () => { /* ... */ })
  it('does NOT emit any span for arbitrary interactive keystrokes', async () => {
    // assert zero TraceEvent with flow='agentOrch:stop' when data='a'
  })
})
```

## 5. Acceptance Criteria

- [ ] `Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll` thêm vào `tracers.ts` đúng tên `agentOrch:spawn|stop|resume|switch|statusPoll`
- [ ] `dispatch()` trong `agent-rpc-dispatch.ts` resume tracer hạ tầng `agent:rpc` từ `params._trace.id` cho MỌI method (không riêng agent.*) — fix GAP-1 tập trung tại một điểm
- [ ] Case `'agent.exec'` phát `agentOrch:spawn` span, `ok()` chứa `{ binary, exitCode }`, `fail()` khi timeout hoặc exitCode ≠ 0
- [ ] `handleAgentSpawn()` chọn `agentOrchResume` thay vì `agentOrchSpawn` khi `req.resumeId` có giá trị (BL-AG-03), dựa trên field thật đã tồn tại trong code (`AgentSpawnRequest.resumeId`)
- [ ] Trường `env`/API key KHÔNG bao giờ xuất hiện trong field của bất kỳ span nào — chỉ `accountId`, `binary`, `modelId`, `ptyId`
- [ ] KHÔNG có span mới tạo cho mỗi `pty.onData` frame (BL-AG-05) — chỉ 1 `step('first-output')` khi chuyển từ idle sang running
- [ ] `handleAgentSendInput()` chỉ phát `agentOrch:stop` khi `data === '\x03'` — không phát span cho input tương tác thông thường (xác nhận bằng test đếm span count)
- [ ] Khi gửi qua Agent WS JSON-RPC 2.0, tất cả các resume đều đọc từ `params._trace.id` (nested), không phải field phẳng `params.traceId`
