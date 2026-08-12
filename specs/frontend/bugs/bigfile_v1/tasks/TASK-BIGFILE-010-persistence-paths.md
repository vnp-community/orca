# TASK-BIGFILE-010 — Move: `persistence-paths.ts`

**Loại:** Move (cơ học) · **Effort:** S · **Phụ thuộc:** — · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-009-persistence.md`

## Kết quả thực thi (2026-08-12)

- Đọc lại thực tế dòng 337–483: đúng như task doc (không có drift số dòng
  ở vùng đầu file, khác với các file bigfile khác đã gặp trước đây).
  Nhưng phát hiện vùng này KHÔNG chỉ chứa 2 export đã biết — có 3 hàm
  private xen giữa (`getDataFile`, `getGithubCacheFile`,
  `gcStaleWorktreeMeta` + hằng số `WORKTREE_META_GC_GRACE_MS`,
  `readGithubCacheSnapshot`) không thuộc domain "paths" (chúng là
  github-cache-snapshot / worktree-meta-GC), việc đó không nằm trong kế
  hoạch gốc.
- Phát hiện quan trọng: `initDataPath`, `getDataFile`,
  `getCanonicalUserDataPath` dùng CHUNG 2 biến module-private `_dataFile`/
  `_userDataDir`. Nếu chỉ chuyển đúng 2 export như Output gốc mô tả (bỏ lại
  `getDataFile`), `getDataFile()` ở `persistence.ts` sẽ luôn rơi vào nhánh
  "safety fallback" (không set được `_dataFile` nữa vì `initDataPath` đã
  chuyển đi module khác) → lặng lẽ bỏ qua override `ORCA_DATA_DIR` mỗi lần
  gọi. Đây là lỗi thật, không phải lý thuyết.
- Xử lý: chuyển CẢ BỘ BA `initDataPath` + `getDataFile` +
  `getCanonicalUserDataPath` (giữ nguyên state module-private đi cùng
  nhau) sang `persistence-paths.ts`. `getDataFile` vẫn giữ **không phải
  API public** của `persistence.ts` (không đưa vào barrel export) — chỉ
  `import { getDataFile } from './persistence-paths'` để dùng nội bộ
  (`getGithubCacheFile`, `Store` constructor). 4 hàm còn lại
  (`getGithubCacheFile`, `WORKTREE_META_GC_GRACE_MS`,
  `gcStaleWorktreeMeta`, `readGithubCacheSnapshot`) ở nguyên
  `persistence.ts` — không dùng chung state, chỉ gọi `getDataFile()` như
  một hàm import bình thường.
- Dọn thêm import top-level bị unused sau khi `app` (electron) không còn
  được dùng làm value trong `persistence.ts` (chỉ còn `safeStorage`).
- `persistence.ts`: 6,659 → **6,461 dòng** sau cả TASK-010 + TASK-011
  (xem chi tiết riêng ở TASK-011). File mới `persistence-paths.ts`: 59
  dòng.
- Xác minh: `npx tsc --noEmit` với tsconfig tạm (scope
  `frontend/src/main/**` + `shared/**`, dọn sau khi xong) — 0 lỗi mới
  trong `persistence.ts`/`persistence-paths.ts` (65 lỗi pre-existing
  không liên quan ở các file khác, không đổi trước/sau). `oxlint` trên 2
  file: exit 0, sạch.
- `gitnexus impact` không dùng được (MCP server báo "Connection closed"
  nhiều lần) — thay bằng grep thủ công toàn repo (`frontend/src`) cho
  `initDataPath`/`getCanonicalUserDataPath`: không có importer nào ngoài
  `persistence.ts` chính nó và `ssh-remote-cli-host-passthrough.ts` (dùng
  qua `getCanonicalUserDataPath` từ `../persistence` — vẫn hoạt động nhờ
  barrel export).

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
