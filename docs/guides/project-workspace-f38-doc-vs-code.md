# Project Workspace (F38): tài liệu vs. code thật, và phương án merge

**Cập nhật:** 2026-08-13

> Nối tiếp [terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md)
> (mô hình `OrcaProject` nói chung) — tài liệu này đi sâu riêng vào
> [F38 — Project Workspace](../features/F38-project-workspace.md): đối chiếu từng phần giữa đặc
> tả và code thật, rồi đưa phương án hợp nhất.
>
> Phần Task/Automation/AI orchestration (F37, F14, và hệ điều phối multi-agent) được tách riêng
> sang [task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md).
> Kế hoạch thực thi (thứ tự, phụ thuộc) xem
> [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md).

## 1. F38 mô tả gì

Khi chọn 1 Project, toàn bộ giao diện chuyển sang **Project Workspace** — môi trường hợp nhất
gồm 5 tab ngang hàng (**Explorer, Git, Agent, Workflows, Tasks**) + terminal panel dưới cùng.
Mọi thao tác chạy trực tiếp trên dev server của project qua relay. Trạng thái doc: "🚧 Phát
triển", P0, v5.0+.

## 2. Code thật: đối chiếu từng phần

### 2.1 Cấu trúc layout — khác cấu trúc, không chỉ khác chi tiết

| | Doc | Code thật (`WorkspaceLayout.tsx`) |
|---|---|---|
| Explorer | 1 tab ngang hàng Git/Agent/Workflows/Tasks | **Panel cố định bên trái, luôn hiển thị** — không phải tab |
| Tab bar | `[Explorer] [Git] [Agent] [Tasks]` (4, có Explorer) | `git \| tasks \| workflows \| agent` (4, **không có Explorer**, có thêm Workflows) |
| Panel phải | Không nhắc tới | Panel chi tiết có thể ẩn/hiện, nội dung theo tab (`git`/`tasks` mới có nội dung) |
| Terminal | Bottom panel, tiêu chí chấp nhận rõ ràng "PTY sessions on project's dev server" | `<div>Terminal — coming soon</div>` — **placeholder, chưa nối PTY** |

### 2.2 Tab "Agent" — có nút, bấm vào trống trơn (bug thật, không chỉ lệch doc)

`WorkspaceTabBar.tsx` render đủ 4 nút tab, bấm được, kể cả "Agent" (icon `Bot`). Nhưng
`WorkspaceLayout.tsx` chỉ có 3 nhánh render:
```typescript
{activeTab === 'git'       && <GitPanel />}
{activeTab === 'tasks'     && <TaskGraphPanel projectId={project.id} />}
{activeTab === 'workflows' && <WorkflowMonitor />}
// KHÔNG có nhánh cho 'agent' → panel giữa trống khi chọn tab Agent
```

**Quan trọng**: `AgentPanel.tsx` (287 dòng) **không phải scaffolding chết** — tag
`BUG-FE-ORCH-001`, dùng `useAppStore`/`RemoteAgentSession` từ `store/slices/remote-agent-sessions`
(store THẬT, đang chạy, cùng hệ thống agent orchestration dùng ở mọi nơi khác trong app). Nó
cần prop `worktreeId: string` — chỉ đơn giản là **chưa được nối vào `WorkspaceLayout.tsx`**,
không phải phải viết lại từ đầu. Vướng mắc duy nhất: `currentWorktree` trong `WorkspaceContext`
là state chết (không ai gọi `setCurrentWorktree()`), nên chưa có `worktreeId` thật để truyền
vào.

### 2.3 Tên file/component lệch bảng "Yêu cầu kỹ thuật" của doc

| Doc liệt kê | Thực tế |
|---|---|
| `ProjectSelector.tsx` | Không tồn tại — có `ProjectSwitcher.tsx` |
| `WorkflowsPanel.tsx` | Không tồn tại — dùng `WorkflowMonitor` (từ `components/workflow/`) |
| `TasksPanel.tsx` | Không tồn tại — dùng `TaskGraphPanel` (từ `components/task/`) |
| `RemoteFileViewer.tsx` | Không tồn tại — có `FileViewer.tsx` (chưa rõ tương đương chức năng) |
| `ServerStatusBar.tsx` | Không tồn tại — **nhưng app đã có sẵn** `RuntimeHostStatusRow.tsx`/`SshStatusSegment.tsx` (status-bar chính, đang chạy thật) làm cùng việc |

### 2.4 `WorkspaceContext` shape cũng lệch

Doc: `{ project, devServer, connection, resolvedProfile, currentWorktree }`.
Code thật: `{ project, isOffline, isInitializing, gitStatus, fileTree, resolvedProfile,
activeAgentSessionId, currentWorktree, switchProject, refreshGitStatus, refreshFileTree,
setCurrentWorktree, emit, on }` — không có `devServer`/`connection`, có thêm
`gitStatus`/`fileTree`/`isOffline`/`isInitializing` mà doc không mô tả.

### 2.5 Code xây RỘNG hơn doc ở mảng Git

`components/workspace/git/` có 8 file (`BranchManager`, `CommitForm`, `DiffViewer`,
`GitHistory`, `GitPanel`, `PullRequestForm`, `PullRequestList`, `StagingArea`) — 1 hệ thống Git
UI khá đầy đủ mà F38 hoàn toàn không mô tả chi tiết. Cộng thêm `FileContextMenu`,
`FileSearchPanel`, `FileTreeNode`, `NoProjectSelected`, `OfflineBanner`,
`WorkspaceSkeletonLoader`, `WorkspaceTabBar` — tổng **18 file** trong `components/workspace/`,
nhiều hơn hẳn 9 file doc liệt kê.

### 2.6 Vẫn là "mounted nhưng không ai thấy" — như đã ghi ở guide trước

`WorkspaceLayout.tsx`/`ProjectSwitcher.tsx`/`ProjectSettings.tsx` **không được import/render ở
bất kỳ đâu trong cây UI thật** — chỉ tồn tại trong file test của chính chúng. Toàn bộ 18 file
trên, dù chất lượng code không tệ (đặc biệt `AgentPanel.tsx`/Git subsystem), **hiện không ai
dùng được**.

## 3. Đánh giá từng khác biệt — giữ code, sửa code, hay cần quyết định?

| Khác biệt | Đề xuất | Vì sao |
|---|---|---|
| Explorer = panel cố định thay vì tab | **Cập nhật doc theo code** | Panel cố định tiện hơn — không mất Explorer khi chuyển tab. Đây là cải tiến, không phải thiếu sót. |
| Tab "Agent" trống | **Sửa code** | Chỉ cần nối `AgentPanel` vào nhánh render + có `worktreeId` thật. `AgentPanel.tsx` đã sẵn, không viết lại. |
| Terminal placeholder | **Sửa code** | Tái dùng hạ tầng PTY/terminal-pane đã có sẵn và chạy ổn ở app chính, không xây mới từ đầu. |
| Tên file lệch (`ProjectSelector` vs `ProjectSwitcher`...) | **Cập nhật doc theo code** | Đổi tên code đang chạy để khớp doc cũ không có giá trị, chỉ tốn công. |
| `WorkspaceContext` thiếu `devServer`/`connection` | **Cập nhật doc theo code** | 2 field đó có thể lấy qua `project.devServerId` sẵn có — không cần thêm field mới vào context. |
| `ServerStatusBar.tsx` chưa xây | **Tái dùng, không xây mới** | Ghép `RuntimeHostStatusRow`/`SshStatusSegment` (đã chạy thật) vào Workspace thay vì viết component riêng trùng chức năng. |
| Git subsystem rộng hơn doc | **Cập nhật doc theo code** | Ghi nhận đúng những gì đã xây — đây là phần hoàn thiện nhất, nên là phần đầu tiên tài liệu hoá tử tế. |
| `currentWorktree` chưa bao giờ được set | **Cần quyết định trước** | Nguồn cấp `worktreeId` cho Workspace là gì — chọn từ sidebar hiện có, hay Workspace tự có bộ chọn worktree riêng? Chưa có câu trả lời trong doc lẫn code. |

## 4. Phương án merge — theo mức độ ưu tiên

**Bước 0 (làm trước tất cả, quan trọng nhất): quyết định có hoàn thiện F38 hay không.**
Toàn bộ 18 file này đã tồn tại nhiều tháng mà chưa ai mount vào UI thật — trước khi đầu tư thêm
effort (sửa tab Agent, nối terminal...), cần câu trả lời rõ: **có kế hoạch release F38 hay
không?** Nếu không, nên dán nhãn rõ ràng (comment ở đầu mỗi file, hoặc xoá hẳn) thay vì để code
chết tiếp tục tích luỹ — matching bài học từ `workspace-slice.ts` (càng để lâu, càng dễ tái diễn
kiểu bug "đè lên code thật" đã gặp tuần này.

**Nếu quyết định tiếp tục, theo thứ tự:**

1. **Viết lại F38 theo code thật** (không phải ngược lại) — cập nhật layout diagram, bảng "Yêu
   cầu kỹ thuật" (tên file đúng), shape `WorkspaceContext`, và bổ sung mô tả Git subsystem đang
   thiếu hoàn toàn trong doc gốc.
2. **Quyết định nguồn `currentWorktree`** — đây là khoá cho cả tab Agent lẫn terminal panel.
   Đề xuất: tái dùng cơ chế chọn worktree đã có ở sidebar chính (`WorktreeList.tsx`) thay vì xây
   bộ chọn riêng cho Workspace.
3. **Nối tab Agent**: thêm nhánh `activeTab === 'agent' && <AgentPanel worktreeId={...} />`,
   dùng `worktreeId` từ bước 2.
4. **Nối terminal panel**: tái dùng `terminal-pane`/PTY infra hiện có (đã chạy ổn định trong
   app chính, không xây song song 1 hệ thống terminal thứ 2).
5. **Ghép `ServerStatusBar`**: dùng lại `RuntimeHostStatusRow`/`SshStatusSegment` thay vì viết
   mới.
6. **Mount `ProjectSwitcher`/`WorkspaceLayout` vào layout thật** — bước cuối cùng, chỉ làm khi
   1–5 đã xong, vì mount sớm sẽ lộ ngay các phần còn dở (tab Agent trống, terminal giả) cho
   người dùng thật.

**Rủi ro nếu làm ngược thứ tự** (mount trước, hoàn thiện sau): người dùng thấy tab Agent trống,
terminal "coming soon" ngay khi vừa bật tính năng — trải nghiệm tệ hơn không có tính năng.
