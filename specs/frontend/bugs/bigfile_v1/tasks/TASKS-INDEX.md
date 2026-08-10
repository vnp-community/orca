# TASKS-INDEX — `bigfile_v1` execution tasks

**Nguồn:** `../solutions/SOLUTION-FE-BIGFILE-*.md`
**Mục tiêu thiết kế:** mỗi task file **tự đủ** (self-contained) — AI thực thi
1 task **chỉ cần đọc đúng file task đó**, KHÔNG cần đọc solution doc, bug doc,
hay toàn bộ file nguồn khổng lồ. Điều này tối thiểu hoá token cost: task
"Move" chỉ định rõ số dòng cần đọc (thường vài trăm dòng, không phải hàng
nghìn); task "Investigate" giới hạn rõ phạm vi đọc.

## Cách dùng

1. Đọc **đúng 1 dòng** trong bảng dưới để biết task tiếp theo cần làm (theo
   thứ tự `#`, tôn trọng cột **Phụ thuộc**).
2. Mở **đúng 1 file** `TASK-BIGFILE-NNN-*.md`.
3. Thực thi theo đúng "Các bước" trong file đó — không cần mở file nào khác
   trừ khi task yêu cầu rõ.
4. Đánh dấu `[x]` trong bảng dưới + đổi `Status` trong task file khi xong.

## 2 loại task

- **Move** — cơ học, ranh giới dòng đã xác định sẵn, rủi ro thấp, không cần
  thiết kế lại. Thực thi trực tiếp theo template.
- **Investigate** — cần đọc 1 vùng code lớn/không có ranh giới sẵn để THIẾT
  KẾ (không thực thi ngay). Output là 1 ghi chú thiết kế + có thể sinh thêm
  task Move mới vào chính thư mục này (đặt tên tiếp theo dãy số hiện có).

## Bảng tổng hợp (thứ tự thực hiện — theo `SOLUTION-FE-BIGFILE-001`)

| # | Task | Loại | File đích/nguồn | Effort | Phụ thuộc | Status |
|---|---|---|---|---|---|---|
| 1 | [TASK-BIGFILE-001](./TASK-BIGFILE-001-ipc-pty-pane-key-registry.md) | Move | `ipc/pty.ts` → `pty-pane-key-registry.ts` | S (thực tế: state dùng chéo pervasive) | — | ⛔ Blocked |
| 2 | [TASK-BIGFILE-002](./TASK-BIGFILE-002-ipc-pty-startup-color-query.md) | Move | `ipc/pty.ts` → `pty-startup-color-query.ts` | S (thực tế: state dùng chéo pervasive) | — | ⛔ Blocked |
| 3 | [TASK-BIGFILE-003](./TASK-BIGFILE-003-ipc-pty-host-env.md) | Move | `ipc/pty.ts` → `pty-host-env.ts` | S (thực tế: state dùng chéo pervasive) | — | ⛔ Blocked |
| 4 | [TASK-BIGFILE-004](./TASK-BIGFILE-004-ipc-pty-ownership-registry.md) | Move | `ipc/pty.ts` → `pty-ownership-registry.ts` | S (thực tế: state dùng chéo pervasive) | — | ⛔ Blocked |
| 5 | [TASK-BIGFILE-005](./TASK-BIGFILE-005-ipc-pty-renderer-delivery-debug.md) | Move | `ipc/pty.ts` → `pty-renderer-delivery-debug.ts` | S (thực tế: state dùng chéo pervasive) | — | ⛔ Blocked |
| 6 | [TASK-BIGFILE-006](./TASK-BIGFILE-006-ipc-pty-provider-listener-binding.md) | Move | `ipc/pty.ts` → `pty-provider-listener-binding.ts` | S (thực tế: state dùng chéo pervasive) | — | ⛔ Blocked |
| 7 | [TASK-BIGFILE-007](./TASK-BIGFILE-007-ipc-pty-register-handlers-investigate.md) | Investigate | `ipc/pty.ts` `registerPtyHandlers` | M | 1–6 | ⛔ Blocked (chặn bởi 1–6) |
| 8 | [TASK-BIGFILE-008](./TASK-BIGFILE-008-orca-runtime-tail-buffer.md) | Move | `orca-runtime.ts` → `orca-runtime-tail-buffer.ts` | S (thực tế: L) | — | ✅ Done (`596be55bc`) |
| 9 | [TASK-BIGFILE-009](./TASK-BIGFILE-009-orca-runtime-types.md) | Move | `orca-runtime.ts` → `orca-runtime-types.ts` | M | — | ⬜ |
| 10 | [TASK-BIGFILE-010](./TASK-BIGFILE-010-persistence-paths.md) | Move | `persistence.ts` → `persistence-paths.ts` | S | — | ⬜ |
| 11 | [TASK-BIGFILE-011](./TASK-BIGFILE-011-persistence-migration.md) | Investigate+Move | `persistence.ts` → `persistence-migration.ts` | M | — | ⬜ |
| 12 | [TASK-BIGFILE-012](./TASK-BIGFILE-012-worktreelist-helpers.md) | Investigate+Move | `WorktreeList.tsx` → `worktree-list-helpers.ts` | M | — | ⬜ |
| 13 | [TASK-BIGFILE-013](./TASK-BIGFILE-013-worktreelist-visibility-listener.md) | Move | `WorktreeList.tsx` → `worktree-list-visibility-listener.ts` | S | — | ⬜ |
| 14 | [TASK-BIGFILE-014](./TASK-BIGFILE-014-browserpane-annotation-card.md) | Move | `BrowserPane.tsx` → `browser-pane-annotation-card.tsx` | S | — | ⬜ |
| 15 | [TASK-BIGFILE-015](./TASK-BIGFILE-015-browserpane-remote.md) | Move | `BrowserPane.tsx` → `browser-pane-remote.tsx` | M | — | ⬜ |
| 16 | [TASK-BIGFILE-016](./TASK-BIGFILE-016-browserpane-local.md) | Move | `BrowserPane.tsx` → `browser-pane-local.tsx` | M | 15 | ⬜ |
| 17 | [TASK-BIGFILE-017](./TASK-BIGFILE-017-github-item-dialog-shared.md) | Move | `GitHubItemDialog.tsx` + `PullRequestPage.tsx` → `github-item-dialog-shared.ts` | M | — | ⬜ |
| 18 | [TASK-BIGFILE-018](./TASK-BIGFILE-018-githubitemdialog-tabs-investigate.md) | Investigate | `GitHubItemDialog.tsx` (component chính) | L | 17 | ⬜ |
| 19 | [TASK-BIGFILE-019](./TASK-BIGFILE-019-pullrequestpage-tabs-investigate.md) | Investigate | `PullRequestPage.tsx` (component chính) | L | 17 | ⬜ |
| 20 | [TASK-BIGFILE-020](./TASK-BIGFILE-020-sourcecontrol-helpers.md) | Move | `SourceControl.tsx` → `source-control-helpers.ts` | S | — | ⬜ |
| 21 | [TASK-BIGFILE-021](./TASK-BIGFILE-021-sourcecontrol-commit-area.md) | Move | `SourceControl.tsx` → `source-control-commit-area.tsx` | M | — | ⬜ |
| 22 | [TASK-BIGFILE-022](./TASK-BIGFILE-022-sourcecontrol-compare-summary.md) | Move | `SourceControl.tsx` → `source-control-compare-summary.tsx` | M | — | ⬜ |
| 23 | [TASK-BIGFILE-023](./TASK-BIGFILE-023-sourcecontrol-banners.md) | Move | `SourceControl.tsx` → `source-control-banners.tsx` | S | — | ⬜ |
| 24 | [TASK-BIGFILE-024](./TASK-BIGFILE-024-sourcecontrol-tree-rows.md) | Move | `SourceControl.tsx` → `source-control-tree-rows.tsx` | M | — | ⬜ |
| 25 | [TASK-BIGFILE-025](./TASK-BIGFILE-025-sourcecontrol-action-button.md) | Move | `SourceControl.tsx` → `source-control-action-button.tsx` | S | — | ⬜ |
| 26 | [TASK-BIGFILE-026](./TASK-BIGFILE-026-sourcecontrolinner-investigate.md) | Investigate | `SourceControl.tsx` `SourceControlInner` | L | 20–25 | ⬜ |
| 27 | [TASK-BIGFILE-027](./TASK-BIGFILE-027-taskpage-types.md) | Move | `TaskPage.tsx` → `task-page-types.ts` | S | — | ⬜ |
| 28 | [TASK-BIGFILE-028](./TASK-BIGFILE-028-taskpage-linear-cells.md) | Move | `TaskPage.tsx` → `task-page-linear-cells.tsx` | S | 27 | ⬜ |
| 29 | [TASK-BIGFILE-029](./TASK-BIGFILE-029-taskpage-jira-banner.md) | Move | `TaskPage.tsx` → `task-page-jira-banner.tsx` | S | — | ⬜ |
| 30 | [TASK-BIGFILE-030](./TASK-BIGFILE-030-taskpage-github-cells.md) | Move | `TaskPage.tsx` → `task-page-github-cells.tsx` | M | — | ⬜ |
| 31 | [TASK-BIGFILE-031](./TASK-BIGFILE-031-taskpage-pagination.md) | Move | `TaskPage.tsx` → `task-page-pagination.tsx` | S | — | ⬜ |
| 32 | [TASK-BIGFILE-032](./TASK-BIGFILE-032-taskpage-state-investigate.md) | Investigate | `TaskPage.tsx` component chính (83 state) | L | 27–31 | ⬜ |
| 33 | [TASK-BIGFILE-033](./TASK-BIGFILE-033-pty-connection-test-coverage.md) | Test | `pty-connection.ts` | M | — | ⬜ |
| 34 | [TASK-BIGFILE-034](./TASK-BIGFILE-034-pty-connection-branches-investigate.md) | Investigate | `pty-connection.ts` `connectPanePty` | L | 33 | ⬜ |
| 35 | [TASK-BIGFILE-035](./TASK-BIGFILE-035-orca-runtime-service-domains-investigate.md) | Investigate | `orca-runtime.ts` `OrcaRuntimeService` | L | 8, 9 | ⬜ |

**Effort:** S = nhỏ (<30 phút, 1 file, <300 dòng di chuyển) · M = trung bình
(vài trăm–~1,000 dòng, hoặc cần đọc thêm để xác nhận ranh giới) · L = lớn
(cần đọc/thiết kế sâu, không có ranh giới sẵn — luôn là loại Investigate).

## ⚠️ Phát hiện khi thực thi (2026-08-10)

TASK-001 đến 007 (`ipc/pty.ts`) đã được kiểm tra thực tế (đọc code quanh các
symbol đích, dòng 210–5145) và **bị đánh dấu ⛔ Blocked** — KHÔNG thực thi.
Lý do: các symbol đích (`getPtyIdForPaneKey`, `registerPaneKeyTeardownListener`,
`hasPendingRendererSerializerForPaneKey`, `getProvider`, `getProviderForPty`,
`tryGetProviderForPty`, `getAppPtyId`, `getRelayPtyId`,
`stripRemotePaneEnvWhenHooksDisabled`, `hasPtyProviderForInspection`,
`getProviderForStartupTerminalColorReply`, ...) không tách rời được cơ học
như task doc mô tả — chúng chia sẻ state module-private
(`paneKeyPtyId`, `ptyPaneKey`, `pendingByPaneKey`,
`paneSpawnReservationsByPaneKey`, `ptyPendingGenByPtyId`,
`rendererSerializerByPtyId`, `paneKeyTeardownListeners`) và được gọi tại
40+ điểm rải khắp `registerPtyHandlers` (dòng 1140–5145). Đây là lỗi thiết
kế của chính task doc gốc (dựa trên `grep -n "^export"` nông, không phát
hiện được state/helper private dùng chéo — cùng loại lỗi đã gặp và sửa được
ở TASK-008, nhưng ở đây mức độ đan xen sâu hơn nhiều, không đủ an toàn để
tự ý tách khi chưa có thiết kế lại).

**Việc cần làm trước khi mở lại nhóm task này**: viết lại TASK-001–007 thành
1 (hoặc vài) task **Investigate** (không phải Move) cho toàn bộ
pane-key-registry + provider-resolution state trong `ipc/pty.ts`, output là
ghi chú thiết kế xác định ranh giới thật (có thể là "trích cả object/class
đóng gói state" thay vì trích từng hàm rời) — theo đúng khuôn của TASK-007
(vốn đã đúng là Investigate) — trước khi sinh lại các task Move con.

**Task Investigate không tự thực thi split** — output là 1 ghi chú thiết kế
(+ có thể là các task Move mới, đặt tên tiếp `TASK-BIGFILE-036`, `037`, ...
theo đúng format các task hiện có). Không đoán trước cấu trúc trong task
Investigate — đó chính là lý do nó tách biệt khỏi task Move.

## Nguyên tắc chung cho MỌI task (không lặp lại trong từng file)

1. `gitnexus impact({target: "<symbol>", direction: "upstream"})` trước khi
   di chuyển bất kỳ symbol nào — dừng và báo cáo nếu risk = HIGH/CRITICAL.
2. Copy nguyên văn, không đổi logic (refactor nội dung là việc khác, sau).
3. Barrel re-export tại file nguồn — không đổi import ở bất kỳ nơi nào khác.
4. Sau khi xong: `pnpm run typecheck && pnpm run lint`, chạy test liên quan
   nếu có, `gitnexus detect_changes({scope: "all"})`,
   `node scripts/find-frontend-bigfiles.mjs` để xác nhận số dòng giảm đúng
   dự kiến.
5. 1 task = 1 commit riêng.

## Tham khảo

- Chiến lược & thứ tự đầy đủ: `../solutions/SOLUTION-FE-BIGFILE-001-strategy-and-sequencing.md`
- Bug tổng quan: `../BUG-FE-BIGFILE-001-frontend-oversized-files-overview.md`
