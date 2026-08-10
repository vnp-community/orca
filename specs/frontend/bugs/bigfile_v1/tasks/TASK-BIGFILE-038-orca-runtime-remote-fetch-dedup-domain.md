# TASK-BIGFILE-038 — Move (composition): Remote fetch dedup/cache domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-008, 009
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3) · Sinh ra từ `TASK-BIGFILE-035`

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts`
- Đọc **đúng dòng 15,770–16,015** (KHÔNG đọc phần khác của file).
- Method cần chuyển (9 method, xác nhận lại khi đọc):
  `getCanonicalFetchKey`, `enqueueRemoteFetch`, `getFreshFetchCompletedAt`,
  `rememberFreshFetchCompletedAt`, `getOrStartRemoteFetch`,
  `getOrStartRemoteTrackingBaseRefresh`, `fetchRemoteWithCache`,
  `resolveRemoteTrackingBase`, `hasRemoteTrackingRef`
- Field private cần: `canonicalFetchKeyCache`, `fetchInflight`,
  `fetchLastCompletedAt`, `remoteFetchQueueTail`.
- Type liên quan: `RemoteFetchResult`, `RemoteTrackingBase` (đã tách ở
  TASK-009, `./orca-runtime-types`).

## Output

- File mới: `frontend/src/main/runtime/orca-runtime-remote-fetch-cache.ts`
  — class mới (ví dụ `RemoteFetchCacheDomain`) nhận dependency qua
  constructor (git provider/dispatch cần dùng để fetch thật — xác nhận cụ
  thể khi đọc code, khả năng cao cần `getRemoteFilesystemProvider` hoặc
  tương đương đã import sẵn ở đầu `orca-runtime.ts`).
- `orca-runtime.ts`: thêm field `private remoteFetchCache = new
  RemoteFetchCacheDomain({ ... })`, 9 method forward — GIỮ NGUYÊN chữ ký.

## Các bước

1. `gitnexus impact({target: "fetchRemoteWithCache", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 15,770–16,015, xác nhận method + field + dependency ngoài
   (git provider nào được dùng bên trong khối này).
3. Tạo `orca-runtime-remote-fetch-cache.ts`, copy nguyên văn, đổi
   `this.xxx` → `this.deps.xxx`.
4. Sửa `orca-runtime.ts`: thêm field, forward 9 method.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — giảm ~150-230 dòng
- [ ] Test git-remote/fetch liên quan (nếu có) pass

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-remote-fetch-cache.ts
```
