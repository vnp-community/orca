# FE-REQ-TASK-018-05: Khung `RequestPage`, nút sidebar, badge dùng chung, i18n nền

**From Solution:** [FE-REQ-SOL-018](../solutions/FE-REQ-SOL-018-request-frontend-foundation.md) mục 2.5 (định tuyến), phần thành phần dùng chung
**Priority:** P0
**Area:** frontend / components
**File:** `frontend/src/renderer/src/components/request/RequestPage.tsx`, `RequestPageHeader.tsx`, `RequestUnsupportedNotice.tsx`, `RequestStatusBadge.tsx`, `RequestTypeBadge.tsx`, `RequestSourceBadge.tsx`, `ApprovalStatusBadge.tsx`, `request-status-presentation.ts` (đều mới); `components/sidebar/SidebarRequestNavButton.tsx` (mới), `SidebarNav.tsx` (sửa, gần dòng 71); `App.tsx` (sửa: lazy `RequestPage`); `i18n/locales/{en,es,ja,ko,zh}.json`; `i18n/request-locale-coverage.test.ts` (mới); `docs/ui/pages/requests.md` (mới), `docs/ui/page-tree.md` (sửa)
**Depends on:** FE-REQ-TASK-018-03, 018-04
**Status:** [~] PARTIAL — components, badges, RequestPageHeader, 5-locale keys, request-locale-coverage.test, RequestPage/badge/sidebar tests and docs/ui/pages/requests.md pass; e2e tests/e2e/request-page.spec.ts not written/run

## Context

- `TaskPage` lazy từ `App.tsx` (`docs/ui/pages/tasks.md`). Nút sidebar: `SidebarNav.tsx:71` `<SidebarTaskNavButton />`.
- `TaskSourceBadge.tsx` là mẫu badge nguồn (chỉ link `http/https`). `TaskStatusBadge.tsx` dùng emoji: KHÔNG làm theo, dùng icon `lucide-react` và token.
- Token màu: `status-success`, `destructive`, `primary`, `muted-foreground` (`src/renderer/src/assets/main.css`); không hex, không lớp màu Tailwind thô (`guides/STYLEGUIDE.md`).
- i18n: khoá đọc theo tên dạng `auto.components.<...>`, qua `translate(key, fallback)`; có đủ 5 locale `en,es,ja,ko,zh` (`i18n/locales/*.json`); mẫu test `i18n/task-jira-link-locale-coverage.test.ts`; `lint` gốc có `verify:localization-catalog`/`coverage` (chưa kiểm chứng chạy được).
- Tabs có sẵn: `components/ui/tabs.tsx`.

## Việc cần làm

1. `request-status-presentation.ts`: `getRequestStatusPresentation(status) → {icon, toneClass, labelKey}`; `completed`/`approved` dùng `status-success`, `cancelled`/`rejected` dùng `destructive`, `analyzing|planning|executing|classifying` dùng `primary`, còn lại `muted-foreground`; `getRequestTypePresentation(type)`.
2. Badge: `RequestStatusBadge({status})`, `RequestTypeBadge({type})`, `RequestSourceBadge({provider,ref,url})` (chặn mọi URL không phải `http:`/`https:`), `ApprovalStatusBadge({status})`. Mọi badge có `aria-label` bằng nhãn i18n; trạng thái không chỉ truyền đạt bằng màu (có icon và chữ).
3. `RequestPage`: `Tabs` Requests | Hộp duyệt (kèm số) | Backlog (`requestPage.section`); thân mỗi tab là placeholder `<div data-testid="request-tab-...">` do CR-019/022/023 điền; khi `requestFlowSupport==='unsupported'` thay toàn thân bằng `RequestUnsupportedNotice`; khi `'unknown'` hiện `Skeleton`. Nút đóng gọi `closeRequestPage()`; phím `Escape` đóng khi tiêu điểm không ở ô nhập.
4. `RequestPageHeader`: tiêu đề, bộ chọn dự án (dùng danh sách dự án có sẵn trong store, như `TaskPage`), chỗ cho nút "Tạo Request" (CR-019 điền).
5. `SidebarRequestNavButton`: icon lucide (ví dụ `Inbox`), tooltip, chấm số `pendingApprovalCount` (hiện `99+` khi lớn hơn 99, ẩn khi 0), ẩn khi `requestFlowSupport!=='supported'`; thêm vào `SidebarNav.tsx`.
6. `App.tsx`: `const RequestPage = lazy(() => import('./components/request/RequestPage'))`, render khi `activeView==='requests'` (Suspense như `TaskPage`).
7. i18n: thêm khoá nền với tiền tố `auto.components.request.`: `RequestPage.title`, `RequestPage.tab.requests|approvals|backlog`, `RequestUnsupportedNotice.title|body`, `RequestStatus.<11 giá trị>`, `RequestType.<11 loại>.label|description`, `ApprovalStatus.<5>`, `SidebarRequestNavButton.label|badge`, `error.forbidden|notFound|conflict|invalidState|network|unavailable|rateLimited|unknown` (dùng cho mọi CR sau); đủ 5 locale.
8. `request-locale-coverage.test.ts`: danh sách khoá, kiểm mỗi locale có chuỗi không rỗng.
9. Tài liệu: `docs/ui/pages/requests.md` theo khuôn `docs/ui/pages/tasks.md`; thêm mục vào `docs/ui/page-tree.md`.

## Kiểm thử

- Component: `RequestStatusBadge`/`RequestTypeBadge` (đủ giá trị, có `unknown`), `RequestSourceBadge` (`javascript:` không thành link), `SidebarRequestNavButton` (ẩn/hiện, `99+`), `RequestPage` (đổi tab, `unsupported`, `unknown` Skeleton, Escape), mở rộng `SidebarNav.test.tsx`.
- Phím tắt: `Escape` không đóng khi tiêu điểm ở `input/textarea`.
- Test không có hex: quét file mới bằng test đơn giản (`/#[0-9a-fA-F]{3,8}\b/` trong nguồn `.tsx` mới).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request src/renderer/src/components/sidebar/SidebarNav src/renderer/src/i18n/request-locale-coverage`.
- E2E (`tests/e2e/request-page.spec.ts`, mới; chạy `pnpm test:e2e` hoặc lệnh `npx playwright test tests/e2e/request-page.spec.ts --config tests/playwright.config.ts --project electron-headless` từ gốc repo, chưa kiểm chứng): mở trang bằng `window.__store`, kiểm nút sidebar ẩn khi runtime không có kênh.

## Tiêu chí hoàn thành

- [ ] Nút sidebar và `RequestPage` hiện khi `supported`, ẩn khi `unsupported`; chấm số cập nhật khi `setPendingApprovalCount`.
- [ ] Chuỗi mới đủ 5 locale; test phủ khoá xanh.
- [ ] Không hex, không emoji, chỉ token.
- [ ] Trạng thái rỗng, tải, lỗi của khung đều có UI.

## Rủi ro và lưu ý

- Nút sidebar thêm vào `SidebarNav.tsx` chạm khu vực rất đông người sửa: tách thành component riêng, diff một dòng.
- `docs/ui/page-tree.md` có thể có khuôn riêng; đọc trước khi sửa.
