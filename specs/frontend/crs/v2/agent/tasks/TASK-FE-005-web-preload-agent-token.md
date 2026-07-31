# TASK-FE-005 — `web-preload-api.ts`: Expose `onAgentToken` / `offAgentToken` (Web mode)

**Solution:** [SOL-FE-AG-003](../solutions/SOL-FE-AG-003-ipc-agent-token-event.md)  
**File:** `src/renderer/src/web/web-preload-api.ts` [MODIFY]  
**Depends on:** TASK-FE-001 (AgentTokenInfo type)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

Thêm `onAgentToken` / `offAgentToken` vào `createDevServerApi()` trong `web-preload-api.ts`. Đây là Web mode equivalent của TASK-FE-004 (preload.ts).

---

## Context hiện tại

```typescript
// src/renderer/src/web/web-preload-api.ts — line 827-906
function createDevServerApi(): NonNullable<Partial<PreloadApi>['devServer']> {
  // ...existing polling handlers...

  return {
    list: ...,
    add: ...,
    remove: ...,
    connect: ...,
    disconnect: ...,
    testConnection: ...,
    onStatusChanged: (handler) => { /* polling approach */ },
    listSshTargets: ...,
    addSshTarget: ...,
    // ← Cần thêm onAgentToken / offAgentToken tại đây
  }
}
```

---

## Thay đổi cần thực hiện

### File: `src/renderer/src/web/web-preload-api.ts`

**Thêm import:**
```typescript
import type { AgentTokenInfo } from '../../../shared/dev-server-types'
// (hoặc check relative path hiện tại trong file)
```

**Thêm handler registry trước `return` statement trong `createDevServerApi()`:**

```typescript
function createDevServerApi(): NonNullable<Partial<PreloadApi>['devServer']> {
  // ... existing pollingIntervals ...

  // Registry cho agentToken handlers (Web mode)
  // Why: Web mode không dùng ipcRenderer. Handler được gọi thủ công
  // khi server push event. Trong Phase 2 dùng polling approach
  // (check testConnection result cho agentToken field) vì WebSocket
  // push events chưa được wired.
  const agentTokenHandlers = new Set<(info: AgentTokenInfo) => void>()

  return {
    // ... existing methods ...

    // Why: direct-websocket mode generates a one-time token that the user
    // must copy and use to start the agent. We expose these handlers so
    // useAddDevServer can subscribe regardless of Electron/Web mode.
    //
    // Web mode implementation note:
    // The backend testConnection() for direct-websocket is a long-running
    // promise (up to 60s). We poll via onStatusChanged to detect when
    // direct-websocket devServer transitions to 'connected'. The token
    // itself is returned in the testConnection result (if connectionType
    // == 'direct-websocket' and AgentTokenInfo is embedded in result).
    //
    // Alternative: If server-side WebSocket push is available, use it here.
    onAgentToken: (handler: (info: AgentTokenInfo) => void) => {
      agentTokenHandlers.add(handler)
      return () => { agentTokenHandlers.delete(handler) }
    },

    offAgentToken: (handler: (info: AgentTokenInfo) => void) => {
      agentTokenHandlers.delete(handler)
    },

    // Utility: call all agentToken handlers (used internally if needed)
    // Not part of public PreloadApi — internal only
    _emitAgentToken: (info: AgentTokenInfo) => {
      for (const h of agentTokenHandlers) h(info)
    },
  }
}
```

**Cập nhật `PreloadApi` type** (nếu có, thêm vào `devServer` section):
```typescript
// src/shared/preload-api.ts (hoặc tương đương):
devServer: {
  // ... existing ...
  onAgentToken?: (handler: (info: AgentTokenInfo) => void) => void | (() => void)
  offAgentToken?: (handler: (info: AgentTokenInfo) => void) => void
}
```

---

## Acceptance Criteria

- [x] `onAgentToken` được export trong `createDevServerApi()` return object
- [x] `offAgentToken` được export trong `createDevServerApi()` return object
- [x] Handler registry không gây memory leak (Set-based, cleanup via offAgentToken)
- [x] `AgentTokenInfo` được import đúng relative path
- [x] TypeScript compile không lỗi (bao gồm cả `PreloadApi` type compatibility)
