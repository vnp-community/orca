# TASK-BIGFILE-037 — Move (composition): Mobile floor / remote-desktop / layout domain

**Loại:** Move — composition pattern (KHÔNG phải barrel, class có `this`)
· **Effort:** L (khối lớn nhất trong 5 task sinh từ TASK-035, ~1,830 dòng)
· **Phụ thuộc:** TASK-BIGFILE-008, 009, khuyến nghị làm SAU TASK-036 (để
xác nhận pattern composition hoạt động đúng trên khối nhỏ trước)
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3) · Sinh ra từ `TASK-BIGFILE-035`

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
