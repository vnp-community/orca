# TASK-BIGFILE-010 — Move: `persistence-paths.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-009-persistence.md`

## Input

- File nguồn: `frontend/src/main/persistence.ts`
- Đọc **đúng dòng 337–483**.
- Symbol cần chuyển: `initDataPath`, `getCanonicalUserDataPath`

## Output

- File mới: `frontend/src/main/persistence-paths.ts`
- File nguồn thay bằng:
  ```ts
  export { initDataPath, getCanonicalUserDataPath } from './persistence-paths'
  ```

## Các bước

1. `gitnexus impact({target: "initDataPath", direction: "upstream"})` +
   tương tự cho `getCanonicalUserDataPath` — dừng nếu risk HIGH/CRITICAL
   (đây là persistence trung tâm, kiểm tra kỹ hơn bình thường).
2. Đọc dòng 337–483, copy nguyên văn + import cần thiết.
3. Tạo file mới, paste. Sửa `persistence.ts` thành barrel export.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `persistence.ts` giảm
      ~150 dòng
- [ ] Chạy test persistence hiện có (không chỉ typecheck — đây là lớp ảnh
      hưởng dữ liệu người dùng)

## Rollback

```
git checkout -- frontend/src/main/persistence.ts
rm frontend/src/main/persistence-paths.ts
```
