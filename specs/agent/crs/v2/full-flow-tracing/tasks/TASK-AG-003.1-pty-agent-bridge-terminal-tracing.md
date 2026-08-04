# TASK-AG-003.1: Add terminal:create/resize/destroy spans to pty-agent-bridge.ts

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-003](../solutions/SOL-AG-TRACE-003-terminal-management.md)
**CR Ref:** [CR-TRACE-003](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-003-terminal-management.md)
**Precondition:** Phase 0 (`Tracer.start(fields?, resume?)`); idempotent với [TASK-AG-001.1](./TASK-AG-001.1-agent-git-handler-worktree-tracing.md) trên `tracers.ts` (thêm cộng dồn, không xung đột)
**Estimated time:** 2h
**Status:** ✅ Done (2026-08-03) — implemented against the current (2026-08 PTY-daemon-reattach) shape of the file, which had changed concurrently since this task was written: `handlePtyCreate`/`handlePtyAttach` now take a `notify` callback param, and a new `handlePtyAttach()` function exists (reattach after WS reconnect). Added a 4th tracer `Tracers.terminalReattach` (`terminal:reattach`) for it — not in the original spec below, kept `terminalCreate`/`Resize`/`Destroy` as designed. `handlePtyWrite`/`handlePtyScrollback` left untouched as specified.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
codegraph explore "handlePtyCreate"
codegraph explore "handlePtyResize"
codegraph explore "handlePtyDestroy"
codegraph explore "handlePtySendSignal"
```

Cả 5 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis cho từng symbol:

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
gitnexus_impact({ target: "handlePtyCreate", direction: "upstream" })
gitnexus_impact({ target: "handlePtyResize", direction: "upstream" })
gitnexus_impact({ target: "handlePtyDestroy", direction: "upstream" })
gitnexus_impact({ target: "handlePtySendSignal", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh — phạm vi hẹp hơn các CR khác

`pty-agent-bridge.ts` (case `pty.create/write/resize/destroy/scrollback/sendSignal` trong `agent-rpc-dispatch.ts`) chỉ dùng cho **PTY riêng của AI agent** — KHÔNG phải terminal người dùng thông thường (đó chạy trong Electron Main qua `LocalPtyProvider`/`SshPtyProvider`, ngoài phạm vi `src/relay/`). Toàn bộ thay đổi chỉ nằm trong file này.

**Quyết định thiết kế — KHÔNG trace `pty.write` và `pty.scrollback`:** `pty.write` (mỗi keystroke) tần suất cực cao, `pty.scrollback` chỉ đọc buffer in-memory (`Array.slice()`), không I/O — cả hai đều thuộc loại "biến đổi tần suất cao / in-memory thuần tuý" bị loại trừ theo CR-TRACE-000 §5. KHÔNG sửa `handlePtyWrite()`/`handlePtyScrollback()`.

## File: `src/shared/trace/tracers.ts` [MODIFY, idempotent]

Nếu TASK-AG-001.1 hoặc backend-side terminal-management solution đã thêm entry `terminal:*`, bỏ qua block này.

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries...

  // ─── CR-TRACE-003: Terminal Management ──────────────────────────────────────
  /** BL-TM-01/02 — create PTY (regular terminal.create/.split, AND agent-PTY pty.create) */
  terminalCreate:  createTracer('terminal:create'),
  /** BL-TM-02 — resize PTY */
  terminalResize:  createTracer('terminal:resize'),
  /** BL-TM-03 — destroy/save-scrollback PTY */
  terminalDestroy: createTracer('terminal:destroy'),
} as const
```

## File: `src/relay/pty-agent-bridge.ts` [MODIFY]

### Imports + resume helper

```typescript
// src/relay/pty-agent-bridge.ts
// (thêm vào phần import hiện có)
import type { AgentLogger } from './agent-logger'
import { Tracers } from '../shared/trace/tracers'   // [NEW]

// ─── Trace propagation helper ───────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested tại params._trace.id (CR-TRACE-000 §3.3).
// Hiện tại KHÔNG có caller thật nào gửi _trace cho pty.* — helper này là
// "forward-compatible": hoạt động đúng ngay khi caller phía Orca Server được nối vào.
function resumeFrom(params: Record<string, unknown>): { id: string } | undefined {   // [NEW]
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}
```

### `handlePtyCreate()` — `Tracers.terminalCreate`

```typescript
export async function handlePtyCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const cols       = typeof params.cols === 'number' ? params.cols : 80
  const rows       = typeof params.rows === 'number' ? params.rows : 24
  const rawCwd     = typeof params.cwd  === 'string' ? params.cwd  : ''
  const span = Tracers.terminalCreate.start(                                        // [NEW]
    { origin: 'agent-pty', cols, rows },
    resumeFrom(params)
  )

  const nodePty = await import('node-pty').catch(() => null)
  if (!nodePty) {
    span.fail('node-pty not available on this host')                               // [NEW]
    return {
      jsonrpc: '2.0', id,
      error: { code: -32603, message: 'node-pty is not available on this host' },
    }
  }

  const cwd        = safeCwd(rawCwd)
  const envOverride = (params.env && typeof params.env === 'object' && !Array.isArray(params.env))
    ? params.env as Record<string, string>
    : {}

  const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
  const envShell      = typeof envOverride.SHELL     === 'string' ? envOverride.SHELL.trim()   : ''
  const shell = shellOverride || envShell || (process.env.SHELL ?? '/bin/sh')

  const ptyId = `agent-pty-${nextAgentPtyId++}`

  try {
    span.step('node-pty-spawn', { shell, cwd })                                     // [NEW]
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const term = (nodePty as any).spawn(shell, [], {
      name: 'xterm-256color',
      cols,
      rows,
      cwd,
      env: { ...process.env, TERM: 'xterm-256color', ...envOverride } as NodeJS.ProcessEnv,
    })

    const entry: AgentPtyEntry = { pty: term, cwd, cols, rows, shell, buf: '' }
    AGENT_PTY_MAP.set(ptyId, entry)

    term.onData((data: string) => { appendScrollback(entry, data) })

    log.info(`pty.create (agent): id=${ptyId} cwd=${cwd} shell=${shell}`)
    span.ok({ ptyId, shell, cwd })                                                  // [NEW]
    return { jsonrpc: '2.0', id, result: { id: ptyId, cols, rows, cwd, shell } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    log.error(`pty.create (agent): failed: ${msg}`)
    span.fail(err, { cwd, shell })                                                  // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.create failed: ${msg}` } }
  }
}
```

### `handlePtyResize()` — `Tracers.terminalResize`

```typescript
export async function handlePtyResize(
  id:     string | number | null,
  params: Record<string, unknown>,
  _log:   AgentLogger,
): Promise<object> {
  const ptyId = typeof params.id   === 'string' ? params.id   : ''
  const cols  = typeof params.cols === 'number' ? params.cols : 80
  const rows  = typeof params.rows === 'number' ? params.rows : 24
  const span = Tracers.terminalResize.start({ origin: 'agent-pty', ptyId, cols, rows }, resumeFrom(params))  // [NEW]

  if (!ptyId) {
    span.fail('missing id')                                                         // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.resize: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span.fail('pty not found', { ptyId })                                           // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  try {
    entry.pty.resize(cols, rows)
    entry.cols = cols
    entry.rows = rows
    span.ok({ ptyId, cols, rows })                                                  // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { ptyId })                                                       // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.resize failed: ${msg}` } }
  }
}
```

### `handlePtyDestroy()` — `Tracers.terminalDestroy`

```typescript
export async function handlePtyDestroy(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const ptyId   = typeof params.id === 'string' ? params.id : ''
  const graceful = params.graceful !== false
  const span = Tracers.terminalDestroy.start({ origin: 'agent-pty', ptyId, graceful }, resumeFrom(params))  // [NEW]

  if (!ptyId) {
    span.fail('missing id')                                                         // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.destroy: missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span.ok({ ptyId, alreadyDead: true })                                           // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true, alreadyDead: true } }
  }

  try {
    if (process.platform === 'win32') { entry.pty.kill() }
    else { entry.pty.kill(graceful ? 'SIGTERM' : 'SIGKILL') }
    AGENT_PTY_MAP.delete(ptyId)
    log.info(`pty.destroy (agent): id=${ptyId}`)
    span.ok({ ptyId, graceful })                                                    // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { ptyId })                                                       // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.destroy failed: ${msg}` } }
  }
}
```

### `handlePtySendSignal()` — chỉ mở span `terminal:destroy` khi tín hiệu kết thúc tiến trình

```typescript
export async function handlePtySendSignal(
  id:     string | number | null,
  params: Record<string, unknown>,
  log:    AgentLogger,
): Promise<object> {
  const ptyId  = typeof params.id     === 'string' ? params.id     : ''
  const signal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'
  const isTerminating = signal === 'SIGKILL' || signal === 'SIGTERM'                // [NEW]
  const span = isTerminating                                                        // [NEW]
    ? Tracers.terminalDestroy.start({ origin: 'agent-pty', ptyId, signal, via: 'pty.sendSignal' }, resumeFrom(params))
    : undefined

  if (!ptyId) {
    span?.fail('missing id')                                                        // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: 'pty.sendSignal: missing id' } }
  }
  if (!ALLOWED_SIGNALS.has(signal)) {
    span?.fail(`signal not allowed: ${signal}`)                                     // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32602, message: `Signal not allowed: ${signal}` } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    span?.fail('pty not found', { ptyId })                                          // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `PTY not found: ${ptyId}` } }
  }

  try {
    if (process.platform !== 'win32') { entry.pty.kill(signal) }
    else { entry.pty.kill() }
    log.info(`pty.sendSignal (agent): id=${ptyId} signal=${signal}`)
    span?.ok({ ptyId, signal })                                                     // [NEW]
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span?.fail(err, { ptyId })                                                      // [NEW]
    return { jsonrpc: '2.0', id, error: { code: -32603, message: `pty.sendSignal failed: ${msg}` } }
  }
}
```

`handlePtyWrite()` và `handlePtyScrollback()` — **không sửa**, giữ nguyên nguyên trạng.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep -E "pty-agent-bridge|tracers" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [x] `Tracers.terminalCreate`/`terminalResize`/`terminalDestroy`/`terminalReattach` tồn tại trong `tracers.ts`
- [x] `handlePtyCreate()`, `handlePtyResize()`, `handlePtyDestroy()`, `handlePtyAttach()` phát span tương ứng, `ok()` chứa `ptyId` (+ `cols/rows`/`graceful`/`wasWithinGracePeriod`/`replayBytes` tuỳ method), `fail()` khi lỗi
- [x] Mọi span đều có field `origin: 'agent-pty'`
- [x] `handlePtySendSignal()` chỉ phát `terminal:destroy` khi `signal` là `SIGKILL` hoặc `SIGTERM` — không phát cho `SIGINT`/`SIGHUP`/`SIGTSTP`
- [x] `handlePtyWrite()` và `handlePtyScrollback()` KHÔNG sửa, KHÔNG phát bất kỳ span nào
- [x] Mọi span resume `id` từ `params._trace.id` khi có mặt
- [x] KHÔNG đụng `src/main/providers/local-pty-provider.ts`, `ssh-pty-provider.ts`, hay `terminal-scrollback-snapshots.ts`
- [x] `pnpm run typecheck:node` pass (0 lỗi liên quan `pty-agent-bridge`/`shared/trace`)
- [x] `gitnexus_detect_changes()` xác nhận chỉ symbol trong `pty-agent-bridge.ts` + `shared/trace/index.ts` bị ảnh hưởng
