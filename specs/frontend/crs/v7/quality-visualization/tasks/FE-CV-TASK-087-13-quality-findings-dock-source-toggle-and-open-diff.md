# FE-CV-TASK-087-13: Ô nguồn "Cấu trúc | Kiểm tra" trong dock và liên kết mở diff

**From Solution:** [FE-CV-SOL-087-quality-diff-annotations](../solutions/FE-CV-SOL-087-quality-diff-annotations.md) mục 2.4
**Priority:** P1
**Area:** frontend / review shell
**File:** sửa nhỏ `FindingsToolbar`/dock của FE-CV-SOL-059; `frontend/src/renderer/src/components/review-map/quality/findings/QualityFindingsDockSource.tsx` (mới); liên kết từ lens ("Xem {n} phát hiện") và từ `QualityGateReasonRow`
**Depends on:** 087-12, 087-07; FE-CV-SOL-059, FE-CV-SOL-053
**Status:** [ ] TODO

## Context

- PQ-06: một dock, hai nguồn, không trộn; thang/hành động/khoá khác (`Finding` bỏ qua/đã xử lý theo `findingKey`; `QualityFinding` miễn trừ theo `fingerprint`).
- `openReviewDiffAtSymbol` (053) nhận `SymbolRef`, phát hiện chỉ có `(file,line)`: cần lõi `openReviewDiffAtPath` hoặc dùng `openAnnotationLocation` (đã có `components/editor/check-annotation-open.ts`, mẫu `CheckRunAnnotations.tsx`).
- FE-CV-SOL-059/053 do agent khác soạn: tham chiếu theo tên, đối chiếu khi chúng ra.

## Việc cần làm

1. `ToggleGroup` nguồn ở đầu `FindingsToolbar`; chọn "Kiểm tra" gắn `QualityFindingsList`; số đếm từng nguồn riêng.
2. "Xem diff": `openReviewDiffAtPath` nếu 053 cung cấp, nếu không `openAnnotationLocation`; ngoài thay đổi → "Mở tệp"; đường dẫn qua `resolveAnnotationPathInsideWorktree`.
3. Nhận `selectedFingerprint` (từ bấm glyph) → cuộn và làm nổi dòng; nhận bộ lọc từ `QualityGateReasonRow` (`category`/`tool`).
4. Phím tắt chỉ khi registry chung đã chốt (câu hỏi mở 2); mặc định: `Enter` xem diff.

## Kiểm thử

Chuyển nguồn giữ bộ lọc từng nguồn; số đếm tách; `selectedFingerprint` cuộn đúng; đường dẫn `..` bị chặn. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/quality/findings/QualityFindingsDockSource`.

## Tiêu chí hoàn thành

- [ ] Không trộn hai danh sách.
- [ ] "Xem diff" mở đúng dòng hoặc mở tệp.

## Rủi ro

- Khoá/hành vi `FindingsToolbar` của 059 có thể đổi: phối hợp khi 059 triển khai.
