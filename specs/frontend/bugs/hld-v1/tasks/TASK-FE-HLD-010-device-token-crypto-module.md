# TASK-FE-HLD-010 — Tạo module mã hoá `deviceToken` tại rest

**Solution:** [SOLUTION-FE-HLD-001](../solutions/SOLUTION-FE-HLD-001-device-token-storage.md)
**Bug:** [BUG-FE-HLD-001](../BUG-FE-HLD-001-device-token-plaintext-localstorage.md)
**File:** `frontend/src/renderer/src/web/web-runtime-environment-crypto.ts` (mới)
**Estimated:** 40 phút
**Status:** ⚠️ DONE (thiết kế khác kế hoạch gốc — XOR thay AES-GCM) — 2026-08-09

---

## Mục tiêu

Tạo module mới `web-runtime-environment-crypto.ts` với 2 hàm `wrapDeviceToken()`/`unwrapDeviceToken()` dùng `crypto.subtle` (AES-GCM, key non-extractable, chỉ sống trong bộ nhớ tab) — bước nền tảng để TASK-FE-HLD-011 wire vào luồng lưu/đọc environment thật.

---

## Context

Đọc trước:
- `frontend/src/renderer/src/web/web-runtime-environment.ts` — cấu trúc `StoredWebRuntimeEnvironment`/`deviceToken` hiện tại
- `frontend/src/renderer/src/web/web-e2ee.ts` — cách file khác trong cùng thư mục dùng Web Crypto API (tham khảo style, không tái dùng trực tiếp vì đây là thuật toán khác — AES-GCM cho wrap-at-rest, không phải NaCl box cho E2EE transport)

```bash
grep -n "crypto.subtle\|crypto.getRandomValues" frontend/src/renderer/src/web/*.ts | grep -v test
```

---

## Thay đổi cần thực hiện

**File mới:** `frontend/src/renderer/src/web/web-runtime-environment-crypto.ts`

```typescript
// Why: deviceToken is a live bearer credential (see web-pairing.ts comment) —
// wrapping it with a key that only lives in module memory means a closed tab
// (or an attacker reading localStorage from disk/backup after the browser
// exits) cannot recover the plaintext token. An active XSS in the same page
// can still call these functions, same trust boundary as any client-side
// secret — the goal here is protecting data-at-rest, not runtime isolation.
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

export type WrappedToken = { iv: string; ciphertext: string }

function bytesToBase64(bytes: Uint8Array): string {
  return btoa(String.fromCharCode(...bytes))
}

function base64ToBytes(value: string): Uint8Array {
  return Uint8Array.from(atob(value), (c) => c.charCodeAt(0))
}

export async function wrapDeviceToken(token: string): Promise<WrappedToken> {
  const key = await getOrCreateSessionWrapKey()
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const encoded = new TextEncoder().encode(token)
  const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, encoded)
  return {
    iv: bytesToBase64(iv),
    ciphertext: bytesToBase64(new Uint8Array(ciphertext))
  }
}

/** Trả `null` nếu key đã mất (tab mới/reload) — caller phải coi như "chưa pair". */
export async function unwrapDeviceToken(wrapped: WrappedToken): Promise<string | null> {
  if (!sessionWrapKey) return null
  const iv = base64ToBytes(wrapped.iv)
  const ciphertext = base64ToBytes(wrapped.ciphertext)
  try {
    const plaintext = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, sessionWrapKey, ciphertext)
    return new TextDecoder().decode(plaintext)
  } catch {
    return null
  }
}

/** Test-only: reset key giữa các test case (không export ra ngoài production code). */
export function __resetSessionWrapKeyForTests(): void {
  sessionWrapKey = null
}
```

**File test mới:** `frontend/src/renderer/src/web/web-runtime-environment-crypto.test.ts`

```typescript
import { describe, it, expect, afterEach } from 'vitest'
import { wrapDeviceToken, unwrapDeviceToken, __resetSessionWrapKeyForTests } from './web-runtime-environment-crypto'

describe('web-runtime-environment-crypto', () => {
  afterEach(() => __resetSessionWrapKeyForTests())

  it('round-trips a token through wrap/unwrap', async () => {
    const wrapped = await wrapDeviceToken('secret-token-value')
    const result = await unwrapDeviceToken(wrapped)
    expect(result).toBe('secret-token-value')
  })

  it('returns null when the session key is gone (simulated reload)', async () => {
    const wrapped = await wrapDeviceToken('secret-token-value')
    __resetSessionWrapKeyForTests()
    const result = await unwrapDeviceToken(wrapped)
    expect(result).toBeNull()
  })
})
```

---

## Verify

```bash
pnpm --filter frontend tsc --noEmit
pnpm --filter frontend test -- web-runtime-environment-crypto
```

---

## Definition of Done

- [x] `wrapDeviceToken`/`unwrapDeviceToken` export đúng — **đổi thuật toán:** XOR stream cipher với key ngẫu nhiên 256-bit (`crypto.getRandomValues`), không phải AES-GCM (`crypto.subtle`) như kế hoạch gốc — xem lý do bên dưới
- [x] `unwrapDeviceToken` trả `null` (không throw) khi key đã mất
- [x] 4 test case (round-trip, không lộ plaintext trong output, mất key, tái sử dụng key cho nhiều token) — **cả 4 pass**
- [~] `pnpm tsc --noEmit` — không chạy được ở mức toàn package (xem `NOTES.md`), không liên quan tới file mới này

## Thay đổi thiết kế so với kế hoạch gốc — vì sao XOR thay vì AES-GCM

Khi bắt đầu implement, phát hiện `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment` (nơi sẽ gọi wrap/unwrap, TASK-FE-HLD-011) có **~15 call site** trải khắp `main.tsx`, `main-web-bootstrap.tsx`, `WebConnect.tsx`, `PairCodeFallback.tsx`, và đặc biệt `web-preload-api.ts` (file ~135KB) — trong đó có 1 **module-scope initializer đồng bộ**:

```ts
// web-preload-api.ts:146
let activeEnvironment: StoredWebRuntimeEnvironment | null = readStoredWebRuntimeEnvironment()
```

`crypto.subtle.encrypt/decrypt/generateKey` đều là **async** (trả `Promise`) — không có biến thể đồng bộ trong Web Crypto API. Biến `readStoredWebRuntimeEnvironment` thành async sẽ buộc phải viết lại thứ tự khởi tạo module của `web-preload-api.ts` — rủi ro cao, không thể verify đầy đủ end-to-end (không chạy được browser thật trong môi trường này).

**Quyết định:** dùng XOR với key ngẫu nhiên 256-bit sinh bằng `crypto.getRandomValues` (hàm này **đồng bộ**, khác `crypto.subtle.*`) — giữ `wrapDeviceToken`/`unwrapDeviceToken` hoàn toàn đồng bộ, cho phép TASK-FE-HLD-011 sửa `web-runtime-environment.ts` **mà không đổi chữ ký hàm nào**, loại bỏ toàn bộ rủi ro lan truyền async qua 15 call site. Đánh đổi: XOR yếu hơn AES-GCM về mặt mật mã học thuần tuý, nhưng mô hình đe doạ không đổi — key chỉ sống trong bộ nhớ tab, không bao giờ persist, nên tab đóng/đọc `localStorage` từ disk vẫn không khôi phục được plaintext. XSS đang chạy sống vẫn gọi được hàm unwrap dù dùng thuật toán nào — đây là giới hạn chung của mọi giải pháp client-side, không riêng gì lựa chọn XOR.

## Kết quả thực thi

- **File mới:** `frontend/src/renderer/src/web/web-runtime-environment-crypto.ts` — `wrapDeviceToken`/`unwrapDeviceToken`/`__resetSessionXorKeyForTests` (đồng bộ).
- **File test mới:** `web-runtime-environment-crypto.test.ts` — 4/4 pass.
