# BUG-FE-BIGFILE-001 — 111 file trong `frontend/src` vượt quá 1,000 dòng

**Mức độ:** 🟠 High (code health / maintainability)
**Status:** 🔴 Open
**Module:** `frontend/src/**` (111 file, xem bảng đầy đủ bên dưới)
**Phát hiện:** 2026-08-10, quét bằng `scripts/find-frontend-bigfiles.mjs`

---

## Mô tả

Quét toàn bộ `frontend/src/**/*.{ts,tsx}` (loại trừ `.d.ts`, giữ cả file test) với
ngưỡng 1,000 dòng cho ra **111 file** vượt ngưỡng, lớn nhất là
`frontend/src/main/runtime/orca-runtime.ts` với **26,730 dòng**.

Đây **không phải** cùng một việc với `BUG-FE-HLD-006` (240 file có comment
`eslint-disable`/`oxlint-disable max-lines`). Hai bug này giao nhau nhưng khác góc
nhìn:

- `BUG-FE-HLD-006` = vi phạm chính sách "không được tắt rule" trong `AGENTS.md`
  (đo bằng: có disable comment hay không).
- `BUG-FE-BIGFILE-001` (bug này) = vấn đề khả năng bảo trì thực tế (đo bằng: số
  dòng thực tế, bất kể có lint sạch hay không). Ngưỡng oxlint mặc định cho `.ts`
  là 300 dòng, `.tsx` là 400 dòng — cả 111 file này đều đã vượt xa các ngưỡng đó
  từ 2.5× đến gần 90× (`orca-runtime.ts`), nên gần như chắc chắn tất cả đều nằm
  trong danh sách disable của `BUG-FE-HLD-006` (đã xác nhận thủ công với top 3
  file lớn nhất — cả 3 đều có `eslint-disable max-lines` ở dòng 1).

## Phương pháp đo

Script mới `scripts/find-frontend-bigfiles.mjs` (không phụ thuộc ngoài, chỉ dùng
`node:fs`, chạy được trên cả Windows/macOS/Linux theo yêu cầu `AGENTS.md`):

```bash
node scripts/find-frontend-bigfiles.mjs --threshold=1000
# --format=json | --format=csv   → xuất máy đọc được
# --include-tests                → mặc định đã BAO GỒM file *.test.ts/*.spec.ts
# --root=frontend/src            → mặc định
```

Cách đếm dòng giống editor/oxlint: dòng trắng cuối file (do newline kết thúc
file) không tính là 1 dòng thừa.

## Phân loại mức độ

| Tier | Ngưỡng | Số file |
|---|---|---|
| 🔴 Critical | > 5,000 dòng | 10 |
| 🟠 High | 2,000 – 5,000 dòng | 35 |
| 🟡 Medium | 1,000 – 2,000 dòng | 66 |
| **Tổng** | | **111** |

10 file nhóm Critical đã được tách thành bug riêng, có phân tích cấu trúc
(export top-level, gợi ý điểm tách) — xem mục "Bug con" bên dưới. 101 file còn
lại (High + Medium) liệt kê đầy đủ trong bảng, chưa phân tích chi tiết từng
file — ưu tiên xử lý nhóm Critical trước.

## Bug con (nhóm Critical, > 5,000 dòng) & Solution

| ID | File | Dòng | Solution |
|---|---|---|---|
| [BUG-FE-BIGFILE-002](./BUG-FE-BIGFILE-002-orca-runtime.md) | `frontend/src/main/runtime/orca-runtime.ts` | 26,730 | [solution](./solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md) |
| [BUG-FE-BIGFILE-003](./BUG-FE-BIGFILE-003-taskpage.md) | `frontend/src/renderer/src/components/TaskPage.tsx` | 12,833 | [solution](./solutions/SOLUTION-FE-BIGFILE-003-taskpage.md) |
| [BUG-FE-BIGFILE-004](./BUG-FE-BIGFILE-004-sourcecontrol.md) | `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` | 8,370 | [solution](./solutions/SOLUTION-FE-BIGFILE-004-sourcecontrol.md) |
| [BUG-FE-BIGFILE-005](./BUG-FE-BIGFILE-005-githubitemdialog.md) | `frontend/src/renderer/src/components/GitHubItemDialog.tsx` | 7,852 | [solution](./solutions/SOLUTION-FE-BIGFILE-005-githubitemdialog-and-pullrequestpage.md) |
| [BUG-FE-BIGFILE-006](./BUG-FE-BIGFILE-006-pty-connection.md) | `frontend/src/renderer/src/components/terminal-pane/pty-connection.ts` | 7,600 | [solution](./solutions/SOLUTION-FE-BIGFILE-006-pty-connection.md) |
| [BUG-FE-BIGFILE-007](./BUG-FE-BIGFILE-007-pullrequestpage.md) | `frontend/src/renderer/src/components/PullRequestPage.tsx` | 7,372 | [solution](./solutions/SOLUTION-FE-BIGFILE-007-pullrequestpage.md) (→ 005) |
| [BUG-FE-BIGFILE-008](./BUG-FE-BIGFILE-008-worktreelist.md) | `frontend/src/renderer/src/components/sidebar/WorktreeList.tsx` | 6,877 | [solution](./solutions/SOLUTION-FE-BIGFILE-008-worktreelist.md) |
| [BUG-FE-BIGFILE-009](./BUG-FE-BIGFILE-009-persistence.md) | `frontend/src/main/persistence.ts` | 6,659 | [solution](./solutions/SOLUTION-FE-BIGFILE-009-persistence.md) |
| [BUG-FE-BIGFILE-010](./BUG-FE-BIGFILE-010-browserpane.md) | `frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx` | 5,841 | [solution](./solutions/SOLUTION-FE-BIGFILE-010-browserpane.md) |
| [BUG-FE-BIGFILE-011](./BUG-FE-BIGFILE-011-ipc-pty.md) | `frontend/src/main/ipc/pty.ts` | 5,185 | [solution](./solutions/SOLUTION-FE-BIGFILE-011-ipc-pty.md) |

**Thứ tự thực hiện & chiến lược chung đề xuất:**
[SOLUTION-FE-BIGFILE-001](./solutions/SOLUTION-FE-BIGFILE-001-strategy-and-sequencing.md)
(barrel/facade pattern, thứ tự rủi ro thấp → cao, xử lý gộp cặp file trùng
lặp 005/007).

## Bảng đầy đủ 111 file (sắp xếp giảm dần theo số dòng)

| # | Lines | Tier | File |
|---|---|---|---|
| 1 | 26,730 | 🔴 Critical | `frontend/src/main/runtime/orca-runtime.ts` |
| 2 | 12,833 | 🔴 Critical | `frontend/src/renderer/src/components/TaskPage.tsx` |
| 3 | 8,370 | 🔴 Critical | `frontend/src/renderer/src/components/right-sidebar/SourceControl.tsx` |
| 4 | 7,852 | 🔴 Critical | `frontend/src/renderer/src/components/GitHubItemDialog.tsx` |
| 5 | 7,600 | 🔴 Critical | `frontend/src/renderer/src/components/terminal-pane/pty-connection.ts` |
| 6 | 7,372 | 🔴 Critical | `frontend/src/renderer/src/components/PullRequestPage.tsx` |
| 7 | 6,877 | 🔴 Critical | `frontend/src/renderer/src/components/sidebar/WorktreeList.tsx` |
| 8 | 6,659 | 🔴 Critical | `frontend/src/main/persistence.ts` |
| 9 | 5,841 | 🔴 Critical | `frontend/src/renderer/src/components/browser-pane/BrowserPane.tsx` |
| 10 | 5,185 | 🔴 Critical | `frontend/src/main/ipc/pty.ts` |
| 11 | 4,969 | 🟠 High | `frontend/src/main/github/client.ts` |
| 12 | 4,886 | 🟠 High | `frontend/src/renderer/src/store/slices/worktrees.ts` |
| 13 | 4,693 | 🟠 High | `frontend/src/renderer/src/store/slices/editor.ts` |
| 14 | 4,511 | 🟠 High | `frontend/src/renderer/src/store/slices/github.ts` |
| 15 | 4,409 | 🟠 High | `frontend/src/renderer/src/hooks/useComposerState.ts` |
| 16 | 3,974 | 🟠 High | `frontend/src/shared/agent-hook-listener.ts` |
| 17 | 3,919 | 🟠 High | `frontend/src/renderer/src/components/right-sidebar/ChecksPanel.tsx` |
| 18 | 3,908 | 🟠 High | `frontend/src/shared/types.ts` |
| 19 | 3,809 | 🟠 High | `frontend/src/renderer/src/web/web-preload-api.ts` |
| 20 | 3,748 | 🟠 High | `frontend/src/renderer/src/hooks/useIpcEvents.ts` |
| 21 | 3,545 | 🟠 High | `frontend/src/renderer/src/store/slices/terminals.ts` |
| 22 | 3,466 | 🟠 High | `frontend/src/preload/api-types.ts` |
| 23 | 3,220 | 🟠 High | `frontend/src/renderer/src/components/terminal-pane/TerminalPane.tsx` |
| 24 | 3,152 | 🟠 High | `frontend/src/renderer/src/store/slices/repos.ts` |
| 25 | 3,114 | 🟠 High | `frontend/src/main/runtime/rpc/methods/terminal.ts` |
| 26 | 2,990 | 🟠 High | `frontend/src/renderer/src/components/automations/AutomationsPage.tsx` |
| 27 | 2,840 | 🟠 High | `frontend/src/renderer/src/App.tsx` |
| 28 | 2,733 | 🟠 High | `frontend/src/renderer/src/runtime/web-session-tabs-sync.ts` |
| 29 | 2,709 | 🟠 High | `frontend/src/renderer/src/components/right-sidebar/checks-panel-content.tsx` |
| 30 | 2,664 | 🟠 High | `frontend/src/main/ipc/worktree-remote.ts` |
| 31 | 2,662 | 🟠 High | `frontend/src/renderer/src/store/slices/ui.ts` |
| 32 | 2,644 | 🟠 High | `frontend/src/main/browser/agent-browser-bridge.ts` |
| 33 | 2,590 | 🟠 High | `frontend/src/renderer/src/store/slices/agent-status.ts` |
| 34 | 2,449 | 🟠 High | `frontend/src/renderer/src/components/Terminal.tsx` |
| 35 | 2,380 | 🟠 High | `frontend/src/renderer/src/components/status-bar/StatusBar.tsx` |
| 36 | 2,344 | 🟠 High | `frontend/src/renderer/src/components/WorktreeJumpPalette.tsx` |
| 37 | 2,301 | 🟠 High | `frontend/src/shared/keybindings.ts` |
| 38 | 2,253 | 🟠 High | `frontend/src/main/git/status.ts` |
| 39 | 2,244 | 🟠 High | `frontend/src/renderer/src/store/slices/linear.ts` |
| 40 | 2,145 | 🟠 High | `frontend/src/renderer/src/components/editor/CombinedDiffViewer.tsx` |
| 41 | 2,114 | 🟠 High | `frontend/src/renderer/src/store/slices/browser.ts` |
| 42 | 2,112 | 🟠 High | `frontend/src/renderer/src/components/activity/ActivityPrototypePage.tsx` |
| 43 | 2,091 | 🟠 High | `frontend/src/renderer/src/components/editor/MarkdownPreview.tsx` |
| 44 | 2,083 | 🟠 High | `frontend/src/renderer/src/components/status-bar/WorkspaceSpaceManagerPanel.tsx` |
| 45 | 2,070 | 🟠 High | `frontend/src/renderer/src/store/slices/tabs.ts` |
| 46 | 1,964 | 🟡 Medium | `frontend/src/renderer/src/components/terminal-pane/use-terminal-pane-lifecycle.ts` |
| 47 | 1,953 | 🟡 Medium | `frontend/src/renderer/src/components/sidebar/WorktreeCard.tsx` |
| 48 | 1,912 | 🟡 Medium | `frontend/src/renderer/src/components/settings/AccountsPane.tsx` |
| 49 | 1,886 | 🟡 Medium | `frontend/src/main/browser/browser-manager.ts` |
| 50 | 1,885 | 🟡 Medium | `frontend/src/main/runtime/orca-runtime-files.ts` |
| 51 | 1,857 | 🟡 Medium | `frontend/src/main/claude-accounts/runtime-auth-service.ts` |
| 52 | 1,849 | 🟡 Medium | `frontend/src/renderer/src/components/floating-terminal/FloatingTerminalPanel.tsx` |
| 53 | 1,846 | 🟡 Medium | `frontend/src/main/browser/cdp-bridge.ts` |
| 54 | 1,842 | 🟡 Medium | `frontend/src/renderer/src/components/new-workspace/SmartWorkspaceNameField.tsx` |
| 55 | 1,841 | 🟡 Medium | `frontend/src/main/runtime/orca-runtime-browser.ts` |
| 56 | 1,823 | 🟡 Medium | `frontend/src/main/browser/browser-cookie-import.ts` |
| 57 | 1,806 | 🟡 Medium | `frontend/src/main/github/project-view.ts` |
| 58 | 1,783 | 🟡 Medium | `frontend/src/main/rate-limits/service.ts` |
| 59 | 1,748 | 🟡 Medium | `frontend/src/main/agent-hooks/server.ts` |
| 60 | 1,720 | 🟡 Medium | `frontend/src/renderer/src/components/settings/Settings.tsx` |
| 61 | 1,696 | 🟡 Medium | `frontend/src/renderer/src/components/status-bar/ResourceUsageStatusSegment.tsx` |
| 62 | 1,695 | 🟡 Medium | `frontend/src/renderer/src/components/GitLabItemDialog.tsx` |
| 63 | 1,670 | 🟡 Medium | `frontend/src/main/codex-accounts/runtime-home-service.ts` |
| 64 | 1,668 | 🟡 Medium | `frontend/src/shared/telemetry-events.ts` |
| 65 | 1,662 | 🟡 Medium | `frontend/src/main/git/runner.ts` |
| 66 | 1,589 | 🟡 Medium | `frontend/src/renderer/src/components/workspace-cleanup/WorkspaceCleanupDialog.tsx` |
| 67 | 1,571 | 🟡 Medium | `frontend/src/main/codex/hook-service.ts` |
| 68 | 1,571 | 🟡 Medium | `frontend/src/renderer/src/components/LinearItemDrawer.tsx` |
| 69 | 1,533 | 🟡 Medium | `frontend/src/main/ipc/ssh.ts` |
| 70 | 1,521 | 🟡 Medium | `frontend/src/renderer/src/components/right-sidebar/PortsPanel.tsx` |
| 71 | 1,519 | 🟡 Medium | `frontend/src/main/linear/issues.ts` |
| 72 | 1,513 | 🟡 Medium | `frontend/src/renderer/src/runtime/sync-runtime-graph.ts` |
| 73 | 1,436 | 🟡 Medium | `frontend/src/renderer/src/components/NewWorkspaceComposerCard.tsx` |
| 74 | 1,417 | 🟡 Medium | `frontend/src/main/gitlab/client.ts` |
| 75 | 1,414 | 🟡 Medium | `frontend/src/renderer/src/lib/pane-manager/pane-terminal-output-scheduler.ts` |
| 76 | 1,403 | 🟡 Medium | `frontend/src/main/git/worktree.ts` |
| 77 | 1,394 | 🟡 Medium | `frontend/src/renderer/src/components/github-project/ProjectViewWrapper.tsx` |
| 78 | 1,376 | 🟡 Medium | `frontend/src/renderer/src/components/settings/RepositoryHooksSection.tsx` |
| 79 | 1,376 | 🟡 Medium | `frontend/src/renderer/src/components/settings/RuntimeEnvironmentsPane.tsx` |
| 80 | 1,356 | 🟡 Medium | `frontend/src/renderer/src/components/sidebar/worktree-list-groups.ts` |
| 81 | 1,343 | 🟡 Medium | `frontend/src/renderer/src/components/tab-bar/TabBar.tsx` |
| 82 | 1,329 | 🟡 Medium | `frontend/src/renderer/src/components/onboarding/use-onboarding-flow.ts` |
| 83 | 1,310 | 🟡 Medium | `frontend/src/main/ssh/ssh-relay-session.ts` |
| 84 | 1,291 | 🟡 Medium | `frontend/src/main/rate-limits/claude-fetcher.ts` |
| 85 | 1,283 | 🟡 Medium | `frontend/src/shared/source-control-ai.ts` |
| 86 | 1,258 | 🟡 Medium | `frontend/src/main/linear/projects.ts` |
| 87 | 1,225 | 🟡 Medium | `frontend/src/main/ssh/ssh-connection.ts` |
| 88 | 1,208 | 🟡 Medium | `frontend/src/main/github/work-item-details.ts` |
| 89 | 1,192 | 🟡 Medium | `frontend/src/main/ssh/ssh-relay-deploy.ts` |
| 90 | 1,191 | 🟡 Medium | `frontend/src/main/codex-usage/scanner.ts` |
| 91 | 1,189 | 🟡 Medium | `frontend/src/main/cli/cli-installer.ts` |
| 92 | 1,163 | 🟡 Medium | `frontend/src/renderer/src/runtime/runtime-file-client.ts` |
| 93 | 1,157 | 🟡 Medium | `frontend/src/main/git/repo.ts` |
| 94 | 1,151 | 🟡 Medium | `frontend/src/renderer/src/components/github-project/ProjectCell.tsx` |
| 95 | 1,142 | 🟡 Medium | `frontend/src/main/rate-limits/codex-fetcher.ts` |
| 96 | 1,127 | 🟡 Medium | `frontend/src/main/providers/local-pty-provider.ts` |
| 97 | 1,066 | 🟡 Medium | `frontend/src/main/claude-accounts/service.ts` |
| 98 | 1,060 | 🟡 Medium | `frontend/src/shared/runtime-types.ts` |
| 99 | 1,054 | 🟡 Medium | `frontend/src/renderer/src/components/terminal-pane/remote-runtime-pty-transport.ts` |
| 100 | 1,039 | 🟡 Medium | `frontend/src/renderer/src/components/feature-wall/BrowserAnimatedVisual.tsx` |
| 101 | 1,036 | 🟡 Medium | `frontend/src/renderer/src/components/terminal-pane/pty-transport.ts` |
| 102 | 1,035 | 🟡 Medium | `frontend/src/main/opencode-usage/scanner.ts` |
| 103 | 1,035 | 🟡 Medium | `frontend/src/main/text-generation/commit-message-text-generation.ts` |
| 104 | 1,034 | 🟡 Medium | `frontend/src/main/attribution/terminal-attribution.ts` |
| 105 | 1,030 | 🟡 Medium | `frontend/src/main/codex-accounts/service.ts` |
| 106 | 1,029 | 🟡 Medium | `frontend/src/renderer/src/components/UpdateCard.tsx` |
| 107 | 1,025 | 🟡 Medium | `frontend/src/renderer/src/components/LinearIssueWorkspace.tsx` |
| 108 | 1,025 | 🟡 Medium | `frontend/src/renderer/src/components/editor/EditorContent.tsx` |
| 109 | 1,012 | 🟡 Medium | `frontend/src/main/runtime/orchestration/db.ts` |
| 110 | 1,005 | 🟡 Medium | `frontend/src/renderer/src/components/settings/AgentsPane.tsx` |
| 111 | 1,003 | 🟡 Medium | `frontend/src/renderer/src/components/editor/IpynbViewer.tsx` |

## Hậu quả

- Review khó: 1 PR chạm vào 1 trong 10 file Critical gần như chắc chắn phải kéo
  qua hàng nghìn dòng không liên quan để hiểu context.
- Risk khi sửa cao hơn: file càng lớn, càng nhiều trạng thái/nhánh logic đan
  xen trong cùng 1 scope — bằng chứng trực tiếp: điều tra BUG-FE-PTY-001
  (`specs/frontend/bugs/terminal-management./`, và memory
  `bug-fe-pty-001-investigation.md`) phải đọc sâu vào chính
  `pty-connection.ts` (7,600 dòng, #6 trong bảng) và
  `remote-runtime-pty-transport.ts` (1,054 dòng, #99) — cả hai đều nằm trong
  danh sách này.
- `orca-runtime.ts` (26,730 dòng) là 1 class `OrcaRuntimeService` ôm toàn bộ
  live graph, PTY handles, waiters, mobile floor/layout state, quản lý
  worktree — theo đúng comment giải thích lý do disable ngay dòng 1 của chính
  file đó.

## Đề xuất fix

1. Không tách tất cả 111 file cùng lúc — bắt đầu từ 10 file Critical (bug con
   `BUG-FE-BIGFILE-002` → `011`), mỗi file có gợi ý điểm tách cụ thể dựa trên
   các export top-level hiện có.
2. Với các file component `.tsx` lớn (`TaskPage.tsx`, `SourceControl.tsx`,
   `GitHubItemDialog.tsx`, `PullRequestPage.tsx`, `BrowserPane.tsx`,
   `WorktreeList.tsx`) — tách theo pattern container/presentational: giữ 1 file
   "orchestrator" nhỏ, chuyển các sub-component/hook nội bộ ra file riêng.
3. Với file service/class lớn (`orca-runtime.ts`, `persistence.ts`,
   `pty-connection.ts`, `pty.ts`) — tách theo domain trách nhiệm (đã có gợi ý
   trong từng bug con), tương tự cách `orca-runtime-files.ts` (#50, 1,885
   dòng) và `orca-runtime-browser.ts` (#55, 1,841 dòng) đã được tách RA KHỎI
   `orca-runtime.ts` trước đây — chứng minh pattern tách này đã có tiền lệ
   trong chính codebase, chỉ chưa làm triệt để.
4. Sau khi tách xong nhóm Critical, chạy lại
   `node scripts/find-frontend-bigfiles.mjs` để cập nhật bảng và đánh giá lại
   ngưỡng ưu tiên cho nhóm High.
5. Cân nhắc thêm `pnpm check:bigfiles` vào `package.json` (không chặn CI, chỉ
   báo cáo) để theo dõi xu hướng tăng/giảm theo thời gian, tương tự
   `check:max-lines-ratchet` nhưng không phải ratchet chặn — chỉ mục đích quan
   sát.

## Tham khảo

- Script: `scripts/find-frontend-bigfiles.mjs`
- Chính sách liên quan: `AGENTS.md` → "Lint Rules: Do Not Disable Max Lines"
- Bug liên quan: `BUG-FE-HLD-006` (`specs/frontend/bugs/hld-v1/`)
- Ratchet hiện có: `config/max-lines-baseline.txt`,
  `config/scripts/check-max-lines-ratchet.mjs`
