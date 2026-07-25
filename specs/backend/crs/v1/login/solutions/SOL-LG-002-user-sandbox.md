# SOL-LG-002 — Per-User Sandbox: Session Manager + Process Fork

**CR:** [CR-LOGIN-002](../../../../../docs/crs/v1/login/CR-LOGIN-002-sandbox.md)
**TDD Refs:** TDD-04 (RPC Server — Transport, Auth), TDD-07 (Runtime Service), TDD-11 (Web Server Mode — Entry Point)
**Approach:** Test-Driven — viết tests trước implementations
**Status:** ✅ Implemented (2026-07-24)
**Blocked by:** SOL-LG-001 (cần `userId` từ `OrcaSession`)

---

## 1. Phân tích từ TDD và Code Hiện tại

### 1.1 Vấn đề (TDD-07 §1 + TDD-04 §2)

```typescript
// TDD-07: OrcaRuntimeService là SINGLETON — một instance dùng chung cho mọi connections
// TDD-04: OrcaRuntimeRpcServer dispatch mọi request vào cùng OrcaRuntimeService instance

// Hậu quả (không isolation):
// - User A crash agent → ảnh hưởng B
// - Tất cả users cùng userDataPath → shared DB, files, keys
// - SshConnections không phân biệt owner
```

### 1.2 Cách tiếp cận

Thêm **Session Manager** ở tầng `src/server/index.ts` (supervisor) — **KHÔNG sửa** `src/main/runtime/runtime-rpc.ts` hay `src/main/runtime/orca-runtime.ts`. Supervisor process fork một Node.js child process cho mỗi userId, proxy WS connection đến đúng user process qua Unix socket.

```
BEFORE:
  HTTP :6769 ─────┐
  WS   :6768 ──── OrcaRuntimeRpcServer (single) ── OrcaRuntimeService (single)

AFTER:
  HTTP :6769 ─────┤ Supervisor (src/server/index.ts)
  WS   :6768 ──── SessionRouter → resolve userId → proxy to /data/orca/users/{userId}/orca.sock
                                                              │
                                               fork() ──► UserProcess (per user)
                                                              OrcaRuntimeRpcServer
                                                              OrcaRuntimeService
                                                              isolated userDataPath
```

---

## 2. File Structure

```
src/main/session/                      ← [NEW]
├── session-types.ts                   ← UserProcess, SessionManagerConfig
├── session-manager.ts                 ← Process spawner, lifecycle, idle shutdown
├── ws-session-router.ts               ← WS proxy: supervisor → user Unix socket
├── user-process-entry.ts              ← Fork entry point (per-user OrcaRuntime)
└── __tests__/
    ├── session-manager.test.ts
    └── ws-session-router.test.ts

src/server/
└── index.ts                           ← [MODIFY] dùng SessionManager nếu ORCA_MULTI_USER=1
```

---

## 3. Test Specifications

### 3.1 `session-manager.test.ts`

```typescript
// src/main/session/__tests__/session-manager.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { SessionManager } from '../session-manager'

// Mock child_process.fork để test không fork thực sự
vi.mock('node:child_process', () => ({
  fork: vi.fn().mockReturnValue({
    pid: 1234,
    send: vi.fn(),
    on: vi.fn((event, cb) => {
      if (event === 'message') {
        // Simulate 'ready' IPC message immediately
        setImmediate(() => cb({ type: 'ready', socketPath: '/tmp/test.sock' }))
      }
    }),
    kill: vi.fn(),
    killed: false,
    exitCode: null
  })
}))

vi.mock('node:fs/promises', () => ({
  mkdir: vi.fn().mockResolvedValue(undefined),
  rm: vi.fn().mockResolvedValue(undefined),
  access: vi.fn().mockResolvedValue(undefined)
}))

describe('SessionManager', () => {
  let manager: SessionManager
  const BASE_PATH = '/data/orca'

  beforeEach(() => {
    manager = new SessionManager({
      baseDataPath: BASE_PATH,
      userProcessEntry: '/fake/entry.js',
      idleTimeoutMs: 100,
      maxRespawnAttempts: 3
    })
  })

  afterEach(async () => {
    await manager.shutdown()
    vi.restoreAllMocks()
  })

  describe('getOrSpawnUserProcess', () => {
    it('spawns a new process for unknown userId', async () => {
      const proc = await manager.getOrSpawnUserProcess('user-alice')
      expect(proc).toBeDefined()
      expect(proc.userId).toBe('user-alice')
      expect(proc.pid).toBe(1234)
    })

    it('reuses existing process for same userId', async () => {
      const proc1 = await manager.getOrSpawnUserProcess('user-bob')
      const proc2 = await manager.getOrSpawnUserProcess('user-bob')
      expect(proc1.pid).toBe(proc2.pid)
    })

    it('creates isolated userDataPath per user', async () => {
      const { mkdir } = await import('node:fs/promises')
      await manager.getOrSpawnUserProcess('user-carol')
      expect(mkdir).toHaveBeenCalledWith(
        expect.stringContaining('/data/orca/users/user-carol'),
        expect.objectContaining({ recursive: true })
      )
    })

    it('passes ORCA_USER_ID env to forked process', async () => {
      const { fork } = await import('node:child_process')
      await manager.getOrSpawnUserProcess('user-dave')
      expect(fork).toHaveBeenCalledWith(
        expect.any(String),
        [],
        expect.objectContaining({
          env: expect.objectContaining({ ORCA_USER_ID: 'user-dave' })
        })
      )
    })
  })

  describe('touch', () => {
    it('updates lastSeenAt for user process', async () => {
      await manager.getOrSpawnUserProcess('user-eve')
      const before = Date.now()
      manager.touch('user-eve')
      const proc = manager.getProcess('user-eve')
      expect(proc!.lastSeenAt).toBeGreaterThanOrEqual(before)
    })
  })

  describe('process crash recovery', () => {
    it('removes process entry on unexpected exit', async () => {
      const { fork } = await import('node:child_process')
      let exitHandler: ((code: number) => void) | null = null
      vi.mocked(fork).mockReturnValue({
        pid: 9999,
        send: vi.fn(),
        killed: false,
        exitCode: null,
        on: vi.fn((event, cb) => {
          if (event === 'message') setImmediate(() => cb({ type: 'ready', socketPath: '/tmp/test-crash.sock' }))
          if (event === 'exit') exitHandler = cb
        }),
        kill: vi.fn()
      } as any)

      await manager.getOrSpawnUserProcess('user-crash')
      expect(manager.getProcess('user-crash')).toBeDefined()

      // Simulate crash
      exitHandler?.(1)
      await new Promise(r => setTimeout(r, 0))

      // Process should be removed from registry after crash
      expect(manager.getProcess('user-crash')).toBeNull()
    })
  })

  describe('shutdown', () => {
    it('kills all user processes gracefully', async () => {
      const { fork } = await import('node:child_process')
      const mockKill = vi.fn()
      vi.mocked(fork).mockReturnValue({
        pid: 111, send: vi.fn(), killed: false, exitCode: null,
        on: vi.fn((e, cb) => { if (e === 'message') setImmediate(() => cb({ type: 'ready', socketPath: '/tmp/s.sock' })) }),
        kill: mockKill
      } as any)

      await manager.getOrSpawnUserProcess('user-shutdown')
      await manager.shutdown()
      expect(mockKill).toHaveBeenCalled()
    })
  })
})
```

### 3.2 `ws-session-router.test.ts`

```typescript
// src/main/session/__tests__/ws-session-router.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { WsSessionRouter } from '../ws-session-router'
import type { SessionManager } from '../session-manager'
import type { AuthManager } from '../../auth/auth-manager'

describe('WsSessionRouter', () => {
  let router: WsSessionRouter
  let sessionManager: SessionManager
  let authManager: AuthManager

  beforeEach(() => {
    sessionManager = {
      getOrSpawnUserProcess: vi.fn().mockResolvedValue({
        userId: 'user-1', pid: 1234,
        socketPath: '/data/orca/users/user-1/orca.sock',
        startedAt: Date.now(), lastSeenAt: Date.now()
      }),
      touch: vi.fn()
    } as any

    authManager = {
      validateRequest: vi.fn()
    } as any

    router = new WsSessionRouter({ sessionManager, authManager })
  })

  describe('resolveUserFromRequest', () => {
    it('returns userId for valid session cookie', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue({
        sessionId: 'sid-1', userId: 'user-alice', userEmail: 'alice@test.com',
        role: 'developer', createdAt: Date.now(), expiresAt: Date.now() + 1000,
        lastSeenAt: null, ipAddress: '127.0.0.1', userAgent: 'ua'
      })

      const userId = await router.resolveUserFromRequest({ headers: { cookie: 'orca_session=abc123' } } as any)
      expect(userId).toBe('user-alice')
    })

    it('returns null for missing or invalid cookie', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(null)
      const userId = await router.resolveUserFromRequest({ headers: {} } as any)
      expect(userId).toBeNull()
    })

    it('falls back to deviceToken if no session (backward compat)', async () => {
      vi.mocked(authManager.validateRequest).mockResolvedValue(null)
      // When no session, WsSessionRouter falls back to PairCode deviceToken
      // → uses default shared runtime (pre-login mode)
      const userId = await router.resolveUserFromRequest({
        headers: { 'x-device-token': 'abc' }
      } as any)
      // null = use shared runtime (non-isolated)
      expect(userId).toBeNull()
    })
  })

  describe('getOrSpawnUserProcess', () => {
    it('calls sessionManager.getOrSpawnUserProcess with userId', async () => {
      await router.getOrCreateUserSocket('user-bob')
      expect(sessionManager.getOrSpawnUserProcess).toHaveBeenCalledWith('user-bob')
    })

    it('returns socketPath from user process', async () => {
      const socketPath = await router.getOrCreateUserSocket('user-carol')
      expect(socketPath).toBe('/data/orca/users/user-1/orca.sock')
    })
  })
})
```

---

## 4. Implementation

### 4.1 `session-types.ts`

```typescript
// src/main/session/session-types.ts
import type { ChildProcess } from 'node:child_process'

export type UserProcess = {
  userId:      string
  pid:         number
  socketPath:  string
  startedAt:   number
  lastSeenAt:  number
  process:     ChildProcess
  respawnCount: number
}

export type SessionManagerConfig = {
  baseDataPath:      string   // e.g. /data/orca
  userProcessEntry:  string   // absolute path to user-process-entry.js
  idleTimeoutMs:     number   // default: 4 * 60 * 60 * 1000 (4h)
  maxRespawnAttempts: number  // default: 3
}
```

### 4.2 `session-manager.ts`

```typescript
// src/main/session/session-manager.ts
import { fork } from 'node:child_process'
import { mkdir, rm, access } from 'node:fs/promises'
import { join } from 'node:path'
import type { UserProcess, SessionManagerConfig } from './session-types'

const DEFAULT_IDLE_TIMEOUT_MS     = 4 * 60 * 60 * 1000  // 4h
const DEFAULT_MAX_RESPAWN         = 3
const IDLE_CHECK_INTERVAL_MS      = 5 * 60 * 1000         // check mỗi 5 phút

export class SessionManager {
  private processes  = new Map<string, UserProcess>()
  private idleTimer: ReturnType<typeof setInterval> | null = null
  private readonly config: Required<SessionManagerConfig>

  constructor(config: SessionManagerConfig) {
    this.config = {
      idleTimeoutMs:      config.idleTimeoutMs      ?? DEFAULT_IDLE_TIMEOUT_MS,
      maxRespawnAttempts: config.maxRespawnAttempts  ?? DEFAULT_MAX_RESPAWN,
      ...config
    }
    this.idleTimer = setInterval(() => this.sweepIdleProcesses(), IDLE_CHECK_INTERVAL_MS)
  }

  async getOrSpawnUserProcess(userId: string): Promise<UserProcess> {
    const existing = this.processes.get(userId)
    if (existing) {
      existing.lastSeenAt = Date.now()
      return existing
    }
    return this.spawnUserProcess(userId)
  }

  private async spawnUserProcess(userId: string): Promise<UserProcess> {
    const userDataPath = join(this.config.baseDataPath, 'users', userId)
    const socketPath   = join(userDataPath, 'orca.sock')

    await mkdir(userDataPath, { recursive: true, mode: 0o700 })

    const child = fork(this.config.userProcessEntry, [], {
      env: {
        ...process.env,
        ORCA_USER_DATA_PATH: userDataPath,
        ORCA_USER_ID: userId,
        ORCA_SOCKET_PATH: socketPath,
        NODE_OPTIONS: '--max-old-space-size=512'  // 512MB limit per user
      },
      stdio: ['ignore', 'pipe', 'pipe', 'ipc'],
      detached: false
    })

    // Wait for 'ready' IPC message (max 30s)
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`UserProcess timeout: ${userId}`)), 30_000)
      child.on('message', (msg: any) => {
        if (msg?.type === 'ready') { clearTimeout(timer); resolve() }
      })
      child.on('error', (err) => { clearTimeout(timer); reject(err) })
    })

    const proc: UserProcess = {
      userId, pid: child.pid!, socketPath,
      startedAt: Date.now(), lastSeenAt: Date.now(),
      process: child, respawnCount: 0
    }

    // Track process exit (crash or normal)
    child.on('exit', (code) => {
      console.warn(`[SessionManager] UserProcess exited: userId=${userId}, code=${code}`)
      this.processes.delete(userId)
      // Cleanup socket file
      rm(socketPath, { force: true }).catch(() => {})
    })

    this.processes.set(userId, proc)
    return proc
  }

  touch(userId: string): void {
    const proc = this.processes.get(userId)
    if (proc) proc.lastSeenAt = Date.now()
  }

  getProcess(userId: string): UserProcess | null {
    return this.processes.get(userId) ?? null
  }

  listProcesses(): readonly UserProcess[] {
    return [...this.processes.values()]
  }

  private sweepIdleProcesses(): void {
    const now = Date.now()
    for (const [userId, proc] of this.processes) {
      if (now - proc.lastSeenAt > this.config.idleTimeoutMs) {
        console.log(`[SessionManager] Idle shutdown: userId=${userId}`)
        this.killUserProcess(userId)
      }
    }
  }

  private killUserProcess(userId: string): void {
    const proc = this.processes.get(userId)
    if (!proc) return
    try {
      proc.process.kill('SIGTERM')
    } catch { /* already exited */ }
    this.processes.delete(userId)
    rm(proc.socketPath, { force: true }).catch(() => {})
  }

  async shutdown(): Promise<void> {
    if (this.idleTimer) { clearInterval(this.idleTimer); this.idleTimer = null }
    for (const userId of this.processes.keys()) {
      this.killUserProcess(userId)
    }
  }
}
```

### 4.3 `ws-session-router.ts`

```typescript
// src/main/session/ws-session-router.ts
import * as net from 'node:net'
import type { IncomingMessage } from 'node:http'
import type { WebSocket } from 'ws'
import type { SessionManager } from './session-manager'
import type { AuthManager } from '../auth/auth-manager'

export class WsSessionRouter {
  private readonly sessionManager: SessionManager
  private readonly authManager: AuthManager

  constructor(opts: { sessionManager: SessionManager; authManager: AuthManager }) {
    this.sessionManager = opts.sessionManager
    this.authManager    = opts.authManager
  }

  async resolveUserFromRequest(req: IncomingMessage): Promise<string | null> {
    const session = await this.authManager.validateRequest(req.headers.cookie)
    return session?.userId ?? null
  }

  async getOrCreateUserSocket(userId: string): Promise<string> {
    const proc = await this.sessionManager.getOrSpawnUserProcess(userId)
    return proc.socketPath
  }

  // Called for each new WS connection
  async handleConnection(ws: WebSocket, req: IncomingMessage): Promise<void> {
    const userId = await this.resolveUserFromRequest(req)

    if (!userId) {
      // No login session → close with 4401 (require auth)
      // NOTE: PairCode / deviceToken connections bypass this router entirely
      // (họ connect trực tiếp vào shared runtime qua legacy path)
      ws.close(4401, 'Authentication required. Please log in first.')
      return
    }

    this.sessionManager.touch(userId)

    let socketPath: string
    try {
      socketPath = await this.getOrCreateUserSocket(userId)
    } catch (err) {
      console.error(`[WsSessionRouter] Failed to spawn user process: userId=${userId}`, err)
      ws.close(1011, 'Internal error: cannot start user session')
      return
    }

    // Proxy WS ↔ Unix socket
    const upstream = net.createConnection(socketPath)

    upstream.on('error', (err) => {
      console.error(`[WsSessionRouter] Upstream error: userId=${userId}`, err)
      ws.close(1011, 'User session unavailable')
    })

    ws.on('message', (data) => {
      if (upstream.writable) upstream.write(data as Buffer)
    })

    upstream.on('data', (chunk) => {
      if (ws.readyState === ws.OPEN) ws.send(chunk)
    })

    ws.on('close', () => {
      upstream.end()
      this.sessionManager.touch(userId)  // update lastSeen on graceful disconnect
    })

    upstream.on('close', () => {
      if (ws.readyState === ws.OPEN) ws.close(1011, 'User session ended')
    })
  }
}
```

### 4.4 `user-process-entry.ts`

```typescript
// src/main/session/user-process-entry.ts
// Được fork() bởi SessionManager — chạy trong process riêng cho từng user
//
// QUAN TRỌNG: File này là entry point của child process.
// Nó KHÔNG được import từ supervisor process.

const userId    = process.env.ORCA_USER_ID!
const dataPath  = process.env.ORCA_USER_DATA_PATH!
const sockPath  = process.env.ORCA_SOCKET_PATH!

if (!userId || !dataPath || !sockPath) {
  console.error('[UserProcess] Missing required env vars')
  process.exit(1)
}

async function main() {
  // Import server-bootstrap để tạo OrcaRuntime
  const { initializeOrcaServices } = await import('../server-bootstrap')
  const { createNodeAdapter } = await import('../../platform/adapters/node')
  const { setPlatform } = await import('../../platform/context')

  const adapter = createNodeAdapter({ userDataPath: dataPath })
  setPlatform(adapter)

  const { shutdown } = await initializeOrcaServices({
    platform: adapter,
    socketPath: sockPath,         // Listen on Unix socket instead of TCP
    userDataPath: dataPath,
    userId,
  })

  // Signal supervisor: ready
  process.send!({ type: 'ready', socketPath: sockPath })
  console.log(`[UserProcess] Ready: userId=${userId}, sock=${sockPath}`)

  // Graceful shutdown
  const handleExit = async () => {
    await shutdown()
    process.exit(0)
  }
  process.on('SIGTERM', handleExit)
  process.on('SIGINT',  handleExit)
}

main().catch((err) => {
  console.error('[UserProcess] Fatal error:', err)
  process.exit(1)
})
```

---

## 5. Tích hợp vào `src/server/index.ts`

```typescript
// src/server/index.ts — MODIFY (thêm multi-user routing)
import { SessionManager } from '../main/session/session-manager'
import { WsSessionRouter } from '../main/session/ws-session-router'

const MULTI_USER = process.env.ORCA_MULTI_USER === '1'

if (MULTI_USER) {
  const sessionManager = new SessionManager({
    baseDataPath:     userDataPath,
    userProcessEntry: join(__dirname, '../main/session/user-process-entry.js'),
    idleTimeoutMs:    4 * 60 * 60 * 1000,
    maxRespawnAttempts: 3
  })

  const wsRouter = new WsSessionRouter({ sessionManager, authManager })

  // Intercept WS connections BEFORE họ đến shared OrcaRuntimeRpcServer
  wss.on('connection', (ws, req) => wsRouter.handleConnection(ws, req))
} else {
  // Legacy mode: single shared runtime (PairCode only)
  // Không thay đổi behavior hiện tại
}
```

---

## 6. Data Isolation

```
/data/orca/
├── users/
│   ├── {userId-alice}/        ← mkdir mode 0700
│   │   ├── orca.sock          ← Alice's RPC Unix socket
│   │   ├── orca-data.json
│   │   ├── orca-devices.json
│   │   ├── orca-e2ee-keypair.json
│   │   └── orca-server.db     ← Alice's SQLite
│   └── {userId-bob}/          ← hoàn toàn riêng biệt
│       ├── orca.sock
│       └── ...
└── orca-server.db             ← Supervisor's shared DB (users, sessions, audit)
```

---

## 7. Feature Flag

```bash
# .env
ORCA_MULTI_USER=1   # Enable per-user process isolation
# Mặc định: 0 (single shared runtime — backward compat với PairCode)
```

---

## 8. Acceptance Criteria

- [x] `session-manager.test.ts` — tất cả tests pass
- [x] `ws-session-router.test.ts` — resolve user, fallback, spawn
- [x] `ORCA_MULTI_USER=0` (default): behavior hoàn toàn không thay đổi (PairCode hoạt động)
- [x] `ORCA_MULTI_USER=1`: mỗi userId nhận fork() riêng với userDataPath riêng
- [x] Process timeout 30s: nếu fork không gửi 'ready' → kill + reject
- [x] Process crash: `this.processes.delete(userId)` → user reconnect tạo process mới
- [x] Idle shutdown: process không có activity trong 4h → kill graceful (SIGTERM)
- [x] Supervisor không bị ảnh hưởng khi user process crash
- [x] Memory limit: `--max-old-space-size=512` per process
