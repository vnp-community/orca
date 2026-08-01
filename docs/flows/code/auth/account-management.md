# Account Management — Orca Server

> **Scope**: Quản lý devices, users, và access tokens trong Orca
> **Key files**:
> - [`src/main/runtime/device-registry.ts`](../../src/main/runtime/device-registry.ts) — DeviceRegistry class
> - [`src/main/runtime/mobile-pairing-files.ts`](../../src/main/runtime/mobile-pairing-files.ts) — File paths
> - [`src/shared/rbac-types.ts`](../../src/shared/rbac-types.ts) — OrcaUser, ScopedPairingToken, OrcaAccessPolicy
> - [`src/main/runtime/runtime-rpc.ts`](../../src/main/runtime/runtime-rpc.ts) — Device management RPCs

---

## 1. Tổng quan

Account Management trong Orca hiện tại hoạt động theo mô hình **device-centric** (không có user account). Mỗi "account" là một **DeviceEntry** — một credential pair giữa client và server.

```
Orca Server
  │
  └── DeviceRegistry (orca-devices.json)
        │
        ├── DeviceEntry [scope=runtime]   ← Web browser / CLI
        │     deviceId: uuid
        │     token: 48-hex (credential)
        │     scope: 'runtime'
        │     pairedAt: timestamp
        │     lastSeenAt: timestamp
        │
        ├── DeviceEntry [scope=mobile]    ← Mobile app (iOS/Android)
        │     ...
        │
        └── ScopedPairingToken (in-memory)  ← RBAC-limited (24h)
              token: 64-hex
              userId, userEmail, userName
              allowedServerIds, allowedProjects
              agentTrust: 'minimal'|'standard'|'full'
              expiresAt: +24h
```

---

## 2. DeviceEntry — Dữ liệu device

```typescript
// src/main/runtime/device-registry.ts
export type DeviceEntry = {
  deviceId:   string   // UUID v4 — định danh device
  name:       string   // "Web Browser", "CLI 7/24/2026", "iPhone 15"
  token:      string   // 48-hex — credential dùng trong E2EE auth
  scope:      'mobile' | 'runtime'
  pairedAt:   number   // timestamp ms — khi tạo
  lastSeenAt: number   // 0 = chưa dùng; >0 = lần cuối connect
}
```

**Lưu trữ:**
```
/data/orca/orca-devices.json  (chmod 600)
[
  { "deviceId": "uuid-1", "name": "Web Browser", "token": "bbc860...", "scope": "runtime", ... },
  { "deviceId": "uuid-2", "name": "iPhone 15",   "token": "f097fe...", "scope": "mobile",  ... }
]
```

---

## 3. DeviceRegistry — API

### 3.1 Tạo device

```typescript
// Tạo device mới (sinh token mới)
addDevice(name: string, scope: DeviceScope = 'mobile'): DeviceEntry
// → randomBytes(24).toString('hex') = 48-char token

// Lấy pending device (chưa dùng = lastSeenAt===0), tạo mới nếu chưa có
// → tránh orphan tokens khi click "Regenerate QR" nhiều lần
getOrCreatePendingDevice(name: string, scope: DeviceScope): DeviceEntry

// Rotate: xoá pending token cũ, tạo mới
// → dùng khi muốn revoke token bị leak trước khi ai đó dùng
rotatePendingDevice(name: string, scope: DeviceScope): DeviceEntry
```

### 3.2 Validate & Update

```typescript
validateToken(token: string): DeviceEntry | null
// → tìm device có token khớp → used in E2EE auth step

updateLastSeen(deviceId: string): void
// → gọi sau khi E2EE auth thành công → lastSeenAt = Date.now()
```

### 3.3 Xoá device (Revocation)

```typescript
removeDevice(deviceId: string): boolean
// → xoá khỏi array → save() → device không còn auth được nữa
// → runtime: OrcaRuntimeRpcServer.wsTransport.terminateClientConnections(device.token)
```

Khi xoá device:
1. Xoá khỏi `orca-devices.json`
2. Gọi `wsTransport.terminateClientConnections(token)` → đóng WS connections đang active dùng token đó

---

## 4. ScopedPairingToken — RBAC-limited tokens

Đây là extension cho multi-user access với RBAC, hiện tại chưa được triển khai ở UI nhưng types và logic đã có:

```typescript
// src/shared/rbac-types.ts
export type ScopedPairingToken = {
  token:            string         // 64-hex (32 random bytes)
  userId:           string
  userEmail:        string
  userName:         string
  teams:            string[]
  allowedServerIds: string[] | '*' // fleet server constraints
  allowedProjects:  string[]       // project constraints
  agentTrust:       'minimal' | 'standard' | 'full'
  issuedAt:         number
  expiresAt:        number         // issuedAt + 24h
}
```

**TTL**: 24 giờ, in-memory (không persist qua restart).

```typescript
// DeviceRegistry methods
generateScopedToken(user, allowedServerIds, allowedProjects, agentTrust): ScopedPairingToken
getScopedToken(token): ScopedPairingToken | null  // null nếu expired
revokeScopedToken(token): void
revokeAllUserTokens(userId): void                 // revoke khi user bị deactivate
pruneExpiredTokens(): void                        // cleanup định kỳ
```

---

## 5. OrcaUser — User model (kế hoạch)

Types đã định nghĩa trong `src/shared/rbac-types.ts` nhưng chưa có implementation:

```typescript
export type OrcaIdentityProvider = 'github' | 'google' | 'keycloak' | 'none'

export type OrcaUser = {
  id:             string
  email:          string
  name:           string
  avatarUrl?:     string
  teams:          string[]
  projects:       string[]
  role:           'developer' | 'lead' | 'admin'
  provider:       OrcaIdentityProvider
  providerUserId: string
}

export type OrcaAccessPolicy = {
  id:   string
  name: string
  teams?:           string[]
  roles?:           OrcaUser['role'][]
  users?:           string[]                   // email list
  allowedServers:   '*' | string[]
  allowedProjects?: '*' | string[]
  agentTrust?:      'minimal' | 'standard' | 'full'
  canCreateWorktrees?:  boolean
  canDeleteWorktrees?:  boolean
  canAccessProduction?: boolean
}

export type OrcaSsoConfig = {
  provider:      OrcaIdentityProvider
  clientId:      string
  discoveryUrl?: string      // OIDC discovery endpoint
  allowedOrg?:   string      // GitHub org restriction
  allowedDomain?: string     // Google domain restriction
  redirectUri?:  string
}
```

---

## 6. RPC Methods — Device Management

Các methods exposed qua RPC (sau khi authenticated):

```typescript
// Lấy danh sách devices của user
'mobile.listDevices'    → DeviceEntry[]

// Tạo QR code / pairing URL cho mobile
'mobile.getPairingQR'   → { pairingUrl, qrDataUrl, deviceId }
'mobile.rotatePairingQR' → { pairingUrl, qrDataUrl, deviceId }  // rotate token

// Revoke device
'mobile.removeDevice'   → { success: boolean }
```

```typescript
// src/main/ipc/mobile.ts — handler cho getPairingQR
const offer = runtimeRpc.createPairingOffer({
  name: 'Mobile',
  scope: 'mobile',
  rotate: args.rotate
})
if (!offer.available) return { error: 'pairing not available' }

const qrDataUrl = await QRCode.toDataURL(offer.pairingUrl)
return {
  pairingUrl:   offer.pairingUrl,
  qrDataUrl,
  webClientUrl: offer.webClientUrl,
  deviceId:     offer.deviceId
}
```

---

## 7. Lifecycle: Device từ tạo đến revoke

```
[1. Tạo]
  Admin / System gọi createPairingOffer()
    → DeviceRegistry.getOrCreatePendingDevice("Web Browser", "runtime")
    → DeviceEntry { lastSeenAt: 0 } ← pending
    → encodePairingOffer() → PairCode

[2. Pairing]
  Client dùng PairCode → WS connect → E2EE handshake
    → e2ee_auth { deviceToken }
    → validateToken(token) → DeviceEntry found
    → DeviceRegistry.updateLastSeen(deviceId) ← lastSeenAt = now
    → state: ACTIVE

[3. Active]
  Client kết nối / ngắt kết nối nhiều lần
    → mỗi lần reconnect: E2EE auth lại với cùng deviceToken
    → updateLastSeen() mỗi lần auth thành công

[4. Revoke]
  Admin gọi removeDevice(deviceId) hoặc mobile.removeDevice RPC
    → xoá khỏi orca-devices.json
    → wsTransport.terminateClientConnections(token)
    → WS connections đang dùng token này bị đóng (code 4401)
    → Client không thể auth lại với token này
```

---

## 8. File permissions & Security

| File | Permission | Nội dung |
|------|-----------|---------|
| `orca-devices.json` | 600 (rw-------) | Array of DeviceEntry với tokens |
| `orca-e2ee-keypair.json` | 600 | Server Curve25519 keypair |
| `orca-runtime.json` | 600 | Socket path + authToken |

`writeSecureJsonFile()` trong `src/shared/secure-file.ts` tự động set chmod 600.

---

## 9. Hiện trạng & Roadmap

### Hiện tại (implemented)

- ✅ DeviceEntry với token 48-hex
- ✅ `getOrCreatePendingDevice` / `rotatePendingDevice`
- ✅ `validateToken`, `updateLastSeen`, `removeDevice`
- ✅ `ScopedPairingToken` (in-memory, 24h TTL) với đầy đủ RBAC fields
- ✅ `OrcaUser`, `OrcaAccessPolicy`, `OrcaSsoConfig` types (chưa implement)
- ✅ `revokeAllUserTokens(userId)` — chuẩn bị cho multi-user

### Chưa có

- ❌ User accounts (local login, SSO)
- ❌ Persistent sessions (cookie-based)
- ❌ Admin UI để quản lý devices/users
- ❌ Policy enforcement (allowedServers, allowedProjects)
- ❌ Audit log cho device actions

Xem [CR-LOGIN-001](../crs/v1/login/CR-LOGIN-001-auth.md) và [CR-LOGIN-004](../crs/v1/login/CR-LOGIN-004-admin.md) cho kế hoạch implementation.
