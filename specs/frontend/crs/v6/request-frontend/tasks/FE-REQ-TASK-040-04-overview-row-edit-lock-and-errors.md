# FE-REQ-TASK-040-04: Dòng nhánh gốc ở Overview: xem, sửa, khoá, lỗi; hành động ở thẻ lỗi Execution

**From Solution:** [FE-REQ-SOL-040](../solutions/FE-REQ-SOL-040-request-base-branch-picker.md)
**Priority:** P1
**Area:** frontend / request
**File:** `components/request/{RequestBaseRefRow.tsx,RequestOverviewTab.tsx}` (sửa), `components/request/execution/ExecutionFailureCard.tsx` + `execution-failure-cause.ts` (sửa), test
**Depends on:** FE-REQ-TASK-040-01, 040-02
**Status:** [ ] TODO

---

## Context

- `request.setBaseRef` dùng `expectedVersion`; xung đột phải tải lại và hỏi (C12).
- Thẻ lỗi Execution hiển thị nguyên nhân từ `request.executionSummary` (`lastCause`).

## Việc cần làm

1. `RequestBaseRefRow` theo SOL-040 mục 2.4: giá trị, bút, ổ khoá + tooltip theo `baseRefLockReason`, cảnh báo `baseRefUnverified`.
2. Lưu bằng `callRequestRpc(SET_BASE_REF, {id, baseRef, expectedVersion})`; thành công cập nhật store và `version`.
3. Xử lý `REQUEST_VERSION_CONFLICT` (tải lại + hỏi), `REQUEST_BASE_REF_LOCKED` (toast trung tính, làm mới, bỏ nút sửa), `INVALID`/`NOT_FOUND` (dưới ô).
4. Quyền: ẩn bút nếu người xem không có quyền sửa (theo kiểu `ChangeTypeConfirmDialog`); nếu lỡ gọi, hiển thị lỗi quyền chuẩn.
5. Ẩn dòng khi `!supportsBaseRef` hoặc khi luồng của loại đó không có thực thi và không có Plan OpenSpec (`question`, `spike`) theo `request.flow`.
6. `execution-failure-cause.ts`: thêm nguyên nhân `base_ref_not_found` (nhãn, mô tả); `ExecutionFailureCard` hiện nút "Change starting branch" khi `!baseRefLocked`.
7. Sự kiện thời gian thực (`request.subscribe`) làm mới dòng này khi người khác đổi.

## Kiểm thử

- Component: các trạng thái sửa được / khoá (3 lý do) / chưa kiểm; từng mã lỗi; xung đột; ẩn theo loại và theo hỗ trợ.
- Thẻ lỗi: nút hiện đúng điều kiện.
- Test Overview hiện có xanh.

## Tiêu chí hoàn thành

- [ ] Xem, sửa, khoá và mọi lỗi hoạt động.
- [ ] Thẻ lỗi `base_ref_not_found` có hành động sửa khi chưa khoá.
- [ ] Cập nhật thời gian thực.

## Rủi ro và lưu ý

- Phối hợp backend: `base_ref_not_found` phải xuất hiện trong `lastCause` (TASK-REQ-039-03).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
