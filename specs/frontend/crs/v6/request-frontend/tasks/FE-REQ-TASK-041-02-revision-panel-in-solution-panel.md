# FE-REQ-TASK-041-02: `AnalysisRevisionPanel` trong `SolutionPanel`

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `renderer/src/components/request/solution/{AnalysisRevisionPanel.tsx,SolutionPanel.tsx}` (sửa), test
**Depends on:** FE-REQ-TASK-041-01
**Status:** [ ] TODO

---

## Context

- `SolutionPanel` dựng thanh quyết định và danh sách phiên bản; `ordered` là danh sách phiên bản, `viewId` chọn bản xem.
- Panel chỉ có ý nghĩa khi `awaiting_analysis_approval` và xem bản hiện hành.

## Việc cần làm

1. `AnalysisRevisionPanel`: textarea, bộ đếm, chip, nút "Revise analysis", trạng thái "Analyzing version n+1…" khi `busy`, thông báo lỗi dưới ô; `aria-live` cho trạng thái; nhãn truy cập đầy đủ.
2. `SolutionPanel`: chèn panel cạnh `SolutionDecisionBar`; khi `busy` khoá Approve, Choose, Reject, Regenerate, Revise (truyền cờ vào `SolutionDecisionBar`).
3. Ẩn theo loại không có cổng Analysis (`request.flow`) và khi xem bản cũ.
4. Ctrl/Cmd+Enter gửi (theo nền tảng bằng kiểm tra runtime, nhãn `⌘`/`Ctrl+`), không phím tắt nào khác.
5. Sau thành công, xoá ô và chuyển xem sang bản mới khi nó xuất hiện.
6. Dùng token `main.css`, primitive `Textarea`, `Button`; không màu cứng.

## Kiểm thử

- Component: các trạng thái (rỗng, đếm, chip chèn câu, đang chạy, lỗi, ẩn theo loại/quyền/bản cũ); khoá nút khi `busy`; phím tắt theo nền tảng.
- Test `SolutionPanel`/`SolutionDecisionBar` hiện có xanh.

## Tiêu chí hoàn thành

- [ ] Chỉnh bằng một thao tác, không qua từ chối.
- [ ] Nút quyết định bị khoá trong lúc chạy.
- [ ] Đúng truy cập bàn phím và đọc màn hình.

## Rủi ro và lưu ý

- `SolutionPanel.tsx` đã dài: tách nếu cần để dưới `max-lines`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
