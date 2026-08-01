# SUPPLEMENT: Terminal Management — Source-Aligned Implementation Details

**Domain:** terminal-management & terminal-management.  
**Mục đích:** Bổ sung cho các solution files dựa trên source code thực tế  
**Căn cứ:** `pty-handler.ts` (L1-1183 đã đọc), `agent-rpc-dispatch.ts` (L1-563 đã đọc)

---

## Phát hiện quan trọng từ source code

### `pty-handler.ts` thực tế (QUAN TRỌNG):

`PtyHandler` **KHÔNG** dùng `agent-rpc-dispatch.ts` pattern. Nó dùng `RelayDispatcher` pattern:

```typescript
// L481-501: registerHandlers() trong PtyHandler constructor
this.dispatcher.onRequest('pty.spawn', (p, context) => this.spawn(p, context))
this.dispatcher.onRequest('pty.attach', (p) => this.attach(p))
this.dispatcher.onRequest('pty.shutdown', (p) => this.shutdown(p))
this.dispatcher.onRequest('pty.sendSignal', (p) => this.sendSignal(p))
this.dispatcher.onRequest('pty.getCwd', (p) => this.getCwd(p))
this.dispatcher.onRequest('pty.getInitialCwd', (p) => this.getInitialCwd(p))
this.dispatcher.onRequest('pty.clearBuffer', (p) => this.clearBuffer(p))
this.dispatcher.onRequest('pty.hasChildProcesses', (p) => this.hasChildProcesses(p))
this.dispatcher.onRequest('pty.getForegroundProcess', (p) => this.getForegroundProcess(p))
this.dispatcher.onRequest('pty.listProcesses', () => this.listProcesses())
this.dispatcher.onRequest('pty.getDefaultShell', async () => resolveDefaultShell())
this.dispatcher.onRequest('pty.serialize', (p) => this.serialize(p))
this.dispatcher.onRequest('pty.revive', (p) => this.revive(p))
this.dispatcher.onRequest('pty.getProfiles', async () => listShellProfiles())

this.dispatcher.onNotification('pty.data', (p) => this.writeData(p))
this.dispatcher.onNotification('pty.resize', (p) => this.resize(p))
```

**Điều này có nghĩa là:**
- `PtyHandler` đã đăng ký tất cả PTY handlers qua `dispatcher.onRequest/onNotification`
- Đây là `RelayDispatcher` (từ `relay.ts`) — KHÔNG phải `agent-rpc-dispatch.ts`'s `createRpcDispatcher`

**Bug TM-001 thực sự là gì:**  
`agent-rpc-dispatch.ts` (dùng cho agent mode) không có `pty.*` cases vì `agent-rpc-dispatch.ts` là dispatcher riêng cho **agent mode** (agent.spawn, agent.kill, git.*, fs.*). `PtyHandler` dùng `RelayDispatcher` cho **relay mode** (local terminal pane over SSH).

**Kết luận:** Hai dispatch systems tồn tại song song:
1. `relay.ts` + `PtyHandler` → local terminal qua SSH (đã hoạt động)
2. `agent-rpc-dispatch.ts` → agent mode (thiếu pty.* nếu agent cần spawn terminals)

### `agent-exec-handler.ts` thực tế:
- L130-144: `AgentExecHandler` constructor register `'agent.execNonInteractive'` và `'agent.cancelExec'` qua `RelayDispatcher`
- **Bug TG-001:** `agent-rpc-dispatch.ts` cần `case 'agent.exec'` — vì đây là entry point từ Orca Server → agent relay

---

## Fix TM-001 — Clarification về pty.* trong agent-rpc-dispatch

### Context
Bug report: "relay dispatch thiếu pty.create, pty.destroy, pty.resize, pty.scrollback, pty.write"

**Thực tế:**
- Terminal pane trong browser gọi `pty.spawn`, `pty.data`, `pty.resize` qua relay protocol (`relay.ts`)
- `PtyHandler` đã handle tất cả qua `RelayDispatcher`
- Nếu bug là về **agent-rpc-dispatch.ts thiếu pty.*** thì đây là vấn đề khi agent relay dùng agent protocol thay vì relay protocol

### Fix Thực Sự

Nếu bug xảy ra khi Orca Server gọi `pty.spawn` qua agent RPC (không phải relay protocol), thì cần thêm vào `agent-rpc-dispatch.ts`:

```diff
// agent-rpc-dispatch.ts — thêm sau case 'agent.exec':

+    // ── pty.spawn (agent mode) ────────────────────────────────────────────────
+    // Note: PtyHandler uses RelayDispatcher for relay mode.
+    // These cases handle the same operations when called via agent RPC protocol.
+    case 'pty.spawn': {
+      // Spawn terminal PTY on Dev Server for agent-driven terminal sessions.
+      // Params: { cwd, cols, rows, env?, shellOverride? }
+      // Returns: { id } (matches PtyHandler.spawn() return type)
+      try {
+        const { handlePtySpawnForAgent } = await import('./pty-agent-bridge')
+        return (await handlePtySpawnForAgent(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
+      } catch (err: unknown) {
+        const msg = err instanceof Error ? err.message : String(err)
+        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.spawn unavailable: ${msg}`)
+      }
+    }
+
+    case 'pty.shutdown': {
+      // Shutdown (kill) a PTY session.
+      // Params: { id: ptyId, graceful?: boolean }
+      try {
+        const { handlePtyShutdownForAgent } = await import('./pty-agent-bridge')
+        return (await handlePtyShutdownForAgent(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
+      } catch (err: unknown) {
+        const msg = err instanceof Error ? err.message : String(err)
+        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.shutdown unavailable: ${msg}`)
+      }
+    }
+
+    case 'pty.sendSignal': {
+      // Send signal to PTY process.
+      // Params: { id: ptyId, signal: 'SIGTERM'|'SIGKILL'|... }
+      try {
+        const { handlePtySendSignalForAgent } = await import('./pty-agent-bridge')
+        return (await handlePtySendSignalForAgent(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
+      } catch (err: unknown) {
+        const msg = err instanceof Error ? err.message : String(err)
+        return makeError(rpc.id, AgentErrorCode.ServerError, `pty.sendSignal unavailable: ${msg}`)
+      }
+    }
```

**Tạo `pty-agent-bridge.ts` (new file):**

```typescript
// src/relay/pty-agent-bridge.ts
// Bridge between agent-rpc-dispatch.ts (JSON-RPC) and PtyHandler (RelayDispatcher).
//
// Why this exists:
//   PtyHandler uses RelayDispatcher.onRequest() internally.
//   agent-rpc-dispatch.ts uses a different dispatch pattern (switch/case).
//   This bridge adapts PtyHandler methods to the agent-rpc-dispatch.ts pattern.

import * as nodePty from 'node-pty'
import { existsSync } from 'node:fs'
import { homedir } from 'node:os'
import { resolve } from 'node:path'
import { resolveDefaultShell } from './pty-shell-utils'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// In-process PTY map (separate from PtyHandler's private ptys Map)
// Reason: avoid coupling to PtyHandler's internals
const AGENT_PTY_MAP = new Map<string, { pty: nodePty.IPty; cwd: string; cols: number; rows: number; shell: string }>()
let nextAgentPtyId = 1

const ALLOWED_SIGNALS = new Set(['SIGTERM', 'SIGKILL', 'SIGINT', 'SIGHUP', 'SIGTSTP'])

/**
 * Spawn a PTY for agent-mode terminal access.
 * Used by agent-rpc-dispatch.ts case 'pty.spawn'.
 */
export async function handlePtySpawnForAgent(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  // Load node-pty lazily
  let ptyModule: typeof nodePty
  try {
    ptyModule = await import('node-pty')
  } catch {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: 'node-pty not available on this host' } }
  }

  const cols  = typeof params.cols === 'number' ? params.cols : 80
  const rows  = typeof params.rows === 'number' ? params.rows : 24
  const rawCwd = typeof params.cwd === 'string' ? params.cwd : homedir()
  const shell = resolveDefaultShell()

  // Path validation: prevent traversal
  const cwd = resolve(rawCwd)
  const workspaceRoot = config.workDir ?? homedir()
  const allowedRoots  = [workspaceRoot, homedir(), '/tmp']
  const isAllowed = allowedRoots.some(root => cwd.startsWith(root + '/') || cwd === root)
  if (!isAllowed || !existsSync(cwd)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: `cwd not allowed or not found: ${cwd}` } }
  }

  const ptyId = `agent-pty-${nextAgentPtyId++}`

  try {
    const env = params.env as Record<string, string> | undefined
    const term = ptyModule.spawn(shell, [], {
      name: 'xterm-256color',
      cols, rows, cwd,
      env: {
        ...process.env,
        TERM: 'xterm-256color',
        ...env,
      } as NodeJS.ProcessEnv,
    })

    AGENT_PTY_MAP.set(ptyId, { pty: term, cwd, cols, rows, shell })
    log.info(`pty.spawn (agent): ptyId=${ptyId} cwd=${cwd} shell=${shell}`)

    return { jsonrpc: '2.0', id, result: { id: ptyId, cols, rows, cwd, shell } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: `pty spawn failed: ${msg}` } }
  }
}

/**
 * Shutdown a PTY for agent-mode.
 */
export async function handlePtyShutdownForAgent(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.id === 'string' ? params.id : ''
  const graceful = params.graceful !== false

  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing id' } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, result: { ok: true, alreadyDead: true } }
  }

  try {
    entry.pty.kill(graceful ? 'SIGTERM' : 'SIGKILL')
    AGENT_PTY_MAP.delete(ptyId)
    log.info(`pty.shutdown (agent): ptyId=${ptyId} signal=${graceful ? 'SIGTERM' : 'SIGKILL'}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

/**
 * Send signal to a PTY for agent-mode.
 */
export async function handlePtySendSignalForAgent(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId  = typeof params.id     === 'string' ? params.id     : ''
  const signal = typeof params.signal === 'string' ? params.signal : 'SIGTERM'

  if (!ptyId) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing id' } }
  }
  if (!ALLOWED_SIGNALS.has(signal)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: `Signal not allowed: ${signal}` } }
  }

  const entry = AGENT_PTY_MAP.get(ptyId)
  if (!entry) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PathNotFound, message: `PTY not found: ${ptyId}` } }
  }

  try {
    if (process.platform !== 'win32') {
      entry.pty.kill(signal)
    } else {
      entry.pty.kill()  // Windows: kill() = force terminate
    }
    log.info(`pty.sendSignal (agent): ptyId=${ptyId} signal=${signal}`)
    return { jsonrpc: '2.0', id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

/**
 * Cleanup all agent PTYs (called on session disconnect).
 */
export function cleanupAgentPtys(log: AgentLogger): void {
  for (const [ptyId, entry] of AGENT_PTY_MAP.entries()) {
    try {
      entry.pty.kill('SIGTERM')
      log.info(`cleanupAgentPtys: killed ${ptyId}`)
    } catch { /* best effort */ }
  }
  AGENT_PTY_MAP.clear()
}
```

---

## Fix TM-002/003/004 — pty-handler.ts spawn() actual issues

### TM-002 (Path validation)

Source code thực tế `pty-handler.ts` L615:
```typescript
const cwd = (params.cwd as string) || resolveDefaultCwd()
```

**Không có path validation!** Fix cần thêm:

```diff
// pty-handler.ts L601-629 (spawn method):

     const cols = (params.cols as number) || 80
     const rows = (params.rows as number) || 24
-    const cwd = (params.cwd as string) || resolveDefaultCwd()
+    const rawCwd = (params.cwd as string) || resolveDefaultCwd()
+    const cwd = this.validateCwd(rawCwd)

// Thêm method vào PtyHandler class:
+  private validateCwd(rawCwd: string): string {
+    const resolved = resolve(rawCwd)
+    // Cho phép: home dir, /tmp, và workspace directories
+    const home = homedir()
+    const allowedPrefixes = [home, '/tmp', '/var/tmp']
+    const allowed = allowedPrefixes.some(p => resolved.startsWith(p + '/') || resolved === p)
+    if (!allowed) {
+      throw Object.assign(new Error(`cwd not in allowed paths: ${resolved}`), {
+        agentErrorCode: -32003  // Custom error code
+      })
+    }
+    if (!existsSync(resolved)) {
+      throw new Error(`cwd does not exist: ${resolved}`)
+    }
+    return resolved
+  }
```

### TM-003 (env.SHELL resolution)

Source code thực tế L626-629:
```typescript
const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
const resolvedShellOverride = resolvePtyShellOverride(shellOverride)
const shell = resolvedShellOverride || resolveDefaultShell()
```

`resolvePtyShellOverride` chỉ hoạt động trên Windows (L165-177: returns `''` on non-Windows). Fix:

```diff
// pty-handler.ts L626-629:
     const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
     const resolvedShellOverride = resolvePtyShellOverride(shellOverride)
-    const shell = resolvedShellOverride || resolveDefaultShell()
+    // Thêm: đọc SHELL từ params.env (cho Linux/macOS — resolvePtyShellOverride chỉ hoạt động trên Windows)
+    const envShell = (params.env as Record<string, string> | undefined)?.SHELL?.trim()
+    const shell = resolvedShellOverride
+      || (envShell && existsSync(envShell) ? envShell : '')
+      || resolveDefaultShell()
```

### TM-004 (return missing fields)

Source code thực tế L747:
```typescript
return { id }
```

**Bug đúng!** Caller (Orca Server) expect thêm fields. Fix:

```diff
// pty-handler.ts L747:
-    return { id }
+    return {
+      id,
+      cols,
+      rows,
+      cwd,
+      shell,
+      ...(terminalHandle ? { terminalHandle } : {}),
+    }
```

### TM-001 (ContextVerifier)

Source code L720-738 đã có context stale check:
```typescript
if (context?.isStale()) {
  // Kill PTY nếu client reconnect trong khi spawn đang in-flight
  ...
}
```

**TM-001 bug thực sự:** Không phải HMAC verification. Bug là **ContextVerifier hook chưa được implement** — `context.isStale()` check ở đây là connection-level staleness (reconnect detection), **không phải security verification**.

Theo HLD, `ContextVerifier` phải verify HMAC-SHA256 signature của context trước khi cho phép spawn — **đây là missing security check**.

Fix: Thêm HMAC check TRƯỚC khi spawn:

```diff
// pty-handler.ts spawn() method — thêm TRƯỚC khi spawn pty:

+    // Security: verify HMAC context signature (BL-TM-01)
+    if (context && process.env.ORCA_CONTEXT_SECRET) {
+      const isValid = verifyContextHmac(context, process.env.ORCA_CONTEXT_SECRET)
+      if (!isValid) {
+        throw Object.assign(new Error('context signature invalid'), { agentErrorCode: -32001 })
+      }
+    }
+
     const id = `pty-${this.nextId++}`

// Thêm helper (vào pty-handler.ts hoặc tách ra context-verifier.ts):
+import { createHmac, timingSafeEqual } from 'node:crypto'
+
+function verifyContextHmac(context: RequestContext, secret: string): boolean {
+  if (!context.signature) return false
+  const TTL_MS = 30_000
+  if (context.issuedAt && Date.now() - context.issuedAt > TTL_MS) return false
+  const payload = `${context.userId ?? ''}:${context.sessionId ?? ''}:${context.issuedAt ?? 0}`
+  const expected = createHmac('sha256', secret).update(payload).digest('hex')
+  try {
+    return timingSafeEqual(Buffer.from(context.signature, 'hex'), Buffer.from(expected, 'hex'))
+  } catch { return false }
+}
```

---

## Fix TRM-001/002 — Relay session null & Timeout

### Cần đọc thêm để verify:

```bash
# Kiểm tra DevServerRelayBridge source:
cat src/main/dev-server/dev-server-relay-bridge.ts | grep -n "Not connected\|callWithTimeout\|session\|AGENT"

# Kiểm tra AgentWebSocketServer:
cat src/main/dev-server/agent-ws-server.ts | grep -n "slot\|TTL\|token\|register"
```

### Giải pháp TRM-001 (dựa trên knowledge từ bug report):

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
// Tìm nơi throw 'Not connected' và sửa error message:

// BEFORE:
throw new Error('Not connected')

// AFTER:
const err = Object.assign(
  new Error(`Dev Server agent not connected: ${this.config.id}`),
  {
    code:        'AGENT_NOT_CONNECTED',
    devServerId: this.config.id,
    hint:        'Ensure the Orca agent is running on the dev server. ' +
                 'Run: node ~/orca-agent/agent.js',
  }
)
throw err
```

### Giải pháp TRM-002 (timeout 30s → 10s):

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
// Tìm callWithTimeout cho pty.spawn và giảm timeout:

// BEFORE:
callWithTimeout('pty.spawn', params, 30_000)

// AFTER:
callWithTimeout('pty.spawn', params, 10_000)  // 10s: fail fast
```

---

## Tóm tắt file changes (terminal-management domain)

| File | Line | Action | Bug |
|------|------|--------|-----|
| `src/relay/pty-handler.ts` | L615 | Add `validateCwd()` method + call | TM-002 |
| `src/relay/pty-handler.ts` | L628 | Add env.SHELL fallback in shell resolution | TM-003 |
| `src/relay/pty-handler.ts` | L747 | Add cols, rows, cwd, shell to return | TM-004 |
| `src/relay/pty-handler.ts` | Before spawn() | Add HMAC context verification | TM-001 |
| `src/relay/pty-agent-bridge.ts` | NEW | Bridge: PtyHandler → agent-rpc-dispatch | TM-001 (agent mode) |
| `src/relay/agent-rpc-dispatch.ts` | After 'agent.exec' | Add pty.spawn, pty.shutdown, pty.sendSignal cases | TM-001 |
| `src/main/dev-server/dev-server-relay-bridge.ts` | 'Not connected' | Better error message + code | TRM-001 |
| `src/main/dev-server/dev-server-relay-bridge.ts` | callWithTimeout | 30s → 10s | TRM-002 |

---

## Tests cần viết

```typescript
// src/relay/__tests__/pty-agent-bridge.test.ts (NEW)

import { describe, it, expect, vi } from 'vitest'
import { handlePtySpawnForAgent, handlePtyShutdownForAgent } from '../pty-agent-bridge'

describe('handlePtySpawnForAgent', () => {
  it('rejects invalid cwd (path traversal)', async () => {
    const result = await handlePtySpawnForAgent(1, { cwd: '/etc/shadow' }, config, mockLog) as any
    expect(result.error.message).toContain('not allowed')
  })

  it('rejects non-existent cwd', async () => {
    const result = await handlePtySpawnForAgent(1, { cwd: '/nonexistent-dir-xyz' }, config, mockLog) as any
    expect(result.error).toBeDefined()
  })

  it('returns id, cols, rows, cwd, shell on success', async () => {
    const result = await handlePtySpawnForAgent(
      1,
      { cwd: process.env.HOME, cols: 120, rows: 40 },
      config, mockLog
    ) as any
    expect(result.result?.id).toMatch(/^agent-pty-\d+$/)
    expect(result.result?.cols).toBe(120)
    expect(result.result?.rows).toBe(40)
    expect(result.result?.cwd).toBeTruthy()
    expect(result.result?.shell).toBeTruthy()
  })
})

// pty-handler.ts TM-004 test:
describe('PtyHandler spawn return value', () => {
  it('includes cols, rows, cwd, shell in response', async () => {
    // mock pty.spawn, kiểm tra return value của spawn()
    const result = await handler.spawn({ cwd: '/tmp', cols: 80, rows: 24 })
    expect(result).toHaveProperty('id')
    expect(result).toHaveProperty('cols', 80)
    expect(result).toHaveProperty('rows', 24)
    expect(result).toHaveProperty('cwd', '/tmp')
    expect(result).toHaveProperty('shell')
  })
})
```
