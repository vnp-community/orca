# FE-CV-TASK-053-04: Dữ liệu và `SymbolDetailPanel`

**From Solution:** [FE-CV-SOL-053-impact-lens-and-symbol-detail](../solutions/FE-CV-SOL-053-impact-lens-and-symbol-detail.md) mục 4.4
**Priority:** P0
**Area:** frontend / review-map
**File:** `SymbolDetailPanel.tsx`, `SymbolRelationList.tsx`, `SymbolCoveringTests.tsx`, `SymbolRelatedFlows.tsx`, `SymbolSourcePreview.tsx`, `hooks/useSymbolDetail.ts` (mới), tests
**Depends on:** FE-CV-TASK-050-13, 053-01, 051-05
**Status:** [x] DONE

## Context

- `symbol {key | name+file, includeSource}`; `includeSource` mặc định **true** ⇒ luôn gửi `false` cho phần đầu; `SymbolDetail.incoming/outgoing` keyed theo kind; `sourceOmitted`; quyền `read_source`; ≤ 320 KiB; U9.

## Việc cần làm

1. `useSymbolDetail(worktreeId, ref)`: `symbol {includeSource:false}`; `useSymbolSource` tải khi mở mục (`includeSource:true`), huỷ/bỏ khỏi cache khi đóng panel.
2. Panel: đầu, hành động ("Xem diff", "Đặt làm trung tâm", khe `SymbolDetailActionsSlot` cho SOL-060), gọi bởi/gọi tới (gộp theo kind, ≤ 20 + xem thêm; bấm ⇒ chọn symbol theo `{name,file}`), luồng liên quan, test phủ (lười, `impact includeTests`), mã nguồn (`<pre><code>` có số dòng, ≤ 200 dòng, không tô cú pháp; `sourceOmitted` ⇒ nhãn riêng).
3. Lỗi từng mục riêng + Thử lại; `NOT_AUTHORIZED` ⇒ ẩn mục mã; `SYMBOL_NOT_FOUND` ⇒ "Không tìm thấy symbol này trong index; có thể index cũ"; `ambiguous` ⇒ hộp chọn; `Esc` đóng, trả tiêu điểm.
4. Hiển thị mã nguồn/chuỗi tự do qua `maskSensitiveText` của FE-CV-TASK-057-01 (dùng lại, không tạo bản mới).
5. Dòng "Số dòng theo index tại {commit}; có thể lệch" khi index cũ/`OVERLAY`.

## Kiểm thử

- Một lời gọi `symbol` khi mở; mã chỉ khi mở mục; lỗi từng mục không đóng panel; `sourceOmitted` các giá trị.

## Tiêu chí hoàn thành

- [ ] Nội dung mã không bị ghi log hay giữ lâu; test xanh.

## Rủi ro

- `incoming/outgoing` không có `key`/dòng: điều hướng qua `{name,file}` có thể mơ hồ.
