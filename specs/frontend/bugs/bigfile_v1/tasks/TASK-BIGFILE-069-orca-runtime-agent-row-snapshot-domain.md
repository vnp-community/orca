# TASK-BIGFILE-069 — Move: OSC 9999 agent-status retention domain (rủi ro cao)

**Loại:** Move — composition pattern, rủi ro CAO (tiếp nối 067/068) ·
**Effort:** M · **Phụ thuộc:** TASK-BIGFILE-068
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Cụm liền kề ngay sau TASK-BIGFILE-068: `emitTerminalAgentStatusEvents`
(gọi từ `onPtyData`)/`retainAgentRowSnapshot`/`clearAgentRowSnapshotsForPty`
— retain payload OSC 9999 mới nhất theo `paneKey` để `worktree.ps` phục
vụ mobile cùng nguồn với sidebar desktop. Nhỏ và sạch hơn 067/068 nhiều
(chỉ 5 host dep), tách ngay tiếp nối cùng đợt.

## Kết quả thực thi (2026-08-11)

- Domain: `emitTerminalAgentStatusEvents` (public), `retainAgentRowSnapshot`
  (private), `clearAgentRowSnapshotsForPty` (public) — dòng gốc 2363–2479,
  117 dòng.
- 5 host dependency: `getGraph`, `getLeavesForPty`, `makeRuntimePaneKey`,
  `getOnTerminalAgentStatus` (đọc field readonly gán 1 lần ở constructor,
  field ở lại `orca-runtime.ts` vì chỉ dùng đúng ở đây — không cần host
  getter phức tạp, nhưng vẫn theo mẫu closure chuẩn cho nhất quán),
  `getLatestAgentStatusByPaneKey` (field `latestAgentStatusByPaneKey` ĐÃ
  có getter công khai từ TASK-BIGFILE-053 cho
  `orca-runtime-mobile-session-notify.ts` — tái sử dụng đúng field, KHÔNG
  chuyển field vì dùng rộng rãi ở nơi khác, ví dụ
  `getFreshRetainedAgentStatusForMobileTab`).
- 2 method cần public + forwarding (bị gọi từ `onPtyData`/`onPtyExit`/
  `dropDisconnectedPtyRecord`): `emitTerminalAgentStatusEvents`,
  `clearAgentRowSnapshotsForPty`.
- `ProcessedAgentStatusChunk` (từ `'../../shared/agent-status-osc'`) —
  move-only thật sự sau khi TASK-068 đã dùng hết các nơi khác.
  `RuntimeSyncedLeaf` — cùng biến thể phát hiện lại: import trực tiếp từ
  `'../../shared/runtime-types'` (nguồn gốc), không phải từ
  `'./orca-runtime'` (chỉ import lại cục bộ, không re-export).
- `orca-runtime.ts`: 8,571 → **8,468 dòng**. File mới: 145 dòng — dưới
  ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sạch ngay lần chạy đầu tiên trừ 2 lỗi import quen thuộc đã sửa
  ngay). `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi
  phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Cùng rủi ro như 067/068 (gọi từ `onPtyData`, zero test) nhưng cụm nhỏ
  hơn, logic đơn giản hơn (chủ yếu là Map read/write + change detection).
  Khuyến nghị kiểm thử cùng đợt: `worktree.ps` hiển thị đúng agent row
  inline trên mobile, state transition detection không bỏ sót
  (working→idle→blocked...).
- Phần còn lại của `orca-runtime.ts` (~8,468 dòng): `onPtyData` chính nó
  (dispatcher gọi vào ~10 domain đã tách, khó tách thêm vì bản thân nó
  LÀ orchestration logic, không phải 1 domain riêng), `createTerminal`,
  `graph`, `getWorktreePs`/`attachAgentRowsToSummaries`. Từ đây trở đi
  không còn "cụm nhiều method cùng chủ đề" rõ ràng nữa — chỉ còn vài
  method dispatcher lớn tự thân.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **8,468 dòng (68.3% giảm)** qua 37 task
(TASK-BIGFILE-036 đến 069, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và
063 là state-container Extract).
