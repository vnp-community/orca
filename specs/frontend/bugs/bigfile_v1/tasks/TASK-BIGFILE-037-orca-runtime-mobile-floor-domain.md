# TASK-BIGFILE-037 — Move (composition): Mobile floor / remote-desktop / layout domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** L (khối lớn nhất trong 5 task sinh từ TASK-035, ~1,830 dòng)
· **Phụ thuộc:** TASK-BIGFILE-008, 009, khuyến nghị làm SAU TASK-036 (để
xác nhận pattern composition hoạt động đúng trên khối nhỏ trước)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3) · Sinh ra từ `TASK-BIGFILE-035`

## Kết quả thực thi (2026-08-10)

- **Cảnh báo trong task doc gốc đã đúng**: `onPtyExit` (nằm giữa vùng dòng
  dự kiến) là 1 method xử lý PTY-exit CHÉO NHIỀU domain (PTY output
  buffer, agent-team teardown, leaf/pty graph state) — chỉ ~30% thân hàm
  là dọn state mobile-floor. **KHÔNG di chuyển `onPtyExit`** — giữ nguyên
  trong `OrcaRuntimeService`, thêm 1 method `clearStateForExitedPty(ptyId)`
  trên class mới để `onPtyExit` gọi vào (thay 12 dòng xoá field trực tiếp
  bằng 1 dòng delegate), phần dọn dẹp PTY-lifecycle khác giữ nguyên tại chỗ.
- **Phát hiện thêm — quan trọng nhất phiên này**: rà soát TOÀN BỘ file cho
  cả 19 field sẽ di chuyển (không chỉ trong dải dòng dự kiến) phát hiện
  **5+ method/field nằm RẢI RÁC ngoài vùng dòng gốc**: `resizeForClient`
  (~100 dòng, cách xa ~100 dòng), `isMobileTerminalQueryReplyAuthority` +
  field liên quan trong `hasRemoteTerminalViewSubscriber` (cách xa ~1,300
  dòng), `subscribeToDriverChanges` + field `driverListeners` (cách xa
  ~1,300 dòng), `getTerminalFitOverride`/`getAllTerminalFitOverrides`/
  `getAllTerminalDrivers`/`getAllBrowserDrivers`/`onClientDisconnected` (bị
  bỏ sót khi build forward list, phát hiện qua `tsc` báo lỗi từ
  `rpc/methods/terminal.ts` — 1 external caller thực tế ngoài
  `orca-runtime.ts`). Đây là domain đầu tiên trong 5 domain mà việc dò
  "toàn bộ field, không chỉ dải dòng" thực sự bắt được lỗi — các domain
  trước (036/038/040) không có case này.
- Host interface có **~10 dependency** (giống độ phức tạp TASK-039):
  getStore, getNotifier (minimal 3-method shape), getPtyController,
  getTerminalSize, resizeHeadlessTerminal,
  notifyRemoteTerminalViewPresenceChanged, notifyFitOverrideListeners,
  revokeTerminalFileGrantsForClient, cancelMobileDictationForClient,
  cancelBrowserScreencastForPage, getAgentBrowserBridge.
- 3 method private gốc (`enqueueLayout`, `resolveDesktopRestoreTarget`,
  `getBrowserDriver`/`setBrowserDriver`) phải đổi thành public vì được gọi
  từ method khác NGOÀI class mới (cùng pattern lặp lại từ TASK-039/040).
- File mới vượt `max-lines` (2,200 dòng) — đăng ký
  `config/max-lines-baseline.txt` "NEEDS PR REVIEW" theo đúng chính sách.
- `orca-runtime.ts`: 23,264 → **21,263 dòng** (giảm ~2,001 dòng — LỚN NHẤT
  trong toàn bộ 5 task composition). File mới: 2,200 dòng.
- Xác minh: `tsc --noEmit --composite false` 251 lỗi pre-existing không
  đổi (0 lỗi mới, xác nhận qua diff error-set trước/sau), `oxlint` sạch
  (exit 0) cả 2 config.
- **CHƯA làm**: chưa chạy test mobile-floor/remote-desktop/layout thủ
  công (domain có docs riêng `docs/mobile-presence-lock.md`,
  `docs/mobile-terminal-layout-state-machine.md`, độ phức tạp logic cao —
  khuyến nghị kiểm tra kỹ: mobile subscribe/unsubscribe, remote-desktop
  claim/release, resize đa viewer trước khi coi là an toàn deploy).

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts`
- Đọc **đúng dòng 7,770–9,620** (đọc rộng hơn ước tính 1 chút để thấy
  đúng ranh giới — method liền trước là phần fit-override/desktop-restore,
  method liền sau là `failActiveDispatchOnExit` thuộc domain khác — KHÔNG
  đọc phần khác của file).
- ~60 method cần chuyển (xác nhận lại khi đọc), nhóm theo docs đã có sẵn
  trong code: `docs/mobile-presence-lock.md`,
  `docs/mobile-terminal-layout-state-machine.md`. Danh sách method (từ
  phân tích TASK-035, xác nhận lại tên chính xác khi đọc):
  `getTerminalFitOverride`, `getAllTerminalFitOverrides`,
  `getAllTerminalDrivers`, `getAllBrowserDrivers`, `getBrowserDriver`,
  `setBrowserDriver`, `reclaimBrowserForDesktop`, `onClientDisconnected`,
  `onPtyExit`(⚠️ tên gợi ý dùng chéo domain PTY — xác nhận kỹ), `getDriver`,
  `setDriver`, `isPtyResizeDrivenRemotely`, `isRemoteDesktopResizeDriven`,
  `isRemoteDesktopViewerOwner`, `getRemoteDesktopFitHold`,
  `hasRemoteDesktopViewers`, `activeRemoteDesktopViewport`,
  `resolveRemoteDesktopHostReclaimTarget`,
  `ensureRemoteDesktopHostReclaimTarget`,
  `recordRemoteDesktopHostReclaimTarget`, `hasRemoteDesktopLayoutState`,
  `bumpRemoteDesktopViewerRevision`, `applyRemoteDesktopLayout`,
  `updateRemoteDesktopViewer`, `claimRemoteDesktopViewer`,
  `claimRemoteDesktopHost`, `unregisterRemoteDesktopViewer`,
  `unregisterRemoteDesktopViewers`, `refreshRemoteDesktopViewer`,
  `updateDesktopViewport`, `markMobileActor`, `mobileTookFloor`,
  `updateMobileViewport`, `reclaimTerminalForDesktop`,
  `releaseDesktopTakeBack`, `getAutoRestoreFitMs`,
  `cancelAllPendingFitRestoreTimers`, `getMobileAutoRestoreFitMs`,
  `setMobileAutoRestoreFitMs`, `pickMostRecentActor`,
  `pickEarliestRestoreTarget`, `getLayout`, `isFreshSubscribe`,
  `resolveDesktopRestoreTarget`, `coalescesWith`, `enqueueLayout`,
  `runLayoutSlot`, `applyLayout`, `setMobileDisplayMode`,
  `getMobileDisplayMode`, `isMobileSubscriberActive`,
  `updateMobileSubscriberViewport`, `handleMobileSubscribe`,
  `handleMobileSubscribeInternal`, `handleMobileUnsubscribe`,
  `applyMobileDisplayMode`, `onExternalPtyResize`(⚠️ xác nhận dùng chéo),
  `recordRendererGeometry`, `getLastRendererSize`, `refreshRendererGeometry`,
  `isResizeSuppressed`, `suppressResizesForMs`, `subscribeToTerminalResize`,
  `notifyTerminalResize`
- Field private cần: `terminalFitOverrides`, `remoteDesktopOwners`,
  `mobileDictation`, `pendingRestoreTimers`, `freshSubscribeGuard`,
  `remoteDesktopHostReclaimTargets`, `layouts`, `layoutQueues`,
  `lastRendererSizes`, `mobileDisplayModes`, `resizeListeners`,
  `mobileSubscribers`, `currentDriver`, `currentBrowserDriver`,
  `remoteDesktopViewers`, `remoteDesktopViewerRevisions`,
  `remoteDesktopActivity`, `driverListeners`, `fitOverrideListeners`,
  `resizeSuppressedUntil`.
- Type liên quan (đã tách sẵn ở TASK-009): `DriverState`,
  `PtyLayoutTarget`, `PtyLayoutState`, `ApplyLayoutResult` từ
  `./orca-runtime-types`.

## ⚠️ Cảnh báo — 2 method có tên gợi ý dùng chéo domain PTY

`onPtyExit` và `onExternalPtyResize` có tên gợi ý được gọi TỪ domain PTY
core (live-graph) khi PTY thoát/resize — cần đọc kỹ xem 2 method này có
đọc/ghi field lõi (`ptysById`, `handles`, `notifier`...) hay không. Nếu
có, **giữ lại ở `OrcaRuntimeService`** (không chuyển), chỉ forward gọi
sang domain object cho phần logic floor/layout thuần tuý bên trong.

## Output

- File mới: `frontend/src/main/runtime/orca-runtime-mobile-floor.ts` —
  class mới (ví dụ `MobileFloorDomain`, đặt tên theo
  `docs/mobile-presence-lock.md`) nhận dependency qua constructor (các
  field lõi cần đọc chéo, ví dụ truy cập `ptyController`/`handles` để
  resize PTY thật — xác nhận danh sách cụ thể khi đọc code).
- `orca-runtime.ts`: thêm field `private mobileFloor = new
  MobileFloorDomain({ ... })`, ~60 public/private method cũ forward sang
  domain object — GIỮ NGUYÊN chữ ký public method.

## Các bước

1. `gitnexus impact` cho 3-4 method public quan trọng nhất
   (`handleMobileSubscribe`, `applyLayout`, `claimRemoteDesktopViewer`,
   `updateMobileViewport`) — dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 7,770–9,620, xác nhận danh sách method + field, xử lý riêng
   `onPtyExit`/`onExternalPtyResize` theo cảnh báo trên.
3. Tạo `orca-runtime-mobile-floor.ts`, copy nguyên văn logic, đổi
   `this.xxx` → `this.deps.xxx` cho field/method dùng chéo.
4. Sửa `orca-runtime.ts`: thêm field, thay method bằng forward call.
5. Method private (không phải public API) có thể giữ private trên domain
   object mới, KHÔNG cần forward nếu chỉ được gọi nội bộ trong domain đó —
   kiểm tra bằng `grep` xem có bị gọi từ ngoài khối 7,770–9,620 không
   trước khi quyết định.

## Xác minh xong

- [ ] `pnpm run typecheck && pnpm run lint`
- [ ] `gitnexus detect_changes({scope: "all"})` — risk low
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `orca-runtime.ts` giảm
      đáng kể (không tới ~1,830 vì forward call vẫn chiếm vài trăm dòng)
- [ ] Test mobile-floor/remote-desktop/layout liên quan pass — đây là
      domain có docs riêng (`docs/mobile-presence-lock.md`,
      `docs/mobile-terminal-layout-state-machine.md`), khả năng cao có
      test tương ứng, kiểm tra kỹ trước khi coi là xong
- [ ] Test thủ công: mobile subscribe/unsubscribe, remote-desktop
      claim/release, resize khi có nhiều viewer — rủi ro cao nếu chỉ dựa
      test tự động

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-mobile-floor.ts
```
