# CR-GH-003: Web mode preflight context — devServerId resolution

**ID:** CR-GH-003  
**Priority:** 🟠 High  
**Component:** `src/renderer/src/store/slices/preflight.ts`, `src/renderer/src/web/web-preload-api.ts`  
**Depends on:** CR-GH-001  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-05-Context-Injection, FE-SOL-01  
**Tasks:** TASK-01 (backend devServerId routing), FE-TASK-09, FE-TASK-10

## Acceptance Criteria — Verified

1. ✅ Khi user chọn Dev Server trong Settings, `preflight.check` gửi `devServerId` — preflight.ts slice L111-113
2. ✅ Orca Server proxy `preflight.check` đến đúng Dev Server — devServerManager.getRelay()
3. ✅ Cache invalidate khi đổi activeDevServerId — preflightStatusContextKey
4. ✅ Preflight re-check khi kết nối/ngắt dev server — clearRemotePreflightStatus() action

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | runtime-rpc.ts | devServerManager injected (L48, L462-467) |
| Frontend | slices/preflight.ts | devServerId sent in RPC + relay cache |
| Frontend | source-control-preflight-card-status.ts | mergePreflightStatuses() |


---

## Vấn đề

### Hiện tại: Renderer không gửi `devServerId` khi gọi `preflight.check`

**File:** `src/renderer/src/store/slices/preflight.ts`
```typescript
const request = (
  runtimeTarget.kind === 'environment'
    ? callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', force ? { force } : {})
    //                                                                   ↑ Không có devServerId!
    : window.api.preflight.check(preflightArgs)
)
```

Trong Web mode (`runtimeTarget.kind === 'environment'`), `preflight.check` được gọi qua WebSocket RPC đến Orca Server, nhưng không kèm `devServerId` nào. Server không biết phải proxy đến Dev Server nào.

### Vấn đề thứ 2: Không có active Dev Server context trong Web mode

Web mode không có khái niệm "active dev server" tương đương Electron mode. User phải chọn dev server để làm việc, nhưng thông tin này không được truyền vào `preflight.check`.

---

## Analysis: Cách Electron mode xử lý

```typescript
// Electron: preflight context được set qua local WSL/Platform context
const context = getLocalPreflightContext(get())
// → { wslDistro: 'Ubuntu' } hoặc undefined

// Web mode: phải thêm devServerId vào context
// → { devServerId: 'ds-abc123' }
```

### Current `getLocalPreflightContext` logic:

```typescript
// src/renderer/src/lib/local-preflight-context.ts
export function getLocalPreflightContext(state: AppState): PreflightRuntimeContext | undefined {
  // ... WSL distro detection, etc.
  // Web mode: returns undefined → no remote context
}
```

---

## Proposed Solution

### 1. Extend `PreflightRuntimeContext` (shared type)

**File:** `src/shared/types.ts` hoặc `src/renderer/src/lib/local-preflight-context.ts`
```typescript
export type PreflightRuntimeContext = {
  wslDistro?: string
  devServerId?: string  // [NEW] Web mode: target dev server
}
```

### 2. `getLocalPreflightContext` trả về devServerId trong Web mode

**File:** `src/renderer/src/lib/local-preflight-context.ts`
```typescript
export function getLocalPreflightContext(state: AppState): PreflightRuntimeContext | undefined {
  // [CR-GH-003] Web mode: check for active dev server
  if (isWebMode()) {
    const activeDevServerId = state.settings?.activeDevServerId  // or wherever stored
    if (activeDevServerId) {
      return { devServerId: activeDevServerId }
    }
  }
  
  // Existing WSL logic...
  const wslDistro = getWslDistroFromSettings(state)
  return wslDistro ? { wslDistro } : undefined
}
```

### 3. `preflight.check` RPC call kèm `devServerId`

**File:** `src/renderer/src/store/slices/preflight.ts`
```typescript
const context = getLocalPreflightContext(get())
// context = { devServerId: 'ds-abc123' } trong Web mode

const request = (
  runtimeTarget.kind === 'environment'
    ? callRuntimeRpc<PreflightStatus>(
        runtimeTarget, 
        'preflight.check', 
        { force: force || undefined, ...context }  // [CR-GH-003] include devServerId
      )
    : window.api.preflight.check(preflightArgs)
)
```

### 4. Active Dev Server selection

Cần thêm UI/logic để user chọn dev server trong Web mode Settings:

```typescript
// New store state
interface AppState {
  settings: {
    activeDevServerId?: string  // [NEW] selected dev server for web mode
  }
}
```

---

## Context Key cho cache

`localPreflightContextKey` cần include `devServerId`:

```typescript
export function localPreflightContextKey(ctx?: PreflightRuntimeContext): string {
  if (ctx?.devServerId) return `devserver:${ctx.devServerId}`
  if (ctx?.wslDistro) return `wsl:${ctx.wslDistro}`
  return 'local'
}
```

Điều này đảm bảo:
- Cache bị invalidate khi đổi dev server
- Preflight check lại khi kết nối/ngắt kết nối dev server

---

## Files cần thay đổi

### [MODIFY] `src/shared/types.ts`
- Thêm `devServerId?: string` vào `PreflightRuntimeContext`

### [MODIFY] `src/renderer/src/lib/local-preflight-context.ts`
- `getLocalPreflightContext()` detect web mode, return `{ devServerId }`
- `localPreflightContextKey()` include `devServerId` trong key

### [MODIFY] `src/renderer/src/store/slices/preflight.ts`
- Pass `context` (bao gồm `devServerId`) vào `preflight.check` RPC call

### [MODIFY] `src/renderer/src/components/settings/` (Settings UI)
- Thêm "Active Dev Server" selector trong web mode
- Update `settings.activeDevServerId` khi user chọn

### [MODIFY] `src/main/runtime/rpc/methods/preflight.ts`
- Handler nhận `devServerId` từ params (theo CR-GH-001)

---

## Acceptance Criteria

1. Khi user chọn Dev Server trong Settings, `preflight.check` gửi `devServerId`
2. Orca Server proxy `preflight.check` đến đúng Dev Server
3. Cache key thay đổi khi đổi dev server → re-check xảy ra
4. `preflightStatusContextKey` = `'devserver:ds-abc123'` trong Web+DevServer mode
5. Nếu không có dev server nào được chọn → fallback về Orca Server check

## Related

- CR-GH-001: Server-side relay proxy
- CR-GH-004: Session isolation
