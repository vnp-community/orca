# TASK-FE-HLD-011 — Wire wrap/unwrap vào lưu/đọc environment + xử lý mất key

**Solution:** [SOLUTION-FE-HLD-001](../solutions/SOLUTION-FE-HLD-001-device-token-storage.md)
**Bug:** [BUG-FE-HLD-001](../BUG-FE-HLD-001-device-token-plaintext-localstorage.md)
**File:** `frontend/src/renderer/src/web/web-runtime-environment.ts`, `frontend/src/renderer/src/web/web-runtime-client.ts`
**Estimated:** 30 phút
**Status:** ✅ DONE (đơn giản hơn kế hoạch gốc nhờ giữ hàm đồng bộ) — 2026-08-09
**Phụ thuộc:** TASK-FE-HLD-010

---

## Mục tiêu

Bọc `deviceToken` bằng `wrapDeviceToken()`/`unwrapDeviceToken()` (TASK-FE-HLD-010) trước khi ghi/đọc `localStorage`, và xử lý đúng trường hợp `unwrapDeviceToken()` trả `null` (yêu cầu re-pair thay vì lỗi mơ hồ).

---

## Context

```bash
grep -n "saveStoredWebRuntimeEnvironment\|readStoredWebRuntimeEnvironment\|deviceToken" frontend/src/renderer/src/web/web-runtime-environment.ts
```

Đọc trước: `web-runtime-environment.ts` toàn bộ (đã đọc trong quá trình audit — cấu trúc `StoredWebRuntimeEnvironment.endpoints[].deviceToken`).

---

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/web/web-runtime-environment.ts`

Import module mới:
```typescript
import { wrapDeviceToken, unwrapDeviceToken, type WrappedToken } from './web-runtime-environment-crypto'
```

Đổi type `endpoints[].deviceToken: string` → lưu dạng `WrappedToken` trên đĩa, nhưng giữ API công khai (`getPreferredWebPairingOffer`) trả `string` plaintext như cũ để không phải sửa mọi call site:

```typescript
// StoredWebRuntimeEnvironment — đổi field lưu trên đĩa:
// deviceToken: string  →  deviceTokenWrapped: WrappedToken

export async function saveStoredWebRuntimeEnvironment(
  environment: StoredWebRuntimeEnvironment
): Promise<void> {
  const wrapped = {
    ...environment,
    endpoints: await Promise.all(
      environment.endpoints.map(async (ep) => ({
        ...ep,
        deviceToken: undefined,
        deviceTokenWrapped: ep.deviceToken ? await wrapDeviceToken(ep.deviceToken) : null
      }))
    )
  }
  window.localStorage.setItem(ENVIRONMENT_STORAGE_KEY, JSON.stringify(wrapped))
}

export async function readStoredWebRuntimeEnvironment(): Promise<StoredWebRuntimeEnvironment | null> {
  const raw = window.localStorage.getItem(ENVIRONMENT_STORAGE_KEY)
  if (!raw) return null
  const parsed = JSON.parse(raw)
  if (!parsed.id || !parsed.name || parsed.endpoints.length === 0) return null

  const endpoints = await Promise.all(
    parsed.endpoints.map(async (ep: { deviceTokenWrapped: WrappedToken | null }) => {
      const deviceToken = ep.deviceTokenWrapped ? await unwrapDeviceToken(ep.deviceTokenWrapped) : ''
      return { ...ep, deviceToken: deviceToken ?? '' }
    })
  )
  // Why: nếu BẤT KỲ endpoint nào không unwrap được (key mất do reload) và
  // deviceTokenWrapped không null (nghĩa là lẽ ra phải có token), coi cả
  // environment là không dùng được — ép người dùng pair lại thay vì chạy với
  // deviceToken rỗng gây lỗi auth khó hiểu ở tầng RPC.
  const hasUnrecoverableToken = parsed.endpoints.some(
    (ep: { deviceTokenWrapped: WrappedToken | null }, i: number) =>
      ep.deviceTokenWrapped !== null && endpoints[i].deviceToken === ''
  )
  if (hasUnrecoverableToken) {
    clearStoredWebRuntimeEnvironment()
    return null
  }

  return { ...parsed, endpoints }
}
```

> [!IMPORTANT]
> `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment` đổi từ sync sang **async** — phải cập nhật MỌI call site (`web-pairing.ts`/`PairCodeFallback.tsx`/`WebConnect.tsx`/`main-web-bootstrap.tsx`/`web-preload-api.ts`) thêm `await`. Chạy `pnpm tsc --noEmit` sau khi sửa xong sẽ tự liệt kê hết các nơi cần sửa (lỗi kiểu "Promise is not assignable").

**File:** `frontend/src/renderer/src/web/web-runtime-client.ts`

Xác nhận constructor `WebRuntimeClient` nhận `deviceToken` đã unwrap sẵn (plaintext) từ `getPreferredWebPairingOffer()` — không cần sửa gì trong file này nếu `getPreferredWebPairingOffer` giữ nguyên chữ ký trả `WebPairingOffer` với `deviceToken: string` như cũ (chỉ đổi ở tầng lưu trữ, không đổi ở tầng dùng).

---

## ⚠️ Thay đổi lớn so với kế hoạch gốc — đọc trước khi xem "Thay đổi cần thực hiện" ở trên

Đoạn "Thay đổi cần thực hiện" phía trên (đổi `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment` sang **async**, field `deviceTokenWrapped`) là **kế hoạch gốc, KHÔNG được áp dụng**. TASK-FE-HLD-010 đổi thuật toán từ AES-GCM (`crypto.subtle`, async) sang XOR (`crypto.getRandomValues`, đồng bộ) chính xác để tránh việc này — 2 hàm **giữ nguyên chữ ký đồng bộ**, field vẫn tên `deviceToken` (giá trị đổi từ plaintext sang wrapped), không có field `deviceTokenWrapped` nào cả. Xem "Kết quả thực thi" bên dưới cho code thật đã áp dụng.

## Verify

```bash
pnpm --filter frontend test -- web-runtime-environment web-preload-api
```

## Definition of Done

- [x] `deviceToken` không còn ghi plaintext vào `localStorage` — ghi dạng wrapped (base64 XOR output, field `deviceToken` giữ nguyên tên, không đổi thành `deviceTokenWrapped` như kế hoạch gốc — xem lý do bên dưới)
- [x] `readStoredWebRuntimeEnvironment` trả `null` (ép re-pair) khi unwrap thất bại cho token lẽ ra phải có
- [x] **Không cần sửa call site nào khác** — vì TASK-FE-HLD-010 đổi sang thuật toán đồng bộ (XOR), `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment` **giữ nguyên chữ ký đồng bộ**, nên tất cả ~15 call site (`main.tsx`, `main-web-bootstrap.tsx`, `WebConnect.tsx`, `PairCodeFallback.tsx`, `web-preload-api.ts` — kể cả module-scope initializer dòng 146) hoạt động không cần sửa gì
- [x] Test mới `web-runtime-environment.test.ts` (chưa từng có test trước đây) — 5 test case, cả 5 pass
- [x] Test hiện có `web-preload-api.test.ts` — phát hiện + fix 1 regression (xem bên dưới)

## Thay đổi so với kế hoạch gốc

Không đổi tên field thành `deviceTokenWrapped` như bản kế hoạch — giữ nguyên tên field `deviceToken` trong `StoredWebRuntimeEnvironment`, chỉ đổi **giá trị** (wrapped thay vì plaintext) ngay bên trong `saveStoredWebRuntimeEnvironment`/`readStoredWebRuntimeEnvironment`. Lý do: giữ nguyên type `StoredWebRuntimeEnvironment`/`WebPairingOffer` không đổi, giảm blast radius xuống chỉ còn 2 hàm này (đúng tinh thần "không đổi call site" mà việc chuyển sang XOR đồng bộ đã cho phép).

## Regression phát hiện + fix trong lúc verify

`web-preload-api.test.ts` có helper `writeStoredRuntimeEnvironment()` ghi thẳng `deviceToken: 'token'` (plaintext) vào `localStorage` giả lập, bỏ qua `saveStoredWebRuntimeEnvironment`. Sau khi sửa `readStoredWebRuntimeEnvironment` để unwrap, helper này khiến 44 test fail (`unwrapDeviceToken('token')` → `null` vì key trong bộ nhớ chưa từng được set → code coi là "unrecoverable" → xoá storage → mọi RPC call sau đó throw "Pair this web client with an Orca server first."). Đã sửa: `writeStoredRuntimeEnvironment` chuyển sang `async`, dùng `wrapDeviceToken` (import động, cùng "epoch" module với `installApi()` sau `vi.resetModules()` để chia sẻ đúng in-memory key), cập nhật `await` ở 40 call site (sed cơ học, không sửa tay từng chỗ). Kết quả: `web-preload-api.test.ts` từ 44 fail → còn **1 fail duy nhất**, và fail đó là do thiếu file `src/preload/gitlab.ts` (khoảng trống hạ tầng có sẵn từ trước, không liên quan — cùng loại với `preload-no-change.test.ts` đã ghi trong `NOTES.md`).

## Kết quả thực thi

- **File sửa:** `web-runtime-environment.ts` (wrap trong `saveStoredWebRuntimeEnvironment`, unwrap + xử lý "unrecoverable" trong `readStoredWebRuntimeEnvironment`).
- **File mới:** `web-runtime-environment.test.ts` (5 test, module chưa từng có test trước đây).
- **File sửa (fix regression):** `web-preload-api.test.ts` (`writeStoredRuntimeEnvironment` → async + wrap, 40 call site thêm `await`).
- **Kết quả test toàn `src/renderer/src/web/`:** 44 fail → 4 fail (cả 4 đều là khoảng trống hạ tầng `src/preload/` có sẵn từ trước — không liên quan tới thay đổi này).
