# TASK-PC-002 — Thêm `createSessionWebRuntimeEnvironment()` vào web-runtime-environment.ts

**Solution:** [SOL-PC-002 §Thay đổi 2](../solutions/SOL-PC-002-browser-session-rpc.md)  
**Bug:** [BUG-PC-001](../BUG-PC-001-browser-requires-paircode.md)  
**File:** `src/renderer/src/web/web-runtime-environment.ts`  
**Phụ thuộc:** Không  
**Estimated:** 20 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thêm factory function `createSessionWebRuntimeEnvironment()` — tạo một `StoredWebRuntimeEnvironment` đặc biệt cho session-based auth (không cần Pair Code / E2EE). Dùng để `requireActiveEnvironment()` không throw khi user đã login qua email/password.

---

## Context

Đọc trước:
- `src/renderer/src/web/web-runtime-environment.ts` — toàn bộ file (117 dòng)
- `src/shared/runtime-environments.ts` — L28-34 để hiểu các required fields

**Type cần satisfy:**
```typescript
StoredWebRuntimeEnvironment = Omit<PublicKnownRuntimeEnvironment, 'endpoints'> & {
  endpoints: {
    id: string
    kind: 'websocket'
    label: string
    endpoint: string
    deviceToken: string    // ← phải có nhưng với session mode: empty string ''
    publicKeyB64: string   // ← phải có nhưng với session mode: empty string ''
  }[]
}
// Required base fields: id, name, createdAt, updatedAt, lastUsedAt, runtimeId, preferredEndpointId
```

---

## Thay Đổi Cần Thực Hiện

### File: `src/renderer/src/web/web-runtime-environment.ts`

**THÊM VÀO CUỐI FILE** (sau `redactStoredWebRuntimeEnvironment`):

```typescript
/**
 * Create a StoredWebRuntimeEnvironment for session-based auth.
 *
 * Used when ORCA_MULTI_USER=1 and the user has logged in via /auth/local
 * (email + password). WsSessionRouter routes WebSocket connections via
 * session cookie — no E2EE Pair Code required.
 *
 * The generated environment:
 *   - Points to ws(s)://same-host/ws (session-authenticated WS endpoint)
 *   - Has stable id 'session-auth' (no random uuid — consistent across reloads)
 *   - deviceToken = '' and publicKeyB64 = '' (no E2EE — cookie is the auth)
 *   - connectionType = 'session' (distinguishes from 'pairing' path)
 *
 * @param location - window.location (or equivalent for testability)
 */
export function createSessionWebRuntimeEnvironment(
  location: Pick<Location, 'protocol' | 'host'>
): StoredWebRuntimeEnvironment {
  const wsProtocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsEndpoint = `${wsProtocol}//${location.host}/ws`
  const now = Date.now()
  const envId = 'session-auth'

  return {
    id: envId,
    name: 'Orca Session',
    createdAt: now,
    updatedAt: now,
    lastUsedAt: null,
    runtimeId: null,
    preferredEndpointId: `ws-${envId}`,
    endpoints: [
      {
        id: `ws-${envId}`,
        kind: 'websocket',
        label: 'Session WebSocket',
        endpoint: wsEndpoint,
        // No E2EE — WsSessionRouter validates session cookie instead
        deviceToken: '',
        publicKeyB64: ''
      }
    ]
  }
}
```

---

## Verify

```bash
# TypeScript compile check
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "web-runtime-environment" | head -10
# Expected: không có lỗi

# Build frontend
pnpm build:frontend:web 2>&1 | tail -20
# Expected: build thành công, không có type error
```

---

## Definition of Done

- [x] Function `createSessionWebRuntimeEnvironment` đã được export từ `web-runtime-environment.ts`
- [x] Function nhận `location: Pick<Location, 'protocol' | 'host'>` (testable, không dùng `window.location` trực tiếp)
- [x] Trả về `StoredWebRuntimeEnvironment` đúng shape (TypeScript happy)
- [x] `deviceToken` và `publicKeyB64` là empty string `''` (không phải null — type yêu cầu string)
- [x] `id = 'session-auth'` (stable, không random)
- [x] TypeScript compile OK
