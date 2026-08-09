# TASK-FE2E-008 — Test dynamic import + đo bundle size trước/sau

**Source Solution:** [SOL-FE2E-003](../solutions/SOL-FE2E-003-lazy-split-pairing-bundle.md) §3, §4
**Priority:** P1
**Loại:** Test + đo lường
**Depends on:** TASK-FE2E-007
**Status:** ⚠️ DONE — AC-1 chỉ đạt 1 phần, đúng như giới hạn đã ghi nhận sẵn trong SOL-FE2E-003 §2.2 — 2026-08-09
**Estimated:** 30 phút

---

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/web/__tests__/main-web.test.ts` (tạo mới nếu chưa có test cho `main.tsx`)

```ts
vi.mock('./pair-code-app-entry', () => ({
  mountPairCodeApp: vi.fn()
}))

it('dynamically imports pair-code-app-entry only when /auth/config 404s', async () => {
  mockFetch.mockResolvedValueOnce({ ok: false, status: 404 })
  await import('../main')
  const { mountPairCodeApp } = await import('./pair-code-app-entry')
  expect(mountPairCodeApp).toHaveBeenCalled()
})

it('does NOT import pair-code-app-entry when /auth/config returns 200', async () => {
  mockFetch.mockResolvedValueOnce({ ok: true, status: 200 })
  const importSpy = vi.spyOn(await import('./pair-code-app-entry'), 'mountPairCodeApp')
  await import('../main')
  expect(importSpy).not.toHaveBeenCalled()
})
```

> [!IMPORTANT]
> Điều chỉnh setup mock `fetch`/`vi.resetModules()` theo đúng convention test hiện có của file `main.tsx`/`main-web-bootstrap.test.ts` trong cùng thư mục — không tạo pattern mock mới không nhất quán.

## Đo bundle size

```bash
cd frontend
pnpm build
ls -la out/web/assets/*.js | sort -k5 -n
grep -l "nacl" out/web/assets/*.js
```

Ghi lại kết quả (tên chunk chứa `nacl`, kích thước) vào phần "Kết quả thực thi" của file task này sau khi chạy — **kỳ vọng:** chunk chứa `nacl` là 1 file riêng (không phải chunk entry chính `web-index-*.js`).

## Verify

```bash
node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/web/__tests__/main-web.test.ts
```

## Definition of Done

- [x] Test case pass — viết **3 test** (nhiều hơn kế hoạch 2, thêm case network-error) xác nhận đúng nhánh nào gọi `mountPairCodeApp` — cả 3 pass ngay lần chạy đầu
- [x] Bundle build thành công (`pnpm build`, ~1 phút)
- [~] Xác nhận `nacl` không còn nằm trong chunk entry chính — **KHÔNG đạt hoàn toàn**, xem "Kết quả đo bundle" bên dưới — đúng như giới hạn SOL-FE2E-003 §2.2 đã ghi nhận trước
- [x] Số liệu bundle size ghi lại bên dưới

## Kết quả đo bundle (thật, sau khi `pnpm build`)

| Chunk | Kích thước (raw / gzip) | Chứa `nacl`? | Ghi chú |
|---|---|---|---|
| `web-index-CtHsu70F.js` (entry, tải bởi MỌI browser) | 255.49 kB / 74.39 kB | **Có** (1 match) | Qua `main-web-bootstrap.tsx` → `web-preload-api.ts` (import tĩnh `WebRuntimeClient`) — nhánh dùng bởi CẢ 2 use case, không tách được (SOL-FE2E-003 §2.2 đã ghi rõ, không nằm trong scope CR này) |
| `pair-code-app-entry-CkvU4lxW.js` (chỉ tải khi 404) | 2.3 kB | Không | Nhẹ, đúng kỳ vọng |
| `WebConnect-N5XPg_ur.js` (chỉ tải khi 404, qua `lazyWithRetry`) | 4.0 kB | Không | Nhẹ, đúng kỳ vọng |

**Đối chiếu AC-1 gốc của CR-FE2E-003** (*"Bundle 200-case không chứa web-e2ee.ts/web-runtime-client.ts/WebConnect.tsx"*): **đạt 1 phần** — `WebConnect.tsx` (component + logic UI pairing) đã tách thành công khỏi entry chunk (giảm ~110 dòng code trong `main.tsx`, 2 chunk lazy riêng biệt chỉ 2.3KB+4KB). **`web-e2ee.ts`/`WebRuntimeClient` KHÔNG tách được** — vẫn nằm trong entry chunk 255KB vì `web-preload-api.ts` import tĩnh không điều kiện, dùng chung bởi cả `bootstrapWebApp()` lẫn `pair-code-app-entry.tsx`. Đây **không phải lỗi thực thi** — SOL-FE2E-003 §2.2 đã dự đoán và chủ động loại trừ việc tách `web-preload-api.ts` ra khỏi phạm vi CR này để "tránh trộn 2 rủi ro (dead-code splitting + client-selection refactor) trong 1 lần đổi".

**Kết luận:** thành công tách UI pairing (`WebConnect`) và code khởi tạo `WebRoot`/`WebRootBoundary` khỏi bundle chính; **chưa** giảm được phần nặng nhất (thư viện crypto `nacl`) — cần 1 CR/task riêng (đã được SOL-FE2E-003 ghi nhận là follow-up ngoài phạm vi) để tách `web-preload-api.ts` thành 2 biến thể theo nhánh nếu muốn đạt AC-1 đầy đủ.
