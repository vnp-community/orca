# FE-CV-TASK-058-04: `StorageLens`, canvas chỉ đọc, nút secret không lộ giá trị

**From Solution:** [FE-CV-SOL-058](../solutions/FE-CV-SOL-058-storage-lens.md) mục 2.3, 2.4, 2.6
**Priority:** P2
**Area:** frontend / renderer components
**File:** `frontend/src/renderer/src/components/review-map/storage/{StorageLens,StorageToolbar,StorageCanvas,StorageLegend,StorageInferenceNotice,StorageSecretNode}.tsx` (mới) + test
**Depends on:** FE-CV-TASK-058-01, 058-02, 058-03; FE-CV-SOL-051-review-workspace-shell
**Status:** [x] DONE

## Context

- Che hai lớp (backend + `maskSensitiveText`); không đường "hiện giá trị". STYLEGUIDE: không overclaim; màu chỉ cho trạng thái, kèm ký hiệu.

## Việc cần làm

1. `StorageLens` nối hook → view model → layout; trạng thái: tải theo ngưỡng (200 ms trì hoãn qua SSH, khoá điều khiển ngay), rỗng ("Không tìm thấy cấu hình lưu trữ trong repo này"), `warnings[]` từ backend (văn bản thuần), `stale`/`offline` do khung, lỗi inline + "Thử lại".
2. `StorageToolbar`: toggle `dev|prod`, công tắc "Hiện legacy", "Chỉ thành phần bị đổi + liên quan" (mặc định bật khi có nút bị đổi), lọc `kind` bằng `Badge`.
3. `StorageCanvas` (`@xyflow/react`, `nodesConnectable={false}`, `onlyRenderVisibleElements`), bốn `nodeTypes` (`storageService|storageStore|storageTopic|storageSecret`) khai báo ngoài render.
4. `StorageSecretNode`: chỉ tên khoá + đường dẫn Vault + nhãn "giá trị không bao giờ hiển thị"; nút "Sao chép tên khoá"; không prop nào mang giá trị; icon `ShieldAlert` + tooltip khi `masked`.
5. `StorageInferenceNotice`: nhãn "suy luận" theo `confidence` (persistent inline, không toast), kèm `redactedCount` ("N giá trị đã che ở backend").
6. `StorageLegend` dựng từ cùng bảng ký hiệu `+ ~ −`/`related`/nét đứt.

## Kiểm thử

- `StorageSecretNode`: dữ liệu giả có giá trị trong `payload`/`via` ⇒ DOM không chứa giá trị; không có nút hiện/sao chép giá trị.
- `StorageLens`: rỗng, `prod` thiếu dữ liệu + `warnings`, lỗi + thử lại, ẩn khi `unsupported`; canvas mock `@xyflow/react`.
- Spy `console` + kiểm `localStorage`: fixture secret giả không xuất hiện.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/storage`.

## Tiêu chí hoàn thành

- [ ] Các tiêu chí liên quan canvas/secret ở SOL-058 mục 5.
- [ ] Không hex; không `components/code-review/*`; không thêm thư viện; không `max-lines` disable.
- [ ] Reduced-motion tắt animation khung nhìn.

## Rủi ro

- Che quá tay có thể làm tên khoá khó đọc (kiểm khi tinh chỉnh mẫu ở 057-01).
