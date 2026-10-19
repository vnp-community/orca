# FE-REQ-TASK-041-03: Danh sách phiên bản: trạng thái, ghi chú, số bản; xem bản cũ ở chế độ chỉ đọc

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `renderer/src/components/request/solution/{SolutionVersionSwitcher.tsx,SolutionPanel.tsx}` (sửa), `solution-view-model.ts` (sửa nhỏ), test
**Depends on:** FE-REQ-TASK-041-01
**Status:** [ ] TODO

---

## Context

- `SolutionVersionSwitcher` hiện chỉ đánh dấu "Older version" và ẩn khi < 2 bản.
- Xem bản cũ phải loại bỏ mọi hành động quyết định.

## Việc cần làm

1. Mỗi mục: "Version n" (từ `seq`, dự phòng thứ tự `generatedAt`), ghi chú rút gọn 80 ký tự từ `revisionFeedback` hoặc "First proposal", thời điểm tương đối, huy hiệu trạng thái (Current, Superseded, Rejected, Approved) — huy hiệu có chữ, không chỉ màu.
2. Danh sách có giới hạn chiều cao, phân trang khi > 20, không lồng cuộn gây khó dùng bàn phím.
3. `SolutionPanel`: khi `viewId` khác bản hiện hành, chế độ chỉ đọc (ẩn Approve, Choose, Reject, Revise, Regenerate), dải "You're viewing an older version. Back to current".
4. Khối thu gọn "What was asked for this version" hiển thị `revisionFeedback` đầy đủ (văn bản thuần, không HTML; xuống dòng giữ nguyên).
5. Giữ hành vi hiện có khi chỉ 1 bản (ẩn).

## Kiểm thử

- Component: huy hiệu, ghi chú, dự phòng không có `seq`, phân trang, chế độ chỉ đọc, quay lại bản hiện hành, văn bản phản hồi chứa `<script>` hiển thị như chữ.
- Test `SolutionVersionSwitcher` hiện có xanh.

## Tiêu chí hoàn thành

- [ ] Bản cũ xem được nhưng không quyết định được.
- [ ] Phản hồi hiển thị an toàn (không diễn giải HTML).
- [ ] Hoạt động với số bản lớn.

## Rủi ro và lưu ý

- Số vòng không giới hạn: kiểm hiệu năng với 100 phiên bản giả.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
