# SOLUTION: CLI Headless Domain — Fix tất cả Bugs

**Domain:** cli-headless  
**TDD Reference:** TDD-02 (Main Process), TDD-03 (Daemon Layer), TDD-04 (RPC Server)  
**Files cần thay đổi:** `src/main/daemon/pty-daemon.ts`, `src/cli/headless-dispatcher.ts`  
**Tổng số bugs:** 2 (CLI-001, BE-CLI-001)

---

## BUG-BE-CLI-001 — Fix daemon Unix socket not implemented

**Mức độ:** 🔴 HIGH  
**Root cause:** PTY Daemon chưa implement Unix socket IPC → CLI headless mode không thể communicate với daemon.

### Fix — Implement Unix socket server trong daemon

Theo TDD v5 Addendum E `CR-LOGIN-002` (SessionManager Unix socket pattern):

```typescript
// src/main/daemon/pty-daemon.ts

import { createServer, Server, Socket } from 'node:net'
import { existsSync, mkdirSync, unlinkSync } from 'node:fs'
import { dirname } from 'node:path'

export class PtyDaemon {
  private server?: Server
  private socketPath: string

  constructor(
    private readonly userDataPath: string,
    private readonly userId: string,
  ) {
    this.socketPath = `${userDataPath}/users/${userId}/daemon.sock`
  }

  /**
   * Start Unix socket server.
   * CLI headless mode connect vào đây để gửi commands.
   */
  async start(): Promise<void> {
    // Cleanup stale socket
    if (existsSync(this.socketPath)) {
      unlinkSync(this.socketPath)
    }

    // Ensure socket directory exists
    mkdirSync(dirname(this.socketPath), { recursive: true, mode: 0o700 })

    this.server = createServer((socket) => this.handleConnection(socket))

    await new Promise<void>((resolve, reject) => {
      this.server!.listen(this.socketPath, () => {
        // Set permissions: only owner can read/write
        import('node:fs').then(({ chmodSync }) => chmodSync(this.socketPath, 0o600))
        resolve()
      })
      this.server!.once('error', reject)
    })

    console.log(`[PtyDaemon] Listening on ${this.socketPath}`)
  }

  private handleConnection(socket: Socket): void {
    let buffer = ''

    socket.on('data', (chunk) => {
      buffer += chunk.toString('utf-8')
      
      // Process complete JSON messages (newline-delimited)
      const lines = buffer.split('\n')
      buffer = lines.pop() ?? ''  // last incomplete line stays in buffer

      for (const line of lines) {
        if (!line.trim()) continue
        try {
          const msg = JSON.parse(line)
          this.handleMessage(msg, socket)
        } catch {
          socket.write(JSON.stringify({ error: 'Invalid JSON' }) + '\n')
        }
      }
    })

    socket.on('error', (err) => {
      console.error('[PtyDaemon] Socket error:', err.message)
    })
  }

  private async handleMessage(msg: any, socket: Socket): Promise<void> {
    const { id, method, params } = msg

    try {
      const result = await this.dispatch(method, params)
      socket.write(JSON.stringify({ id, result }) + '\n')
    } catch (err) {
      socket.write(JSON.stringify({ id, error: { message: String(err) } }) + '\n')
    }
  }

  private async dispatch(method: string, params: unknown): Promise<unknown> {
    switch (method) {
      case 'pty.spawn':
        return await this.spawnPty(params as any)
      case 'pty.kill':
        return await this.killPty(params as any)
      case 'pty.write':
        return await this.writePty(params as any)
      case 'pty.resize':
        return await this.resizePty(params as any)
      case 'daemon.status':
        return { pid: process.pid, activePtys: this.ptys.size }
      default:
        throw new Error(`Unknown method: ${method}`)
    }
  }

  async stop(): Promise<void> {
    this.server?.close()
    if (existsSync(this.socketPath)) {
      unlinkSync(this.socketPath)
    }
  }
}
```

---

## BUG-CLI-001 — Fix headless automation dispatcher not wired

**Mức độ:** 🟠 HIGH  
**Root cause:** CLI headless mode có dispatcher nhưng không được wired vào daemon → automation commands không execute.

### Fix — Wire HeadlessDispatcher với PtyDaemon

```typescript
// src/cli/headless-dispatcher.ts

import { Socket, createConnection } from 'node:net'

export class HeadlessDispatcher {
  private socket?: Socket
  private pendingRequests = new Map<string, { resolve: Function; reject: Function }>()
  private buffer = ''
  private reqId = 0

  constructor(private readonly socketPath: string) {}

  async connect(): Promise<void> {
    await new Promise<void>((resolve, reject) => {
      this.socket = createConnection(this.socketPath, () => resolve())
      this.socket.once('error', reject)
      this.socket.on('data', (chunk) => this.handleData(chunk))
    })
  }

  private handleData(chunk: Buffer): void {
    this.buffer += chunk.toString('utf-8')
    const lines = this.buffer.split('\n')
    this.buffer = lines.pop() ?? ''

    for (const line of lines) {
      if (!line.trim()) continue
      try {
        const msg = JSON.parse(line)
        const pending = this.pendingRequests.get(msg.id)
        if (!pending) continue
        this.pendingRequests.delete(msg.id)
        if (msg.error) pending.reject(new Error(msg.error.message))
        else pending.resolve(msg.result)
      } catch { /* ignore parse errors */ }
    }
  }

  async call(method: string, params?: unknown): Promise<unknown> {
    if (!this.socket) throw new Error('Not connected to daemon')
    
    const id = String(++this.reqId)
    return new Promise((resolve, reject) => {
      this.pendingRequests.set(id, { resolve, reject })
      this.socket!.write(JSON.stringify({ id, method, params }) + '\n')
      
      // Request timeout
      setTimeout(() => {
        if (this.pendingRequests.has(id)) {
          this.pendingRequests.delete(id)
          reject(new Error(`Request timeout: ${method}`))
        }
      }, 30_000)
    })
  }

  async disconnect(): Promise<void> {
    this.socket?.end()
  }
}

// Wiring trong CLI entry:
// src/cli/index.ts (hoặc src/cli/commands/headless.ts)

export async function runHeadlessAutomation(workflowId: string, userId: string): Promise<void> {
  const socketPath = `${process.env.ORCA_USER_DATA_PATH ?? os.homedir() + '/.orca'}/users/${userId}/daemon.sock`
  
  const dispatcher = new HeadlessDispatcher(socketPath)
  await dispatcher.connect()

  try {
    const result = await dispatcher.call('workflow.execute', { workflowId, userId })
    console.log('[HeadlessMode] Workflow result:', result)
  } finally {
    await dispatcher.disconnect()
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/daemon/pty-daemon.ts` | Add Unix socket server + message dispatch | BE-CLI-001 |
| `src/cli/headless-dispatcher.ts` | NEW — Unix socket client | CLI-001 |
| `src/cli/commands/headless.ts` | Wire dispatcher → daemon | CLI-001 |
| `src/main/server-bootstrap.ts` | Start PtyDaemon with Unix socket | BE-CLI-001 |

---

## Verification Plan

```bash
# Test daemon Unix socket:
# 1. Start daemon → verify socket file created with 0600 permissions
# 2. Connect via dispatcher → verify round-trip JSON-RPC
# 3. Send pty.spawn → verify PTY spawned
# 4. Kill daemon → verify socket cleaned up

pnpm vitest run src/main/daemon/__tests__/
pnpm vitest run src/cli/__tests__/headless.test.ts
```
