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
