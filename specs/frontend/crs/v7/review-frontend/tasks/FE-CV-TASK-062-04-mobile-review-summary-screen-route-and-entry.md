# FE-CV-TASK-062-04: Màn hình, route và điểm vào mobile

**From Solution:** [FE-CV-SOL-062](../solutions/FE-CV-SOL-062-mobile-review-summary.md) mục 2.4
**Priority:** P2
**Area:** mobile / UI
**File:** `mobile/app/h/[hostId]/review-summary/[worktreeId].tsx`, `mobile/src/components/{MobileReviewSummaryScreenView,MobileReviewSummaryHeader,MobileReviewMetricGrid,MobileReviewFindingRow}.tsx`, `mobile-review-summary-styles.ts` (mới); `MobileDiffReviewDrawers.tsx`, `source-control/MobileSourceControlBranchCard.tsx` (sửa nhỏ)
**Depends on:** FE-CV-TASK-062-03
**Status:** [x] DONE (verified 2026-10-08)

## Context

- Mẫu: `app/h/[hostId]/review/[worktreeId].tsx`, `MobileDiffReviewScreenView.tsx` (`SafeAreaView`, `useResponsiveLayout()`). Màu từ `colors` (`theme/mobile-theme.ts`), icon `lucide-react-native`, không hex.

## Việc cần làm

1. Route lấy `hostId`, `worktreeId`, `name?`, dựng controller và view.
2. View theo wireframe SOL-062 2.4: header (Back, ↻), banner index (stale/truncated), lưới số liệu, danh sách phát hiện (`FlatList`, mở rộng tại chỗ), bộ lọc, trạng thái loading/unavailable/error; hai cột khi `isWideLayout`; `RefreshControl`.
3. Severity: icon + nhãn chữ + màu `statusRed/statusAmber/textSecondary`; không `evidence`.
4. Điểm vào: overflow "Review summary" (luôn hiện) và chip ở thẻ nhánh khi đã biết khả dụng (xác nhận vị trí khi làm).

## Kiểm thử

- Không có test component RN: kiểm tay (thiết bị/giả lập) theo danh sách trạng thái; `mobile/scripts/mock-server-rpc-handlers.ts` thêm `codeIntel.reviewSummary` giả (chưa đọc kỹ file) để dựng các trạng thái.
- Chạy `tsc`/lint của mobile theo cấu hình mobile (chưa kiểm chứng).

## Tiêu chí hoàn thành

- [ ] Các tiêu chí UI của SOL-062 mục 5.
- [ ] Không hex; không đồ thị; không thêm thư viện.

## Rủi ro

- Không có test tự động cho UI; rủi ro hồi quy bố cục.

## Ghi chú triển khai (2026-10-07)

- Điểm vào: `onOpenReviewSummary` tuỳ chọn thêm vào `useMobileDiffReviewController` (trả lại nguyên giá trị) + hành động overflow "Review Summary" ở `MobileDiffReviewDrawers.tsx`. Chip ở `MobileSourceControlBranchCard` chưa làm (Q5 mở). Chưa thêm handler giả ở `mobile/scripts/mock-server-rpc-handlers.ts`. Không có banner "loading ≥ 3 s" theo giai đoạn.

## Ghi chú triển khai (2026-10-08)

- Chip: `session/mobile-review-summary-chip.ts` (thuần: `buildMobileReviewSummaryChip` chỉ trả chip khi `ready`, tone theo lỗi/cảnh báo/mức rủi ro, luôn kèm chữ; `buildMobileReviewSummaryRoute` dùng chung cho overflow ở `review/[worktreeId].tsx`), `session/use-mobile-review-summary-chip.ts` (một lần thăm dò mỗi worktree + kết nối, không polling, bộ đếm thế hệ), `source-control/MobileReviewSummaryChipRow.tsx`; `MobileSourceControlBranchCard` thêm 2 prop tuỳ chọn, `MobileSourceControlPanel` thêm 1 lời gọi hook. GitNexus impact: BranchCard LOW; **Panel HIGH** (ảnh hưởng `SessionScreen`) — thay đổi chỉ cộng thêm, chip ẩn khi không khả dụng.
- Mock-server: `scripts/mock-server-review-summary-data.ts`, chọn trạng thái bằng `MOCK_REVIEW_SUMMARY=ready|stale|truncated|empty|flag_off|no_binding|index_missing|tool_unavailable|unsupported|error`; một dòng `if` trong `mock-server-rpc-handlers.ts`.
- Render test: `test-support/react-native-static-render-mock.ts` (mock RN thành thẻ DOM, `react-dom/server`), môi trường `node`, không cần thiết bị. Banner "loading ≥ 3 s" theo giai đoạn: không làm vì hợp đồng §8 không có trường giai đoạn.
- Không chạm `desktop/`.
