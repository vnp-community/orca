# CR-GH-001: `preflight.check` phải route đến Dev Server trong Web mode

**ID:** CR-GH-001  
**Priority:** 🔴 Critical  
**Component:** `src/main/runtime/rpc/methods/preflight.ts`, `src/main/ipc/preflight.ts`  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-01-CLI-Preflight, FE-SOL-01, FE-SOL-04  
**Tasks:** TASK-01 (backend), FE-TASK-09, FE-TASK-10

## Acceptance Criteria — Verified

1. ✅ `preflight.check` trong Web mode trả về `gh.installed = true` khi `gh` có trên Dev Server — via relay proxy (`preflight.ts` L32-40)
2. ✅ `preflight.check` trong Web mode trả về `gh.authenticated = true` khi Dev Server đã `gh auth login` — relay executes on Dev Server
3. ✅ Fallback: nếu không có DevServer connected, check trên Orca Server — `params.devServerId` optional, falls back to local (`preflight.ts` L30)
4. ✅ `preflightStatusError = null` khi relay call thành công

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/runtime/rpc/methods/preflight.ts` | `devServerId` optional param → relay proxy (L11-40) |
| Backend | `src/main/runtime/runtime-rpc.ts` | `devServerManager` injected vào `RpcDispatcher` (L48, L462-467) |
| Frontend | `src/renderer/src/store/slices/preflight.ts` | `activeDevServerId` gửi kèm RPC call (L111-113) |
| Frontend | `source-control-preflight-card-status.ts` | `mergePreflightStatuses()` — relay result ưu tiên cho CLI cards |

---

## Vấn đề

### Kiến trúc hiện tại (Web mode)

```
Browser → WebSocket → Orca Server (172.20.2.39)
                            │
                    preflight.check RPC
                            │
                    runPreflightCheck()
                            │
                   execLocalPreflightCommand('gh', ...)
                            │
                    /usr/bin/gh (trên Orca container)
                            │
                  → gh installed=true, authenticated=false
                    (vì container chưa login GitHub)
```

**Kết quả:** UI hiển thị "Not authenticated" cho GitHub — đây là thông tin của **container**, không phải của **dev machine** (172.20.2.31).

### Kiến trúc mục tiêu

```
Browser → WebSocket → Orca Server (172.20.2.39)
                            │
                    preflight.check RPC
                            │
                 [CR-GH-001] Resolve active DevServer
                            │ (nếu có DevServer connected)
                    SSH Relay → Dev Server (172.20.2.31)
                            │
                    relay.call('preflight.check', {})
                            │
                  → gh installed=true/false, authenticated=true/false
                    (thông tin thực của dev machine)
```

---

## Root Cause Analysis

### Code path hiện tại

**File:** `src/main/runtime/rpc/methods/preflight.ts`
```typescript
// HIỆN TẠI: gọi runPreflightCheck() trực tiếp trên Orca Server process
defineMethod({
  name: 'preflight.check',
  params: PreflightCheck,
  handler: async (params) => runPreflightCheck(params.force)  // ← chạy TRÊN Orca Server
})
```

**File:** `src/main/ipc/preflight.ts` — `runPreflightCheck()` gọi:
```typescript
execLocalPreflightCommand('gh', ['auth', 'status'])  // ← process.execFile trên Orca Server container
```

### Sự khác biệt giữa Electron mode và Web mode

| Mode | `preflight.check` handler | Execution target |
|------|--------------------------|-----------------|
| **Electron** | `ipcMain.handle('preflight:check')` → `runPreflightCheck()` | Local machine (đúng) |
| **Web** | RPC server `preflight.check` → `runPreflightCheck()` | **Orca Server container (sai)** |
| **Web + DevServer** | Phải → relay → dev server | Dev Server (172.20.2.31) ✓ |

### Onboarding flow đã làm đúng (Electron mode)

```typescript
// src/main/ipc/onboarding-ipc.ts
ipcMain.handle('onboarding.getPreflightStatus', async (_event, params) => {
  const relay = devServerManager.getRelay(params.devServerId)
  const raw = await relay.call<{...}>('preflight.check', {}, 30_000)
  //                              ↑ relay call đến DEV SERVER ✓
})
```

**Vấn đề:** Handler này chỉ được dùng trong Onboarding flow (Electron), không có trong Web mode.

---

## Proposed Solution

### Option A: Server-side context injection (Recommended)

Modify `preflight.check` RPC handler để nhận `devServerId` và proxy qua relay:

**File:** `src/main/runtime/rpc/methods/preflight.ts`
```typescript
const PreflightCheck = z.object({
  force: z.boolean().optional(),
  devServerId: z.string().optional()  // [NEW] target dev server
})

defineMethod({
  name: 'preflight.check',
  params: PreflightCheck,
  handler: async (params, context) => {
    // [CR-GH-001] Nếu có devServerId, proxy qua SSH relay
    if (params.devServerId) {
      const relay = context.devServerManager.getRelay(params.devServerId)
      if (!relay) throw new Error(`Dev server '${params.devServerId}' not connected`)
      return relay.call<PreflightStatus>('preflight.check', { force: params.force }, 30_000)
    }
    // Fallback: local check (backward-compatible)
    return runPreflightCheck(params.force)
  }
})
```

### Option B: Auto-detect active dev server

Handler tự động detect dev server đang active khi không có `devServerId` explicit:

```typescript
handler: async (params, context) => {
  // [CR-GH-001] Auto-detect connected dev server
  const activeDevServer = context.devServerManager.list()
    .find(ds => ds.status === 'connected')
  
  if (activeDevServer) {
    const relay = context.devServerManager.getRelay(activeDevServer.id)!
    return relay.call<PreflightStatus>('preflight.check', { force: params.force }, 30_000)
  }
  
  return runPreflightCheck(params.force)
}
```

---

## Files cần thay đổi

### [MODIFY] `src/main/runtime/rpc/methods/preflight.ts`
- Thêm `devServerId` optional vào `PreflightCheck` schema
- Thêm relay proxy logic trong `preflight.check` handler
- Inject `devServerManager` vào handler context

### [MODIFY] `src/main/runtime/runtime-rpc.ts`
- Pass `devServerManager` vào RPC method context
- Cần xem `OrcaRuntimeRpcServer` constructor để thêm dependency

### [MODIFY] `src/renderer/src/store/slices/preflight.ts`
- Khi `runtimeTarget.kind === 'environment'` (Web mode), gửi `devServerId` kèm theo RPC call

### [MODIFY] `src/renderer/src/web/web-preload-api.ts`
- `createPreflightApi().check()` gửi kèm `devServerId` của active dev server

---

## Acceptance Criteria

1. `preflight.check` trong Web mode trả về `gh.installed = true` khi `gh` có trên Dev Server
2. `preflight.check` trong Web mode trả về `gh.authenticated = true` khi Dev Server đã `gh auth login`
3. Fallback: nếu không có DevServer connected, check trên Orca Server (backward-compatible)
4. `preflightStatusError` = null khi relay call thành công
5. GitHub Integration card hiển thị đúng trạng thái (không phải "Unavailable")

---

## Dependencies

- Dev Server phải đang connected (SSH relay active)
- `orca-relay` binary trên Dev Server phải handle `preflight.check` RPC

## Related

- CR-GH-002: `gh auth login` via Remote PTY
- CR-GH-003: Web mode preflight context resolution
