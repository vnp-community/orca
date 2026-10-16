# FE-REQ-TASK-037-01: Nút Requests ở sidebar thành hàng có nhãn

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.1, CR-037 mục 2.1
**Priority:** P1
**Area:** frontend / sidebar
**File:** `frontend/src/renderer/src/components/sidebar/SidebarRequestNavButton.tsx` (sửa), `SidebarRequestNavButton.test.tsx`, `SidebarNav.test.tsx` (sửa)
**Depends on:** không (làm được ngay, không cần backend)
**Status:** [x] DONE

---

## Context

- Nút hiện là `size-8` chỉ có icon `Inbox`; các hàng khác trong `SidebarNav.tsx` (Automations, Orca Mobile) dùng `flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] font-medium tracking-tight`, active `bg-worktree-sidebar-accent text-worktree-sidebar-accent-foreground`, không active `text-worktree-sidebar-foreground/60 hover:bg-worktree-sidebar-foreground/8`, icon `size-4 shrink-0` đổi `strokeWidth` 2.25/1.75.
- Pill đếm của "Agents": `rounded-full bg-primary px-1.5 py-px text-[10px] font-semibold text-primary-foreground`. Đã chốt dùng màu `primary` (F10), không `destructive`.
- Nhãn có sẵn: `auto.components.request.SidebarRequestNavButton.label` ("Requests"); giữ `data-testid="sidebar-requests"`.
- Nút ẩn khi `requestFlowSupport !== 'supported'` (giữ).

## Việc cần làm

1. Đổi markup sang hàng như các mục khác: icon `Inbox` (`size-4 shrink-0`), `<span className="flex-1">` nhãn, pill số chờ duyệt bên phải khi `pendingApprovalCount > 0`, giới hạn `99+` (giữ `formatBadgeCount`).
2. Truy cập: `aria-current="page"` khi active (thay `aria-pressed`), `title` và `aria-label` có số ("Requests, 3 pending approvals"), giữ chuỗi `badge` có `{{count}}`.
3. Giữ `onClick={() => setActiveView('requests')}`; không thêm menu ngữ cảnh ẩn (khác Automations/Mobile, không có cài đặt ẩn).
4. Không thêm khoá i18n mới nếu khoá hiện có đủ; nếu cần chuỗi kết hợp "Requests, N", thêm khoá và đủ 5 locale.

## Kiểm thử

- Hiện nhãn "Requests" và icon; active khi `activeView='requests'`; không active thì lớp mờ như các hàng khác.
- Pill hiện `3`, `99+` khi 120, ẩn khi 0; `aria-label` chứa số.
- Ẩn khi `requestFlowSupport !== 'supported'`.
- `SidebarNav.test.tsx` vẫn qua (thứ tự hàng: Onboarding, Tasks, Requests, Automations, Orca Mobile...).
- `pnpm --filter orca-frontend test -- src/renderer/src/components/sidebar/SidebarRequestNavButton src/renderer/src/components/sidebar/SidebarNav` (chưa chạy).

## Tiêu chí hoàn thành

- [x] Hàng Requests có icon và nhãn cùng kiểu với Tasks/Automations.
- [x] Pill số dùng `primary`, `99+` đúng, không còn đè góc icon.
- [x] `aria-current`, `aria-label`, `title` đúng.
- [x] Không có màu hex hay token mới; test xanh.

## Rủi ro và lưu ý

- Chiều rộng sidebar hẹp: pill không được làm nhãn xuống dòng (`min-w-0`, `truncate` cho nhãn nếu cần).
- Test snapshot ở nơi khác phụ thuộc kích thước nút cũ; tìm bằng `sidebar-requests`.

## Ghi chú triển khai

- `SidebarRequestNavButton.tsx`: nút `size-8` đổi thành hàng `flex w-full items-center gap-2 rounded-md px-2 py-1.5 ...` giống Automations/Agents (active `bg-worktree-sidebar-accent`, không active mờ `text-worktree-sidebar-foreground/60`); icon `Inbox` `size-4 shrink-0`, `strokeWidth` 2.25/1.75; nhãn `<span className="min-w-0 flex-1 truncate">`; pill `bg-primary text-primary-foreground` (`data-testid="sidebar-requests-badge"`), `99+` giữ `formatBadgeCount`.
- Truy cập: `aria-current="page"` khi active (bỏ `aria-pressed`); `title` và `aria-label` ghép từ hai khoá sẵn có: "Requests, 3 pending approvals" (không thêm khoá i18n mới, không đụng 5 locale).
- `SidebarRequestNavButton.test.tsx`: thêm ca kiểm tra hàng có nhãn, pill `primary`, `aria-current`, tên truy cập có số. `SidebarNav.test.tsx` không cần sửa (store giả không có `requestFlowSupport`, nên nút ẩn) và vẫn xanh.
- Đã chạy: `npx vitest run --config config/vitest.config.ts` cho SidebarRequestNavButton, SidebarNav, request-locale-coverage: 3 file, 28 test xanh. `tsc` không có lỗi mới ở file đã sửa.
