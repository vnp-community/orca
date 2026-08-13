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
>
> **⚠️ Đính chính 2026-08-13**: nội dung dưới đây về cấu trúc UI (`WorkspaceLayout.tsx` và cụm
> `components/workspace/`) vẫn đúng — được đọc trực tiếp từ `frontend/src/renderer/`, không phải
> từ tiền đề sai `frontend/src/main/`. Nhưng khẳng định ngầm định "chưa có backend đứng sau" là
> **sai** — `backend/src/main/project/ProjectService.ts` là backend thật, đang chạy, có FK thật
> từ hệ Task. Vấn đề thật sự không phải "chưa xây backend", mà là **`WorkspaceContext` gọi 2/4
> RPC method sai** (`git.status` sai tham số luôn fail, `workspace.listFiles` không tồn tại) —
> xem chi tiết mục A/B trong
> [audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md) mục B5, và giải pháp
> cụ thể trong [fix-proposals-per-issue.md](./fix-proposals-per-issue.md).

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

### 2.4 `WorkspaceContext` shape cũng lệch — VÀ 2/4 RPC method nó gọi thật sự lỗi

Doc: `{ project, devServer, connection, resolvedProfile, currentWorktree }`.
Code thật: `{ project, isOffline, isInitializing, gitStatus, fileTree, resolvedProfile,
activeAgentSessionId, currentWorktree, switchProject, refreshGitStatus, refreshFileTree,
setCurrentWorktree, emit, on }` — không có `devServer`/`connection`, có thêm
`gitStatus`/`fileTree`/`isOffline`/`isInitializing` mà doc không mô tả.

**Đã xác nhận bằng backend thật**: `switchProject()` gọi 4 RPC —

| Call | Backend thật | Verdict |
|---|---|---|
| `project.get({projectId})` | Khớp, nhưng lệch kiểu trả về (`visibility:'company'` vs type khai `'public'`; `createdAt: Date` vs type khai `number`) | Chạy nhưng dữ liệu sai kiểu |
| `git.status({projectId})` | Schema thật yêu cầu `worktree: string`, không phải `projectId` | **Luôn fail validation** |
| `workspace.listFiles(...)` | **Không tồn tại** trên backend | **Luôn "method not found"** |
| `profile.getResolved({})` | Khớp | OK |

2 lỗi trên bị `.catch(() => null)` nuốt âm thầm — `WorkspaceContext` không bao giờ báo lỗi thật,
chỉ lặng lẽ trả `null`. Chi tiết đầy đủ:
[audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md) mục B5.

### 2.5 Code xây RỘNG hơn doc ở mảng Git

`components/workspace/git/` có 8 file (`BranchManager`, `CommitForm`, `DiffViewer`,
`GitHistory`, `GitPanel`, `PullRequestForm`, `PullRequestList`, `StagingArea`) — 1 hệ thống Git
UI khá đầy đủ mà F38 hoàn toàn không mô tả chi tiết. Cộng thêm `FileContextMenu`,
`FileSearchPanel`, `FileTreeNode`, `NoProjectSelected`, `OfflineBanner`,
`WorkspaceSkeletonLoader`, `WorkspaceTabBar` — tổng **18 file** trong `components/workspace/`,
nhiều hơn hẳn 9 file doc liệt kê.

### 2.6 "Mounted nhưng không ai thấy" — xác nhận lại, cụm còn LỚN hơn 18 file đã liệt kê

Re-grep toàn bộ `frontend/src/renderer`: `WorkspaceLayout.tsx`/`ProjectSwitcher.tsx`/
`ProjectSettings.tsx` **không được import/render ở bất kỳ đâu trong cây UI thật** — chỉ tồn tại
trong file test của chính chúng, `App.tsx` không tham chiếu bất kỳ cái nào.

**Mở rộng mới**: `WorkspaceContext`'s provider **có mount thật** (`main.tsx`,
`web/main-web-bootstrap.tsx`), và `useWorkspace()` được dùng thật bởi thêm ~14 component ngoài
18 file trong `components/workspace/`: `TaskPromptEditor.tsx`, `TaskDetail.tsx`,
`CodeReviewPanel`, `commit-message-generator.tsx`, `annotation-panel.tsx` và những component
Git đã liệt kê ở §2.5 — **không component nào trong toàn bộ cụm này reachable từ `App.tsx`**.

Có thêm 1 hệ **thứ 2 cạnh tranh**: `WorkspaceContextV6.tsx` + `WorkspaceContextBridge.ts`
(`getWorkspaceProvider()`/`getUseWorkspace()`, gate bằng flag `__ORCA_WORKSPACE_V6__`) — nhưng
`main.tsx` import thẳng `WorkspaceProvider` từ bản V5 gốc, bỏ qua bridge hoàn toàn → V6 tự nó
cũng là code chết, chưa từng được dùng dù có vẻ là ý định thay thế V5.

→ Toàn bộ cụm này, dù chất lượng code không tệ (đặc biệt `AgentPanel.tsx`/Git subsystem), **hiện
không ai dùng được** — provider sống, cả 2 thế hệ (V5 lẫn V6) cây consumer đều chết.

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
| `git.status`/`workspace.listFiles` gọi sai contract | **Sửa code, làm TRƯỚC mọi việc khác** | `git.status` cần đổi tham số từ `projectId` sang `worktree` đúng schema thật; `workspace.listFiles` cần đổi tên gọi thành 1 trong 4 method thật (`workspace.init/teardown/refreshFileTree/refreshGitStatus`) — nếu không sửa, mọi bước sau đều build trên nền tảng gọi API sai |
| `WorkspaceContextV6`/`WorkspaceContextBridge` tồn tại song song, chưa từng dùng | **Cần quyết định trước** | Có ý định thay V5 bằng V6 không? Nếu không, nên xoá hẳn V6 thay vì để 2 bản cạnh tranh cùng tồn tại mãi |

## 4. Phương án merge — theo mức độ ưu tiên

**Bước 0 (làm trước tất cả, quan trọng nhất): quyết định có hoàn thiện F38 hay không.**
Toàn bộ 18 file này đã tồn tại nhiều tháng mà chưa ai mount vào UI thật — trước khi đầu tư thêm
effort (sửa tab Agent, nối terminal...), cần câu trả lời rõ: **có kế hoạch release F38 hay
không?** Nếu không, nên dán nhãn rõ ràng (comment ở đầu mỗi file, hoặc xoá hẳn) thay vì để code
chết tiếp tục tích luỹ — matching bài học từ `workspace-slice.ts` (càng để lâu, càng dễ tái diễn
kiểu bug "đè lên code thật" đã gặp tuần này.

**Nếu quyết định tiếp tục, theo thứ tự:**

1. **Sửa RPC contract sai** (`git.status` tham số, `workspace.listFiles` tên method) — làm
   trước tiên vì mọi bước sau đều phụ thuộc `WorkspaceContext` lấy được dữ liệu thật.
2. **Viết lại F38 theo code thật** (không phải ngược lại) — cập nhật layout diagram, bảng "Yêu
   cầu kỹ thuật" (tên file đúng), shape `WorkspaceContext`, và bổ sung mô tả Git subsystem đang
   thiếu hoàn toàn trong doc gốc.
3. **Quyết định nguồn `currentWorktree`** — đây là khoá cho cả tab Agent lẫn terminal panel.
   Đề xuất: tái dùng cơ chế chọn worktree đã có ở sidebar chính (`WorktreeList.tsx`) thay vì xây
   bộ chọn riêng cho Workspace.
4. **Nối tab Agent**: thêm nhánh `activeTab === 'agent' && <AgentPanel worktreeId={...} />`,
   dùng `worktreeId` từ bước 3.
5. **Nối terminal panel**: tái dùng `terminal-pane`/PTY infra hiện có (đã chạy ổn định trong
   app chính, không xây song song 1 hệ thống terminal thứ 2).
6. **Ghép `ServerStatusBar`**: dùng lại `RuntimeHostStatusRow`/`SshStatusSegment` thay vì viết
   mới.
7. **Xoá `WorkspaceContextV6`/`WorkspaceContextBridge`** nếu quyết định không dùng (mục 3 bảng
   trên) — tránh 2 hệ cạnh tranh tồn tại mãi.
8. **Mount `ProjectSwitcher`/`WorkspaceLayout` vào layout thật** — bước cuối cùng, chỉ làm khi
   1–7 đã xong, vì mount sớm sẽ lộ ngay các phần còn dở (tab Agent trống, terminal giả) cho
   người dùng thật.

**Rủi ro nếu làm ngược thứ tự** (mount trước, hoàn thiện sau): người dùng thấy tab Agent trống,
terminal "coming soon" ngay khi vừa bật tính năng — trải nghiệm tệ hơn không có tính năng.
