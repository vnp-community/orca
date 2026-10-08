# FE-CV-TASK-059-05: `FindingsPanel`: danh sách, hàng, popover Bỏ qua / Đã xử lý / Mở lại

**From Solution:** [FE-CV-SOL-059](../solutions/FE-CV-SOL-059-contract-lens-and-findings.md) mục 2.3, 2.4
**Priority:** P1
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/review-map/findings/{FindingsPanel,FindingsToolbar,FindingsList,FindingRow,FindingDismissPopover}.tsx` (mới) + test
**Depends on:** FE-CV-TASK-059-02, 059-03; FE-CV-SOL-051-review-workspace-shell (dock đáy, `react-resizable-panels`); `lib/screen-submit-shortcut.ts`, `components/ShortcutKeyCombo.tsx` (đã có)
**Status:** [~] PARTIAL — component + dock đáy đã gắn (ReviewBottomDock + review-dock-registry; ReviewWorkspace.companions.test 8/8, findings/ 36/36 PASS); thiếu: chip "N phát hiện" ở thanh tóm tắt; phím j/k/Enter/d/r/Esc vẫn xử lý cục bộ trong panel (chưa qua registry SOL-052)

## Context

- PQ-05/06: Bỏ qua áp cho cả repo, **không** miễn cổng cho phát hiện `error`; hai nguồn tách (tab "Cấu trúc" = `Finding`).
- Toast chỉ cho xác nhận thoáng qua; lỗi cần đọc là inline persistent.

## Việc cần làm

1. `FindingsPanel` gắn vào dock đáy (tab "Cấu trúc"), thu gọn được, chip "N phát hiện" ở thanh tóm tắt; thanh công cụ: chip kind + đếm, severity, "Chỉ do thay đổi này" (`origin=introduced`), "Hiện đã bỏ qua/đã xử lý (N)", tìm kiếm, `indexFreshness` chip.
2. `FindingsList`: nhóm theo `kind`, ảo hoá > 50 dòng, "Tải thêm" khi có `nextPageToken`, ghi "Đang hiển thị X / Y".
3. `FindingRow`: icon kind, tiêu đề, severity (icon + chữ), `path:line`, nhãn `origin`, `confidence`, owner; hành động `[Xem trong đồ thị] [Xem diff|Mở tệp] [Ghi chú] [Bỏ qua ▾] [Đã xử lý] [Mở lại]`; "Ghi chú" gọi luồng SOL-060 (chưa có ⇒ ẩn); lỗi inline trên dòng.
4. `FindingDismissPopover`: preset lý do bắt buộc (đặt vào `reason`) + ô `note` (`ui/textarea`, ≤ 500), `Mod+Enter` xác nhận (`isScreenSubmitShortcut`), chip phím `ShortcutKeyCombo`; copy nêu "áp cho cả repo" và không hứa miễn cổng.
5. Phím cục bộ `j/k/Enter/d/r/Esc`; bỏ qua khi `isEditableTarget`; đăng ký qua registry của SOL-052 nếu có.
6. Trạng thái: rỗng "Không phát hiện vấn đề nào trong phạm vi index" (+ commit/ngày index), skeleton, lỗi inline + "Thử lại", `forbidden` nêu "Bạn không có quyền bỏ qua phát hiện".

## Kiểm thử

- `FindingDismissPopover` (lý do bắt buộc; `Mod+Enter`: giả lập `navigator.userAgent` Mac ⇒ `metaKey`, khác ⇒ `ctrlKey`); `FindingRow` (hành động khác nhau theo `origin`; "Xem trong đồ thị" ẩn khi không có đích); `FindingsPanel` (rỗng/lỗi/tải/`nextPageToken`); hoàn nguyên khi lỗi; nút khoá trong lúc chờ.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/findings`.

## Tiêu chí hoàn thành

- [ ] Các tiêu chí 5–9 của SOL-059 mục 5.
- [ ] Không có câu "an toàn"; không trộn `QualityFinding`.

## Rủi ro

- Xung đột phím `j/k` với "Thứ tự đọc" (SOL-052): chỉ kích hoạt khi tiêu điểm trong dock.

## Ghi chú triển khai (2026-10-07)

- `FindingsPanel` nhận props (`changedFiles`, `graphSymbolKeys`, `availableLensIds`, `onOpenDiff`, `onOpenFile`) và có thể gắn vào dock khi khung có; hành động "Ghi chú" dùng `ReviewNoteButton` (neo `finding`) khi finding có `evidence[0].path`.
- Thử lại trên dòng lặp lại đúng thao tác lỗi cuối (ignore/resolve/restore). `d` bấm nút "Ignore" của dòng đang chọn để mở popover.
- Danh sách > 50 dòng dùng cửa sổ ảo (phẳng); ≤ 50 nhóm theo kind.

## Ghi chú tích hợp (W6, 2026-10-07)

Dock đáy thu gọn được (mặc định đóng, không mount panel khi đóng) trong `shell/ReviewBottomDock.tsx`; nguồn đăng ký ở `shell/review-dock-registry.ts` (`registerReviewDockPanel`, cờ `requiresQuality`) và `shell/review-dock-builtin-panels.tsx` (Finding cấu trúc = `findings`, ghi chú = `notes`). `QualityFinding` của lens quality (W5-A) phải đăng ký panel riêng id khác qua registry — hai nguồn không gộp. Thêm `review_findings_summary` (một lần mỗi lần mount panel, chỉ đếm). Sai lệch: nút dock dùng `aria-pressed`, không dùng role `tab`, để không lẫn với tab lens (test cũ đếm 7 tab).
