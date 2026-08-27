# Project Workspace (F38): tài liệu vs. code thật, và phương án merge

**Cập nhật:** 2026-08-13

> **✅ Đính chính 2026-08-13 (Giai đoạn 2c hoàn thành, bước tích hợp cuối)**: các mục 2, 3, 4, 5, 6
> của phương án merge ở mục 4 dưới đây đã xong (Agent tab, terminal panel, `ServerStatusBar`,
> `currentWorktree` từ sidebar, và mount vào `App.tsx`). Mục 7 (xoá `WorkspaceContextV6`/
> `WorkspaceContextBridge`) **KHÔNG làm** — quyết định #2 chốt giữ nguyên, không động (đã có 1 lần
> agent tích hợp xoá nhầm theo gợi ý cũ của mục này, phát hiện và khôi phục ngay trong cùng phiên
> — xem ghi chú ở mục 3 bảng dưới). Ngoại lệ còn lại: mục 1 (`git.status` param `projectId`→
> `worktree`) **CHƯA sửa** —
> để nguyên có chủ đích theo commit `edf89c96e` ("cần `worktreeId` thật, phụ thuộc giai đoạn sau"
> — nay `worktreeId` đã có qua sync sidebar, nhưng bản thân lời gọi `git.status` trong
> `WorkspaceContext.tsx` vẫn truyền `{projectId}` thay vì `{worktree}`, lỗi vẫn bị `.catch(() =>
> null)` nuốt âm thầm). Mount vào `App.tsx` **cộng thêm** một nút "Project Workspace (Beta)" mới
> trong sidebar nav — dẫn tới `activeView: 'workspace'` — **không thay thế** luồng Project/Repo
> sidebar hiện có. Chi tiết đầy đủ: [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md)
> Giai đoạn 2c.

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

| Khác biệt | Đề xuất | Trạng thái | Vì sao |
|---|---|---|---|
| Explorer = panel cố định thay vì tab | **Cập nhật doc theo code** | ✅ (ghi nhận ở đây) | Panel cố định tiện hơn — không mất Explorer khi chuyển tab. Đây là cải tiến, không phải thiếu sót. |
| Tab "Agent" trống | **Sửa code** | ✅ `WorkspaceLayout.tsx:81-85` nối `<AgentPanel worktreeId={currentWorktree.id} />` (rẽ nhánh `<NoWorktreeSelected />` khi chưa có worktree) | Chỉ cần nối `AgentPanel` vào nhánh render + có `worktreeId` thật. `AgentPanel.tsx` đã sẵn, không viết lại. |
| Terminal placeholder | **Sửa code** | ✅ `WorkspaceLayout.tsx:105-114` dùng `WorkspaceTerminalPanel` (tái dùng PTY infra thật) | Tái dùng hạ tầng PTY/terminal-pane đã có sẵn và chạy ổn ở app chính, không xây mới từ đầu. |
| Tên file lệch (`ProjectSelector` vs `ProjectSwitcher`...) | **Cập nhật doc theo code** | ✅ (ghi nhận ở đây, mục 2.3) | Đổi tên code đang chạy để khớp doc cũ không có giá trị, chỉ tốn công. |
| `WorkspaceContext` thiếu `devServer`/`connection` | **Cập nhật doc theo code** | ✅ (ghi nhận ở đây, mục 2.4) | 2 field đó có thể lấy qua `project.devServerId` sẵn có — không cần thêm field mới vào context. |
| `ServerStatusBar.tsx` chưa xây | **Tái dùng, không xây mới** | ✅ `WorkspaceLayout.tsx:25,127-129` ghép `SshStatusSegment` vào status bar | Ghép `RuntimeHostStatusRow`/`SshStatusSegment` (đã chạy thật) vào Workspace thay vì viết component riêng trùng chức năng. |
| Git subsystem rộng hơn doc | **Cập nhật doc theo code** | ✅ (ghi nhận ở mục 2.5) | Ghi nhận đúng những gì đã xây — đây là phần hoàn thiện nhất, nên là phần đầu tiên tài liệu hoá tử tế. |
| `currentWorktree` chưa bao giờ được set | **Quyết định #8: tái dùng sidebar** | ✅ `WorktreeList.tsx` sync `setCurrentWorktree()` từ lựa chọn sidebar (roadmap quyết định #8) | Nguồn cấp `worktreeId` cho Workspace là gì — chọn từ sidebar hiện có, hay Workspace tự có bộ chọn worktree riêng? Đã chốt: tái dùng sidebar. |
| `git.status`/`workspace.listFiles` gọi sai contract | **Sửa code, làm TRƯỚC mọi việc khác** | ⚠️ 1/2 — `workspace.listFiles`→`workspace.refreshFileTree` đã sửa (Giai đoạn 1, 1.8); `git.status` vẫn truyền `{projectId}` thay vì `{worktree}` thật, lỗi bị `.catch(() => null)` nuốt âm thầm — CHƯA sửa, để lại follow-up | `git.status` cần đổi tham số từ `projectId` sang `worktree` đúng schema thật; `workspace.listFiles` cần đổi tên gọi thành 1 trong 4 method thật — nếu không sửa, mọi bước sau đều build trên nền tảng gọi API sai |
| `WorkspaceContextV6`/`WorkspaceContextBridge` tồn tại song song, chưa từng dùng | **Giữ nguyên, không động** | ❌ KHÔNG xoá — quyết định #2 (2026-08-13) chốt giữ nguyên cho kế hoạch nâng cấp sau, dù xác nhận 0 importer thật qua gitnexus + grep (code chết vô hại, không phải bug) | F38 dùng V5. Có kế hoạch thay bằng V6 sau, chưa phải bây giờ — quyết định #2 rõ ràng là "giữ", không phải "xoá" |
| Mount `ProjectSwitcher`/`WorkspaceLayout` vào layout thật | **Mount tối thiểu, không thay Project/Repo sidebar** | ✅ `App.tsx`: `activeView === 'workspace'` render `<ProjectSwitcher />` + `<WorkspaceLayout />`; nút mới "Project Workspace (Beta)" trong `SidebarNav.tsx` (không đụng luồng Project/Repo hiện có) | Bước cuối cùng — chỉ mount khi Agent/Terminal/ServerStatusBar/currentWorktree đã nối thật, để tránh lộ UI dở cho người dùng. |

### 2.7 Bug phát hiện khi mount (2026-08-13), đã sửa trước khi ship: `ProjectSwitcher` đọc sai nguồn dữ liệu

Agent tích hợp phát hiện `ProjectSwitcher.tsx:21` đọc `useAppStore(s => (s as any).projects as
any[] ?? [])` — nhưng `projects` trên store chính (`slices/auth.ts`) là `string[]` (danh sách
project ID trong session token), không phải mảng `OrcaProject {id, name, devServerId}`. Hậu quả:
switcher luôn trống, hoặc ném `TypeError` nếu session có project ID (`.filter(p => p.name...)`
trên string). Đây là bug thật đủ nghiêm trọng để hoãn ship — sửa ngay trước khi commit thay vì
để lại follow-up: `ProjectSwitcher` giờ tự fetch qua `project.list` RPC (không tham số, trả
`OrcaProject[]`, method thật đã có sẵn từ `backend/src/main/project/project-rpc-handler.ts`) lúc
mount, thay vì đọc field sai của store. Test cập nhật theo (`ProjectSwitcher.test.tsx`, mock
`callRuntimeRpc` thay vì mock field `projects` giả).

## 4. Phương án merge — theo mức độ ưu tiên

**Bước 0 (làm trước tất cả, quan trọng nhất): quyết định có hoàn thiện F38 hay không.**
Toàn bộ 18 file này đã tồn tại nhiều tháng mà chưa ai mount vào UI thật — trước khi đầu tư thêm
effort (sửa tab Agent, nối terminal...), cần câu trả lời rõ: **có kế hoạch release F38 hay
không?** Nếu không, nên dán nhãn rõ ràng (comment ở đầu mỗi file, hoặc xoá hẳn) thay vì để code
chết tiếp tục tích luỹ — matching bài học từ `workspace-slice.ts` (càng để lâu, càng dễ tái diễn
kiểu bug "đè lên code thật" đã gặp tuần này.

**Quyết định: tiếp tục.** Trạng thái từng bước (2026-08-13, bước tích hợp cuối Giai đoạn 2c):

1. **Sửa RPC contract sai** (`git.status` tham số, `workspace.listFiles` tên method) — làm
   trước tiên vì mọi bước sau đều phụ thuộc `WorkspaceContext` lấy được dữ liệu thật.
   ⚠️ **1/2** — `workspace.listFiles` đã sửa (Giai đoạn 1, 1.8). `git.status` vẫn gọi sai
   (`{projectId}` thay vì `{worktree}`), lỗi bị nuốt âm thầm — follow-up còn mở, xem mục 2.4/§3.
2. **Viết lại F38 theo code thật** — ✅ file này (mục 2, 3 ở trên) đã cập nhật theo code thật.
3. **Quyết định nguồn `currentWorktree`** — ✅ chốt tái dùng sidebar (`WorktreeList.tsx`, quyết
   định #8 roadmap), đã nối (`WorktreeList.tsx` gọi `setCurrentWorktree()`).
4. **Nối tab Agent** — ✅ `WorkspaceLayout.tsx:81-85`.
5. **Nối terminal panel** — ✅ `WorkspaceLayout.tsx:105-114` (`WorkspaceTerminalPanel`).
6. **Ghép `ServerStatusBar`** — ✅ `WorkspaceLayout.tsx:127-129` (`SshStatusSegment`).
7. **Xoá `WorkspaceContextV6`/`WorkspaceContextBridge`** — ❌ KHÔNG làm, quyết định #2 chốt giữ
   nguyên (0 importer thật, nhưng để dành cho kế hoạch nâng cấp sau).
8. **Mount `ProjectSwitcher`/`WorkspaceLayout` vào layout thật** — ✅ mount tối thiểu: nút mới
   "Project Workspace (Beta)" trong `SidebarNav.tsx` → `activeView: 'workspace'` → `App.tsx`
   render `<ProjectSwitcher />` + `<WorkspaceLayout />`. Không thay thế luồng Project/Repo sidebar
   hiện có (2 khái niệm song song, theo
   [terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md)).
   Biết trước rủi ro: mục 2.7 (`ProjectSwitcher` chưa có nguồn dữ liệu `OrcaProject` thật) và mục 1
   ở trên (`git.status` vẫn sai) vẫn còn — chấp nhận được vì nhãn "(Beta)", nằm trong
   `RecoverableRenderErrorBoundary`, và không chặn luồng chính.

**Rủi ro đã né được nhờ làm đúng thứ tự** (mount sau khi 2–7 xong): người dùng bấm vào nút Beta
thấy Agent tab/terminal panel/status bar hoạt động thật — không còn placeholder/tab trống.

## 5. 🐛 Bug thật phát hiện qua live test (2026-08-13, sau deploy 1.4.174), đã sửa

Người dùng report: `Project Workspace (Beta) → Create new Project` bấm vào không hiển thị gì.
Nguyên nhân: `ProjectSwitcher.tsx`'s `CommandItem` "Create New Project" **không có `onSelect`
handler** — đây là phần scaffolding chưa từng được hoàn thiện từ trước khi cả cụm 18 file
`components/workspace/` bị mount vào app thật, không bị catch bởi review vì không có test nào
assert hành vi click cho item này (chỉ có `data-testid` để tồn tại trong DOM).

**Đã sửa**: nối `onSelect` mở `CreateProjectDialog.tsx` (mới) — form tạo `OrcaProject` thật, gọi
đúng `project.create` RPC (`name`, `description?`, `devServerId` (chọn từ `devServer.list`),
`repoPath`, `visibility`), thành công thì `switchProject()` sang project vừa tạo + refetch list.
Đây là ví dụ thứ 2 (sau bug `ProjectSwitcher` đọc sai field store) của cùng 1 lớp rủi ro: **mount
xong không có nghĩa là đã test hết đường đi người dùng thật** — checklist "1–7 xong trước khi
mount" chỉ đảm bảo các tab/panel không trống, không đảm bảo mọi nút bấm bên trong đã có handler.
Khuyến nghị theo dõi thêm: cần 1 vòng test thủ công/E2E qua từng nút bấm trong `WorkspaceLayout`
trước khi bỏ nhãn "(Beta)".
