# TASK-BIGFILE-250 — Investigate: pane-key-registry + provider-resolution state trong `ipc/pty.ts`

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** — (thay thế toàn bộ TASK-BIGFILE-001..007; xem trường Status
đã cập nhật trong 7 file đó, mỗi file trỏ về task này)
**Status:** ✅ Done (ghi chú thiết kế — sinh 2 task Move mới
TASK-BIGFILE-251, TASK-BIGFILE-252; xác nhận 3 cụm còn lại KHÔNG tách được
bằng Move, cần thiết kế riêng hoặc giữ nguyên)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md` (chưa cập
nhật lại trong task này — xem mục "Việc CHƯA làm")

## Bối cảnh

TASK-001 đến 007 (`ipc/pty.ts`, 5,185 dòng, không đổi từ 2026-08-10) bị đánh
dấu ⛔ Blocked vì các symbol đích chia sẻ state module-private dùng chéo
trong `registerPtyHandlers` (dòng 1459–5185, ~3,727 dòng, 1 hàm export duy
nhất đăng ký toàn bộ IPC channel PTY). Ghi chú blocking gốc trong
`TASKS-INDEX.md` § "Phát hiện khi thực thi" liệt kê 7 Map/Set module-private
dùng chéo "tại 40+ điểm rải khắp registerPtyHandlers" nhưng KHÔNG trích dẫn
dòng cụ thể — task này verify lại bằng dữ liệu thật.

## Phương pháp

Khác 2 tiền lệ `TASK-BIGFILE-035`/`054` (field-span trên 1 class, đo
`this.field`), ở đây đối tượng đo là **free function + module-scope
`let`/`const`** bên trong 1 file (không có `this`). Quy trình:

1. Đọc tuyến tính toàn bộ file bằng `Read` (2 lượt: dòng 1–1300, dòng
   1300–1700) để nắm cấu trúc thật (import, state module-level, các nhóm
   hàm) — KHÔNG dừng ở `grep -n "^export"` nông như task doc gốc.
2. Với mỗi symbol đích (biến state hoặc hàm), `grep -n "\b<symbol>\b"
   pty.ts` trên TOÀN FILE (không giới hạn theo khoảng dòng task gốc đoán),
   lấy `count` + dòng đầu/cuối.
3. Lọc riêng các lần xuất hiện có `line >= 1459` (bên trong
   `registerPtyHandlers`) để phân biệt "chỉ dùng ở module scope trước hàm
   đó" (an toàn) khỏi "bị đọc/ghi trực tiếp bên trong chính
   `registerPtyHandlers`" (rủi ro thật).
4. Với các lần xuất hiện bên trong `registerPtyHandlers`, đọc code quanh
   dòng đó để phân biệt: gọi qua 1 hàm wrapper thuần (an toàn, chỉ cần đổi
   import) so với thao tác trực tiếp lên `Map`/`Set`/biến `let` module-scope
   (rủi ro — xem phát hiện mới ở Cụm 4/5 dưới).

Phát hiện 1 loại ràng buộc **task 035/054 chưa gặp**: một số biến
module-scope là `let` bị **REASSIGN** (không chỉ đọc/ghi nội dung) từ bên
trong `registerPtyHandlers` (pattern "bridge" — module-scope function
pointer được closure-registration ghi đè bằng implementation thật). Nếu di
chuyển khai báo `let` đó sang file khác rồi `import`, TypeScript báo lỗi
biên dịch tại chỗ reassign vì import binding là read-only ở module nhập —
đây là lỗi CỨNG (compile-time), không phải rủi ro runtime mơ hồ như các
Map dùng chéo.

## Kết quả — 5 cụm, 3 hình dạng rủi ro khác nhau

### Cụm 1: pane-key-registry (TASK-BIGFILE-001) — tách được, nhưng KHÔNG bằng Move thuần

State: `paneKeyPtyId`, `ptyPaneKey`, `pendingByPaneKey`,
`paneSpawnReservationsByPaneKey`, `ptyPendingGenByPtyId`,
`rendererSerializerByPtyId`, `paneKeyTeardownListeners`,
`pendingSerializerGenSeq`, `pendingPaneSerializerCleanupRegistered` — tất cả
khai báo dòng 201–268. Nhóm hàm module-level thao tác chúng (dòng 219–381,
tự chứa, không hàm nào trong nhóm này gọi ra ngoài nhóm): `getPtyIdForPaneKey`,
`registerPaneKeyTeardownListener`, `hasPendingRendererSerializerForPaneKey`,
`parseValidPaneKey`, `isValidPaneKey`, `rememberPaneKeyForPty`,
`cleanupPendingPaneSerializersForSender`, `registerPendingPaneSerializerCleanup`,
`declarePendingPaneSerializer`, `reservePaneSpawn`, `clearPaneSpawnReservation`,
`rejectPaneSpawnReservation`, `resolvePaneSpawnReservation`,
`settlePendingPaneSerializer`.

**Điểm gọi thật bên trong `registerPtyHandlers` (dòng ≥1459), xác nhận bằng
grep — 19 điểm, KHÔNG phải "40+":**

- Trực tiếp thao tác Map/Set (6 điểm, rủi ro thật — không thể barrel-export
  hàm để che giấu):
  - dòng 5132: `paneKeyPtyId.get(args.paneKey)`
  - dòng 4216, 4223: `pendingByPaneKey.has/get(validatedPaneKey)`
  - dòng 3468, 4218: `rendererSerializerByPtyId.has(...)`
  - dòng 5134: `rendererSerializerByPtyId.add(ptyId)`
  - dòng 4225: `ptyPendingGenByPtyId.set(result.id, pending.gen)`
  - dòng 3110, 3995: `paneSpawnReservationsByPaneKey.get(...)`
- Gọi qua hàm wrapper thuần (13 điểm, an toàn — chỉ cần đổi import):
  `rememberPaneKeyForPty` (3265, 4309), `reservePaneSpawn` (3116, 4000),
  `resolvePaneSpawnReservation` (3279, 4392), `rejectPaneSpawnReservation`
  (3287, 4400), `settlePendingPaneSerializer` (5126, 5145),
  `declarePendingPaneSerializer` (5116).

`clearProviderPtyState` (dòng 1114–1199, xem Cụm 2) CŨNG đọc/ghi
`ptyPaneKey`, `paneKeyPtyId`, `ptyPendingGenByPtyId`, `rendererSerializerByPtyId`,
`paneKeyTeardownListeners`, `settlePendingPaneSerializer` — đây là entry
point teardown dùng chung, KHÔNG thuộc riêng cụm 1 (nó cũng đọc/ghi state ở
Cụm 2 và nhiều Set/Map khác, xem `clearProviderPtyState` trong Cụm 2).

**Kết luận:** tách được, nhưng phải là **"trích cả object/class đóng gói
state"** (đúng gợi ý trong đề bài) — 1 class (ví dụ `PaneKeySerializerRegistry`)
sở hữu 9 field state + expose method cho toàn bộ nhóm hàm hiện có, CỘNG
THÊM method mới cho 6 điểm thao tác Map trực tiếp ở trên (ví dụ
`hasRendererSerializer(ptyId)`, `markRendererSerializerRegistered(ptyId)`,
`getPendingSerializerEntry(paneKey)`, `recordPendingGenForPty(ptyId, gen)`,
`getPaneSpawnReservation(paneKey)`, `getPtyIdForPaneKeyRaw(paneKey)`) và 1
method `teardownForPty(id, paneKey)` bọc đúng đoạn logic pane-key hiện nằm
trong `clearProviderPtyState` (dòng 1140, 1157, 1164, 1169–1198) để hàm đó
gọi 1 lời thay vì 6 thao tác Map rời rạc. → sinh **TASK-BIGFILE-252**.

Ghi chú phụ: `registerPaneKeyTeardownListener` HIỆN KHÔNG có caller nào
trong toàn repo (`grep -rn` xác nhận 0 điểm gọi ngoài định nghĩa) — export
sống nhưng chết (dead), khác với comment dòng 223–228 mô tả 1 consumer dự
kiến ở `main/index.ts` (không tồn tại trong code hiện tại). Không thuộc
phạm vi sửa ở đây, chỉ ghi nhận để tránh giả định sai khi thiết kế TASK-252.

### Cụm 2: provider-resolution + ownership (TASK-BIGFILE-002, phần ownership của TASK-BIGFILE-004) — LÕI THẬT, xác nhận đúng ghi chú blocking gốc

State: `localProvider` (21 lần dùng, dòng 166–5183), `ptyConnectionProviders`
(10 lần, 170–5015), `ptyOwnership` (23 lần, 180–5012). Span của cả 3 gần
như PHỦ TOÀN BỘ `registerPtyHandlers` (dòng 394→5183 trên tổng 1459→5185,
~93% chiều dài hàm) — cùng hình dạng với `ptyController`/`graph` trong
`OrcaRuntimeService` ở TASK-BIGFILE-035/054 (core state, không tách được
bằng Move cơ học).

`ptyOwnership` bị thao tác trực tiếp (không qua wrapper) tại 12 điểm bên
trong `registerPtyHandlers`: dòng 1574, 2728, 2867, 3160, 3302, 3357, 4109,
4507, 4532, 4944, 5003, 5012.

`clearProviderPtyState` (dòng 1114–1199) — hàm dọn dẹp PTY dùng chung, gọi
tại **13 điểm**: 2 điểm module-level (`finishPtyShutdown` dòng 524,
`clearPtyOwnershipForConnection` dòng 1106) + **11 điểm bên trong
`registerPtyHandlers`** (1573, 2727, 2866, 3144, 3152, 3217, 3902, 4062,
4072, 4200 — rải từ dòng 1573 đến 4200, ~2,600 dòng). Hàm này chạm tới CẢ
Cụm 1 (pane-key state) LẪN nhiều Set/Map khác không thuộc 2 cụm này
(`ptySizes`, `lastInputAtByPty`, `interactiveOutputCharsByPty`,
`activeRendererPtys`, `visibleRendererPtys`, `rendererVisibilityKnownPtys`,
`pendingHiddenRendererResizeOutputPtys`,
`deliveredHiddenRendererResizeOutputPtys`, `providerSnapshotRequiredPtys`,
`clearBackgroundedDeliverySyncForPty` — closure bridge, xem Cụm 4/5) — đây
chính là điểm hội tụ khiến TASK-BIGFILE-004 (ownership-registry, gồm cả
`clearProviderPtyState`) không tách rời được: nó không phải 1 "registry"
đơn lẻ mà là hub dọn dẹp của ÍT NHẤT 3 cụm state khác nhau.

**Kết luận:** giống hệt kết luận TASK-035/054 cho "lõi" — core state dùng
chéo gần như toàn bộ `registerPtyHandlers`, tách sẽ phải inject provider
registry vào hầu hết mọi IPC handler (tương đương redesign toàn bộ file,
không phải 1 Move/Investigate nhỏ). **KHÔNG sinh task Move cho cụm này.**
`getProvider`, `getProviderForPty`, `tryGetProviderForPty`, `getAppPtyId`,
`getRelayPtyId`, `hasPtyProviderForInspection`,
`getProviderForStartupTerminalColorReply`,
`answerStartupTerminalColorQueriesForPty` (TASK-002) đều đọc `ptyOwnership`/
`localProvider`/`ptyConnectionProviders` trực tiếp hoặc gián tiếp — cùng số
phận.

Lưu ý cross-module: `registerRemotePtyProvider`, `unregisterRemotePtyProvider`,
`getRemotePtyProvider`, `getPtyIdsForConnection`,
`clearPtyOwnershipForConnection`, `clearProviderPtyState`,
`deletePtyOwnership`, `setPtyOwnership`, `answerStartupTerminalColorQueriesForPty`
đã có caller ngoài `ipc/pty.ts` thật (`main/ssh/ssh-relay-session.ts` dòng
36–45) — chữ ký/behavior các export này KHÔNG được đổi bởi bất kỳ thiết kế
tương lai nào cho cụm này.

### Cụm 3: pty-host-env (TASK-BIGFILE-003) — bị chặn "theo nhóm" SAI — thực ra AN TOÀN, Move thuần với phạm vi mở rộng

`buildPtyHostEnv` (834–1032) + 15 hàm helper thuần liền kề (545–821:
`readInheritedPath`, `firstPathEntry`, `promoteAgentTeamsShimPath`,
`deleteRequestedEnvKeys`, `shouldSkipCodexHomeEnvForWindowsShell`,
`getCodexSelectionTargetForPty`, `getCompatibleSelectedCodexHomePath`,
`readEnvWithProcessFallback`, `resolvePiAgentSourceDir`,
`resolveScopedPiAgentSourceDir`, `clearPiAgentShadowEnv`,
`exposePiManagedExtensionEnv`, `mergePtyEnvDeletions`,
`getInheritedAgentHookEnvKeysToDelete`, `restoreOrStripOverlayEnv`,
`isMimoLaunchCommand`, `resolveMimocodeSourceHome`,
`resolveOpenCodeSourceConfigDir`) + `CODEX_HOME_ENV_KEYS` (616) + type
`GetSelectedCodexHomePath` (617) + type `PrepareClaudeAuth` (618–620) +
`AGENT_HOOK_RUNTIME_ENV_KEYS` (208–217, KHÔNG liền kề — khai báo gần đầu
file cùng khối pane-key-registry theo vị trí dòng, nhưng CHỈ được dùng bởi
`getInheritedAgentHookEnvKeysToDelete` dòng 750 và `buildPtyHostEnv` dòng
915 — xác nhận bằng grep KHÔNG có điểm dùng nào khác trong toàn file, kể cả
bên trong `registerPtyHandlers`).

Toàn bộ khối này **KHÔNG đụng** `localProvider`/`ptyOwnership`/
`ptyConnectionProviders`/`paneKeyPtyId`/`ptyPaneKey`/`pendingByPaneKey`/
`paneSpawnReservationsByPaneKey`/`ptyPendingGenByPtyId`/
`rendererSerializerByPtyId`/`paneKeyTeardownListeners` — xác nhận bằng
`grep` trên đúng đoạn 545–1032, 0 kết quả. Các hàm helper ĐƯỢC gọi tại
nhiều điểm rải rác trong `registerPtyHandlers` (`getCodexSelectionTargetForPty`
tại 2946/3672/3843, `promoteAgentTeamsShimPath` tại 3036/3060/3891/3925,
`mergePtyEnvDeletions` tại 3049–3054/3914–3920, v.v.) nhưng vì đây là hàm
THUẦN (nhận tham số, trả giá trị, không đụng state module-level ngoài phạm
vi của chính chúng) — nhiều điểm gọi không phải rủi ro, chỉ cần đổi
`import`, đúng nguyên tắc Move cơ học ban đầu.

**Kết luận:** task doc gốc mô tả phạm vi quá hẹp (chỉ `buildPtyHostEnv`,
545–544, thiếu 15 hàm helper + hằng số + type liền kề) — TASK-BIGFILE-003
bị gộp chặn "theo nhóm" cùng 001/002/004/006 dù bản thân nó không có shared
mutable state. → sinh **TASK-BIGFILE-251** với phạm vi đã sửa đúng.

### Cụm 4: pty-renderer-delivery-debug (TASK-BIGFILE-005) — chặn ĐÚNG nhưng SAI LÝ DO gốc

Không phải "dùng chéo 40+ điểm" — vướng **ES module read-only import
binding**. `PtyRendererDeliveryDebugSnapshot` (type, 1252–1288),
`getPtyRendererDeliveryDebugSnapshot`/`resetPtyRendererDeliveryDebug`
(1354–1360) đọc 2 biến `let` "bridge" module-scope:
`readPtyRendererDeliveryDebugSnapshot` (khai báo dòng 1342, giá trị mặc
định là 1 closure rỗng) và `resetPtyRendererDeliveryDebugSnapshot` (1345).
Cả 2 bị **REASSIGN trực tiếp bên trong `registerPtyHandlers`** ở dòng 1929
và 1930 — gán 1 closure THẬT đọc `pendingData`/`rendererDeliveryAccountingByPty`
(state cục bộ của chính `registerPtyHandlers`, không phải module-scope).
Cùng pattern: `resetRendererDeliveryAccountingForLifecycleReset` (khai báo
1349, bị reset về no-op tại 1478, reassign closure thật tại 1940, gọi tại
1380/4814) và `clearRendererDispatcherReadyWatchdog` (khai báo 1352,
reassign tại 1477 và 1967). `mainDeliveryBreadcrumbs.record(...)` (instance
module-scope, dòng 1292) được gọi tại 4 điểm bên trong
`registerPtyHandlers`: 1867, 1880, 2145, 4862. `installPowerSignalBreadcrumbs()`
(1299–1312) được gọi 1 lần tại dòng 3568.

Nếu các khai báo `let` này di chuyển sang file mới rồi `import` lại vào
`pty.ts`, dòng 1929/1930/1940/1967/1478 sẽ báo lỗi biên dịch TypeScript
("Cannot assign to 'X' because it is an import") — đây là lỗi CỨNG, không
phải rủi ro runtime cần đánh giá thêm.

**Kết luận:** KHÔNG sinh task Move ở đợt này. Fix thật (ngoài phạm vi
Investigate) là đổi các điểm reassign thành gọi 1 hàm setter export từ file
mới (ví dụ `setPtyRendererDeliveryDebugSnapshotReader(fn)` thay vì
`readPtyRendererDeliveryDebugSnapshot = fn`) — đây là 1 thay đổi shape API
nhỏ nhưng không còn là "Move cơ học" (đổi hành vi cách 2 module giao tiếp,
dù không đổi runtime behavior cuối cùng). Cần đọc TRỌN VẸN dòng 1900–1970
(nơi toàn bộ closure bridge của `registerPtyHandlers` được lắp) trước khi
tự tin liệt kê ĐỦ danh sách bridge variable — hiện mới xác nhận 4 (có thể
sót, vì audit chỉ tra theo tên đã biết trước từ đầu file, không đọc tuyến
tính đoạn 1700–1970).

### Cụm 5: pty-provider-listener-binding (TASK-BIGFILE-006) — cùng loại chặn với Cụm 4, quy mô nhỏ hơn

`unbindLocalProviderListeners` (1448–1458) đọc/ghi 3 `let` module-scope:
`localDataUnsub`, `localExitUnsub`, `localBackgroundStreamUnsub` (khai báo
1213–1215). Cả 3 bị **REASSIGN bên trong `registerPtyHandlers`** — dòng
2608 (`localDataUnsub = localProvider.onData(...)`), dòng 2722
(`localExitUnsub = localProvider.onExit(...)`), dòng 2577
(`localBackgroundStreamUnsub = ...`) — và bị ĐỌC lại (unbind trước khi
re-bind) tại dòng 2568–2570. `rebindLocalProviderListeners` (1248–1250,
gọi `rebindProviderListeners?.()`) cùng biến bridge `rebindProviderListeners`
(khai báo 1246, reassign tại dòng 2737 = `bindProviderListeners`) rơi vào
ĐÚNG nhóm rủi ro này dù task gốc không xếp chung.

**Kết luận:** cùng chặn như Cụm 4, cùng lý do kỹ thuật (ES module read-only
binding). Khuyến nghị gộp thiết kế lại: `unbindLocalProviderListeners` +
`rebindLocalProviderListeners` + 4 biến bridge liên quan
(`localDataUnsub`, `localExitUnsub`, `localBackgroundStreamUnsub`,
`rebindProviderListeners`) nên đi CHUNG 1 file/thiết kế (tên gợi ý
`pty-provider-listener-bridge.ts`, đổi "binding" → "bridge" để phản ánh
đúng bản chất: đây là closure bridge module-scope, không phải 1 registry
Map/Set đơn thuần như Cụm 1). KHÔNG sinh task Move ở đợt này — cùng lý do
"chưa đọc trọn vẹn đủ để tự tin" như Cụm 4.

## Task Move sinh ra ở đợt này

| # | Task | Cụm | Mức an toàn |
|---|---|---|---|
| 251 | [TASK-BIGFILE-251](./TASK-BIGFILE-251-ipc-pty-host-env.md) | Cụm 3 — pty-host-env (phạm vi mở rộng) | An toàn — pure function/type/const, Move cơ học |
| 252 | [TASK-BIGFILE-252](./TASK-BIGFILE-252-ipc-pty-pane-key-registry.md) | Cụm 1 — pane-key-registry (object/class) | An toàn có điều kiện — Move (composition), phải sửa đúng 6 điểm thao tác Map trực tiếp thành gọi method |

## KHÔNG sinh task Move cho

- **Cụm 2** (provider-resolution/ownership, TASK-002 + phần ownership của
  TASK-004) — lõi thật dùng chéo ~93% `registerPtyHandlers`, xem lý do ở
  trên. `TASK-BIGFILE-007` (Investigate còn lại của `registerPtyHandlers`)
  giữ nguyên vai trò, nhưng phạm vi thực tế của nó bây giờ PHẢI coi cụm này
  là 1 phần lõi không tách được — không còn giả định "sau khi 001-006 xong,
  còn lại ~3,730 dòng" (001/002/004/005/006 phần lớn không tách, chỉ 003 và
  phần đối tượng-hoá của 001 tách được).
- **Cụm 4, 5** (renderer-delivery-debug, provider-listener-binding/bridge) —
  cần 1 Investigate/thiết kế riêng cho pattern "bridge-to-setter" TRƯỚC khi
  viết Move (rủi ro lỗi biên dịch nếu làm ẩu mà không có danh sách bridge
  variable đầy đủ). Xem "Việc CHƯA làm".

## Nguyên tắc bắt buộc cho TASK-BIGFILE-251/252

1. `gitnexus impact({target: "<symbol>", direction: "upstream"})` cho từng
   symbol export trước khi cắt — đặc biệt xác nhận lại danh sách caller
   ngoài `ipc/pty.ts` (đã xác nhận `main/ssh/ssh-relay-session.ts` dùng 1 số
   export của Cụm 2 — KHÔNG liên quan 251/252, nhưng verify lại không thừa).
2. Đọc đúng dải dòng đã xác nhận ở trên TRƯỚC khi cắt (dữ liệu trên đến từ
   `grep`/`Read` thật ngày hôm nay, nhưng file có thể lệch vài dòng nếu có
   commit khác chèn vào giữa).
3. 1 task = 1 commit riêng. Chạy `pnpm run typecheck` (3 target) sau MỖI
   thay đổi — đây là cách xác nhận nhanh nhất TASK-252 đã sửa đúng cả 6
   điểm thao tác Map trực tiếp (thiếu 1 điểm → lỗi biên dịch "cannot find
   name X" ngay, không phải lỗi runtime im lặng).
4. `gitnexus detect_changes({scope: "compare", base_ref: "main"})` sau mỗi
   task — risk phải ở mức low, chỉ đúng các symbol đã liệt kê bị đổi vị trí.
5. Không có `pty.test.ts` hiện có (`ls frontend/src/main/ipc/` xác nhận) —
   test PTY nằm ở nơi khác (nếu có) hoặc không có test tự động cho file
   này; xác minh xong bắt buộc phải bao gồm 1 vòng kiểm thử thủ công
   spawn/write/resize/kill terminal thật (khớp bối cảnh `BUG-FE-PTY-001`
   đang điều tra song song), không chỉ dựa vào `tsc`/`lint`.

## Việc CHƯA làm (ngoài phạm vi Investigate)

- Không sửa code trong `ipc/pty.ts`.
- Không thiết kế API setter đầy đủ cho Cụm 4/5 (bridge-to-setter) — cần đọc
  trọn vẹn dòng 1700–1970 (toàn bộ đoạn lắp closure bridge trong
  `registerPtyHandlers`) để liệt kê ĐỦ danh sách bridge variable, hiện mới
  xác nhận 6 (4 ở Cụm 4 + 2 ở Cụm 5 — không tính trùng `rebindProviderListeners`)
  bằng cách tra theo tên đã biết trước, có thể còn sót biến bridge khác
  chưa được đặt tên trong 7 file blocked gốc.
- Không cập nhật `TASKS-INDEX.md` (theo yêu cầu của người giao việc).
- Không cập nhật `SOLUTION-FE-BIGFILE-011-ipc-pty.md` — khuyến nghị làm sau
  khi TASK-BIGFILE-251/252 merge, phản ánh đúng cấu trúc mới (chỉ 2 file
  tách ra thay vì 6 như solution gốc mô tả).
- Không đổi/xoá `registerPaneKeyTeardownListener` dù xác nhận dead export —
  ngoài phạm vi task này, cần quyết định riêng (xoá hay giữ cho tương lai).

## Bài học phương pháp

`grep -n "\bSYMBOL\b" file | wc -l` + dòng đầu/cuối là đủ để phát hiện
"dùng chéo hay không dùng chéo", nhưng KHÔNG đủ để phân loại MỨC ĐỘ rủi ro:
phải đọc code quanh từng điểm xuất hiện bên trong hàm khổng lồ để phân biệt
3 hình dạng khác nhau đều bị gắn nhãn "shared state" như nhau ở lần audit
trước nhưng có giải pháp khác hẳn nhau:

1. **Map/Set dùng chéo qua wrapper function thuần** (Cụm 1 phần lớn) — an
   toàn, chỉ cần đổi import của hàm wrapper.
2. **Map/Set bị thao tác trực tiếp (không qua wrapper) trong hàm khổng lồ**
   (Cụm 1: 6 điểm, Cụm 2: 12+ điểm) — cần bọc thành method trên object/class
   trước khi tách được (Cụm 1) hoặc là dấu hiệu core state thật không tách
   được (Cụm 2, khi span quá rộng).
3. **`let` bị REASSIGN (không chỉ đọc/ghi) trong hàm khổng lồ** (Cụm 4, 5)
   — lỗi biên dịch cứng nếu tách bằng `export`/`import` thông thường, cần
   đổi shape API sang setter function trước — khác hẳn 2 loại trên và CHƯA
   xuất hiện trong 2 tiền lệ TASK-035/054 (những field đó luôn là `this.field`
   được đọc/ghi, không phải module-scope function reference bị gán lại).
