# TASK-BIGFILE-011 — Investigate+Move: `persistence-migration.ts`

**Loại:** Investigate+Move (đọc xác nhận ranh giới trước khi cắt) · **Effort:** M
**Phụ thuộc:** — (độc lập với TASK-BIGFILE-010, nên làm sau vì cùng file
nguồn — tránh conflict khi 2 task sửa cùng lúc)
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-009-persistence.md`

## Input

- File nguồn: `frontend/src/main/persistence.ts`
- Đọc **đúng dòng 484–2,648** (trước `export type StoreOptions` dòng 2,649).
- Symbol đã biết cần chuyển: `migrateMobilePairingDataToCanonicalUserDataPath`
  (dòng 484), `sanitizeOnboardingUpdate` (dòng 1,237).

## ⚠️ Ranh giới chưa chắc chắn — PHẢI xác nhận khi đọc

Khoảng cách 484→2,648 (2,165 dòng) cho chỉ 2 export đã biết là rất lớn — khả
năng cao có thêm hàm/type private (không export) nằm giữa 2 hàm này hoặc sau
`sanitizeOnboardingUpdate` mà bảng export top-level ban đầu không bắt được.
**Bước đầu tiên bắt buộc:** quét lại toàn bộ khoảng 484–2,648 để liệt kê ĐẦY
ĐỦ mọi `function`/`const`/`type` (kể cả không export) trước khi quyết định
đúng những gì thuộc "migration" và những gì có thể thuộc phần khác (đọc để
phân loại theo tên/mục đích, không đoán).

## Output

- File mới: `frontend/src/main/persistence-migration.ts` — chứa TOÀN BỘ nội
  dung đã xác nhận thuộc domain migration ở bước đọc trên (không chỉ 2 export
  đã biết).
- File nguồn thay bằng `export { ... } from './persistence-migration'` liệt
  kê đủ các symbol PUBLIC (export) đã chuyển. Symbol private không cần khai
  báo lại ở `persistence.ts` — chúng chỉ tồn tại nội bộ file mới.

## Các bước

1. Đọc dòng 484–2,648, liệt kê đầy đủ mọi định nghĩa (kể cả private).
2. `gitnexus impact` cho `migrateMobilePairingDataToCanonicalUserDataPath` và
   `sanitizeOnboardingUpdate` — dừng nếu risk HIGH/CRITICAL.
3. Xác nhận: các định nghĩa private trong khoảng này có được dùng ở ĐÂU
   KHÁC trong `persistence.ts` (vd trong class `Store` sau dòng 2,649) hay
   không — nếu CÓ dùng trong `Store`, KHÔNG chuyển hàm/type đó (nó thuộc
   ranh giới `persistence-store.ts`, không phải migration); ghi chú lại phát
   hiện này.
4. Copy nguyên văn phần đã xác nhận đúng là migration + import cần thiết.
5. Tạo `persistence-migration.ts`, paste.
6. Sửa `persistence.ts`: xoá phần đã chuyển, thêm barrel export cho các
   symbol public.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `persistence.ts` giảm
      tương ứng số dòng thực tế đã xác nhận ở bước 1 (không nhất thiết đúng
      2,165 — ước tính ban đầu có thể sai do chưa đọc kỹ)
- [ ] Chạy test migration hiện có + **test thủ công**: khởi động app với dữ
      liệu cũ (fixture nếu có), xác nhận migration vẫn chạy đúng

## Rollback

```
git checkout -- frontend/src/main/persistence.ts
rm frontend/src/main/persistence-migration.ts
```
