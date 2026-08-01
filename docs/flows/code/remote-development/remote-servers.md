# Luồng: Remote Servers — Cơ chế hoạt động đầy đủ

**Ngày ghi:** 2026-07-25  
**Trạng thái:** VERIFIED — đã xác nhận với code thực tế  
**Server:** `172.20.2.39` · **Public:** `https://b15.openledger.vn`

---

## Tổng quan

"Remote Servers" (Settings → pane `servers`) là tính năng cho phép **Orca Desktop (Electron)** kết nối với các **Orca Server instances** khác qua WebSocket. Mỗi server instance trở thành một "Runtime Environment" mà user có thể switch giữa các projects chạy trên các machines khác nhau.

> **⚠️ QUAN TRỌNG:** Đây là tính năng **Electron-only**. Trong **web browser mode** (`b15.openledger.vn`), pane `servers` và button "Open Remote Servers" **không có tác dụng** hoặc bị ẩn hoàn toàn.

---

## Kiến trúc tổng thể

```
┌──────────────────────────────────────────────────────────────────────┐
│ Electron Desktop App (User's machine)                                │
│  Settings → servers pane                                             │
│  RuntimeEnvironmentsPane.tsx                                         │
│    → window.api.runtimeEnvironments.*  (Electron IPC)               │
│    → AppStore.runtimeEnvironments slice                              │
└──────────────────────┬───────────────────────────────────────────────┘
                       │ WebSocket JSON-RPC (ws://host:6768)
                       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ Orca Server Instance (Remote machine / Docker container)             │
│  OrcaRuntimeRpcServer (port 6768)                                    │
│  → Auth: device token (từ pairing QR code)                          │
│  → PREFLIGHT_METHODS, LINEAR_METHODS, JIRA_METHODS, ...              │
└──────────────────────────────────────────────────────────────────────┘
```

---

## LUỒNG 1: Thêm Remote Server (Pairing flow)

### 1.1 — Electron: Pairing qua QR code hoặc URL

```
Settings → servers pane → "Add server"
  → AddRemoteHostDialog.tsx
       mode='server' (không phải SSH host)
    → RemoteServerFields.tsx
       user nhập: server URL (ws://host:6768)
       hoặc: scan QR code (web pairing offer)
    → saveRemoteServer()
       → window.api.runtimeEnvironments.add({
             name: 'My Server',
             offer: { endpoint, deviceToken, publicKeyB64 }
           })
         → Electron IPC: 'runtimeEnvironments:add'
         → Main process: lưu environment vào config
         → Return: { environment: PublicKnownRuntimeEnvironment }
    → AppStore: environments list updated
```

**File:** `src/renderer/src/components/sidebar/AddRemoteHostDialog.tsx:173`

### 1.2 — Web mode: Auto-pair qua URL

```
User mở https://b15.openledger.vn/?_orca_offer=<base64>
  → main-web-bootstrap.tsx: decideWebPairingStartup()
       → readPairingInputFromLocation(window.location)
       → decision: 'auto-save-runtime-offer' | 'show-connect'
  → createStoredWebRuntimeEnvironment({ name: 'Orca Server', offer })
  → saveStoredWebRuntimeEnvironment() → localStorage
  → activeEnvironment set → App render với paired env
```

---

## LUỒNG 2: Kết nối tới Remote Server (Runtime connection)

### 2.1 — Chọn environment làm active

```
RuntimeEnvironmentsPane.tsx
  user click "Use this server"
  → updateSettings({ activeRuntimeEnvironmentId: environmentId })
    → AppStore.settings.activeRuntimeEnvironmentId = environmentId
    → getActiveRuntimeTarget(settings) giờ trả { kind: 'environment', environmentId }
    → Tất cả callRuntimeRpc() sẽ route về server này
```

### 2.2 — WebSocket connection lifecycle

```
callRuntimeRpc(target, method, params) [runtime-rpc-client.ts:68]
  ↓
  target.kind === 'environment'
    → window.api.runtimeEnvironments.call({
          selector: { environmentId },
          method, params
        })
      → Electron IPC → Main process
      → OrcaRpcClient.connect(ws://host:6768)
        → Auth handshake: device token trong header
        → Send: { jsonrpc: '2.0', method, params, id }
        ← Receive: { result } hoặc { error }
      ← return result
```

**Hoặc trong web mode:**
```
window.api.runtime.call({ method, params })
  → web-preload-api.ts
  → callRuntimeResult(method, args)
    → WebSocket client (active WebSocket connection)
    → ws://172.20.2.39:6768
    ← JSON-RPC response
```

### 2.3 — Compatibility check

```
callRuntimeRpc() → trước khi gọi method thật:
  → runtimeCompatibilityChecks cache (max 32 entries)
  → nếu chưa có hoặc expired (>60s):
      callRuntimeRpc(target, 'status.get', {})
        → server trả { protocolVersion, orcaVersion }
        → assertRuntimeStatusCompatible(clientVersion, serverVersion)
          → compatible? → proceed
          → incompatible? → RuntimeRpcCallError('incompatible')
```

**File:** `src/renderer/src/runtime/runtime-rpc-client.ts:78`

---

## LUỒNG 3: Preflight trên Remote Server

```
refreshPreflightStatus()
  runtimeTarget = { kind: 'environment', environmentId: 'web-xxxxx' }
  → callRuntimeRpc(target, 'preflight.check', {})
    → WebSocket → server container
    → PREFLIGHT_METHODS handler:
        handler: async (params) => runPreflightCheck(params.force)
        → which git → /usr/bin/git ✅
        → which gh → /usr/bin/gh (v2.96.0) ✅
        → gh auth status → exit 1 (chưa auth)
      ← { git: {installed:true}, gh: {installed:true, authenticated:false}, ... }
  → preflightStatus set trong store
  → GitHubCard: 'not-authenticated' ✅ (không phải 'unavailable')
```

---

## LUỒNG 4: "Open Remote Servers" button

**File:** `src/renderer/src/components/settings/ProviderHostScopeControl.tsx`

### Electron mode
```typescript
const openHostsSettings = (): void => {
  openSettingsPage()                                   // activeView = 'settings'
  openSettingsTarget({
    pane: 'servers',        // → RuntimeEnvironmentsPane
    repoId: null,
    sectionId: 'default-runtime'
  })
}
// → Settings pane 'servers' mở → user thấy danh sách environments
// → Có thể Add/Remove/Switch server
```

### Web mode (fix 2026-07-25)
```typescript
// ProviderHostScopeControl.tsx — detect web mode:
const [isWebServerMode, setIsWebServerMode] = useState(false)

useEffect(() => {
  window.api.cli.getInstallStatus()
    .then(status => {
      if (status.unsupportedReason === 'launch_mode_unavailable')
        setIsWebServerMode(true)   // → ẩn button
    })
}, [])

// Render:
{!isWebServerMode ? <Button onClick={openHostsSettings}>Open Remote Servers</Button> : null}
// → Trong web mode: button bị ẩn (pane 'servers' không render được)
```

**Lý do ẩn:** Trong web mode, `openSettingsPage()` + `openSettingsTarget({ pane: 'servers' })` vẫn thực thi nhưng không render được `RuntimeEnvironmentsPane` (chỉ hoạt động khi có Electron IPC bridge đầy đủ).

---

## LUỒNG 5: Disconnect / Remove environment

```
RuntimeEnvironmentsPane.tsx
  → window.api.runtimeEnvironments.remove({ selector: { environmentId } })
    → Electron IPC → Main process
    → xóa environment khỏi config
    → nếu là activeEnvironmentId → settings.activeRuntimeEnvironmentId = null
    → getActiveRuntimeTarget() giờ trả { kind: 'local' }
```

**Web mode:**
```
window.api.runtimeEnvironments.remove()
  → web-preload-api.ts:
    clearStoredWebRuntimeEnvironment()    // localStorage.removeItem(...)
    disconnectActiveRuntimeEnvironment()  // đóng WebSocket
    activeEnvironment = null
```

---

## Runtime Environments — Data structure

```typescript
// StoredWebRuntimeEnvironment (localStorage):
{
  id: 'web-<uuid>',
  name: 'Orca Server',
  endpoints: [{
    id: string,
    kind: 'websocket',
    label: 'WebSocket',
    endpoint: 'ws://172.20.2.39:6768',
    deviceToken: '<token>',    // auth token
    publicKeyB64: '<pubkey>'   // server public key
  }]
}

// PublicKnownRuntimeEnvironment (Electron store):
{
  id: string,
  name: string,
  endpoints: [...],
  activeRuntimeId?: string,   // populated sau khi status.get
  orcaVersion?: string,
  protocolVersion?: string
}
```

---

## Connection status trong UI

```typescript
// RuntimeEnvironmentsPane: hiển thị connection state
getEnvironmentConnectionStatusLabel(env, runtimeCompatibilityStatus):
  → 'Checking…'        (đang probe status.get)
  → 'Compatible'       (versions match)
  → 'Update client'    (server mới hơn client)
  → 'Update server'    (client mới hơn server)
  → 'Disconnected'     (WS error / timeout)
```

---

## Môi trường deploy

```
[Browser/Desktop]
    ↓ HTTPS/WSS
[Gateway: 103.67.184.32]
  Nginx: 443 → ws://172.20.2.39:6768 (WS proxy)
              → http://172.20.2.39:6769 (HTTP)
    ↓
[Orca Server: 172.20.2.39]
  docker container: orca-server
    port 6768: WebSocket RPC (OrcaRuntimeRpcServer)
    port 6769: HTTP (web SPA + /health + /auth)
```

**WSS proxy config** (`deploy/dev/nginx/conf.d/default.conf`):
```nginx
location /rpc {
  proxy_pass         http://172.20.2.39:6768;
  proxy_http_version 1.1;
  proxy_set_header   Upgrade $http_upgrade;
  proxy_set_header   Connection "upgrade";
}
```

---

## Web mode vs Electron mode — So sánh

| Tính năng | Electron | Web Browser |
|-----------|---------|-------------|
| Thêm Remote Server | ✅ `runtimeEnvironments.add()` IPC | ✅ qua URL pairing |
| Xem danh sách servers | ✅ RuntimeEnvironmentsPane | ❌ Không render |
| "Open Remote Servers" button | ✅ Navigate đến pane | ❌ Bị ẩn (fix 2026-07-25) |
| Switch active environment | ✅ `updateSettings()` | ✅ Tự động qua stored env |
| Preflight check trên server | ✅ qua WS RPC | ✅ qua WS RPC |
| Auth: device token | Stored trong config | Stored trong localStorage |

---

## File reference

| File | Vai trò |
|------|---------|
| `src/renderer/src/components/settings/RuntimeEnvironmentsPane.tsx` | UI pane — list environments, add/remove |
| `src/renderer/src/components/settings/ProviderHostScopeControl.tsx` | "Open Remote Servers" button — web mode aware |
| `src/renderer/src/components/sidebar/AddRemoteHostDialog.tsx` | Dialog thêm remote server |
| `src/renderer/src/components/sidebar/AddRemoteHostFields.tsx` | Form fields cho remote server |
| `src/renderer/src/runtime/runtime-rpc-client.ts` | `callRuntimeRpc` + compatibility check |
| `src/renderer/src/store/slices/runtime-environments.ts` | Store slice — environments list |
| `src/renderer/src/web/web-preload-api.ts` | Web: `createRuntimeEnvironmentsApi()` |
| `src/renderer/src/web/web-runtime-environment.ts` | `StoredWebRuntimeEnvironment` + localStorage |
| `src/renderer/src/web/web-pairing.ts` | `decideWebPairingStartup()` + URL parsing |
| `src/renderer/src/web/main-web-bootstrap.tsx` | Web startup — auto-pair từ URL |
