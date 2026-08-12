# TASK-BIGFILE-252 — Move (composition): `pty-pane-key-registry.ts` (object/class, không phải 3 hàm rời)

**Loại:** Move (composition) — rủi ro trung bình, phải sửa đúng 6 điểm thao
tác Map trực tiếp bên trong `registerPtyHandlers` thành gọi method
**Effort:** M · **Phụ thuộc:** — · **Status:** ⬜ Chưa làm
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md` (mục
`pty-pane-key-registry.ts` cần viết lại theo task này — bản gốc mô tả 3
hàm rời, KHÔNG đúng thực tế)
**Thay thế:** `TASK-BIGFILE-001` (⛔ Superseded — xem
`TASK-BIGFILE-250-ipc-pty-pane-key-registry-investigate.md` § Cụm 1)

## Vì sao KHÔNG thể làm như TASK-BIGFILE-001 mô tả (barrel export 3 hàm)

`TASK-BIGFILE-001` giả định chỉ 3 export (`getPtyIdForPaneKey`,
`registerPaneKeyTeardownListener`, `hasPendingRendererSerializerForPaneKey`)
cần chuyển. Thực tế: 9 biến state (`paneKeyPtyId`, `ptyPaneKey`,
`pendingByPaneKey`, `paneSpawnReservationsByPaneKey`, `ptyPendingGenByPtyId`,
`rendererSerializerByPtyId`, `paneKeyTeardownListeners`,
`pendingSerializerGenSeq`, `pendingPaneSerializerCleanupRegistered`) bị
**thao tác trực tiếp** (không qua hàm wrapper) tại **6 điểm bên trong
`registerPtyHandlers`** (dòng 3110, 3468, 3995, 4216, 4218, 4223, 4225,
5132, 5134 — xem bảng dưới) VÀ bên trong `clearProviderPtyState` (dòng
1114–1199, KHÔNG di chuyển trong task này — xem "Ranh giới với
`clearProviderPtyState`"). Nếu chỉ barrel-export 3 hàm và để 9 biến state ở
lại `pty.ts`, `pty-pane-key-registry.ts` sẽ trùng lặp state với `pty.ts`
(2 module giữ 2 bản Map khác nhau) — SAI. Phải chuyển CẢ state lẫn toàn bộ
14 hàm module-level thao tác nó, đóng gói thành 1 object/class, rồi cập
nhật 6 điểm thao tác Map trực tiếp trong `registerPtyHandlers` thành gọi
method mới trên object đó.

## Input

- File nguồn: `frontend/src/main/ipc/pty.ts` (xác nhận lại `wc -l` và số
  dòng trước khi bắt đầu — audit dưới đến từ 2026-08-12)
- Đọc đúng dòng 201–381 (khai báo state + 14 hàm module-level tự chứa)
- Đọc đúng các đoạn sau bên trong `registerPtyHandlers` (KHÔNG đọc lại toàn
  bộ 1459–5185 — chỉ các đoạn quanh những dòng này, đủ ngữ cảnh để viết
  method mới đúng hành vi cũ):
  - 3090–3160 (dùng `paneSpawnReservationsByPaneKey.get`,
    `reservePaneSpawn`, `rememberPaneKeyForPty`)
  - 3260–3300 (dùng `rememberPaneKeyForPty`, `resolvePaneSpawnReservation`,
    `rejectPaneSpawnReservation`)
  - 3450–3480 (dùng `pendingByPaneKey` gián tiếp qua comment,
    `rendererSerializerByPtyId.has`)
  - 3980–4010 (dùng `paneSpawnReservationsByPaneKey.get`, `reservePaneSpawn`)
  - 4200–4230 (dùng `pendingByPaneKey.has/get`,
    `rendererSerializerByPtyId.has`, `ptyPendingGenByPtyId.set`)
  - 4300–4315, 4385–4405 (dùng `rememberPaneKeyForPty`,
    `resolvePaneSpawnReservation`, `rejectPaneSpawnReservation`)
  - 5100–5150 (dùng `declarePendingPaneSerializer`,
    `settlePendingPaneSerializer`, `paneKeyPtyId.get`,
    `rendererSerializerByPtyId.add`)

## Ranh giới với `clearProviderPtyState` (KHÔNG di chuyển hàm này trong task này)

`clearProviderPtyState` (1114–1199) là hub dọn dẹp teardown dùng chung của
NHIỀU cụm state (xem `TASK-BIGFILE-250` § Cụm 2), không chỉ pane-key. Task
này CHỈ thêm 1 method mới `teardownForPty(id: string): { paneKey: string |
undefined; stillOwnsPaneKey: boolean }` (hoặc kiểu trả tương đương) trên
registry mới, bọc đúng đoạn logic pane-key hiện nằm rải trong
`clearProviderPtyState` (dòng 1140–1141, 1157, 1161, 1164, 1169–1198: đọc
`paneKey`/`stillOwnsPaneKey`, xoá `paneKeyPtyId`/`ptyPaneKey`, settle
`ptyPendingGenByPtyId`, xoá `rendererSerializerByPtyId`, chạy
`paneKeyTeardownListeners`). `clearProviderPtyState` GỌI method mới này
thay vì thao tác 5 Map/Set trực tiếp, nhưng property BÊN NGOÀI phạm vi
pane-key (đọc `unregisterPty`, `agentHookServer.clearPaneKeyAliasesForPty`,
`agentHookServer.clearPaneState`) **giữ nguyên tại chỗ trong `pty.ts`** —
KHÔNG chuyển `agentHookServer`/`unregisterPty` sang file mới, chỉ
`teardownForPty` trả đủ thông tin (`paneKey`, `stillOwnsPaneKey`) để
`clearProviderPtyState` tự quyết định có gọi 2 hàm đó hay không (giữ đúng
thứ tự side-effect hiện tại — xem "Điểm cần cẩn thận" dưới).

## Output

- File mới: `frontend/src/main/ipc/pty-pane-key-registry.ts`, export 1
  instance singleton (ví dụ `export const paneKeySerializerRegistry = new
  PaneKeySerializerRegistry()`, class không export — theo đúng convention
  module-singleton hiện có trong file khác của `ipc/`, ví dụ
  `pty-hidden-delivery-gate.ts` nếu file đó dùng pattern tương tự — xác
  nhận convention thật khi đọc, không tự đặt ra kiểu mới).
- API tối thiểu registry phải có (giữ đúng hành vi 14 hàm module-level hiện
  tại + 6 thao tác Map trực tiếp mới bọc thành method):
  - `getPtyIdForPaneKey(paneKey): string | undefined` (thay `paneKeyPtyId.get`
    tại dòng cũ 5132 VÀ export hiện có `getPtyIdForPaneKey`)
  - `registerTeardownListener(listener): () => void` (thay
    `registerPaneKeyTeardownListener` — LƯU Ý: dead export, xem
    `TASK-BIGFILE-250`, giữ nguyên hành vi dù chưa có caller)
  - `hasPendingRendererSerializer(paneKey): boolean` (thay
    `hasPendingRendererSerializerForPaneKey`)
  - `rememberPaneKeyForPty(ptyId, paneKey): string | null`
  - `declarePendingSerializer(paneKey, sender): number`
  - `settlePendingSerializer(paneKey, gen): void`
  - `hasPendingSerializerEntry(paneKey): boolean` (thay
    `pendingByPaneKey.has(...)` dòng cũ 4216)
  - `getPendingSerializerEntry(paneKey): { gen; ownerWebContentsId } | undefined`
    (thay `pendingByPaneKey.get(...)` dòng cũ 4223)
  - `hasRendererSerializer(ptyId): boolean` (thay `rendererSerializerByPtyId.has`
    dòng cũ 3468, 4218)
  - `markRendererSerializerRegistered(ptyId): void` (thay
    `rendererSerializerByPtyId.add` dòng cũ 5134)
  - `recordPendingGenForPty(ptyId, gen): void` (thay `ptyPendingGenByPtyId.set`
    dòng cũ 4225)
  - `reservePaneSpawn(paneKey): PaneSpawnReservation`
  - `getPaneSpawnReservation(paneKey): PaneSpawnReservation | undefined`
    (thay `paneSpawnReservationsByPaneKey.get` dòng cũ 3110, 3995)
  - `resolvePaneSpawnReservation<T>(paneKey, reservation, response): T`
  - `rejectPaneSpawnReservation(paneKey, reservation, error): void`
  - `teardownForPty(id): { paneKey: string | undefined; stillOwnsPaneKey: boolean }`
    (method MỚI, xem "Ranh giới với `clearProviderPtyState`" trên)
  - Type export lại: `PaneKeyTeardownListener`, `PaneSpawnReservation`,
    `PaneSpawnReservationResult`
- `ipc/pty.ts` xoá dòng 201–381, thay bằng:
  ```ts
  export { paneKeySerializerRegistry } from './pty-pane-key-registry'
  export const getPtyIdForPaneKey = (paneKey: string) =>
    paneKeySerializerRegistry.getPtyIdForPaneKey(paneKey)
  export const registerPaneKeyTeardownListener = (listener: PaneKeyTeardownListener) =>
    paneKeySerializerRegistry.registerTeardownListener(listener)
  export const hasPendingRendererSerializerForPaneKey = (paneKey: string) =>
    paneKeySerializerRegistry.hasPendingRendererSerializer(paneKey)
  ```
  (giữ nguyên chữ ký 3 export cũ — không có caller ngoài `pty.ts` hiện tại
  nhưng KHÔNG đổi public API phòng trường hợp có caller ẩn danh qua dynamic
  import/test mock; xác nhận lại bằng `gitnexus impact` ở bước 1.)

## Điểm cần cẩn thận

- Thứ tự side-effect trong `clearProviderPtyState` hiện tại: đọc `paneKey`/
  `stillOwnsPaneKey` TRƯỚC `unregisterPty(id)`, nhưng notify
  `paneKeyTeardownListeners` SAU khi đã xoá `paneKeyPtyId`/`ptyPaneKey`
  (comment dòng 1186–1189 giải thích rõ lý do: listener đọc lại map phải
  thấy state post-teardown). Method `teardownForPty` mới PHẢI thực hiện
  đúng thứ tự nội bộ này (xoá map trước, notify listener sau) — không đảo
  ngược dù đang đóng gói lại thành 1 lời gọi.
- 6 điểm thao tác Map trực tiếp trong `registerPtyHandlers` (bảng dưới)
  phải đổi 1-1 sang method tương ứng — không gộp/rút gọn logic, kể cả khi
  trông có vẻ dư thừa (mục tiêu task này là Move hành vi giữ nguyên, không
  phải cải tiến logic).

| Dòng cũ (grep 2026-08-12) | Thao tác cũ | Method mới |
|---|---|---|
| 3110, 3995 | `paneSpawnReservationsByPaneKey.get(...)` | `paneKeySerializerRegistry.getPaneSpawnReservation(...)` |
| 3468, 4218 | `rendererSerializerByPtyId.has(...)` | `paneKeySerializerRegistry.hasRendererSerializer(...)` |
| 4216 | `pendingByPaneKey.has(...)` | `paneKeySerializerRegistry.hasPendingSerializerEntry(...)` |
| 4223 | `pendingByPaneKey.get(...)` | `paneKeySerializerRegistry.getPendingSerializerEntry(...)` |
| 4225 | `ptyPendingGenByPtyId.set(...)` | `paneKeySerializerRegistry.recordPendingGenForPty(...)` |
| 5132 | `paneKeyPtyId.get(...)` | `paneKeySerializerRegistry.getPtyIdForPaneKey(...)` |
| 5134 | `rendererSerializerByPtyId.add(...)` | `paneKeySerializerRegistry.markRendererSerializerRegistered(...)` |

- 13 điểm gọi hàm wrapper cũ (`rememberPaneKeyForPty`, `reservePaneSpawn`,
  `resolvePaneSpawnReservation`, `rejectPaneSpawnReservation`,
  `settlePendingPaneSerializer`, `declarePendingPaneSerializer` — dòng
  3116, 3265, 3279, 3287, 3116, 4000, 4309, 4392, 4400, 5116, 5126, 5145,
  xem `TASK-BIGFILE-250` § Cụm 1) chỉ cần đổi thành
  `paneKeySerializerRegistry.<method>(...)`, không đổi tham số.

## Các bước

1. `gitnexus impact({target: "getPtyIdForPaneKey", direction: "upstream"})`
   — lặp lại cho `registerPaneKeyTeardownListener`,
   `hasPendingRendererSerializerForPaneKey`. Dừng nếu risk HIGH/CRITICAL.
2. Đọc dòng 201–381 trọn vẹn + các đoạn liệt kê ở "Input" bên trong
   `registerPtyHandlers`.
3. Viết `pty-pane-key-registry.ts`: class đóng gói 9 field state + toàn bộ
   method liệt kê ở "Output" (bao gồm `teardownForPty` MỚI — logic lấy
   nguyên văn từ đoạn pane-key trong `clearProviderPtyState`).
4. Sửa `ipc/pty.ts`:
   - Xoá dòng 201–381.
   - Thêm export tương thích ngược (xem block ví dụ ở "Output").
   - Sửa `clearProviderPtyState` (1114–1199): thay đoạn pane-key
     (1140–1141, 1157, 1161, 1164, 1169–1198) bằng 1 lời gọi
     `paneKeySerializerRegistry.teardownForPty(id)`, giữ nguyên phần gọi
     `unregisterPty`/`agentHookServer.*` dựa trên kết quả trả về.
   - Sửa 6 điểm thao tác Map trực tiếp (bảng trên) + 13 điểm gọi hàm
     wrapper cũ trong `registerPtyHandlers` thành gọi method trên
     `paneKeySerializerRegistry`.
5. `pnpm run typecheck` (3 target) sau MỖI file sửa — thiếu 1 điểm chuyển
   đổi sẽ báo lỗi "cannot find name" ngay tại dòng đó.

## Xác minh xong

- [ ] `pnpm run typecheck` (3 target: node/cli/web) pass
- [ ] `pnpm run lint` pass
- [ ] `gitnexus detect_changes({scope: "compare", base_ref: "main"})` —
      risk low, chỉ đúng các symbol pane-key-registry + `clearProviderPtyState`
      (nội bộ, không đổi chữ ký) bị đổi
- [ ] `node scripts/find-frontend-bigfiles.mjs` — `ipc/pty.ts` giảm ~180
      dòng ròng (181 dòng định nghĩa gốc trừ đi các dòng export/import thêm
      vào — nhỏ hơn 240 dòng ước tính gốc của TASK-001 vì phần lớn logic
      giờ nằm ở file mới nhưng `pty.ts` vẫn giữ 3 dòng export tương thích +
      sửa `clearProviderPtyState` tại chỗ)
- [ ] Kiểm thử thủ công BẮT BUỘC (không có `pty.test.ts` tự động): mở 1
      pane terminal, remount pane đó (đổi tab rồi quay lại) để trigger lại
      đúng luồng `pty:spawn`/`pty:declarePendingPaneSerializer`/
      `pty:settlePaneSerializer` — xác nhận không tái phát lỗi
      SSH_SESSION_EXPIRED / echo kép đang điều tra ở `BUG-FE-PTY-001` (branch
      hiện tại `fix/pty-session-expired-on-pane-remount` — task này chạm
      ĐÚNG vùng code liên quan, review cẩn thận trước khi merge).

## Rollback

```
git checkout -- frontend/src/main/ipc/pty.ts
rm frontend/src/main/ipc/pty-pane-key-registry.ts
```
