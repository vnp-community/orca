# TASK-BIGFILE-033 — Test: bổ sung coverage cho `pty-connection.ts` TRƯỚC khi tách

**Loại:** Test (bắt buộc trước khi tách, không phải Move/Investigate)
**Effort:** M · **Phụ thuộc:** — · **Status:** ✅ Done (2026-08-11)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-006-pty-connection.md` (Bước 0)

## Bối cảnh (đọc để hiểu lý do, không cần đọc lại toàn bộ solution doc)

File `frontend/src/renderer/src/components/terminal-pane/pty-connection.ts`
vừa là trung tâm của investigation `BUG-FE-PTY-001` (race condition PTY —
xem memory session `bug-fe-pty-001-investigation.md`). Tách 1 hàm 6,650 dòng
(`connectPanePty`) không có test che phủ tốt có rủi ro regression cao.

## Nhiệm vụ

Xác nhận/bổ sung test (`pty-connection.test.ts` cùng thư mục, tạo mới nếu
chưa có) cho tối thiểu 4 luồng:

1. Spawn mới thành công — local / SSH / remote-runtime (3 test case, hoặc
   xác nhận đã có).
2. Retry khi `SSH_SESSION_EXPIRED` — xác nhận có test regression cho fix đã
   áp dụng trong investigation gần đây (`startFreshSpawn`'s onError path —
   xem memory session để biết chi tiết fix nếu cần đối chiếu).
3. Reattach/daemon reattach.
4. Race giữa local pane và host-session mirror pane cho cùng 1 leaf — CHỈ
   nếu logic liên quan còn nằm trong `pty-connection.ts` (xác nhận khi đọc;
   nếu logic này thực ra nằm ở `remote-runtime-pty-transport.ts`, không phải
   file này, thì bỏ qua mục này và ghi chú lại).

## Input cần đọc

- `frontend/src/renderer/src/components/terminal-pane/pty-connection.ts`
  (toàn bộ — đây là task duy nhất trong đợt `bigfile_v1` CẦN đọc hết 1 file
  lớn, vì mục đích là viết test bao phủ, không phải trích đoạn cơ học)
- Test hiện có cùng thư mục (nếu có) để tránh trùng lặp

## Output

- Test file mới/cập nhật, tất cả pass.
- Ghi chú vào `../BUG-FE-BIGFILE-006-pty-connection.md` xác nhận coverage đã
  đủ để tiến hành TASK-BIGFILE-034.

## Xác minh xong

- [x] `pnpm run test` (phạm vi file test liên quan) pass — 427/427,
      `pty-connection.test.ts` qua `vitest run --config config/vitest.config.ts`
- [x] `pnpm run typecheck && pnpm run lint` — `oxlint` sạch trên file test;
      `tsc --noEmit -p tsconfig.json` không có lỗi mới liên quan tới
      `pty-connection.ts`/`pty-connection.test.ts` (968 lỗi pre-existing
      không liên quan, xem memory `bug-fe-pty-001-investigation.md`)
- [x] Coverage cho 4 luồng ở trên — xem ghi chú chi tiết trong
      `../BUG-FE-BIGFILE-006-pty-connection.md`. Luồng 4 (race) chỉ áp dụng
      1 phần: implementation thật của `attachHostSessionMirror` nằm ở
      `remote-runtime-pty-transport.ts` (đã có test riêng); phần
      `mirroredHostAttachRetried` (bound-retry) nằm trong `pty-connection.ts`
      và đã được bổ sung 2 test case mới trong `pty-connection.test.ts`.
