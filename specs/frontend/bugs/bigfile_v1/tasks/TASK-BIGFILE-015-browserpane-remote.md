# TASK-BIGFILE-015 — Move: `browser-pane-remote.tsx`

**Loại:** Move (cơ học) · **Effort:** M · **Phụ thuộc:** — · **Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-010-browserpane.md`

## ⚠️ Kết quả thực thi (2026-08-12)

Đã đọc lại file thực tế thay vì tin số dòng doc gốc (cùng nguyên tắc như
TASK-014). Kết quả khớp gần đúng nhưng có sai lệch quan trọng:

- 12 type (không phải 11 — doc đếm thiếu 1) khớp đúng list tên, nhưng đoạn
  185–307 xen kẽ với 3 hàm không thuộc nhóm này
  (`formatBrowserDownloadProgress`, `decodeRemoteBrowserFrameUrl`,
  `getBrowserPageRuntimeEnvironmentId`) — chỉ 12 type được chuyển, 3 hàm ở
  lại `BrowserPane.tsx` (2 hàm sau cùng export thêm để file mới import lại
  vì vẫn cần dùng).
- `RemoteBrowserPagePane` thực tế kết thúc ở dòng ~2,486 (không phải 2,674);
  2,488–2,500 là `preventAgentSendTargetOutsideDismiss` — helper riêng,
  grep xác nhận **chỉ** `BrowserPagePane` dùng (không phải Remote) → để lại
  cho TASK-016 di chuyển, không đưa vào file này.
- Phát hiện quan trọng: nhiều trong 12 type + phần lớn hàm phụ trợ Remote-ish
  (`toDisplayUrl`, `getBrowserDisplayTitle`, `isRemoteBrowserPageMissingError`,
  `readRemoteCssViewportSize`, v.v.) được dùng chéo bởi các hàm PHỤ TRỢ vẫn ở
  lại `BrowserPane.tsx` (không phải component) và/hoặc bởi `BrowserPagePane`
  (TASK-016) — đây là hàm/type THUẦN (pure), không phải state module-private
  như trường hợp `ipc/pty.ts`, nên an toàn để export + import chéo (không bị
  "blocked"). `BrowserPane.tsx` giữ các hàm này, export thêm, và
  `browser-pane-remote.tsx` import ngược lại từ `./BrowserPane` — đúng như
  solution doc dự kiến ("BrowserPane.tsx ... export {...} cho 3 file trên").
- `WHEEL_DELTA_LINE`/`WHEEL_DELTA_PAGE` (const riêng, chỉ Remote dùng) di
  chuyển cùng thay vì để lại mồ côi.
- `gitnexus impact`/`detect_changes` không dùng được (MCP "Connection
  closed", CLI segfault) — thay bằng grep thủ công đối chiếu toàn bộ usage
  map của từng symbol trước khi di chuyển.

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
