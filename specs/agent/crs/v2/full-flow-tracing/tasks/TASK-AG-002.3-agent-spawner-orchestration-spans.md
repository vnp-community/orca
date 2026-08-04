# TASK-AG-002.3: Add agentOrch:spawn/resume/stop spans to agent-spawner.ts

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-002](../solutions/SOL-AG-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Precondition:** Phase 0 + [TASK-AG-002.1](./TASK-AG-002.1-agentorch-tracers-registration.md)
**Estimated time:** 2h
**Status:** ✅ Done (2026-08-03) — implemented exactly as specced, no concurrent drift found in the 3 target functions. 71/71 existing tests pass (agent-spawner.test.ts + sub-agent-spawner.test.ts); pre-existing unrelated `AgentConfig`/`AgentBinarySpec` type errors in those test files confirmed present before this change too (verified via `git status` — those test files are untouched).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "handleAgentSpawn"
codegraph explore "handleAgentKill"
codegraph explore "handleAgentSendInput"
```

Cả 3 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis cho từng symbol:

```
gitnexus_impact({ target: "handleAgentSpawn", direction: "upstream" })
gitnexus_impact({ target: "handleAgentKill", direction: "upstream" })
gitnexus_impact({ target: "handleAgentSendInput", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

`agent-spawner.ts::handleAgentSpawn()` (RPC `agent.spawn`, node-pty, interactive) đã có tracer hạ tầng `agent:spawn` (`spawnerTracer`) nhưng không resume và không phân biệt spawn mới (BL-AG-01) vs. resume session (BL-AG-03, dựa trên field `req.resumeId` đã tồn tại). `handleAgentKill()` tương tự thiếu domain span. `handleAgentSendInput()` hoàn toàn không có tracer.

**Quyết định thiết kế quan trọng:** `agent.sendInput` phục vụ 2 mục đích tần suất khác nhau — Ctrl+C (`data === '\x03'`, thấp tần suất, ĐÁNG trace là `agentOrch:stop`) và forward keystroke tương tác (cao tần suất, KHÔNG trace, theo CR-TRACE-000 §5). Chỉ mở span khi `data === '\x03'`.

## File: `src/relay/agent-spawner.ts` [MODIFY]

### Imports + resume helper (thêm)

```typescript
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

### `handleAgentSpawn()` — chọn `agentOrchResume` vs `agentOrchSpawn`

Giữ nguyên `spawnerTracer` hạ tầng (`agent:spawn`) song song với domain span mới `orchSpan` — 2 tracer khác granularity, cùng pattern `agent:rpc` + `agent:git` đã có.

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

  // [NEW] BL-AG-01 vs BL-AG-03: cùng code path, phân biệt bằng req.resumeId (đã tồn tại thật)
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

### `handleAgentKill()` — `agentOrch:stop`

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

### `handleAgentSendInput()` — CHỈ mở span khi Ctrl+C

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

**BL-AG-04 (switch account):** không có call site agent-side riêng — chuỗi tuần tự `agent.kill` (dùng `agentOrchStop`) rồi `agent.spawn`/`agent.exec` mới (dùng `agentOrchSpawn`), điều phối hoàn toàn từ backend. Không cần logic mới ở đây — nếu backend gửi `parentTraceId` trong params khi switch, đã pass-through tự nhiên qua cơ chế field trên.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-spawner" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `handleAgentSpawn()` chọn `Tracers.agentOrchResume` thay vì `agentOrchSpawn` khi `req.resumeId` có giá trị (BL-AG-03)
- [ ] Trường `env`/API key KHÔNG bao giờ xuất hiện trong field của bất kỳ span nào — chỉ `accountId`, `binary`, `modelId`, `ptyId`
- [ ] KHÔNG có span mới tạo cho mỗi `pty.onData` frame (BL-AG-05) — chỉ 1 `step('first-output')` khi chuyển từ idle sang running (dùng biến `firstOutputReported`)
- [ ] `handleAgentKill()` phát `agentOrch:stop` với `ok()` khi PTY tìm thấy và kill, và khi PTY đã chết (`note: 'already dead'`)
- [ ] `handleAgentSendInput()` chỉ phát `agentOrch:stop` khi `data === '\x03'` — KHÔNG phát span cho input tương tác thông thường
- [ ] Khi gửi qua Agent WS JSON-RPC 2.0, resume đọc từ `params._trace.id` (nested), không phải field phẳng `params.traceId`
- [ ] `spawnerTracer` (`agent:spawn`) hạ tầng giữ nguyên song song với `orchSpan` mới — không xoá
- [ ] `pnpm run typecheck:node` pass
