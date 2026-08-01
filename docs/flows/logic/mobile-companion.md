# Luồng Dữ liệu — Mobile Companion

**Domain:** Mobile Companion  
**Nghiệp vụ:** BL-MB-01 → BL-MB-04  
**Kiến trúc tham chiếu:** HLD v1 — Mobile Container (React Native), TweetNaCl E2E, APNs/FCM

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Mobile App (React Native) | UI | iOS/Android companion app |
| Orca Desktop / Web Server | Backend | WebSocket server, pairing manager |
| TweetNaCl (NaCl box) | Encryption | End-to-end encryption cho WebSocket |
| APNs / FCM | External | Push notification delivery |
| Main Process | Business Logic | PairingManager, MobileNotificationService |
| SQLite Database | Persistence | Paired devices, notification history |

---

## BL-MB-01 — Pair Mobile Device

```
Người dùng (Sam) — trên Desktop app
    │
    ▼
[Renderer Desktop] Settings → Mobile → "Pair New Device"
    │ contextBridge.invoke('mobile.generatePairingCode')
    ▼
[Main Process — PairingManager.generateCode()]
    ├─ Generate NaCl keypair (public/private key)
    ├─ Encode pairing payload: { desktopPubKey, wsEndpoint, pairingCode }
    ├─ Encode as QR code → base64 PNG
    └─ Hiển thị QR code trong Renderer, timeout 5 phút

[Mobile App] Sam mở Orca mobile, scan QR code
    │ POST https://orca-desktop:6768/api/mobile/pair
    │ Body: { mobilePubKey, pairingCode, deviceInfo }
    ▼
[Main Process — PairingManager.completePairing()]
    ├─ Verify pairingCode (valid + not expired)
    ├─ Derive shared secret: NaCl box(desktopPrivKey, mobilePubKey)
    ├─ INSERT mobile_devices { id, deviceInfo, sharedSecret, pairedAt }  ← SQLite
    └─ emit: mobile:paired { deviceId }
    │
    ▼
[Mobile App] nhận pairing response → lưu desktopPubKey + sharedSecret
    └─ WebSocket connect: ws://orca-desktop:6768/mobile
       Headers: { deviceId, signature: NaCl-sign }

Luồng:
Desktop User → Renderer → IPC → Main (generate keypair + QR)
Mobile User → scan QR → POST /api/mobile/pair → Main (verify + save)
                                               → SQLite (INSERT device)
Mobile → WebSocket connect (authenticated)
```

---

## BL-MB-02 — Gửi Push Notification

```
[Main Process] event xảy ra: agent:completed, agent:error, agent:rateLimited
    │
    ▼
[MobileNotificationService.send()]
    ├─ SELECT devices FROM mobile_devices WHERE userId=?   ← SQLite
    ├─ Build notification payload:
    │   { title: "Agent Done", body: "Task completed in wt-abc", data: { worktreeId } }
    ├─ Encrypt payload: NaCl box(sharedSecret, payload)
    ├─ APNs (iOS):
    │   POST https://api.push.apple.com/3/device/<deviceToken>
    │   Headers: Authorization: Bearer <APNs JWT>
    │   Body: { aps: { alert: { title, body }, badge: 1 }, encryptedData }
    └─ FCM (Android):
        POST https://fcm.googleapis.com/fcm/send
        Headers: Authorization: key=<FCM_SERVER_KEY>
        Body: { to: <deviceToken>, notification: { title, body }, data: { encryptedData } }
    │
    ▼
[APNs/FCM] deliver to device
    │
    ▼
[Mobile App] nhận push notification → decrypt → display notification

Luồng:
Event (agent/system) → Main → SQLite (load devices)
                            → NaCl encrypt
                            → APNs/FCM API → Mobile device
                            → INSERT notifications (log)  ← SQLite
```

---

## BL-MB-03 — Remote Dispatch từ Mobile

```
Người dùng (Sam) trên Mobile App
    │
    ▼
[Mobile App] chọn worktree → nhập prompt → tap "Dispatch"
    │ WebSocket message (encrypted):
    │ NaCl box(sharedSecret, { type: 'dispatch', worktreeId, prompt })
    ▼
[Main Process — MobileDispatchHandler]
    ├─ Verify deviceId + decrypt NaCl box
    ├─ Validate worktreeId tồn tại + user có quyền
    ├─ AgentManager.injectPrompt(sessionId, prompt)  ← Daemon Unix Socket
    │   → write to PTY stdin
    └─ Gửi ack: NaCl box({ type: 'ack', status: 'dispatched' })
    │
    ▼
[Agent Process] nhận prompt, bắt đầu xử lý
    │
    ▼
[Mobile App] nhận ack → hiển thị "Dispatched" status

Luồng:
Mobile User → WebSocket (NaCl encrypted) → Main → decrypt + validate
                                                 → Daemon → PTY stdin → Agent
                                                 → WebSocket ack → Mobile
```

---

## BL-MB-04 — Xem Agent Status từ Mobile

```
Người dùng (Sam/Carlos) trên Mobile App
    │
    ▼
[Mobile App] mở app → request status
    │ WebSocket: NaCl box({ type: 'status.request' })
    ▼
[Main Process — MobileStatusHandler]
    ├─ SELECT worktrees, sessions FROM orca.db   ← SQLite
    ├─ Get live agent states từ AgentManager
    ├─ Build status response:
    │   { worktrees: [{ id, branch, agent: { status, task } }] }
    └─ Encrypt + send: NaCl box(sharedSecret, statusData)
    │
    ▼
[Mobile App] nhận + decrypt → render agent status cards

REAL-TIME UPDATES:
[Main Process] agent:statusChanged event
    → MobileNotificationService.pushStatusUpdate(deviceId, statusData)
    → WebSocket push (nếu app connected) hoặc Push Notification (nếu background)

Luồng:
Mobile → WebSocket (query) → Main → SQLite + AgentManager
                           → WebSocket response (NaCl encrypted)

Background:
Agent event → Main → WebSocket push OR APNs/FCM (if app background)
```

---

## Sơ đồ tổng quan — Mobile Companion

```
┌──────────────────┐  WebSocket (TweetNaCl E2E)   ┌─────────────────────┐
│  Mobile App      │◄────────────────────────────►│  Main Process        │
│  React Native    │                               │  PairingManager      │
│  iOS / Android   │                               │  MobileDispatch      │
└──────┬───────────┘                               │  MobileNotification  │
       │                                           └───────┬─────────────┘
       │ Push                                              │
       ▼                                          ┌───────▼─────────┐
┌──────────────┐                                  │  SQLite         │
│  APNs / FCM  │◄─────────────────────────────────│  mobile_devices │
└──────────────┘                                  │  notifications  │
                                                  └───────┬─────────┘
                                                          │ Unix Socket
                                                  ┌───────▼─────────┐
                                                  │  Daemon / PTY   │
                                                  │  Agent process  │
                                                  └─────────────────┘

Pairing Flow:
Desktop QR display ──── Mobile scan QR ──── POST /api/mobile/pair
                                           ← WebSocket authenticated
```
