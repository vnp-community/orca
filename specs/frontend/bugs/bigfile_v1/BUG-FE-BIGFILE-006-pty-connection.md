# BUG-FE-BIGFILE-006 — `pty-connection.ts` (7,600 dòng)

**Mức độ:** 🔴 Critical
**Status:** 🔴 Open
**Solution:** [SOLUTION-FE-BIGFILE-006](./solutions/SOLUTION-FE-BIGFILE-006-pty-connection.md)
**Module:** `frontend/src/renderer/src/components/terminal-pane/pty-connection.ts`
**Phát hiện:** 2026-08-10, `scripts/find-frontend-bigfiles.mjs` — xem tổng quan
tại `BUG-FE-BIGFILE-001`

---

## Mô tả

7,600 dòng, chỉ 2 export top-level: `STARTUP_CWD_FALLBACK_NOTICE` (const) và
`connectPanePty` (dòng 943 → gần hết file, ~6,650 dòng cho 1 hàm). Đây không
phải component React — không có `useState`/`useEffect` (0/0) — toàn bộ logic
là plain TypeScript orchestration cho việc kết nối 1 pane với PTY (local,
SSH, remote-runtime, daemon reattach, ...).

**Đây chính là file trung tâm của cuộc điều tra BUG-FE-PTY-001** (xem memory
session `bug-fe-pty-001-investigation.md` và
`specs/frontend/bugs/terminal-management./BUG-FE-TM-*`) — file này gọi
`RemoteRuntimePtyTransport`/`SshPtyTransport`/local transport, xử lý retry khi
`SSH_SESSION_EXPIRED`, và là nơi fix #2 trong investigation đó
("`startFreshSpawn`'s onError ... was the one unguarded SSH_SESSION_EXPIRED
retry path") đã được áp dụng.

## Hậu quả

- 1 hàm 6,650+ dòng gần như chắc chắn có rất nhiều closure lồng nhau, nhiều
  nhánh xử lý theo loại transport (local/ssh/remote-runtime) — đúng loại rủi
  ro đã thấy trong BUG-FE-BIGFILE-002 (`orca-runtime.ts`): sửa 1 nhánh dễ ảnh
  hưởng nhánh khác nếu chia sẻ closure state mà không nhận ra.
  - Ví dụ cụ thể từ chính investigation gần đây: `remote-runtime-pty-transport.ts`
    (#99, 1,054 dòng — nhỏ hơn nhiều nhưng vẫn liên quan trực tiếp) đã cần một
    cơ chế "grace-close" mới thêm vào để tránh race giữa 2 transport cho cùng
    1 leaf — loại bug race-condition này càng khó phát hiện khi orchestration
    logic tập trung trong 1 hàm khổng lồ như `connectPanePty`.
- Không có ranh giới hàm rõ ràng để viết unit test riêng cho từng loại
  transport (local/ssh/remote) — muốn test 1 nhánh phải mock toàn bộ ngữ
  cảnh của hàm 6,650 dòng.

## Bằng chứng

```
wc -l pty-connection.ts                                → 7600
grep -n "^export " pty-connection.ts                    → chỉ 2 export (dòng 250, 943)
grep -c "useState(\|useEffect(" pty-connection.ts        → 0 (không phải component)
head -1 pty-connection.ts                                → "/* oxlint-disable max-lines */"
```

## Đề xuất fix

1. Đọc kỹ nội bộ `connectPanePty` để xác định các nhánh theo loại transport
   (local / SSH / remote-runtime / daemon-reattach) — đây thường là ranh giới
   tách tự nhiên nhất cho 1 hàm orchestration lớn.
2. Trích các closure/helper không phụ thuộc trực tiếp state của
   `connectPanePty` ra hàm module-level riêng trước (rủi ro thấp nhất — không
   đổi hành vi, chỉ đổi vị trí định nghĩa).
3. Cân nhắc pattern strategy: 1 hàm `connectPanePty` điều phối ngắn gọn, gọi
   ra `connectLocalPanePty`/`connectSshPanePty`/`connectRemoteRuntimePanePty`
   trong các file riêng theo transport — mỗi transport type đã có transport
   class riêng rồi (`remote-runtime-pty-transport.ts`, tương tự cho SSH), nên
   phần connection/retry logic tương ứng cũng có thể theo cùng ranh giới.
4. Vì file này vừa là trung tâm của 1 bug nghiêm trọng vừa được sửa (race
   condition PTY, xem memory `bug-fe-pty-001-investigation.md`), **nên viết
   thêm test coverage cho các nhánh retry/reattach TRƯỚC khi tách file** —
   tách 1 file không có test che phủ tốt có rủi ro regression cao hơn bình
   thường, đặc biệt với 1 file vừa có lịch sử bug race-condition.

## Tham khảo

- Tổng quan: `BUG-FE-BIGFILE-001`
- Investigation liên quan trực tiếp: memory
  `bug-fe-pty-001-investigation.md`, `specs/frontend/bugs/terminal-management./`
- File liên quan: `remote-runtime-pty-transport.ts` (#99 trong bảng tổng)

## Ghi chú TASK-BIGFILE-033 (test coverage trước khi tách) — 2026-08-11

Đã đọc toàn bộ `pty-connection.ts` (7,600 dòng — không phải 6,650 như mô tả
gốc, `connectPanePty` bắt đầu dòng 943 và chạy tới hết file, con số 6,650
trong solution doc là ước lượng gần đúng, không lệch về bản chất) và
`pty-connection.test.ts` (18,785 dòng, 427 test case đã có trước khi task
này chạy). Xác nhận coverage cho 4 luồng yêu cầu:

1. **Spawn mới local/SSH/remote-runtime**: đã có (nhiều test case, ví dụ
   dòng ~13601–13772 cho remote-runtime, ~4494–4907 cho SSH, và các test
   "genuine fresh spawn" cho local dùng `tabsByWorktree` với `ptyId: null`).
2. **Retry khi `SSH_SESSION_EXPIRED`**: đã có test cho nhánh
   deferred/non-deferred SSH reattach (dòng ~5854–5992, ~15900–15980). PHÁT
   HIỆN THIẾU: nhánh `mirroredHostAttachRetried` (dòng 4343–4367 trong
   `pty-connection.ts`, comment "FIX BUG-FE-PTY-001") — bound-single-retry
   riêng cho lỗi `SSH_SESSION_EXPIRED` từ `onError` của 1 fresh (non-reattach)
   spawn, khác với nhánh reattach-onError đã có test. Đã bổ sung 2 test case
   mới ("retries a fresh (non-reattach) spawn once when a mirrored host
   attach reports session-expired" và "surfaces the error instead of
   retrying again when a retried mirrored host attach also expires") —
   pass.
3. **Reattach/daemon reattach**: đã có, nhiều test case (dòng ~5583–6777).
4. **Race giữa local pane và host-session mirror pane cho cùng 1 leaf**:
   XÁC NHẬN — `attachHostSessionMirror()` (implementation thật của việc
   attach vào 1 host-published PTY handle) nằm ở
   `remote-runtime-pty-transport.ts`, KHÔNG nằm trong `pty-connection.ts`.
   Tuy nhiên phần retry-khi-mirror-chết (`mirroredHostAttachRetried`, mục 2
   ở trên) VẪN nằm trong `pty-connection.ts` — đây chính là phần "race" thuộc
   phạm vi file này (mirror publish trước khi attach hoàn tất → attach thấy
   handle chết → retry 1 lần qua fresh spawn). Đã bổ sung test che phủ ở
   mục 2. Phần còn lại của cơ chế race (grace-close, publish timing) đã có
   test riêng trong `remote-runtime-pty-transport.test.ts` — ngoài phạm vi
   task này.

Kết luận: coverage đủ để tiến hành TASK-BIGFILE-034 (Investigate). 427 test
case trong `pty-connection.test.ts` sau khi bổ sung (425 trước đó + 2 mới),
tất cả pass. `oxlint` sạch trên file test. `tsc --noEmit` không phát sinh
lỗi mới liên quan tới `pty-connection.ts`/`pty-connection.test.ts` (968 lỗi
pre-existing không liên quan, xem memory `bug-fe-pty-001-investigation.md`).
