# SOLUTION: Terminal Management (Detail) Domain — Fix Bugs

**Domain:** terminal-management. (pty-handler security + relay session)  
**TDD Reference:** TDD-AG-01 (Architecture), TDD-AG-03 (Connection Modes)  
**Files cần thay đổi:** `src/relay/pty-handler.ts`, `src/main/dev-server/dev-server-relay-bridge.ts`, `src/main/dev-server/agent-ws-server.ts`  
**Tổng số bugs:** 7 (TM-001~004 + TRM-001~002)

---

## Tổng quan phụ thuộc

```
BUG-TRM-AG-001 (relay session null — agent chưa connect)
    └─ phải fix trước TRM-002 (timeout thường xảy ra sau khi 001 được fix)

BUG-AG-TM-001 (missing ContextVerifier) — độc lập, fix pty-handler.ts
BUG-AG-TM-002 (missing SecureFs path validation) — độc lập, fix pty-handler.ts
BUG-AG-TM-003 (shell resolve ignores env.SHELL) — độc lập, fix pty-handler.ts
BUG-AG-TM-004 (response missing fields) — độc lập, fix pty-handler.ts
```

---

## BUG-TRM-AG-001 — Fix Relay Session Null (Agent chưa kết nối)

**Mức độ:** 🔴 CRITICAL  
**File:** `src/main/dev-server/dev-server-relay-bridge.ts`, `src/main/dev-server/agent-ws-server.ts`  
**Root cause:** `DevServerRelayBridge.callWithTimeout()` throw `"Not connected"` khi agent chưa connect inbound vào Orca Server.

### Fix A — Better error message và error code (UX improvement)

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
// Lines 544-548 — callWithTimeout():

// BEFORE:
if (!session) {
  const span = relayCallTracer.start({ devServerId: this.config.id, method })
  span.fail('Not connected', { method, devServerId: this.config.id })
  throw new Error('Not connected')
}

// AFTER — Cụ thể hơn, user có thể debug:
if (!session) {
  const span = relayCallTracer.start({ devServerId: this.config.id, method })
  span.fail('agent_not_connected', { method, devServerId: this.config.id })

  const err = Object.assign(
    new Error(`Dev Server agent not connected: ${this.config.id}`),
    {
      code:       'AGENT_NOT_CONNECTED',
      devServerId: this.config.id,
      method,
      hint:       'Start the Orca agent on the dev server: node /home/ubuntu/orca-agent/agent.js',
    }
  )
  throw err
}
```

### Fix B — Connection status API để browser hiển thị trạng thái

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts — thêm method:

/**
 * Kiểm tra xem Dev Server agent đã connect chưa.
 * Browser có thể dùng để hiển thị "Agent offline" trước khi tạo terminal.
 */
isAgentConnected(): boolean {
  return this.session !== null
}

/**
 * Lắng nghe khi agent connect/disconnect.
 * Browser có thể subscribe để cập nhật UI realtime.
 */
onConnectionChange(callback: (connected: boolean) => void): () => void {
  const onConnect    = () => callback(true)
  const onDisconnect = () => callback(false)
  this.on('agent:connected',    onConnect)
  this.on('agent:disconnected', onDisconnect)
  return () => {
    this.off('agent:connected',    onConnect)
    this.off('agent:disconnected', onDisconnect)
  }
}
```

### Fix C — Persistent slot (không dùng TTL 60s)

```typescript
// src/main/dev-server/agent-ws-server.ts
// ISSUE: slot có TTL 60s → khi agent connect sau 60s → slot expired → handshake fail

// BEFORE: registerSlot với 60s TTL
registerSlot(agentToken: string, ttlMs = 60_000): string { ... }

// AFTER: Persistent slot (không expire, agent giữ connection dài hạn):
registerPersistentSlot(devServerId: string): { token: string; cleanup: () => void } {
  const token = generateSecureToken()  // crypto.randomBytes(32).toString('hex')

  this.slots.set(token, {
    devServerId,
    createdAt: Date.now(),
    persistent: true,  // ← không expire
  })

  return {
    token,
    cleanup: () => this.slots.delete(token),
  }
}
```

### Fix D — Auto-emit `agentTokenGenerated` khi slot expired (reconnect support)

```typescript
// src/main/dev-server/agent-ws-server.ts:
// Khi slot expired, emit event để trigger re-registration:
private checkSlotExpiry(): void {
  const now = Date.now()
  for (const [token, slot] of this.slots.entries()) {
    if (!slot.persistent && now - slot.createdAt > slot.ttlMs) {
      this.slots.delete(token)
      // Emit để DevServerProvisioner có thể generate new token và gửi cho agent
      this.emit('slotExpired', { devServerId: slot.devServerId })
    }
  }
}
```

---

## BUG-TRM-AG-002 — Fix PTY Spawn Timeout (30s)

**Mức độ:** 🟠 HIGH  
**Files:** `src/relay/pty-handler.ts`, `src/main/dev-server/dev-server-relay-bridge.ts`

### Fix A — Giảm timeout + fail fast (server side)

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts:

// BEFORE:
callWithTimeout('pty.spawn', params, 30_000)  // 30s quá lâu

// AFTER:
callWithTimeout('pty.spawn', params, 10_000)  // 10s — đủ để spawn, fail fast nếu có vấn đề
```

### Fix B — Explicit error handling trong `pty-handler.ts`

```typescript
// src/relay/pty-handler.ts — trong spawn() method:

// BEFORE (no explicit error handling):
const ptyModule = await loadPty()
if (!ptyModule) {
  throw new Error('node-pty not available')
}
const pty = ptyModule.spawn(shell, [], { cwd, ... })

// AFTER — Fail fast với clear error messages:
private async spawn(
  params: Record<string, unknown>,
  context?: RequestContext
): Promise<{ id: string; cols: number; rows: number; cwd: string }> {

  // Check node-pty availability TRƯỚC KHI làm gì khác:
  const ptyModule = await loadPty()
  if (!ptyModule) {
    throw Object.assign(
      new Error('node_pty_unavailable'),
      {
        code: 'PTY_UNAVAILABLE',
        hint: 'node-pty native addon not installed on remote host. Run: npm install node-pty',
      }
    )
  }

  const cwd = this.resolveAndValidateCwd(params)

  // Check cwd tồn tại:
  if (!existsSync(cwd)) {
    throw Object.assign(
      new Error(`pty_cwd_not_found`),
      { code: 'CWD_NOT_FOUND', cwd }
    )
  }

  // Resolve shell:
  const shell = this.resolveShell(params)

  // Check shell binary tồn tại:
  if (!existsSync(shell) && !isInPath(shell)) {
    throw Object.assign(
      new Error(`shell_not_found`),
      { code: 'SHELL_NOT_FOUND', shell }
    )
  }

  const cols = typeof params.cols === 'number' ? params.cols : 80
  const rows = typeof params.rows === 'number' ? params.rows : 24

  try {
    const ptyId = generatePtyId()
    const term  = ptyModule.spawn(shell, [], {
      name: 'xterm-256color',
      cols, rows, cwd,
      env: this.buildPtyEnv(params),
    })

    this.registerPty(ptyId, term)
    return { id: ptyId, cols, rows, cwd }

  } catch (err: unknown) {
    // Propagate error ngay lập tức thay vì hang
    const msg = err instanceof Error ? err.message : String(err)
    throw Object.assign(
      new Error(`pty_spawn_failed: ${msg}`),
      { code: 'PTY_SPAWN_FAILED', detail: msg }
    )
  }
}
```

### Fix C — Preflight check endpoint

```typescript
// src/relay/agent-rpc-dispatch.ts — thêm case:
case 'pty.preflight': {
  /**
   * Check node-pty availability và cwd trước khi spawn.
   * Returns: { nodePtyAvailable: boolean, cwdExists: boolean, shellExists: boolean }
   */
  const cwd   = typeof rpc.params?.cwd   === 'string' ? rpc.params.cwd   : process.cwd()
  const shell = typeof rpc.params?.shell === 'string' ? rpc.params.shell : ''

  const ptyModule          = await loadPty()
  const nodePtyAvailable   = !!ptyModule
  const cwdExists          = existsSync(cwd)
  const resolvedShell      = shell || resolveDefaultShell()
  const shellExists        = existsSync(resolvedShell) || isInPath(resolvedShell)

  return makeOk(rpc.id, {
    nodePtyAvailable,
    cwdExists,
    shellExists,
    resolvedShell,
    issues: [
      !nodePtyAvailable ? 'node-pty not installed' : null,
      !cwdExists        ? `cwd not found: ${cwd}` : null,
      !shellExists      ? `shell not found: ${resolvedShell}` : null,
    ].filter(Boolean),
  })
}
```

---

## BUG-AG-TM-001 — Fix `pty.spawn` thiếu ContextVerifier (HMAC-SHA256)

**Mức độ:** HIGH  
**File:** `src/relay/pty-handler.ts`

```typescript
// src/relay/pty-handler.ts — trong spawn() method, thêm ContextVerifier:

import { ContextVerifier } from './context-verifier'  // cần implement nếu chưa có

private async spawn(
  params: Record<string, unknown>,
  context?: RequestContext
): Promise<{ id: string }> {

  // STEP 1: Context verification (HMAC-SHA256, TTL 30s)
  // Theo HLD BL-TM-01 §Bước 5
  if (context) {
    const verifyResult = ContextVerifier.verify(context)
    if (!verifyResult.ok) {
      throw Object.assign(
        new Error('context invalid'),
        { code: -32001, message: 'context invalid', detail: verifyResult.reason }
      )
    }
  }

  // ... rest of spawn logic
}
```

**Implement `ContextVerifier`:**

```typescript
// src/relay/context-verifier.ts (NEW FILE nếu chưa có):
import { createHmac, timingSafeEqual } from 'node:crypto'

export interface RequestContext {
  userId:    string
  sessionId: string
  issuedAt:  number   // Unix timestamp ms
  signature: string   // HMAC-SHA256(userId+sessionId+issuedAt, CONTEXT_SECRET)
  isStale(): boolean
}

export const ContextVerifier = {
  /**
   * Verify HMAC-SHA256 signature và TTL.
   * Theo HLD: TTL = 30s, key = ORCA_CONTEXT_SECRET env var.
   */
  verify(context: RequestContext): { ok: boolean; reason?: string } {
    const TTL_MS = 30_000  // 30 seconds

    // Check TTL
    if (Date.now() - context.issuedAt > TTL_MS) {
      return { ok: false, reason: 'context expired (TTL 30s)' }
    }

    // Verify HMAC
    const CONTEXT_SECRET = process.env.ORCA_CONTEXT_SECRET
    if (!CONTEXT_SECRET) {
      // If no secret configured, skip verification (development mode)
      // TODO: Require in production
      return { ok: true }
    }

    const payload  = `${context.userId}:${context.sessionId}:${context.issuedAt}`
    const expected = createHmac('sha256', CONTEXT_SECRET).update(payload).digest('hex')

    try {
      const match = timingSafeEqual(
        Buffer.from(context.signature, 'hex'),
        Buffer.from(expected, 'hex')
      )
      return match
        ? { ok: true }
        : { ok: false, reason: 'HMAC signature mismatch' }
    } catch {
      return { ok: false, reason: 'Invalid signature format' }
    }
  },
}
```

---

## BUG-AG-TM-002 — Fix `pty.spawn` thiếu SecureFs path validation cho `cwd`

**Mức độ:** HIGH  
**File:** `src/relay/pty-handler.ts`

```typescript
// src/relay/pty-handler.ts — trong spawn():

// BEFORE (không validate cwd):
const cwd = (params.cwd as string) || resolveDefaultCwd()
const term = pty.spawn(shell, [], { cwd, ... })

// AFTER — SecureFs validation:
import { resolve } from 'node:path'
import { homedir } from 'node:os'

/**
 * Validate và sanitize cwd path.
 * Ngăn path traversal: ../../../etc, /root, /etc, v.v.
 * Theo HLD BL-TM-01 §ISOLATION CHECK + A.5 Security Model.
 */
private resolveAndValidateCwd(params: Record<string, unknown>): string {
  const rawCwd    = typeof params.cwd === 'string' ? params.cwd : ''
  const defaultCwd = homedir()

  if (!rawCwd) return defaultCwd

  // Resolve absolute path (giải quyết ../ , ./ , v.v.)
  const resolved = resolve(rawCwd)

  // Allowed roots: workspace paths và home directory
  const workspaceRoot = process.env.ORCA_WORKSPACE_ROOT ?? homedir()
  const allowedRoots  = [
    workspaceRoot,
    homedir(),
    '/tmp',
  ]

  // Check xem resolved path có nằm trong allowed roots không
  const isAllowed = allowedRoots.some(root => resolved.startsWith(root + '/') || resolved === root)

  if (!isAllowed) {
    throw Object.assign(
      new Error('path not allowed'),
      {
        code:     -32003,
        message:  'path not allowed',
        path:     resolved,
        allowed:  allowedRoots,
      }
    )
  }

  // Extra check: không chứa null bytes
  if (resolved.includes('\0')) {
    throw new Error('path not allowed: contains null bytes')
  }

  return resolved
}
```

---

## BUG-AG-TM-003 — Fix shell resolve không từ `env.SHELL`

**Mức độ:** MEDIUM  
**File:** `src/relay/pty-handler.ts`

```typescript
// src/relay/pty-handler.ts — trong spawn():

// BEFORE (Lines 626-629):
const shellOverride         = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
const resolvedShellOverride = resolvePtyShellOverride(shellOverride)
const shell                 = resolvedShellOverride || resolveDefaultShell()
// ↑ resolveDefaultShell() đọc process.env.SHELL của relay process

// AFTER — Đọc env.SHELL từ params (Backend inject):
private resolveShell(params: Record<string, unknown>): string {
  // Priority order:
  // 1. Explicit shellOverride param
  // 2. SHELL từ env params (Backend inject từ user profile config)
  // 3. Default shell từ relay process (process.env.SHELL)
  // 4. Fallback /bin/bash

  const shellOverride = typeof params.shellOverride === 'string' ? params.shellOverride.trim() : ''
  if (shellOverride) {
    const resolved = resolvePtyShellOverride(shellOverride)
    if (resolved) return resolved
  }

  // Đọc env.SHELL từ params.env (Backend inject)
  const paramEnv = params.env as Record<string, string> | undefined
  const envShell = paramEnv?.SHELL
  if (envShell && typeof envShell === 'string' && envShell.trim()) {
    // Validate: shell phải là absolute path
    const trimmed = envShell.trim()
    if (trimmed.startsWith('/') && existsSync(trimmed)) {
      return trimmed
    }
  }

  // Fallback: relay process's own SHELL
  return resolveDefaultShell()
}

// Sử dụng:
const shell = this.resolveShell(params)
```

---

## BUG-AG-TM-004 — Fix `pty.spawn` response thiếu fields

**Mức độ:** MEDIUM  
**File:** `src/relay/pty-handler.ts`

```typescript
// src/relay/pty-handler.ts — cuối spawn() method:

// BEFORE (Line 747):
return { id }  // Thiếu: handle, cols, rows, cwd

// AFTER — Trả về đầy đủ fields theo HLD:
// Trong spawn() method, track cols, rows, cwd:
const cols      = typeof params.cols === 'number' ? params.cols : 80
const rows      = typeof params.rows === 'number' ? params.rows : 24
const resolvedCwd = this.resolveAndValidateCwd(params)
const shell     = this.resolveShell(params)

// ... spawn pty ...

// Extract terminal handle nếu có (pre-allocated từ Backend):
const terminalHandle = typeof params.env === 'object' && params.env !== null
  ? (params.env as Record<string, string>).ORCA_TERMINAL_HANDLE
  : undefined

return {
  id,                              // ptyId
  handle: terminalHandle ?? id,   // pre-allocated handle hoặc fallback to ptyId
  cols,                            // actual cols used
  rows,                            // actual rows used
  cwd:   resolvedCwd,             // actual resolved cwd
  shell,                           // resolved shell binary
}
```

---

## Tóm tắt file changes

| File | Action | Bugs fixed |
|------|--------|------------|
| `src/relay/pty-handler.ts` | ADD ContextVerifier.verify() call | TM-001 |
| `src/relay/pty-handler.ts` | ADD resolveAndValidateCwd() — SecureFs path validation | TM-002 |
| `src/relay/pty-handler.ts` | ADD resolveShell() — đọc env.SHELL từ params | TM-003 |
| `src/relay/pty-handler.ts` | MODIFY return value — thêm cols, rows, cwd, handle, shell | TM-004 |
| `src/relay/pty-handler.ts` | ADD explicit error handling cho node-pty + cwd check | TRM-002 |
| `src/relay/context-verifier.ts` | **NEW** — HMAC-SHA256 context verification | TM-001 |
| `src/main/dev-server/dev-server-relay-bridge.ts` | MODIFY error message → `AGENT_NOT_CONNECTED` + hint | TRM-001 |
| `src/main/dev-server/dev-server-relay-bridge.ts` | MODIFY timeout 30s → 10s | TRM-002 |
| `src/main/dev-server/dev-server-relay-bridge.ts` | ADD `isAgentConnected()`, `onConnectionChange()` | TRM-001 |
| `src/main/dev-server/agent-ws-server.ts` | ADD `registerPersistentSlot()` | TRM-001 |
| `src/relay/agent-rpc-dispatch.ts` | ADD `pty.preflight` case | TRM-002 |

---

## Verification Plan

```bash
# 1. Type check:
pnpm tsc --noEmit -p config/tsconfig.node.json

# 2. Unit tests:
pnpm vitest run src/relay/__tests__/pty-handler.test.ts
pnpm vitest run src/relay/__tests__/context-verifier.test.ts

# 3. Security tests:
# - Gửi pty.spawn với cwd='../../etc' → expect error 'path not allowed'
# - Gửi pty.spawn với context.issuedAt = (now - 31000) → expect error 'context expired'
# - Gửi pty.spawn với context.signature = 'invalid' → expect error 'HMAC signature mismatch'

# 4. TRM-001 test:
# - Không start agent → gọi terminal.create → expect AGENT_NOT_CONNECTED error với hint message
# - Start agent → reconnect → verify session established

# 5. TRM-002 test:
# - Xóa cwd trên remote trước khi spawn → verify fail fast (không wait 30s)
# - Gọi pty.preflight → verify trả về issues list
# - Verify timeout giảm từ 30s → 10s

# 6. TM-003 test:
# - Gửi pty.spawn với env: { SHELL: '/usr/bin/fish' }
# - Verify pty spawns với fish shell
# - Verify shell integration hoạt động

# 7. TM-004 test:
# - Gửi pty.spawn → verify response có: { id, handle, cols, rows, cwd, shell }
# - Verify cols/rows match what was requested
# - Verify cwd là absolute resolved path
```

---

## ✅ Implementation Status (2026-08-01)

AG-TM-001,002,003,004: validatePtyCwd, env.SHELL, return type DONE. TRM-001,002: AGENT_NOT_CONNECTED + 10s timeout + exponential backoff DONE.
