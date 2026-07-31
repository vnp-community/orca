# TASK-FE-002 — `DevServerManager` re-emit `agentTokenGenerated` bridge event

**Solution:** [SOL-FE-AG-003](../solutions/SOL-FE-AG-003-ipc-agent-token-event.md)  
**File:** `src/main/dev-server/dev-server-manager.ts` [MODIFY]  
**Depends on:** TASK-FE-001 (AgentTokenInfo type), TASK-010 (backend — direct-websocket bridge)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

`DevServerRelayBridge` đã emit sự kiện `'agentTokenGenerated'` (TASK-010 backend).
Cần `DevServerManager` lắng nghe sự kiện này từ bridge và re-emit lên manager level để `dev-server-ipc.ts` có thể broadcast lên renderer.

---

## Context hiện tại

```typescript
// src/main/dev-server/dev-server-manager.ts — line ~178-198
async connect(id: string): Promise<void> {
  // ...
  const bridge = new DevServerRelayBridge(persisted, this.sshManager, this.agentWsServer)
  const info = await bridge.connect()
  this.relays.set(id, bridge)
  // ← Hiện tại KHÔNG lắng nghe bridge events
  // ...
}
```

---

## Thay đổi cần thực hiện

### File: `src/main/dev-server/dev-server-manager.ts`

**Thêm import:**
```typescript
import type { AgentTokenInfo } from '../../shared/dev-server-types'
```

**Sửa method `connect()` — thêm bridge event listener ngay sau khi tạo bridge:**

```typescript
async connect(id: string): Promise<void> {
  const persisted = this.store.list().find((ds) => ds.id === id)
  if (!persisted) throw new Error(`DevServer not found: ${id}`)

  this.setRuntimeState(id, { status: 'connecting', lastError: null })
  this.emit('devServer:statusChanged', id, 'connecting')

  try {
    const bridge = new DevServerRelayBridge(persisted, this.sshManager, this.agentWsServer)

    // ─── NEW: Forward bridge's agentTokenGenerated event to manager level ─────
    // Why: IPC handlers listen on manager events, not individual bridge events.
    // This allows dev-server-ipc.ts to broadcast token to renderer without
    // knowing about individual bridge instances.
    bridge.on('agentTokenGenerated', (info: AgentTokenInfo) => {
      this.emit('devServer:agentToken', info)
    })
    // ─────────────────────────────────────────────────────────────────────────

    const info = await bridge.connect()
    this.relays.set(id, bridge)
    // ... rest unchanged
  }
}
```

**Thêm emit type declaration** (nếu DevServerManager có EventEmitter typing):
```typescript
// Trong class hoặc type declaration, thêm:
// emit('devServer:agentToken', info: AgentTokenInfo): boolean
```

---

## Acceptance Criteria

- [x] `bridge.on('agentTokenGenerated', ...)` được thêm vào trong `connect()` method
- [x] `this.emit('devServer:agentToken', info)` re-emit đúng payload
- [x] `AgentTokenInfo` được import từ `../../shared/dev-server-types`
- [x] Listener chỉ được gắn **một lần** per connect (không accumulate)
- [x] TypeScript compile không lỗi
