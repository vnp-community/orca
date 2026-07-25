# TASK-041: Viết Unit Tests — Phase 3 (Windows, Web Push, Checklist)

**Phase:** 3 — Verification  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) Tests section  
**Depends on:** TASK-029, TASK-032, TASK-034, TASK-039, TASK-040  
**Blocks:** (không — verification task cuối cùng)

---

## Mục tiêu

Viết unit tests đầy đủ cho Phase 3: Windows capabilities, WebPushManager, Push API Routes, Checklist IPC, và Feature Wall logic.

---

## Files cần tạo

1. `src/main/ipc/__tests__/onboarding-windows.test.ts`
2. `src/main/notifications/__tests__/web-push-manager.test.ts`
3. `src/server/__tests__/push-api-routes.test.ts`
4. `src/main/ipc/__tests__/onboarding-checklist.test.ts`
5. `src/shared/__tests__/feature-wall-setup-steps.test.ts`

---

## Test cases cần implement

### `onboarding-windows.test.ts`

```typescript
describe('onboarding.detectWindowsCapabilities', () => {
  it('dev server không phải Windows → throw Error với platform name')
  it('dev server không tồn tại → throw "not found"')
  it('dev server Windows, relay connected → forward đến relay')
  it('cache hit (<60s) → không gọi relay')
  it('cache miss → gọi relay, lưu cache')
  it('pwshVersion được include trong response')
  it('gitBashPath được include khi available')
})
```

### `web-push-manager.test.ts`

```typescript
describe('WebPushManager', () => {
  it('loadOrCreateVapidKeys() tạo keys mới nếu store rỗng')
  it('loadOrCreateVapidKeys() persist keys vào store')
  it('loadOrCreateVapidKeys() reuse keys đã có (không tạo lại)')
  it('saveSubscription() tạo record với id mới')
  it('saveSubscription() deduplicate theo endpoint (upsert)')
  it('removeSubscription() xóa đúng subscription')
  it('sendToAll() gửi đến tất cả subscriptions')
  it('sendToAll() tự xóa subscription bị 410 Gone')
  it('sendToAll() tiếp tục gửi các subscriptions khác khi 1 lỗi')
  it('getPublicKey() trả về VAPID public key')
})
```

### `push-api-routes.test.ts`

```typescript
describe('Push API Routes', () => {
  it('GET /api/vapid-public-key → 200 { publicKey: string }')
  it('POST /api/push-subscribe body hợp lệ → 201 { id: string }')
  it('POST /api/push-subscribe deduplicate endpoint')
  it('POST /api/push-subscribe body không hợp lệ → 400')
  it('POST /api/push-unsubscribe → 204')
  it('Unknown route → không gửi response (pass-through)')
})
```

### `onboarding-checklist.test.ts`

```typescript
describe('onboarding.markChecklistItem', () => {
  it('global item: choseAgent = true → set đúng trong state')
  it('global item không cần devServerId')
  it('per-server item: addedRepo với devServerId → lưu vào perServer[dsId]')
  it('value: false → set false (unmark)')
  it('value mặc định là true')
})

describe('migrateOnboardingChecklist', () => {
  it('flat checklist v1 → migrate sang perServer["local"]')
  it('checklist đã có perServer → không migrate lại (idempotent)')
  it('global items choseAgent, triedCmdJ giữ nguyên sau migrate')
  it('empty per-server items → perServer: {}')
})
```

### `feature-wall-setup-steps.test.ts`

```typescript
describe('isConnectDevServerComplete', () => {
  it('no servers → false')
  it('servers tất cả disconnected → false')
  it('1 server connected → true')
})

describe('isAddDevServerRepoComplete', () => {
  it('activeDevServerId null → false')
  it('không có repo với đúng devServerId → false')
  it('có repo với đúng devServerId → true')
})

describe('getFirstIncompleteFeatureWallSetupStepId', () => {
  it('không có server → "connect-dev-server" ưu tiên tuyệt đối')
  it('có server, chưa add repo → "add-dev-server-repo"')
  it('tất cả done → null')
  it('connect-dev-server trước add-dev-server-repo trong ORDER')
})
```

---

## Acceptance Criteria

- [x] Tất cả test cases được implement (không để empty)
- [x] `WebPushManager` tests mock `web-push` library
- [x] HTTP route tests dùng `http.createServer` mock
- [x] Tất cả tests pass: `npm test`
- [x] Coverage cho các class chính > 80%
