# TASK-BIGFILE-038 — Move (composition): Remote fetch dedup/cache domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** S · **Phụ thuộc:** TASK-BIGFILE-008, 009
**Status:** ✅ Done

## Kết quả thực thi (2026-08-10)

- Domain này hoàn toàn tự chứa (own state, không phụ thuộc field/method
  nào khác của `OrcaRuntimeService`) — xác nhận qua grep trước khi tách,
  nên KHÔNG cần host interface như TASK-036; class mới
  `RuntimeRemoteFetchCache` không có tham số constructor, an toàn để
  khởi tạo eager làm field-level (`new RuntimeRemoteFetchCache()`), không
  gặp lỗi "used before initialization" như TASK-036.
- Cả 6 public method ĐỀU được gọi từ nơi khác trong class (dòng
  14554–16036, ngoài vùng 15588–15825 đã tách) — vẫn cần forward field ở
  `OrcaRuntimeService` (không xoá hẳn API cũ).
- Di chuyển kèm 3 const + 1 hàm helper (`FETCH_FRESHNESS_MS`,
  `REMOTE_FETCH_TIMEOUT_MS`, `REMOTE_FETCH_CACHE_MAX`,
  `setBoundedMapEntry`) — chỉ dùng riêng trong domain này, xác nhận qua
  grep trước khi xoá khỏi `orca-runtime.ts`. Giữ lại
  `DRIFT_PROBE_SUBJECT_LIMIT` (đứng cạnh nhưng dùng ở method khác).
- `orca-runtime.ts`: 24,553 → **24,274 dòng** (giảm ~279 dòng). File mới:
  305 dòng.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không
  đổi (0 lỗi mới), `oxlint` sạch (exit 0) cả 2 config.
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
