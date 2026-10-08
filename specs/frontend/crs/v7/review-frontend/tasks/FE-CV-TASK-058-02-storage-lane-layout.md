# FE-CV-TASK-058-02: Bố cục bốn làn cố định (service, kho, topic, secret)

**From Solution:** [FE-CV-SOL-058](../solutions/FE-CV-SOL-058-storage-lens.md) mục 2.3
**Priority:** P2
**Area:** frontend / renderer (hàm thuần)
**File:** `frontend/src/renderer/src/components/review-map/storage/storage-layout.ts` (mới) + `storage-layout.test.ts`
**Depends on:** FE-CV-TASK-058-01
**Status:** [x] DONE (verified 2026-10-07: storage/storage-pure.test.ts 15/15 PASS, tsc/oxlint sạch)

## Context

- O5: không thêm thư viện bố cục; vài chục nút nên bốn làn cố định + một lượt barycenter là đủ (chưa đo).

## Việc cần làm

1. `layoutStorageLanes({nodes, edges}) → {positions: Map<id,{x,y,width,height}>, laneBounds}`: x cố định theo làn (`service` < `store` < `topic` < `secret`), trong làn sắp theo `name` (tất định) rồi một lượt barycenter theo hàng xóm làn trái; khoảng cách dòng cố định; làn rỗng không chiếm chỗ.
2. Hỗ trợ `filter` (tập id hiển thị) để chế độ "chỉ đổi + liên quan" tính lại bố cục gọn.
3. Không timer, không animation.

## Kiểm thử

- Tất định (cùng input ⇒ cùng output); không chồng nút; làn rỗng bỏ qua; 60 nút không ném; barycenter giảm số cạnh cắt so với thứ tự tên trong một fixture cố định.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/storage/storage-layout`.

## Tiêu chí hoàn thành

- [ ] Hàm thuần, test xanh, không phụ thuộc DOM.
- [ ] Không thêm dependency.

## Rủi ro

- Số cạnh dày (nhiều service dùng chung một kho) vẫn có thể cắt nhau; chấp nhận ở MVP.

## Ghi chú triển khai (2026-10-07)

- Tạo `storage/storage-layout.ts` (`layoutStorageLanes`); hỗ trợ `filter`, làn rỗng không chiếm chỗ, barycenter theo hàng xóm ở làn service. Test nằm chung trong `storage-pure.test.ts`.
