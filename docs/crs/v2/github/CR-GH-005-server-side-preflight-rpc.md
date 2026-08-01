# CR-GH-005: Server-side `preflight.check` RPC proxy — Full Flow Implementation

**ID:** CR-GH-005  
**Priority:** 🟡 Medium  
**Component:** `src/main/runtime/runtime-rpc.ts`, `src/main/runtime/rpc/core.ts`  
**Depends on:** CR-GH-001, CR-GH-003  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-01-CLI-Preflight  
**Tasks:** TASK-01 (rpc context devserver)

## Acceptance Criteria — Verified

1. ✅ `RpcMethodContext` type exists — `src/main/runtime/runtime-rpc.ts` L48 (`devServerManager?`)
2. ✅ `OrcaRuntimeRpcServer` constructor nhận `devServerManager` — L462-467
3. ✅ `dispatch()` builds context và pass vào mỗi handler — `RpcDispatcher`
4. ✅ `preflight.check` handler nhận và sử dụng `ctx.devServerManager` — `methods/preflight.ts` L32

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/runtime/runtime-rpc.ts` | RpcMethodContext + devServerManager injection (L48, L462-467) |
| Backend | `src/main/runtime/rpc/methods/preflight.ts` | Uses ctx.devServerManager.getRelay() (L32-35) |


---

## Vấn đề

CR-GH-001 xác định **cần** proxy `preflight.check` đến relay. CR này xác định **cách** inject `devServerManager` vào RPC handler context — đây là missing infrastructure.

### Hiện tại: RPC handlers không có access đến `devServerManager`

**File:** `src/main/runtime/rpc/core.ts`
```typescript
export type RpcMethod = {
  name: string
  params: ZodType | null
  handler: (params: any) => Promise<unknown>
  //                ↑ không có context!
}
```

**File:** `src/main/runtime/rpc/methods/preflight.ts`
```typescript
handler: async (params) => runPreflightCheck(params.force)
//              ↑ không có devServerManager
```

---

## Analysis: Cách inject context

### Option A: Module-level singleton (hiện tại pattern)

```typescript
// Dùng module-level import (không tốt — global state)
import { devServerManager } from '../../../dev-server/dev-server-manager'
```

Vấn đề: tạo circular dependency, khó test.

### Option B: Context object trong handler (Recommended)

```typescript
export type RpcMethodContext = {
  devServerManager: DevServerManagerInterface
  sessionId?: string
  userId?: string
}

export type RpcMethod = {
  name: string
  params: ZodType | null
  handler: (params: any, context: RpcMethodContext) => Promise<unknown>
  //                         ↑ context được inject khi dispatch
}
```

### Option C: Service locator / DI container

Phức tạp hơn, phù hợp cho future.

---

## Proposed Solution (Option B)

### Step 1: Extend `RpcMethod` type với context

**File:** `src/main/runtime/rpc/core.ts`
```typescript
export type RpcMethodContext = {
  /** DevServerManager để proxy calls sang relay */
  devServerManager: DevServerManagerInterface
  /** Session ID cho multi-user isolation */  
  sessionId?: string
  /** User ID của request */
  userId?: string
}

export type RpcMethod<TParams = unknown> = {
  name: string
  params: ZodType<TParams> | null
  handler: (params: TParams, context: RpcMethodContext) => Promise<unknown>
}
```

### Step 2: `OrcaRuntimeRpcServer` nhận `devServerManager` trong constructor

**File:** `src/main/runtime/runtime-rpc.ts`
```typescript
type OrcaRuntimeRpcServerOptions = {
  runtime: OrcaRuntimeService
  userDataPath: string
  devServerManager: DevServerManagerInterface  // [NEW]
  // ... existing options
}

class OrcaRuntimeRpcServer {
  private devServerManager: DevServerManagerInterface
  
  constructor(options: OrcaRuntimeRpcServerOptions) {
    this.devServerManager = options.devServerManager
    // ...
  }
  
  private buildContext(sessionInfo?: SessionInfo): RpcMethodContext {
    return {
      devServerManager: this.devServerManager,
      sessionId: sessionInfo?.sessionId,
      userId: sessionInfo?.userId
    }
  }
  
  private async dispatch(method: RpcMethod, params: unknown, sessionInfo?: SessionInfo) {
    const context = this.buildContext(sessionInfo)
    return method.handler(params, context)
  }
}
```

### Step 3: `preflight.check` handler dùng context

**File:** `src/main/runtime/rpc/methods/preflight.ts`
```typescript
defineMethod({
  name: 'preflight.check',
  params: PreflightCheck,
  handler: async (params, context) => {
    if (params.devServerId) {
      const relay = context.devServerManager.getRelay(params.devServerId)
      if (!relay) throw new Error(`Dev server '${params.devServerId}' not connected`)
      
      const ghConfigDir = context.sessionId
        ? `/tmp/orca-sessions/${context.sessionId}/gh`
        : undefined
        
      return relay.call<PreflightStatus>('preflight.check', {
        force: params.force,
        ...(ghConfigDir ? { env: { GH_CONFIG_DIR: ghConfigDir } } : {})
      }, 30_000)
    }
    
    // Fallback: local check on Orca Server
    return runPreflightCheck(params.force)
  }
})
```

### Step 4: `server-bootstrap.ts` wire devServerManager vào RPC server

**File:** `src/main/server-bootstrap.ts`
```typescript
const rpcServer = new OrcaRuntimeRpcServer({
  runtime: orcaRuntime,
  userDataPath,
  devServerManager,  // [NEW] inject
  enableWebSocket: true,
  wsPort: ORCA_PORT
})
```

---

## Full Data Flow sau khi implement CR-GH-001 + CR-GH-003 + CR-GH-005

```
1. Browser (HTTPS)
   └─ new WebSocket("wss://b15.openledger.vn")
   
2. Nginx (443) 
   └─ location / { if $http_upgrade = "websocket" → orca:6768 }
   
3. Orca Server :6768 (OrcaRuntimeRpcServer)
   └─ E2EE WebSocket handshake
   └─ Decrypt: { type: "rpc", method: "preflight.check", params: { devServerId: "ds-abc" } }
   
4. RPC Dispatcher
   └─ Find method "preflight.check"
   └─ Validate params: { devServerId: "ds-abc" }
   └─ Build context: { devServerManager, sessionId: "sess-xyz" }
   └─ Call handler(params, context)
   
5. preflight.check handler
   └─ params.devServerId = "ds-abc" → proxy mode
   └─ relay = context.devServerManager.getRelay("ds-abc")
   └─ ghConfigDir = "/tmp/orca-sessions/sess-xyz/gh"
   └─ relay.call("preflight.check", { force: false, env: { GH_CONFIG_DIR: "..." } }, 30_000)
   
6. SSH Relay → Dev Server (172.20.2.31)
   └─ orca-relay binary nhận "preflight.check" RPC
   └─ Set env: GH_CONFIG_DIR=/tmp/orca-sessions/sess-xyz/gh
   └─ isCommandAvailable("gh") → /usr/bin/gh (installed=true)
   └─ execCommand("gh auth status", { env }) → exit 0 (authenticated=true)
   └─ Return: { git: {installed: true}, gh: {installed: true, authenticated: true} }
   
7. Orca Server nhận result
   └─ Encrypt → WebSocket frame
   
8. Browser nhận result
   └─ preflightStatus = { gh: { installed: true, authenticated: true } }
   └─ GitHub Integration card: "Connected" ✓
```

---

## Files cần thay đổi

### [MODIFY] `src/main/runtime/rpc/core.ts`
- Thêm `RpcMethodContext` type
- `RpcMethod.handler` signature: `(params, context) => Promise<unknown>`

### [MODIFY] `src/main/runtime/runtime-rpc.ts`  
- Constructor nhận `devServerManager`
- `dispatch()` build và pass context

### [MODIFY] `src/main/server-bootstrap.ts`
- Inject `devServerManager` vào `OrcaRuntimeRpcServer`

### [MODIFY] `src/main/runtime/rpc/methods/preflight.ts`
- Handler nhận `context`, proxy đến relay

### [MODIFY] `src/main/runtime/rpc/methods/*.ts` (tất cả)
- Update signature để tương thích `(params, context)` 
- Handlers không cần context có thể ignore tham số thứ 2

### [MODIFY] `src/main/runtime/rpc/methods/preflight.test.ts`
- Update tests để mock `RpcMethodContext`

---

## Acceptance Criteria

1. `RpcMethod.handler` nhận `context: RpcMethodContext` làm tham số thứ 2
2. `OrcaRuntimeRpcServer` inject đúng context vào mọi handler
3. `preflight.check` proxy đến Dev Server khi `devServerId` được cung cấp
4. All existing unit tests pass (backward compatible)
5. `devServerManager` không accessible nếu không inject (không dùng global)

---

## Migration Strategy

Vì thay đổi signature của `RpcMethod.handler`, cần:

```typescript
// Cũ:
defineMethod({
  name: 'some.method',
  handler: async (params) => { ... }
})

// Mới (backward compatible — context optional):
defineMethod({
  name: 'some.method', 
  handler: async (params, _context?) => { ... }
})
```

TypeScript sẽ check nhưng không break existing handlers nếu context là optional.

## Related

- CR-GH-001: Proxy logic specification
- CR-GH-003: Context data từ renderer
- CR-GH-004: Session isolation env vars
