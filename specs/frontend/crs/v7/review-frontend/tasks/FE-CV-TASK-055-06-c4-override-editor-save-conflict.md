# FE-CV-TASK-055-06: `C4OverrideEditor` (soạn, lưu, xung đột)

**From Solution:** [FE-CV-SOL-055-architecture-c4-lens](../solutions/FE-CV-SOL-055-architecture-c4-lens.md) mục 4.5
**Priority:** P1
**Area:** frontend / review-map
**File:** `C4OverrideEditor.tsx` (mới), `hooks/useC4Override.ts` (mới), tests
**Depends on:** FE-CV-TASK-055-05, 055-02, FE-CV-TASK-050-07
**Status:** [ ] TODO

## Context

- `c4.get {container}` ⇒ `{container, document, version, updatedBy, updatedAt, seedSource?}`; `c4.save {container, document, expectedVersion}` ⇒ `{version, warnings[]}`; `useConfirmationDialog` (`components/confirmation-dialog.tsx`, **đọc chữ ký**); `ui/sheet` ≥ 560 px; `lib/screen-submit-shortcut.ts` (không bắt buộc).

## Việc cần làm

1. Tải `c4.get`; không có bản ghi ⇒ `version:0`, nháp rỗng + chú thích; hiển thị `seedSource` nếu có.
2. Kiểm tra cục bộ (055-05), danh sách vấn đề bấm đặt con trỏ; "Lưu" khoá khi lỗi/không đổi/đang lưu, khoá ngay khi bấm; thang thời lượng (nhãn sau 1 s, giai đoạn sau 3 s).
3. Lỗi: `VERSION_CONFLICT` ⇒ ba hành động (Tải bản mới, Ghi đè bản của tôi `destructive`, Sao chép); `INVALID_PARAMS {field}` ⇒ danh sách vấn đề; `PAYLOAD_TOO_LARGE`; `NOT_AUTHORIZED` ⇒ chỉ đọc; offline/timeout ⇒ giữ nháp "Chưa lưu".
4. Thành công: "Đã lưu lúc {giờ}" (không toast), hiển thị `warnings`, vô hiệu cache `architecture` và tải lại.
5. Nháp trong `c4Drafts` (≤ 8, ≤ 64 KiB); đóng khi có thay đổi hỏi xác nhận; không chặn phím `Tab`.

## Kiểm thử

- Nạp, gõ, lưu, conflict ba hành động, forbidden, offline, đóng có thay đổi, giới hạn 8 nháp.

## Tiêu chí hoàn thành

- [ ] Không khẳng định "đã lưu" trước khi `save` thành công.

## Rủi ro

- Ngữ nghĩa YAML (đổi tên/gộp/ẩn) chưa có schema (O-12).
