# CR-TRACE-007 — Mobile Companion Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-007 |
| **Tên** | Mobile Companion — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/mobile-companion.md`, `src/main/ipc/mobile.ts`, `src/main/runtime/runtime-rpc.ts`, `src/main/runtime/device-registry.ts`, `src/main/runtime/e2ee-keypair.ts`, `src/main/runtime/rpc/e2ee-channel.ts`, `src/shared/e2ee-crypto.ts`, `src/main/mobile/MobileCompanionService.ts`, `src/main/runtime/rpc/methods/terminal.ts`, `src/main/runtime/rpc/methods/status.ts`, `src/main/runtime/rpc/dispatcher.ts`

---

## 1. Vấn đề

**Lưu ý quan trọng khi đọc CR này:** `docs/flows/logic/mobile-companion.md` mô tả kiến trúc ở mức HLD với các tên thành phần (`PairingManager`, `MobileDispatchHandler`, `MobileStatusHandler`, APNs/FCM) — nhưng grep trực tiếp source cho thấy **các class này không tồn tại** trong codebase hiện tại. Kiến trúc thật sự đã hội tụ về một RPC layer dùng chung với Desktop/CLI/Dev Server:

- Pairing được implement trực tiếp trong `OrcaRuntimeRpcServer` (`createPairingOffer()`, `src/main/runtime/runtime-rpc.ts:548`) + `DeviceRegistry` (`src/main/runtime/device-registry.ts`), không phải một `PairingManager` riêng.
- "Remote Dispatch" và "Xem Agent Status" đi qua **cùng một `RpcDispatcher`/`ALL_RPC_METHODS` registry** mà `devServer.browseDir`, `worktree.*`, `terminal.*` dùng — không có `MobileDispatchHandler`/`MobileStatusHandler` riêng biệt. Method thật là `terminal.send` (BL-MB-03) và `status.get` (BL-MB-04), được lọc qua `MOBILE_RPC_METHOD_ALLOWLIST` (`runtime-rpc.ts:155`).
- Push notification (BL-MB-02) dùng **Web Push chuẩn RFC 8030** (`MobileCompanionService`, `src/main/mobile/MobileCompanionService.ts`) — không phải APNs/FCM như flow doc mô tả.

Vì vậy, khi một thao tác mobile chậm hoặc fail hôm nay, không có cách nào tách bạch được lỗi nằm ở: (a) E2EE handshake/pairing (`E2EEChannel`, `src/main/runtime/rpc/e2ee-channel.ts`), (b) generic RPC dispatch dùng chung với các client khác (`RpcDispatcher.dispatchStreaming`), hay (c) chính handler nghiệp vụ (`terminal.send`, `status.get`). Vì tracer hiện tại (`devServer:*`) chỉ instrument nhánh Dev Server relay, một request từ mobile đi qua đúng dispatcher đó lại **hoàn toàn vô hình** trong TracePanel/log — không phân biệt được request đến từ Browser hay từ Mobile khi debug.

## 2. Thành phần & Transport liên quan

| Thành phần (theo flow doc) | Thành phần thật (đã xác nhận qua grep) | Layer | Transport | Quy ước lan truyền (CR-TRACE-000 §3.3) |
|---|---|---|---|---|
| Mobile App (React Native) | (không đổi) | UI | — | Sinh `traceId` đầu tiên |
| Orca Desktop/Web Server, PairingManager | `OrcaRuntimeRpcServer.createPairingOffer()` (`runtime-rpc.ts:548`), `DeviceRegistry` (`device-registry.ts`) | Backend | IPC (`mobile:getPairingQR`, `src/main/ipc/mobile.ts:79`) rồi WS handshake | Pairing tự nó là IPC nội bộ + QR, không băng qua network; `traceId` chỉ bắt đầu có ý nghĩa **từ khi WS connect** |
| TweetNaCl (NaCl box) | `E2EEChannel` (`src/main/runtime/rpc/e2ee-channel.ts`), `deriveSharedKey/encrypt/decrypt` (`src/shared/e2ee-crypto.ts` hoặc `src/main/runtime/e2ee-keypair.ts`) | Encryption | WebSocket + TweetNaCl box | Hàng "WebSocket + TweetNaCl box (Mobile ↔ Main)" — `traceId` nằm **trong** JSON plaintext trước khi `encrypt()` |
| Main Process — MobileDispatchHandler/MobileStatusHandler | `RpcDispatcher.dispatchStreaming()` (`src/main/runtime/rpc/dispatcher.ts`) dùng chung registry `ALL_RPC_METHODS`; method cụ thể: `terminal.send` (`rpc/methods/terminal.ts:1085`), `status.get` (`rpc/methods/status.ts:5`) | Business Logic | WS RPC (plaintext JSON sau decrypt) | Hàng "WebSocket RPC (Browser ↔ Orca Server)" — vì đây thực chất là cùng RpcRequest `{id, method, params, deviceToken}` |
| APNs/FCM | `MobileCompanionService` (Web Push RFC 8030) (`src/main/mobile/MobileCompanionService.ts`) | External | HTTPS (`webpush.sendNotification`) | Không có transport nào trong bảng §3.3 khớp chính xác — xem mục 5 |
| SQLite Database | `DeviceRegistry` lưu device token/scope; `MobileCompanionService` lưu `PushSubscription` qua store nội bộ | Persistence | — | N/A (in-process) |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  mobilePairFlow:        createTracer('mobile:pair'),        // BL-MB-01
  mobilePushFlow:        createTracer('mobile:push'),        // BL-MB-02
  mobileDispatchFlow:    createTracer('mobile:dispatch'),    // BL-MB-03
  mobileStatusQueryFlow: createTracer('mobile:statusQuery'), // BL-MB-04
}
```

## 4. Instrumentation theo từng sub-flow

### BL-MB-01 — Pair Mobile Device

| Bước | span event | fields | File:function |
|---|---|---|---|
| Renderer gọi tạo QR | `start` | `scope`, `rotate` | `src/main/ipc/mobile.ts` (`mobile:getPairingQR` handler) |
| Tạo/rotate pending device | `step('pairingOffer')` | `deviceId`, `scope` | `src/main/runtime/runtime-rpc.ts:548` (`createPairingOffer`) |
| Mobile connect WS + E2EE handshake | `step('e2eeHandshake')` | `state` (`awaiting_hello`→`ready`) | `src/main/runtime/rpc/e2ee-channel.ts` (`handleHello`/`handleAuth`) |
| Token verify + mark last-seen | `ok` / `fail` | `deviceId`, `lastSeenAt` | `src/main/runtime/device-registry.ts` (`validateToken`, `updateLastSeen`), gọi từ `runtime-rpc.ts` (`onReady` callback) |

```typescript
// src/main/ipc/mobile.ts — trong handler 'mobile:getPairingQR'
const span = Tracers.mobilePairFlow.start({ scope: 'mobile', rotate: !!args?.rotate })
const offer = rpcServer.createPairingOffer({ address: ip, rotate: args?.rotate, name, scope: 'mobile' })
if (!offer.available) {
  span.fail('pairing offer unavailable', { reason: 'ws_or_keypair_missing' })
  return { available: false as const }
}
span.step('pairingOffer', { deviceId: offer.deviceId })
// id lan truyền tiếp cho E2EEChannel qua resume khi WS handshake thật sự diễn ra (mục 5)
span.ok({ deviceId: offer.deviceId })
```

*Ghi chú:* việc "tiếp nối" span này sang lúc mobile thực sự connect WS (handshake xảy ra ở process khác/thời điểm khác, có thể vài phút sau khi quét QR) không tự nhiên phù hợp với mô hình `resume.id` đồng bộ của CR-TRACE-000 §3.1 — QR code chỉ mang `deviceToken`, không mang `traceId`. Khuyến nghị: **không** cố ép hai span nối liền nhau; coi `mobile:pair` là span cục bộ cho việc *tạo offer*, và handshake E2EE khi mobile connect thật sự là span riêng (xem bảng transport ở mục 2, dòng E2EEChannel) — ghi rõ trong code comment để tránh nhầm lẫn khi review.

### BL-MB-02 — Gửi Push Notification

| Bước | span event | fields | File:function |
|---|---|---|---|
| Bắt đầu gửi notify cho user | `start` | `userId`, `title` | `src/main/mobile/MobileCompanionService.ts` (`notify()`) |
| Load subscriptions | (gộp vào `ok`/`fail`, single in-process store read — không đáng `step()` theo CR-TRACE-000 §5) | `subscriptionCount` | `listDevices()` |
| Gửi Web Push tới từng subscription | `step('webpush')` mỗi lần gọi `webpush.sendNotification` (cross-boundary HTTPS call, failable độc lập) | `endpoint` (rút gọn/hash), `statusCode` khi lỗi | `notify()` |
| Kết quả | `ok` (tất cả gửi xong) / `fail` (nếu toàn bộ subscriptions lỗi) | `sent`, `expired` (410) | `notify()` |

```typescript
// src/main/mobile/MobileCompanionService.ts — trong notify()
async notify(userId: string, payload: NotificationPayload): Promise<void> {
  const span = Tracers.mobilePushFlow.start({ userId, title: payload.title })
  const subscriptions = await this.listDevices(userId)
  let sent = 0, expired = 0
  await Promise.all(subscriptions.map(async (sub) => {
    span.step('webpush', { endpoint: hashEndpoint(sub.endpoint) })
    try {
      await webpush.sendNotification(sub, JSON.stringify(payload))
      sent++
    } catch (err) {
      const httpErr = err as { statusCode?: number }
      if (httpErr?.statusCode === 410) expired++
    }
  }))
  span.ok({ sent, expired, total: subscriptions.length })
}
```

*Ghi chú:* flow doc mô tả "APNs/FCM" — không có transport row nào trong CR-TRACE-000 §3.3 khớp Web Push (HTTPS, không phải request/response RPC nội bộ Orca). `mobile:push` không cần nhận `resume.id` từ layer trước vì nó luôn được kích hoạt từ trong process Main (event `agent:completed` v.v., không băng qua wire) — span luôn tạo `id` mới.

### BL-MB-03 — Remote Dispatch từ Mobile (method thật: `terminal.send`)

| Bước | span event | fields | File:function |
|---|---|---|---|
| Nhận & giải mã message từ mobile | `start` (resume theo `traceId` nếu client đã gửi kèm) | `deviceId`/`clientId`, `method` | `src/main/runtime/runtime-rpc.ts:1041` (`handleWebSocketMessage`) |
| Check mobile allowlist | `step('allowlistCheck')` — điểm rẽ nhánh quan trọng (forbidden vs allowed) | `method`, `allowed` | `runtime-rpc.ts` (`MOBILE_RPC_METHOD_ALLOWLIST.has(...)`, dòng ~1086) |
| Dispatch vào registry chung | `step('dispatch')` | `method: 'terminal.send'` | `src/main/runtime/rpc/dispatcher.ts` (`dispatchStreaming`) |
| Ghi vào PTY stdin | `ok` / `fail` | `terminalId`/`sessionId` (nếu có trong params) | `src/main/runtime/rpc/methods/terminal.ts:1085` (`terminal.send` handler) |

```typescript
// src/main/runtime/runtime-rpc.ts — trong handleWebSocketMessage, sau khi decrypt
const span = Tracers.mobileDispatchFlow.start(
  { method: request.method, deviceId: device.deviceId },
  (request as { traceId?: string }).traceId ? { id: (request as { traceId?: string }).traceId! } : undefined
)
if (device.scope === 'mobile' && !MOBILE_RPC_METHOD_ALLOWLIST.has(request.method)) {
  span.fail('method not in mobile allowlist', { method: request.method })
  reply(JSON.stringify(this.buildError(request.id, 'forbidden', `Method '${request.method}' is not available to mobile clients`)))
  return
}
span.step('allowlistCheck', { method: request.method, allowed: true })
// ... await this.dispatcher.dispatchStreaming(request, replyForRequest, { connectionId, clientId: token, traceId: span.id, ... })
```

*Chỉ áp dụng span này cho method thuộc phạm vi "dispatch nghiệp vụ tới agent" (`terminal.send`), không wrap toàn bộ `handleWebSocketMessage` (dùng chung cho mọi RPC mobile, kể cả `browser.*`, `accounts.*` — các method đó nằm ngoài phạm vi CR này, không phải BL-MB-03).*

### BL-MB-04 — Xem Agent Status từ Mobile (method thật: `status.get`)

| Bước | span event | fields | File:function |
|---|---|---|---|
| Nhận request `status.get` | `start` | `deviceId`, `scope` | `runtime-rpc.ts:1041` |
| Dispatch + inject device scope vào response | `step('dispatch')` | `method: 'status.get'` | `runtime-rpc.ts:1126-1129` (`replyForRequest` đặc biệt cho `status.get`, gọi `injectDeviceScope`) |
| Lấy trạng thái runtime | `ok` | `worktreeCount` (nếu đọc được từ response) | `src/main/runtime/rpc/methods/status.ts` (`runtime.getStatus()`) |

```typescript
// src/main/runtime/runtime-rpc.ts — trong handleWebSocketMessage
const span = Tracers.mobileStatusQueryFlow.start({ deviceId: device.deviceId, scope: device.scope })
const replyForRequest =
  request.method === 'status.get'
    ? (response: string): void => {
        span.ok({})
        reply(injectDeviceScope(response, device.scope))
      }
    : reply
```

*Ghi chú:* flow doc mô tả thêm nhánh "REAL-TIME UPDATES" (server chủ động push status thay đổi qua WebSocket hoặc APNs/FCM khi app ở background). **Chưa xác định file cụ thể** cho cơ chế push chủ động này — `MOBILE_RPC_METHOD_ALLOWLIST` có `terminal.subscribe`/`status`-liên quan qua streaming (`RpcDispatcher.dispatchStreaming` hỗ trợ streaming method), nhưng không tìm thấy một hàm kiểu `pushStatusUpdate(deviceId, ...)` riêng khi grep — cần điều tra thêm khi triển khai để quyết định đây là tracer riêng (`mobile:statusPush`) hay chỉ là một `step()` bổ sung trong `mobile:statusQuery` khi request là long-poll/subscribe.

## 5. Lan truyền traceId qua transport của flow này

Flow này là ví dụ cụ thể nhất cho hàng "WebSocket + TweetNaCl box (Mobile ↔ Main)" trong CR-TRACE-000 §3.3, kết hợp với hàng "WebSocket RPC" vì **payload sau khi giải mã chính là một RpcRequest chuẩn** `{ id, method, params, deviceToken }`:

1. **Mobile → Main (`terminal.send`, `status.get`)**: Mobile tạo `traceId` bằng tracer riêng phía client, đặt **field `traceId` ngay trong JSON plaintext** (`{ id, method, params, deviceToken, traceId }`) — cùng cấp với `method`/`params`, **trước khi** gọi `encrypt(JSON.stringify(request), sharedKey)` (`src/shared/e2ee-crypto.ts`). Điều này đúng theo yêu cầu của assignment: đặt `traceId` bên trong payload trước khi mã hoá TweetNaCl, không đặt ngoài envelope ciphertext (`ws.send(ciphertext)` là opaque, không thể mang metadata ngoài).
2. **`E2EEChannel.handleRawMessage()`** (`src/main/runtime/rpc/e2ee-channel.ts:118`) gọi `decrypt(raw, sharedKey)` → trả về plaintext JSON — tại thời điểm này `traceId` đã lộ ra trong object đã parse, sẵn sàng cho `handleWebSocketMessage()` đọc.
3. **`handleWebSocketMessage()`** (`runtime-rpc.ts:1041`) parse `request = JSON.parse(rawMessage) as RpcRequest` — RPC method layer đọc `request.traceId` và gọi `Tracers.mobileDispatchFlow.start(fields, request.traceId ? { id: request.traceId } : undefined)` đúng convention §3.2.
4. Nếu `terminal.send` forward tiếp xuống Dev Server qua `DevServerRelayBridge.call()` (trường hợp agent chạy trên dev server từ xa), đính kèm `traceId: span.id` vào params của `relay.call()` — resume tiếp ở `relay:agentCall` theo đúng CR-TRACE-000 §3.3 hàng `relay.call()`.
5. **Response path**: response cũng đi qua `encryptedReply()` (`e2ee-channel.ts:136`) — không cần mang `traceId` trong response vì span đã `ok()`/`fail()` phía server trước khi encrypt reply.

## Acceptance Criteria

- [ ] `Tracers.mobilePairFlow`, `mobilePushFlow`, `mobileDispatchFlow`, `mobileStatusQueryFlow` được thêm vào `tracers.ts` theo đúng tên `mobile:pair`/`mobile:push`/`mobile:dispatch`/`mobile:statusQuery`
- [ ] `mobile:pair` bao phủ `createPairingOffer()` (`runtime-rpc.ts:548`) và IPC handler `mobile:getPairingQR` (`ipc/mobile.ts`)
- [ ] `mobile:push` bao phủ `MobileCompanionService.notify()`, có `step('webpush')` cho mỗi lần gọi `webpush.sendNotification`
- [ ] `mobile:dispatch` chỉ wrap method `terminal.send` khi `device.scope === 'mobile'`, không wrap toàn bộ `handleWebSocketMessage`
- [ ] `mobile:statusQuery` bao phủ nhánh `status.get` đặc biệt (`injectDeviceScope`) trong `handleWebSocketMessage`
- [ ] `traceId` được đọc/ghi đúng vị trí: **trong** JSON plaintext trước khi `encrypt()`, không đặt ngoài ciphertext envelope
- [ ] Khi `terminal.send` forward xuống Dev Server qua `relay.call()`, `traceId` (== `span.id` của `mobile:dispatch`) được truyền tiếp làm field `traceId` trong params
- [ ] `span.fail()` được gọi khi method không nằm trong `MOBILE_RPC_METHOD_ALLOWLIST` (forbidden case)
- [ ] Cơ chế real-time status push (nếu tồn tại) được điều tra và quyết định rõ: tracer riêng hay step bổ sung — không để lại TODO mơ hồ trong code
