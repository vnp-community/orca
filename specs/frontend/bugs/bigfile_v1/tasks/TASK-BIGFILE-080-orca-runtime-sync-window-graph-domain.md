# TASK-BIGFILE-080 — Move: syncWindowGraph (sync-window-graph domain)

**Loại:** Move — composition pattern, rủi ro cao (renderer→main graph-sync
hot path, mutate `this.graph` trực tiếp) · **Effort:** M · **Phụ thuộc:**
TASK-BIGFILE-041 (RuntimeGraphStore)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Candidate lớn nhất còn lại sau TASK-078/079, để lại từ TASK-078's "Rủi ro còn
lại": `syncWindowGraph` (145 dòng) — method nhận graph publish từ renderer
(mọi lần tạo/đóng terminal, split pane, focus window, reload renderer) và
đối chiếu lại với live PTY graph. Cùng kỹ thuật "tách chính dispatcher, giữ
domain đã tách làm host dependency" đã dùng cho `onPtyData`/`onPtyExit`
(TASK-072/074/077).

## Kết quả thực thi (2026-08-12)

- Domain: nguyên vẹn method `syncWindowGraph` (dòng gốc 760–903, 144 dòng),
  không có helper riêng nào chỉ dùng bởi nó (khác `onPtyExit`'s
  `failActiveDispatchOnExit`).
- 13 host dependency, toàn bộ method/forwarding field đã tồn tại: `getGraph`,
  `syncMobileSessionTabs`/`notifyMobileSessionTabSnapshots` (kiểu qua
  `RuntimeMobileSessionNotifyCommands['...']`), `nextTitleObservationSequence`,
  `getLeafKey`, `recordPtyWorktree`, `makeRuntimePaneKey`,
  `invalidateLeafHandle`, `rebuildLeafPtyIndex`, `refreshWritableFlags`,
  `adoptPreAllocatedHandle`, `buildAgentOrchestrationByPaneKey`, `getStatus`.
- `syncWindowGraph` giữ forwarding field public (không có internal caller
  nào khác, gọi từ RPC/IPC layer bên ngoài).
- Xác minh fidelity bằng diff nguyên văn (chuẩn hoá `this.host.getGraph()` →
  `this.graph`, `this.host.X` → `this.X`) so với `git show HEAD:...` — khớp
  byte-for-byte ngay từ lần viết đầu (không có transcription bug lần này).
- 2 lỗi `tsc` move-only (`RuntimeSyncWindowGraph`/`RuntimeSyncWindowGraphResult`
  không còn dùng trực tiếp trong `orca-runtime.ts`) — dọn sạch.
- 1 lỗi `oxlint no-useless-spread` (`[...graphSyncCallbacks]` clone-trước-khi-
  duyệt, cùng mẫu grandfathered đã dùng ở nhiều file khác) — thêm inline
  disable.
- `orca-runtime.ts`: 5,074 → **4,948 dòng — LẦN ĐẦU XUỐNG DƯỚI 5.000**. File
  mới `orca-runtime-sync-window-graph.ts`: 197 dòng (153 non-blank/non-comment)
  — dưới ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi.
  `oxlint` sạch (exit 0) cả 2 config sau khi thêm 1 disable. `max-lines-ratchet`:
  647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro cao — chạy trên mọi lần renderer publish graph (tạo/đóng terminal,
  split pane, focus window, reload). Khuyến nghị kiểm thử thủ công kỹ: mở/
  đóng terminal nhiều lần liên tục, split pane, reload renderer (F5/Cmd+R)
  trong lúc có PTY đang chạy agent (xác nhận handle CLI không bị mất — comment
  "Why: renderer reloads can briefly republish..." mô tả đúng invariant này),
  2 renderer window (multi-window authoritative window switch), race giữa
  graph sync và PTY exit đồng thời.
- Người dùng đặt mục tiêu <2.000 dòng — còn cách khá xa (4.948 dòng). Các
  cụm method lớn tương tự `syncWindowGraph` không còn nhiều; phần lớn còn lại
  của file là ~45 block composition-wiring (forwarding field liệt kê từng
  method, không thể "tách" thêm mà không đổi kỹ thuật một lần nữa — ví dụ
  chuyển ownership state thực sự ra khỏi `OrcaRuntimeService`, không chỉ
  method). Cần đánh giá lại khả năng đạt <2.000 với rủi ro chấp nhận được.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **4,948 dòng (81.5% giảm — lần đầu dưới 5.000)**
qua 48 task.
