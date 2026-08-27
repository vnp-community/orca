# BUG-FE-BIGFILE-011 — `ipc/pty.ts` (5,185 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-011](./solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md)
**Module:** `frontend/src/main/ipc/pty.ts`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

5,185 dòng, main-process, không phải component. Comment dòng 1: "PTY IPC is
intentionally centralized in one [file]" (cắt bởi grep, đọc trực tiếp file để
lấy đầy đủ lý do).

File này là file `.ts` (không phải `.tsx`) thứ 2 liên quan trực tiếp tới PTY
nằm trong nhóm Critical, sau `pty-connection.ts` (BUG-FE-BIGFILE-006) —
2 file cộng lại **12,785 dòng** cho riêng chủ đề "kết nối/quản lý PTY", chưa
tính `orca-runtime.ts` (BUG-FE-BIGFILE-002) cũng sở hữu phần lớn PTY
lifecycle state (`RuntimePtyController`, `RuntimePtyWorktreeRecord`).

Danh sách export top-level (22 hàm/type), đã tách khá tốt theo domain hẹp
(mỗi hàm 1 trách nhiệm rõ), nhưng vẫn ở chung 1 file:

```
219   getPtyIdForPaneKey / 232 registerPaneKeyTeardownListener
379   hasPendingRendererSerializerForPaneKey
461   answerStartupTerminalColorQueriesForPty
545   type BuildPtyHostEnvOptions / 834 buildPtyHostEnv
1050  registerRemotePtyProvider / 1055 unregisterRemotePtyProvider
1060  getRemotePtyProvider / 1072 getLocalPtyProvider / 1078 setLocalPtyProvider
1083  getPtyIdsForConnection / 1099 clearPtyOwnershipForConnection
1114  clearProviderPtyState / 1201 deletePtyOwnership / 1205 setPtyOwnership
1248  rebindLocalProviderListeners
1252  type PtyRendererDeliveryDebugSnapshot
1354  getPtyRendererDeliveryDebugSnapshot / 1358 resetPtyRendererDeliveryDebug
1448  unbindLocalProviderListeners
1459  registerPtyHandlers                    ← điểm bắt đầu phần IPC handler chính, ~3,700 dòng
5150  registerHeadlessPtyRuntime
5181  killAllPty
```

`registerPtyHandlers` (dòng 1459 → 5150) chiếm **~71% file** (3,691/5,185
dòng) trong 1 hàm duy nhất — đây là điểm nóng thực sự, không phải phần code
phía trên (vốn đã là các hàm nhỏ, độc lập, dễ tách).

## Hậu quả

- Các hàm nhỏ (dòng 219–1448, ~1,230 dòng, chiếm 24% file) đã đủ độc lập để
  tách ngay lập tức mà gần như không rủi ro — chúng chỉ đang "ở nhầm chỗ", không
  có vấn đề thiết kế.
- `registerPtyHandlers` 3,691 dòng là nơi đăng ký toàn bộ IPC handler cho PTY —
  cùng loại rủi ro với `connectPanePty` trong `pty-connection.ts`
  (BUG-FE-BIGFILE-006): 1 hàm khổng lồ orchestration, khó test từng handler
  riêng lẻ.

## Bằng chứng

```
wc -l ipc/pty.ts                                       → 5185
grep -n "^export function\|^export type" ipc/pty.ts     → 22 export, phần lớn nhỏ/độc lập
                                                            (dòng 219–1448)
                                                          → registerPtyHandlers dòng 1459,
                                                            chiếm tới dòng ~5150 (71% file)
head -1 ipc/pty.ts                                      → "/* eslint-disable max-lines -- Why: PTY IPC is intentionally centralized in one ..."
```

## Đề xuất fix

1. **Bước rủi ro thấp nhất, làm ngay**: tách các hàm độc lập (dòng 219–1448)
   sang `pty-ownership-registry.ts` (các hàm `*PtyOwnership*`,
   `*ProviderPtyState*`, `register/unregister/get*PtyProvider`) và
   `pty-renderer-delivery-debug.ts` (`PtyRendererDeliveryDebugSnapshot` và 2
   hàm liên quan) — không đổi hành vi, giảm ngay ~1,200 dòng khỏi file chính.
2. Với `registerPtyHandlers` (3,691 dòng): xác định các nhóm IPC channel theo
   domain (create/attach, write/resize, serialize/scrollback, signal/kill,
   ...) — mỗi nhóm channel có thể tách thành 1 hàm
   `registerPty<Domain>Handlers(ipcMain, ...)` riêng, gọi từ
   `registerPtyHandlers` chính (giữ 1 điểm entry duy nhất như comment dòng 1
   yêu cầu, nhưng không bắt buộc TOÀN BỘ logic phải cùng 1 hàm).
3. Đọc kỹ comment dòng 1 đầy đủ (bị cắt trong bằng chứng ở trên) trước khi
   tách — có thể có lý do kỹ thuật cụ thể (vd: thứ tự đăng ký IPC channel
   quan trọng) cần giữ nguyên khi tách.

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- File liên quan cùng chủ đề PTY: `BUG-FE-BIGFILE-006` (`pty-connection.ts`),
  `BUG-FE-BIGFILE-002` (`orca-runtime.ts`)
