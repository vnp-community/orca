# FE-SOL-01: preflight.check gửi devServerId từ Frontend

> **CRs:** CR-GH-001, CR-GH-003, CR-INT-001  
> **Backend SOL tương ứng:** SOL-01-CLI-Preflight, SOL-05-Context-Injection  
> **TDD:** TDD-FE-02 (State Management), TDD-FE-09 (Onboarding/DevServer)  
> **Status:** ✅ DONE & 🧪 AC Verified (2026-07-25)  
> **Tasks:** [FE-TASK-10](../tasks/FE-TASK-10-preflight-response-update.md)

---

## Vấn đề

Khi browser gọi `preflight.check`, Orca Server không biết phải chạy CLI check trên Dev Server nào. Cần truyền `devServerId` từ renderer để server proxy sang đúng SSH relay.

---

## Thiết kế giải pháp

### 1. Preflight Slice — gửi `devServerId` kèm RPC call

```typescript
// src/renderer/src/store/slices/preflight.ts
// Khi trigger preflight.check trong Web mode, gửi activeDevServerId:
const rpcParams = {
  force: force || undefined,
  // Why: devServerId tells the Orca Server which SSH relay
  // to proxy the CLI check through (gh, glab, git on Dev Server).
  // Omit if no active dev server (falls back to local check).
  ...(get().activeDevServerId ? { devServerId: get().activeDevServerId } : {})
}

callRuntimeRpc(runtimeTarget, 'preflight.check', rpcParams)
```

### 2. Remote Preflight State trong Zustand

```typescript
// src/renderer/src/store/slices/preflight.ts
type PreflightSlice = {
  // ... existing fields ...
  remotePreflightByServer: Record<string, RemotePreflightStatus>
  activeRemotePreflightStatus: RemotePreflightStatus | null

  setRemotePreflightStatus: (devServerId: string, status: RemotePreflightStatus) => void
  clearRemotePreflightStatus: (devServerId: string) => void
}
```

### 3. Web Preload API — `preflight.check` đã expose `devServerId`

`window.api.preflight.check({ force, devServerId })` → `callRuntimeResult('preflight.check', { force, devServerId })`

---

## Implementation Status

### Files đã implement

#### `src/renderer/src/store/slices/preflight.ts` [MODIFIED]

- ✅ `devServerId` gửi kèm `preflight.check` (line 111-113)
- ✅ `remotePreflightByServer: Record<string, RemotePreflightStatus>` — per-server status cache (line 19)
- ✅ `activeRemotePreflightStatus: RemotePreflightStatus | null` — trạng thái của active server (line 20)
- ✅ `setRemotePreflightStatus(devServerId, status)` — action (line 62)
- ✅ `clearRemotePreflightStatus(devServerId)` — action (line 72)
- ✅ `setRemotePreflightStatus` được gọi trong `.then()` callback khi relay trả về (line 128) — **FE-TASK-10**

#### `src/renderer/src/web/web-preload-api.ts` [MODIFIED — existing behavior]

- ✅ `callRuntimeResult('preflight.check', { devServerId? })` — truyền devServerId xuống WebSocket RPC

---

## Acceptance Criteria

1. ✅ Khi `activeDevServerId` có giá trị → `preflight.check` gửi `{ devServerId: 'ds-xxx' }`
2. ✅ Khi `activeDevServerId = null` → gửi `{}` (fallback về local check trên Orca Server)
3. ✅ Response từ relay → `setRemotePreflightStatus(devServerId, adaptedStatus)` (FE-TASK-10)
4. ✅ Card status đọc `activeRemotePreflightStatus` qua `mergePreflightStatuses` (FE-TASK-09)

---

## Tham chiếu Backend

- Backend `preflight.ts` handler kiểm tra `params.devServerId && ctx.devServerManager`
- Relay trả về `{ platform, gh, glab, git }` — toàn bộ CLI status
- Nếu relay không connect → throw `Error('Dev server not connected')`
