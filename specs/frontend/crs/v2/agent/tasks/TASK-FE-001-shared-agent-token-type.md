# TASK-FE-001 — Thêm `AgentTokenInfo` type vào `dev-server-types.ts`

**Solution:** [SOL-FE-AG-003](../solutions/SOL-FE-AG-003-ipc-agent-token-event.md)  
**File:** `src/shared/dev-server-types.ts` [MODIFY]  
**Depends on:** TASK-012 (backend — AgentWebSocketServer)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

Thêm type `AgentTokenInfo` vào shared types để dùng chung giữa main process, preload và renderer.

---

## Thay đổi cần thực hiện

### File: `src/shared/dev-server-types.ts`

**Thêm vào cuối file** (sau các type hiện có):

```typescript
/**
 * Payload emitted by DevServerRelayBridge when a direct-websocket token is generated.
 * Shared between main process (IPC sender) and renderer (IPC receiver).
 */
export type AgentTokenInfo = {
  /** ID of the DevServer this token belongs to */
  devServerId: string
  /** One-time token for agent to authenticate: format "agt-<devServerId>-<timestamp>" */
  agentToken: string
  /** Orca WebSocket URL the agent should connect to: "ws://<host>:6768/agent" */
  orcaUrl: string
}
```

---

## Acceptance Criteria

- [x] `AgentTokenInfo` type exported từ `src/shared/dev-server-types.ts`
- [x] 3 fields: `devServerId: string`, `agentToken: string`, `orcaUrl: string`
- [x] TypeScript compile không lỗi (`pnpm exec tsc --noEmit`)
