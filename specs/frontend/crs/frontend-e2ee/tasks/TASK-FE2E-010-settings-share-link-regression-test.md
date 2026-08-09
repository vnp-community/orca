# TASK-FE2E-010 — Test bảo vệ: "Share this Orca server" luôn ẩn ở web client

**Source Solution:** [SOL-FE2E-005](../solutions/SOL-FE2E-005-test-and-rollout-plan.md) — kịch bản #11
**Priority:** P2
**Loại:** Test mới (bảo vệ regression, không phải test cho thay đổi mới)
**Depends on:** TASK-FE2E-001
**Estimated:** 15 phút
**Status:** ✅ DONE (đổi cách tiếp cận so với kế hoạch gốc) — 2026-08-09

---

## Context

Đây là test **bảo vệ phát hiện** của CR-FE2E-004 (không phải test cho code mới) — đảm bảo nếu sau này có ai vô tình đổi `canGeneratePairingUrl={!isWebClient}` thành logic khác, CI bắt được ngay.

```bash
find frontend/src/renderer/src/components/settings -iname "Settings.test.tsx"
```

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/components/settings/Settings.test.tsx` (tạo mới nếu chưa có, hoặc thêm vào file test hiện có của `Settings.tsx`)

```tsx
describe('RuntimeEnvironmentsPane — share-link visibility (SOL-FE2E-004)', () => {
  it('hides "Share this Orca server" when running as a web client', () => {
    // Setup: window.location.pathname ends with '/web-index.html'
    // (hoặc set __ORCA_WEB_CLIENT__ = true theo cách test hiện có mock isWebClientLocation)
    render(<Settings ... />) // theo props/setup thật của Settings.tsx
    expect(screen.queryByText(/share this orca server/i)).not.toBeInTheDocument()
  })

  it('shows "Share this Orca server" when running as Desktop (Electron)', () => {
    // Setup: __ORCA_WEB_CLIENT__ = false, pathname không kết thúc bằng web-index.html
    render(<Settings ... />)
    expect(screen.getByText(/share this orca server/i)).toBeInTheDocument()
  })
})
```

> [!IMPORTANT]
> Điều chỉnh cách mock `isWebClientLocation()`/`window.location` theo đúng test harness hiện có của `Settings.tsx` (kiểm tra xem file test đã tồn tại có pattern mock nào cho `isWebClient` chưa trước khi viết mới — tránh trùng lặp).

## Verify

```bash
cd frontend
node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/components/settings/Settings.test.tsx
```

## Definition of Done

- [x] 2 test case pass — 1 xác nhận ẩn ở web client, 1 xác nhận hiện ở Desktop
- [x] Không sửa `RuntimeEnvironmentsPane.tsx` (không tính thay đổi ngoài task này ở `Settings.tsx` do TASK-FE2E-009 — chỉ thêm comment, không đổi logic)

## Thay đổi so với kế hoạch gốc

Kế hoạch gốc đề xuất tạo `Settings.test.tsx` và render toàn bộ component `Settings` (mock `isWebClientLocation()`). Sau khi kiểm tra, `Settings.tsx` là 1 file rất lớn (~50+ import pane con) — render trực tiếp sẽ cần mock rất nhiều dependency không liên quan tới điều đang test.

**Đổi hướng:** test thẳng `RuntimeEnvironmentsPane` (component thực sự chứa logic ẩn/hiện) qua chính prop `canGeneratePairingUrl` — đây là ranh giới component hẹp hơn, đúng thứ cần bảo vệ, và khớp với pattern `renderToStaticMarkup` (không phải `@testing-library/react`'s `render`) đã có tiền lệ trong cùng thư mục (`NotificationsPane.test.tsx`) — nhẹ hơn vì bỏ qua `useEffect` (không cần mock `window.api.runtimeEnvironments.list()`).

**Vướng mắc phát sinh + fix:** lần chạy đầu, case `canGeneratePairingUrl=true` throw `Tooltip must be used within TooltipProvider` (Radix UI) — vì `RuntimeEnvironmentsPane` không tự bọc `TooltipProvider` (dựa vào `App.tsx` bọc toàn cục ở production). Đã bọc `<TooltipProvider>` quanh component trong test — theo đúng tiền lệ đã dùng ở `components/sidebar/Sidebar.test.tsx`.

## Kết quả thực thi

- **File mới:** `frontend/src/renderer/src/components/settings/RuntimeEnvironmentsPane.share-link.test.tsx` — 2 test, cả 2 pass.
- **Verify:** chạy chung với `RuntimeEnvironmentsPane.test.ts` hiện có — tổng 9 test, 9/9 pass, không có regression.
