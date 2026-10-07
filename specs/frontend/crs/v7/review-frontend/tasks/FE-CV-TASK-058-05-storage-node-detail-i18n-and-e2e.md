# FE-CV-TASK-058-05: Chi tiết nút Lưu trữ, liên kết ERD, i18n và e2e web

**From Solution:** [FE-CV-SOL-058](../solutions/FE-CV-SOL-058-storage-lens.md) mục 2.5, 2.6, 6
**Priority:** P2
**Area:** frontend / renderer components + i18n + tests
**File:** `frontend/src/renderer/src/components/review-map/storage/StorageNodeDetail.tsx` (mới) + test; `i18n/locales/{en,es,ja,ko,zh}.json`; `i18n/code-intel-locale-coverage.test.ts` (thêm `KEYS`); `tests/e2e/code-intel-web/lenses.web.e2e.ts` (phần Storage)
**Depends on:** FE-CV-TASK-058-04; FE-CV-TASK-057-04 (`setErdService`); FE-CV-SOL-053-impact-lens-and-symbol-detail; FE-CV-TASK-073-02, 073-03
**Status:** [x] DONE

## Context

- Kho có `owner.name` ⇒ "Mở ERD của service" dùng `setErdService`; "Xem diff" chỉ khi `evidence[].path` ∈ `changedFiles` (mở diff theo SOL-053).

## Việc cần làm

1. `StorageNodeDetail`: `kind`, `env`, `owner`, `schemas`, binding vào/ra (`rw|ro`, `via` đã che), publisher/subscriber, `confidence` ("suy luận"), `deployed|supportedByCode|external`, `evidence` (Xem diff/Mở tệp); nút "Mở ERD của service"; topic → bấm service chọn nút trong canvas.
2. Dịch khoá `auto.components.reviewMap.Storage*` sang 4 locale; thêm vào `KEYS`.
3. e2e (fake backend G4 + `lenses.web.e2e.ts`): dev/prod, nút secret không có giá trị, "Mở ERD của service" chuyển lens, canary DSN giả không xuất hiện trong DOM; lens ẩn khi fake backend trả `CODEINTEL_UNAVAILABLE` cho `storage`.
4. Test rò rỉ khoá `storage*` (nếu chưa phủ ở 058-03).

## Kiểm thử

- Testing Library: nút ERD chỉ khi có `owner.name` và `kind` postgres/mysql; "Xem diff" bị khoá khi tệp không đổi; topic liệt kê pub/sub.
- `pnpm --filter orca-frontend test -- src/renderer/src/i18n/code-intel-locale-coverage src/renderer/src/components/review-map/storage`; e2e **chưa chạy**.

## Tiêu chí hoàn thành

- [ ] 5 locale đủ khoá; `no-top-level-translate.test.ts` xanh.
- [ ] e2e Storage xanh trên fake backend khi 073-03 sẵn sàng.

## Rủi ro

- Phụ thuộc SOL-053 cho mở diff; thiếu thì nút ẩn.
