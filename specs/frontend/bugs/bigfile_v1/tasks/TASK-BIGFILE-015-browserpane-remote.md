# TASK-BIGFILE-015 — Move: `browser-pane-remote.tsx`

**Loại:** Move (cơ học) · **Effort:** M · **Phụ thuộc:** — · **Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-010-browserpane.md`

## Input

- File nguồn: `frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx`
- Đọc **2 khối riêng biệt**: dòng 185–307 (11 type `Remote*`/`Pending*`) và
  dòng 901–2,674 (component `RemoteBrowserPagePane`) — copy CẢ HAI vào CÙNG
  1 file mới (chúng liên quan trực tiếp, theo đúng tiền tố tên `Remote*`).
- Symbol cần chuyển:
  - Type (185–307): `BrowserTabPageState`, `BrowserDownloadState`,
    `GrabIntent`, `BrowserOverlayAnchor`, `BrowserOverlayViewport`,
    `RemoteBrowserStreamToken`, `RemoteBrowserStreamSubscription`,
    `RemoteBrowserOperationToken`, `RemoteBrowserContextMenu`,
    `RemoteBrowserViewportSize`, `RemoteBrowserImagePoint`,
    `PendingRemoteBrowserWheel`
  - Component (901–2,674): `RemoteBrowserPagePane`

## Output

- File mới: `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx`
- File nguồn import `RemoteBrowserPagePane` (+ bất kỳ type nào file nguồn còn
  cần dùng trực tiếp) từ file mới.

## Các bước

1. `gitnexus impact({target: "RemoteBrowserPagePane", direction: "upstream"})`
   — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 185–307, copy nguyên văn 11 type.
3. Đọc dòng 901–2,674, copy nguyên văn `RemoteBrowserPagePane` + import cần
   thiết.
4. Tạo file mới, paste CẢ 2 khối (type trước, component sau) + `export` cho
   những gì `BrowserPane.tsx` (file gốc) còn cần dùng trực tiếp.
5. Sửa `BrowserPane.tsx`: xoá cả 2 khối gốc, thêm import từ file mới.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `BrowserPane.tsx` giảm
      ~1,900 dòng
- [ ] Test liên quan (CDP streaming / remote browser) pass

## Rollback

```
git checkout -- frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx
rm frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx
```
