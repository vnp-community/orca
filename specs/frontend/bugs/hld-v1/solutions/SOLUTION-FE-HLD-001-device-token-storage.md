# SOLUTION: BUG-FE-HLD-001 — `deviceToken` E2EE pairing lưu plaintext trong `localStorage`

**Source-verified:** ✅ Dựa trên source code thực tế
**TDD tham chiếu:** [tdd/v5/03-runtime-client-layer.md](../../../tdd/v5/03-runtime-client-layer.md) §"restructure_v1 Addendum" — xác nhận `WebRuntimeClient` (E2EE) là transport chính thức cho mọi RPC **sau khi pairing thành công**, dùng bởi `web-preload-api.ts`. TDD không mô tả cơ chế lưu trữ `deviceToken` cụ thể — đây là khoảng trống thiết kế cần lấp bằng nguyên tắc chung đã áp dụng ở nơi khác trong TDD ([tdd/v5/13-ai-provider-ui.md:19](../../../tdd/v5/13-ai-provider-ui.md#L19): *"credential NEVER logged or sent in plaintext"*).

---

## Root cause

`saveStoredWebRuntimeEnvironment()` ghi toàn bộ `StoredWebRuntimeEnvironment` (bao gồm `deviceToken` plaintext) vào `localStorage` không giới hạn thời gian sống, không mã hoá.

## Fix — 2 giai đoạn

### Giai đoạn 1 (ngắn hạn, không đổi luồng): mã hoá tại rest bằng session-scoped key

`localStorage` không có API mã hoá native, nhưng trình duyệt có `crypto.subtle` — dùng để bọc token bằng 1 key chỉ tồn tại trong bộ nhớ tab hiện tại (không giải quyết hoàn toàn XSS runtime, nhưng chặn được rò rỉ qua backup/inspect devtools/extension đọc `localStorage` khi tab đã đóng).

**File mới:** `frontend/src/renderer/src/web/web-runtime-environment-crypto.ts`

```ts
// Why: deviceToken is a live bearer credential (see web-pairing.ts comment) —
// wrapping it with a key that only lives in module memory means a closed tab
// (or an attacker reading localStorage from disk/backup after the browser
// exits) cannot recover the plaintext token, even though an active XSS in
// the same page still can (same trust boundary as any client-side secret).
let sessionWrapKey: CryptoKey | null = null

async function getOrCreateSessionWrapKey(): Promise<CryptoKey> {
  if (sessionWrapKey) return sessionWrapKey
  sessionWrapKey = await crypto.subtle.generateKey(
    { name: 'AES-GCM', length: 256 },
    false, // not extractable — cannot be read back out, only used in this tab
    ['encrypt', 'decrypt']
  )
  return sessionWrapKey
}

export async function wrapDeviceToken(token: string): Promise<{ iv: string; ciphertext: string }> {
  const key = await getOrCreateSessionWrapKey()
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const encoded = new TextEncoder().encode(token)
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, encoded)
  return {
    iv: btoa(String.fromCharCode(...iv)),
    ciphertext: btoa(String.fromCharCode(...new Uint8Array(ciphertext)))
  }
}

export async function unwrapDeviceToken(wrapped: { iv: string; ciphertext: string }): Promise<string | null> {
  if (!sessionWrapKey) return null // key lost (new tab/reload) — caller must re-pair
  const iv = Uint8Array.from(atob(wrapped.iv), (c) => c.charCodeAt(0))
  const ciphertext = Uint8Array.from(atob(wrapped.ciphertext), (c) => c.charCodeAt(0))
  try {
    const plaintext = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, sessionWrapKey, ciphertext)
    return new TextDecoder().decode(plaintext)
  } catch {
    return null
  }
}
```

**Đổi ở `web-runtime-environment.ts`:** `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment`/`getPreferredWebPairingOffer` bọc/giải `deviceToken` qua `wrapDeviceToken`/`unwrapDeviceToken` trước khi ghi/đọc `localStorage`.

**Đánh đổi cần chấp nhận:** vì `CryptoKey` không extractable và chỉ sống trong memory, **reload trang sẽ mất khả năng giải mã token đã lưu** — cần fallback: nếu `unwrapDeviceToken` trả `null`, coi như "chưa pair", yêu cầu người dùng pair lại (giống hành vi khi token hết hạn). Đây là đánh đổi bảo mật hợp lý — token không còn "vĩnh viễn readable" nữa.

### Giai đoạn 2 (dài hạn — khuyến nghị chính): thu hẹp phạm vi dùng deviceToken

Theo [CR-FE2E series](../../../../../docs/crs/v2/frontend-e2ee/) đã có sẵn: sau khi CR-FE2E-002/003 triển khai, nhánh multi-user (nói chuyện với `backend`) không còn dùng `deviceToken` nữa — chỉ còn nhánh "Desktop Pair Code sharing" (use case B) cần nó. Phạm vi rủi ro giảm xuống chỉ còn 1 use case, và Giai đoạn 1 ở trên chỉ cần áp dụng cho nhánh đó.

## Test cần thêm

- `web-runtime-environment-crypto.test.ts`: wrap → unwrap round-trip đúng; unwrap sau khi "mất key" (simulate reload) trả `null`.
- `web-runtime-environment.test.ts`: cập nhật test hiện có để wrap/unwrap qua mock `crypto.subtle` (đã có `happy-dom`/jsdom hỗ trợ `crypto.subtle` trong môi trường test Vitest hiện tại — xác nhận trước khi viết test).

## Tóm tắt thay đổi

| File | Thay đổi |
|---|---|
| `web-runtime-environment-crypto.ts` (mới) | `wrapDeviceToken`/`unwrapDeviceToken` dùng AES-GCM, key non-extractable |
| `web-runtime-environment.ts` | `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment` gọi wrap/unwrap |
| `web-runtime-client.ts` | Xử lý `unwrapDeviceToken() === null` → yêu cầu re-pair thay vì lỗi mơ hồ |
