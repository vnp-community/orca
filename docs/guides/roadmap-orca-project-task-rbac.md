# Roadmap thực thi: sửa bug + hoàn thiện Project/Task/RBAC/Orchestration

**Cập nhật:** 2026-08-13 — **8/8 quyết định đã chốt** (xem
[decisions-needed.md](./decisions-needed.md)), roadmap này là kế hoạch thực thi cụ thể, sẵn sàng
bắt đầu.

> Lịch sử: bản gốc roadmap (viết trước audit) giả định phần lớn backend "chưa xây" — sai, đã
> đính chính bằng [audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md). Bản
> sau audit (Nhóm 1/2/3) liệt kê 8 điểm cần quyết định trước khi xây — nay cả 8 đã được chốt, đưa
> vào kế hoạch dưới đây theo đúng thứ tự phụ thuộc thật.

## Tóm tắt quyết định đã chốt (chi tiết đầy đủ ở [decisions-needed.md](./decisions-needed.md))

| # | Quyết định |
|---|---|
| 1 | F38 Workspace: **hoàn thiện** |
| 2 | `WorkspaceContextV6`: **giữ nguyên, không động** (có kế hoạch nâng cấp sau, chưa phải bây giờ) — F38 tiếp tục dùng V5. **Đính chính 2026-08-13 (bước tích hợp cuối)**: 1 agent tích hợp đã tự ý xoá `WorkspaceContextV6.tsx`/`WorkspaceContextBridge.ts` dựa trên gợi ý cũ trong `project-workspace-f38-doc-vs-code.md` (viết TRƯỚC khi quyết định #2 được chốt), trái với quyết định thật của người dùng — đã khôi phục lại nguyên trạng ngay khi phát hiện. Xác nhận qua `gitnexus impact` + grep: 0 importer thật ngoài test của chính nó (đúng như audit ghi), nhưng giữ nguyên theo quyết định #2, KHÔNG xoá. |
| 3 | Rule merge multi-team: **`priority: number`, số cao thắng** |
| 4 | `TaskGrantService` vs `ProjectMember`: **giữ tách biệt, không hợp nhất** |
| 5 | `OrcaTask.execute`: **hybrid** — task không subtask/dependency → chạy đơn; có subtask/dependency → qua Orchestration coordinator, tự động chọn trong `TaskAgentExecutor` |
| 6 | `require2FA`: **thêm vào backend**. `integrations`/`fleet`: **còn mở**, giữ nguyên chưa đụng |
| 7 | `OrcaProject` cross-user sharing: **tiến hành theo thiết kế đã đề xuất**, review bảo mật kỹ ở bước đọc-chéo-user |
| 8 | Nguồn `currentWorktree`: **tái dùng sidebar (`WorktreeList.tsx`)** |

## Sơ đồ phụ thuộc thật (sau khi có đủ quyết định)

```
Giai đoạn 1 — SỬA NGAY (10 mục, độc lập, làm song song)
       │
       ├──────────────┬──────────────┬──────────────┐
       ▼              ▼              ▼              ▼
Giai đoạn 2a      Giai đoạn 2b   Giai đoạn 2c   Giai đoạn 2d
Team entity      OrcaProject     F38 Workspace   Task pipeline
(3.1)            sharing (3.2)   hoàn thiện       Source→Plan→
                  — visibility   (3.4, cần 1.8    Execute (3.3)
                  'team' tier    đã xong)          — độc lập,
                  cần 3.1 xong                     chỉ cần backend
                  trước                            Task đã có (rồi)
       │              │              │              │
       │              │              ▼              ▼
       │              │        Giai đoạn 3a    Giai đoạn 3b
       │              │        (không có,      Task Graph UI
       │              │        F38 UI đã       (3.5, cần 3.3
       │              │        xong ở 3.4)      xong trước)
       ▼              ▼
   (độc lập, không có giai đoạn sau)
```

## Giai đoạn 1 — ✅ HOÀN THÀNH (2026-08-13)

Thực thi bằng 6 subagent song song (chia theo file, không đụng nhau), review tổng hợp bằng
`gitnexus detect_changes` + tsc/oxlint/test baseline compare (dùng `git worktree` để so sánh an
toàn, tránh race condition trên working tree chung). Chi tiết từng mục, kể cả phát hiện mới phát
sinh khi thực thi: [fix-proposals-per-issue.md](./fix-proposals-per-issue.md).

| # | Việc | Nơi sửa | Trạng thái |
|---|---|---|---|
| 1.1 | Khởi tạo `AutomationService` trong `server-bootstrap.ts` — mở khoá scheduler | Backend | ✅ (headless dispatcher đầy đủ để lại `TODO`, xem fix-proposals D1) |
| 1.2 | Sửa `project.agentSpawn` (đăng ký thiếu tham số `agentSpawner`) | Backend | ✅ |
| 1.3 | `useProfile.ts`: `profile.getUser` → `profile.getUserProfile` | Frontend | ✅ (sửa cả test mock) |
| 1.4 | Thêm method `profile.listDepts` | Backend | ✅ |
| 1.5 | Sửa route `/admin` trong `http-server.ts` | Backend | ✅ (verify sống: `GET /admin-index.html` → 200) |
| 1.6 | Import `DeptProfileAdmin` vào `AdminApp.tsx` router | Frontend | ✅ (dạng tab tại `/profile`) |
| 1.7 | Sửa 7 chỗ UI Task gọi sai tên RPC (`tasks.*` → `task.*`) | Frontend | ✅ (+ sửa shape tham số sai kèm theo) |
| 1.8 | `WorkspaceContext`: `workspace.listFiles`→`workspace.refreshFileTree` | Frontend | ✅ (`git.status` vẫn để nguyên, chờ Giai đoạn 2c cấp `worktreeId`) |
| 1.9 | Xoá bản `OrcaProfile`/profile-types chết | Frontend | ⚠️ 1/3 — 2 file còn lại có importer thật, chưa xoá được (xem fix-proposals A5) |
| 1.10 | Hợp nhất 2 bản `OrcaTask`/`task-types.ts` | Frontend | ✅ (phát hiện thêm: alias `@shared/task-types` vỡ ở 4 file khác, ngoài phạm vi) |
| 1.11 | Thêm `require2FA` vào backend `SecurityProfileSection` | Backend | ✅ |

**Điều kiện hoàn thành**: mỗi mục có 1 test xác nhận cụ thể (xem
[roadmap cũ/fix-proposals](./fix-proposals-per-issue.md) cho từng case).

## Giai đoạn 2a — Team entity (quyết định #3) — ✅ HOÀN THÀNH (2026-08-13)

| # | Việc | Nơi sửa | Trạng thái |
|---|---|---|---|
| 2a.1 | Bảng metadata `Team` (tái dùng `orca_team_members` làm bảng nối), RPC `team.create/addMember/removeMember/list` | Backend | ✅ `TeamService.ts` + `team-rpc-handler.ts`, wired vào `server-bootstrap.ts` |
| 2a.2 | `departmentId` trên `OrcaUser` | Backend | ✅ |
| 2a.3 | `priority: number` trên `TeamMember` + cascade merge `_sources: 'team:<teamId>'` | Backend | ✅ |
| 2a.4 | UI quản lý Team trong trang Admin | Frontend | ✅ `TeamAdmin.tsx` |

Chi tiết: [user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md) mục
5.2.

## Giai đoạn 2b — `OrcaProject` sharing layer (quyết định #7, phụ thuộc 2a cho tier 'team') — ✅ HOÀN THÀNH (2026-08-13)

| # | Việc | Nơi sửa | Trạng thái |
|---|---|---|---|
| 2b.1 | Bảng `OrcaProjectSourceProject` (join `OrcaProject` ↔ `Project` per-user) | Backend | ✅ `OrcaProjectSourceProjectService.ts`, wired vào `server-bootstrap.ts` |
| 2b.2 | Mở rộng `OrcaProject.visibility` thêm `'department'` — dùng `ProjectMember` cho role, KHÔNG `TaskGrantService` (quyết định #4) | Backend | ✅ |
| 2b.3 | API đọc-chéo-user (`orcaProjects.list()`/`getProjectData()`) — review bảo mật 2 chiều | Backend | ✅ `orca-project-sharing-rpc-handler.ts`, wired vào `server-bootstrap.ts` |
| 2b.4 | Tier `'team'` hoạt động đúng (cần 2a) | Backend | ✅ (2a đã xong trước) |

Chi tiết: [terminal-workspace-project-devserver-architecture.md](./terminal-workspace-project-devserver-architecture.md)
mục "Đề xuất: OrcaProject là lớp SỞ HỮU + CHIA SẺ".

## Giai đoạn 2c — F38 Workspace hoàn thiện (quyết định #1, #2, #8; cần 1.8 xong trước) — ✅ HOÀN THÀNH (2026-08-13)

| # | Việc | Nơi sửa | Trạng thái |
|---|---|---|---|
| 2c.1 | Nguồn `currentWorktree`: tái dùng lựa chọn worktree từ sidebar (`WorktreeList.tsx`, quyết định #8) | Frontend | ✅ |
| 2c.2 | Hoàn tất 1.8 (`git.status` cần `worktreeId` thật) | Frontend | ⚠️ CHƯA — `git.status` vẫn gọi `{projectId}` thay vì `{worktree}` đúng schema; lỗi bị `.catch(() => null)` nuốt âm thầm. Follow-up còn mở. |
| 2c.3 | Nối tab Agent: `activeTab === 'agent' && <AgentPanel worktreeId={...} />` | Frontend | ✅ |
| 2c.4 | Nối terminal panel — tái dùng `terminal-pane`/PTY infra hiện có | Frontend | ✅ |
| 2c.5 | Ghép `ServerStatusBar` từ `RuntimeHostStatusRow`/`SshStatusSegment` có sẵn | Frontend | ✅ |
| 2c.6 | Viết lại F38 doc theo code thật | Docs | ✅ [project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md) |
| 2c.7 | Mount `ProjectSwitcher`/`WorkspaceLayout` vào layout thật (`App.tsx`) — dùng **V5** (`WorkspaceContext.tsx`) theo quyết định #2 | Frontend | ✅ Mount tối thiểu, cộng thêm: nút "Project Workspace (Beta)" mới trong `SidebarNav.tsx` → `activeView: 'workspace'`. KHÔNG thay thế luồng Project/Repo sidebar hiện có. |
| 2c.8 | Xoá `WorkspaceContextV6`/`WorkspaceContextBridge` | Frontend | ❌ KHÔNG làm — quyết định #2 chốt "giữ nguyên, không động". Xác nhận 0 importer thật (code chết vô hại), nhưng giữ nguyên cho kế hoạch nâng cấp sau, không xoá. |

**Follow-up còn mở**: 2c.2 (`git.status` param) — xem
[project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md) §3.
`ProjectSwitcher` đọc nhầm field `projects` (phát hiện lúc mount) đã **sửa ngay trong bước review
cuối** (không để lại follow-up) — xem §2.7 cùng file: giờ fetch qua `project.list` RPC thật thay
vì đọc field sai của `auth` slice.

Chi tiết: [project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md) mục 4.

## Giai đoạn 2d — Pipeline Source→Plan→Execute cho Task (quyết định #5, độc lập) — ✅ HOÀN THÀNH (2026-08-13)

| # | Việc | Nơi sửa | Trạng thái |
|---|---|---|---|
| 2d.1 | Thêm field `OrcaTask.activeExecutionTaskId`/`agentSessionId` (schema + migration) | Backend | ✅ |
| 2d.2 | Rẽ nhánh (a)/(b) trong `TaskAgentExecutor.executeTask()` (không subtask/dependency → đơn; có → orchestration) | Backend | ✅ |
| 2d.3 | `dispatchToOrchestration()` — seed `TaskRow` từ `OrcaTask` subtree, gọi `orchestration.run` | Backend | ✅ `TaskOrchestrationBridge.ts`, wired vào `TaskAgentExecutor` qua `server-bootstrap.ts` (4th param `orchestrationBridge`) |
| 2d.4 | Listener ghi ngược kết quả (cả 2 đường) vào `OrcaTask.status`/`actualHours` | Backend | ✅ `task-execution-result-listener.ts` |

Chi tiết: [task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 9.2/9.4.

## Bước tích hợp cuối Giai đoạn 2 (2026-08-13)

7 mảnh xây song song ở trên (Team, ProfileResolver, Admin UI, OrcaProject sharing, F38
Agent/Terminal tab, Task execution pipeline, Task UI follow-ups) đã được nối vào
`backend/src/main/server-bootstrap.ts` (TeamService, OrcaProjectSourceProjectService,
TaskOrchestrationBridge) và `frontend/src/renderer/src/App.tsx` (mount F38 Workspace). Việc bị
bỏ qua có chủ đích (không bắt buộc theo phạm vi tích hợp): migrate import `ResolvedProfile` trong
`frontend/src/shared/project-types.ts`/`agent/src/shared/project-types.ts` khỏi bản `OrcaProfile`
chết cục bộ (Giai đoạn 1, 1.9 còn ⚠️ 1/3) — không có tiền lệ import cross-package
frontend/agent→backend trong repo, và shape `ResolvedProfile` đã lệch nhau (bản backend có thêm
layer Team) — cần việc riêng, không phải đổi 1 dòng import.

Việc đã biết từ trước nhưng ngoài phạm vi agent D3 được giao (vẫn treo): alias `@shared/task-types`
không resolve được (không tsconfig/vite path nào định nghĩa `@shared/*`) — ảnh hưởng ít nhất 5
file: `TaskCard.tsx`/`TaskGraph.tsx`/`TaskStatusBadge.tsx`/`TaskTreeView.tsx` (đã biết, xem
[fix-proposals-per-issue.md](./fix-proposals-per-issue.md) mục C3) + phát hiện thêm
`frontend/src/renderer/src/store/slices/task.ts` (chưa được liệt kê ở C3). Việc riêng: đổi sang
đường dẫn tương đối, hoặc định nghĩa alias `@shared/*` trong tsconfig/vite config.

## Giai đoạn 3b — UI cho Task graph (F37, chỉ sau Giai đoạn 2d)

Tree/Board/Graph view — dùng đúng RPC đã sửa ở Giai đoạn 1 (1.7, 1.10), dữ liệu đã chạy thật qua
Giai đoạn 2d trước khi build UI — tránh lặp lỗi "tab bấm vào trống" đã thấy với F38's Agent tab
và `TaskGraphPanel`'s stub cũ.

## Có thể bắt đầu ngay hôm nay

**Giai đoạn 1** (11 mục, đã cập nhật thêm 1.11 cho quyết định #6) — làm song song, không phụ
thuộc gì. Sau đó **Giai đoạn 2a/2b/2c/2d có thể chạy song song với nhau** (không phụ thuộc chéo,
trừ 2b cần 2a cho tier `'team'`, và 2c cần 1.8 từ Giai đoạn 1).
