# SOLUTION: Project Workspace Domain — Fix tất cả Bugs

**Domain:** project-workspace  
**TDD Reference:** TDD-07 (Runtime Service), TDD-19 (Project Workspace), TDD-15 (Project Binding)  
**Files cần thay đổi:** `src/main/runtime/rpc/methods/workspace.ts`, `src/main/dev-server/DevServerRelayBridge.ts`  
**Tổng số bugs:** 2 (PW-001, PW-002)

---

## BUG-PW-001 — Fix teardown không cleanup PTYs

**Mức độ:** 🔴 HIGH  
**Root cause:** Khi project/workspace bị teardown, các PTY processes (terminal sessions) không được kill → orphaned processes.

### Fix — Cleanup PTYs trong teardown sequence

```typescript
// src/main/runtime/rpc/methods/workspace.ts

export async function handleWorkspaceTeardown(params: {
  projectId: string
  userId:    string
  force?:    boolean
}, context: RequestContext): Promise<void> {
  const { projectId, userId, force } = params
  const { runtime, devServerManager, ptyManager } = context

  // FIX PW-001: Cleanup PTYs TRƯỚC khi teardown workspace
  // Step 1: Kill tất cả active PTY sessions của user trong project này
  const activePtys = ptyManager.listByProject(projectId, userId)
  
  if (activePtys.length > 0) {
    context.log.info(`[Workspace] Cleaning up ${activePtys.length} PTY sessions for project ${projectId}`)
    
    await Promise.allSettled(activePtys.map(async (pty) => {
      try {
        // Graceful shutdown: SIGTERM + wait 3s + SIGKILL
        await ptyManager.shutdown(pty.id, { graceful: !force, timeoutMs: 3000 })
      } catch (err) {
        context.log.warn(`[Workspace] Failed to cleanup PTY ${pty.id}:`, err)
      }
    }))
  }

  // Step 2: Kill PTYs trên remote Dev Server (nếu có)
  const project = await runtime.getProject(projectId)
  if (project.devServerId) {
    const bridge = devServerManager.getBridge(project.devServerId)
    if (bridge) {
      try {
        await bridge.call('pty.cleanupForProject', {
          projectId,
          userId,
          signal: force ? 'SIGKILL' : 'SIGTERM',
        })
      } catch (err) {
        context.log.warn('[Workspace] Remote PTY cleanup failed:', err)
      }
    }
  }

  // Step 3: Kill any running agents for this project
  const agents = await runtime.listRunningAgents({ projectId, userId })
  for (const agent of agents) {
    await runtime.killAgent(agent.ptyId, 'SIGTERM').catch(() => {})
  }

  // Step 4: Proceed with workspace teardown
  await runtime.teardownWorkspace(projectId, userId)
  
  context.log.info(`[Workspace] Teardown complete: projectId=${projectId} userId=${userId}`)
}
```

---

## BUG-PW-002 — Fix relay không có per-user isolation

**Mức độ:** 🔴 CRITICAL (Security)  
**Root cause:** Relay bridge cho phép một user gọi RPC methods của user khác → data isolation violation.

### Fix — Thêm userId enforcement trong relay calls

```typescript
// src/main/dev-server/DevServerRelayBridge.ts

export class DevServerRelayBridge {
  /**
   * Call RPC method trên Dev Server với userId enforcement.
   * Mọi call đều kèm userId → Dev Server validate ownership.
   */
  async call<T = unknown>(
    method:  string,
    params:  Record<string, unknown> = {},
    options: { userId?: string; timeout?: number } = {},
  ): Promise<T> {
    // FIX PW-002: Inject userId vào tất cả calls
    const enrichedParams = options.userId
      ? { ...params, _userId: options.userId }  // Dev Server dùng _userId để validate
      : params

    return await this.callWithTimeout(method, enrichedParams, options.timeout ?? 30_000) as T
  }

  /**
   * Wrapper với automatic userId injection từ request context.
   */
  async callAsUser<T = unknown>(
    method:  string,
    params:  Record<string, unknown>,
    userId:  string,
    timeout?: number,
  ): Promise<T> {
    return this.call<T>(method, params, { userId, timeout })
  }
}

// Sử dụng trong IPC handlers:
// TRƯỚC:
await bridge.call('pty.spawn', { cwd, cols, rows })

// SAU — luôn pass userId:
await bridge.callAsUser('pty.spawn', { cwd, cols, rows }, req.orcaSession.userId)

// Dev Server (relay/pty-handler.ts) validate _userId:
// PTY phải được tạo bởi đúng userId, không thể attach PTY của user khác

// Wrapper cho WsSessionRouter:
// src/main/session/ws-session-router.ts
// Tất cả WS messages từ user X chỉ được route đến user X's process:
export class WsSessionRouter {
  route(ws: WebSocket, session: OrcaSession): void {
    const userProcess = this.sessionManager.getProcess(session.userId)
    if (!userProcess) {
      ws.close(4503, 'User session not found')
      return
    }
    // Forward to user-specific Unix socket — isolation by socket path
    const socket = createConnection(userProcess.socketPath)
    // Bidirectional pipe WS ↔ Unix socket
    this.pipeWebSocketToSocket(ws, socket, session)
  }

  private pipeWebSocketToSocket(ws: WebSocket, socket: net.Socket, session: OrcaSession): void {
    ws.on('message', (data) => {
      // Inject userId vào mọi message trước khi forward
      try {
        const msg = JSON.parse(data.toString())
        msg._userId = session.userId  // User process sẽ validate
        socket.write(JSON.stringify(msg) + '\n')
      } catch {
        socket.write(data as Buffer)
      }
    })

    socket.on('data', (data) => ws.send(data))
    ws.on('close', () => socket.end())
    socket.on('close', () => ws.close())
  }
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/runtime/rpc/methods/workspace.ts` | Add PTY cleanup in teardown | PW-001 |
| `src/main/daemon/PtyManager.ts` | Add listByProject + shutdown methods | PW-001 |
| `src/main/dev-server/DevServerRelayBridge.ts` | Add callAsUser() + userId injection | PW-002 |
| `src/main/session/ws-session-router.ts` | Add userId enforcement in WS routing | PW-002 |
| `src/relay/pty-handler.ts` | Validate _userId ownership in pty.spawn | PW-002 |

---

## Verification Plan

```bash
# Security test PW-002:
# 1. User A gets ptyId for their terminal
# 2. User B tries to attach to User A's ptyId → expect 403
# 3. User A's relay call → verify _userId injected → Dev Server validates

# Isolation test:
# 1. User A's process crash → verify User B's PTYs unaffected
# 2. Teardown project → verify ALL PTYs for that project cleaned up

pnpm vitest run src/main/runtime/__tests__/workspace.test.ts
pnpm vitest run src/main/session/__tests__/ws-session-router.test.ts
```
