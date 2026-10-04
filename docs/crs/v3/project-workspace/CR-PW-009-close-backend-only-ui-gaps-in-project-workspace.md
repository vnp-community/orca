# CR-PW-009 — Đóng khoảng trống "backend đã xong, UI mồ côi" trong Project Workspace (Beta)

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-PW-009 |
| **Tên** | 3 RPC/tính năng đã implement + test đầy đủ ở backend nhưng không có UI trigger nào trong Project Workspace (Beta); 1 bug UX nhỏ (terminal thiếu `startupCwd`) |
| **Loại** | Feature Completion / Bug Fix |
| **Priority** | 🟡 P1 (giá trị sản phẩm cao — tính năng đã trả tiền phát triển, chỉ thiếu nút bấm) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-15 |
| **Trạng thái** | 🔲 Proposed |
| **Tác giả** | Investigation trong lúc trả lời câu hỏi user về luồng dùng Jira + chọn account AI cho session |
| **Tác động HLD** | `frontend/src/renderer/src/components/workspace/AgentPanel.tsx`, `WorkspaceTerminalPanel.tsx`, Task page Jira browse UI |
| **Tác động Features** | Agent tab (đổi account AI), tạo worktree trực tiếp từ Jira issue, terminal tab tự động cd đúng thư mục |

---

## Bối cảnh & Vấn đề gốc — 3 gap "backend xong, UI mồ côi" + 1 bug nhỏ

Cùng 1 pattern đã gặp lặp lại nhiều lần trong specs (`BUG-FE-PW-002`'s `orcaProjects.*` là ví dụ gốc) — lần này phát hiện thêm 2 trường hợp mới khi trả lời câu hỏi trực tiếp của user:

### 1. `agent.switchAccount` — đổi account AI cho 1 session đang chạy — không có UI

⚠️ **CẬP NHẬT (2026-09-15, cùng ngày viết CR này):** phát hiện `AgentPanel.tsx` (nơi lẽ ra thêm UI này vào) tự nó có thể **hoàn toàn không dùng được trên web** — xem [BUG-FE-PW-005](../../../../specs/frontend/bugs/project-workspace/BUG-FE-PW-005-agent-panel-window-api-undefined-on-web.md). **Nên hoãn việc thêm switchAccount UI cho tới khi BUG-FE-PW-005 được xác nhận/fix** — xây thêm nút trên 1 panel nghi ngờ đã hỏng toàn bộ không có giá trị.


Channel `agent.switchAccount` (`channels_agent.go`) → `SwitchAgentAccount` usecase (infra-fleet-service): kill session cũ → resolve account mới qua `AIProviderResolverClient` → start session mới → attach lại PTY stream, đầy đủ saga. **Grep toàn bộ frontend: 0 caller.** `AgentPanel.tsx` chỉ có dropdown chọn LOẠI CLI (Claude/Codex/Custom) lúc start, không có cách đổi ACCOUNT cụ thể (khi có nhiều account cùng loại) cho session đang chạy.

### 2. `worktree.createFromIssue` — tạo worktree trực tiếp từ Jira issue — không có UI cho RPC MỚI, chỉ có đường vòng qua Composer cũ

Sửa lại nhận định ban đầu: Task page **có sẵn** nút "Use"/"Start workspace" trên mỗi Jira issue (`onStartWorkspace` → `TaskPage.tsx`'s `handleUseJiraItem` → `openComposerForJiraItem(issue)`) — mở dialog Composer cũ, điền sẵn thông tin issue, user tự chọn project/repo/branch rồi submit thủ công. **Đây KHÔNG phải** `worktree.createFromIssue` (RPC mới, backend-go, tự đặt tên nhánh + tự spawn agent + tự đẩy status Jira) — grep xác nhận vẫn **0 caller** cho RPC mới này.

→ Gap thật hẹp hơn ban đầu tưởng: user **có** cách bắt đầu làm việc từ Jira issue hôm nay (qua Composer, thủ công hơn), chỉ là **không có** đường tắt tự động hoá đầy đủ mà RPC mới cung cấp. Cần xác nhận thêm: Composer cũ có thực sự tạo worktree đúng trên hệ backend-go hiện tại (nhiều dual-system issue đã gặp trong phiên này) hay không, trước khi quyết định có đáng để nối thêm RPC mới vào hay build song song.

### 3. Jira self-hosted (Server/Data Center) không kết nối được — xem CR riêng

Xem [CR-JIRA-001](../jira-integration/CR-JIRA-001-support-self-hosted-jira-server-data-center.md) — không gộp vào đây vì phạm vi khác (backend adapter, không phải UI).

### 4. Bug nhỏ: `WorkspaceTerminalPanel` thiếu `startupCwd`

Terminal mở trong tab Terminal của Project Workspace không tự `cd` vào đúng working directory của worktree — component duy nhất gọi `createNewTerminalTab` mà không truyền `startupCwd`, khác mọi nơi khác trong codebase. Xem [BUG-FE-PW-004](../../../../specs/frontend/bugs/project-workspace/BUG-FE-PW-004-workspace-terminal-panel-missing-startup-cwd.md).

## Giải pháp đề xuất

1. **`agent.switchAccount`**: thêm UI vào `AgentPanel.tsx` — dropdown/nút "Switch account" khi có ≥2 account cùng loại CLI đã cấu hình (đọc từ `ai-provider-service`'s account list, đã có sẵn cho Settings → AI Provider Account). Ưu tiên P1 vì backend đã xong hoàn toàn, effort UI thấp.
2. **`worktree.createFromIssue`**: thêm nút "Create worktree" trên mỗi Jira issue trong Task page's Jira browse list, gọi thẳng RPC này. Effort UI thấp, giá trị cao (đây là chính tính năng cốt lõi user hỏi "luồng dùng Jira như nào").
3. **`WorkspaceTerminalPanel`**: 1-dòng fix, thêm `startupCwd={currentWorktree.path}` khi gọi `createNewTerminalTab`.

Cả 3 đều KHÔNG cần thay đổi backend — thuần frontend, effort thấp, giá trị cao (đóng gap tính năng đã trả chi phí phát triển).

## Không thuộc phạm vi CR này

- Thiết kế lại toàn bộ luồng chọn account AI cấp hệ thống (Settings → AI Provider Account) — đã có, chỉ thiếu chỗ ÁP DỤNG nó vào 1 session cụ thể đang chạy.

## Liên quan

- [BUG-FE-PW-004](../../../../specs/frontend/bugs/project-workspace/BUG-FE-PW-004-workspace-terminal-panel-missing-startup-cwd.md)
- [BUG-FE-PW-002](../../../../specs/frontend/bugs/project-workspace/BUG-FE-PW-002-orcaproject-source-project-sharing-no-ui.md) — cùng pattern gốc, ví dụ đầu tiên của lớp bug này
