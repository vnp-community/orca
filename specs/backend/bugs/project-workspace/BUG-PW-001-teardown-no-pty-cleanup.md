# BUG-PW-001 [BACKEND]: `WorkspaceService.teardownWorkspace()` dùng `ProjectService` qua `router.getProject()` nhưng không release PTY sessions trên Dev Server

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-PW-001  
**Note:** WorkspaceService.ts: stopTerminalsForWorktree() on teardown  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`src/main/workspace/WorkspaceService.ts:120`:
```typescript
async teardownWorkspace(projectId: string): Promise<void> {
  // Looks up project.devServerId and calls relayPool.release()
}
```

Teardown chỉ release relay connection pool. Không có:
1. `relay.call('pty.destroy', { ptyId })` để đóng PTY sessions trên Dev Server
2. `DELETE orca_terminal_sessions WHERE projectId=?` để dọn DB
3. `relay.call('agent.kill')` nếu agent đang chạy trong terminal

→ PTY processes tiếp tục chạy trên Dev Server sau khi user switch project hoặc đóng workspace.

## Thực tế

`relayPool.release(devServerId)` — đóng WS connection đến Dev Server. Khi connection đóng:
- Dev Server PTY_REGISTRY: PTY processes vẫn chạy (không có auto-kill khi WS closes)
- Relay `onDisconnect` có cleanup không? Cần verify.

Xem `src/main/dev-server/agent-ws-server.ts` — `PTY_REGISTRY` là module-level singleton (BUG-AG-ORCH-009 cũ) → không có cleanup khi relay disconnects.

## Fix đề xuất

```typescript
async teardownWorkspace(projectId: string): Promise<void> {
  // 1. Load active terminal sessions
  const sessions = await this.pool.withConnection((db) =>
    db.query('SELECT ptyId FROM orca_terminal_sessions WHERE projectId = ?', [projectId])
  )
  
  // 2. Kill PTY sessions on Dev Server
  const relay = await this.router.getRelayForProject(projectId, '__system__').catch(() => null)
  if (relay) {
    await Promise.allSettled(
      sessions.map(s => relay.call('pty.destroy', { ptyId: s.ptyId }))
    )
  }
  
  // 3. Delete DB records
  await this.pool.withConnection((db) =>
    db.query('DELETE FROM orca_terminal_sessions WHERE projectId = ?', [projectId])
  )
  
  // 4. Release relay connection
  // ...existing relay release code
}
```

## Files liên quan

- `src/main/workspace/WorkspaceService.ts:120+`: teardownWorkspace incomplete
