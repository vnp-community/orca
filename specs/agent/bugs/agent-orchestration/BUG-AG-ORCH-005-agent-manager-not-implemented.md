# BUG-AG-ORCH-005: `AgentManager` và `AgentConnectionManager` không tồn tại — BL-AG-01 hoàn toàn broken

## Mức độ: 🔴 CRITICAL

## Tóm tắt

HLD (BL-AG-01) mô tả:
```
[Main Process — AgentManager.start()]
    conn = AgentConnectionManager.getConnection(devServerId)
    conn.call('agent.spawn', { agentBinary, args, cwd, env, userId, cols, rows })
```

Thực tế grep toàn bộ `src/main`:
```
AgentManager          → No results (chỉ tìm thấy trong amp/hook-service.ts với amp.on('agent.start'))
AgentConnectionManager → No results
agent.start IPC       → No results  
contextBridge.invoke('agent.start', ...) → No results
```

**AgentManager và AgentConnectionManager là 2 class trung tâm của BL-AG-01 CHƯA ĐƯỢC IMPLEMENT.**

## Thực tế implement

Thực tế có 2 component liên quan nhưng không tạo ra luồng BL-AG-01:
- `ProfileAwareAgentSpawner`: gọi `relay.call('agent.exec', ...)` — chỉ dùng cho TaskAgentExecutor/WorkflowOrchestrator  
- `RelayConnectionPool` + `DevServerRelayBridge`: quản lý WS connections — **nhưng không expose API cho UI** để trigger `agent.spawn`

## Luồng bị broken

```
User click "Start Agent"
    → contextBridge.invoke('agent.start', { worktreeId, agentType, trustPreset })
    → [Main Process — AgentManager.start()] ← KHÔNG TỒN TẠI
    → [conn = AgentConnectionManager.getConnection()] ← KHÔNG TỒN TẠI
    → conn.call('agent.spawn', ...) ← KHÔNG GỌI ĐƯỢC
```

## Ảnh hưởng

1. UI không thể start AI agent từ worktree card
2. BL-AG-01, BL-AG-03, BL-AG-04 đều phụ thuộc vào `AgentManager` → tất cả broken
3. `ProfileAwareAgentSpawner` chỉ phục vụ TaskAgentExecutor (task automation), không phải interactive agent

## Cần implement

```typescript
// src/main/agent/AgentManager.ts
export class AgentManager {
  constructor(
    private readonly relayPool: RelayConnectionPool,
    private readonly profileResolver: ProfileResolver,
    private readonly aiProviderService: AIProviderService,
    private readonly db: Database
  ) {}

  async start(opts: { userId, worktreeId, agentType, trustPreset }): Promise<{ sessionId, ptyId }>
  async stop(opts: { sessionId, force?: boolean }): Promise<void>
  async resume(opts: { worktreeId, userId }): Promise<{ sessionId }>
  async switchAccount(opts: { sessionId, newAccountId }): Promise<void>
}
```

## Liên quan đến luồng

- **BL-AG-01**: Start Agent — AgentManager.start() missing
- **BL-AG-02**: Stop Agent — AgentManager.stop() missing
- **BL-AG-03**: Resume Session — AgentManager.resume() missing
- **BL-AG-04**: Switch Account — AgentManager.switchAccount() missing

---

## ⏸ Fix Status: DEFERRED

**Reason:** AgentManager is Orca Server responsibility, not relay binary. Deferred to Phase 3 — Orca Server integration.
