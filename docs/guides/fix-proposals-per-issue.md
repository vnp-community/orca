# Đề xuất sửa cho từng bug/case cụ thể

**Cập nhật:** 2026-08-13

> Nguồn: [audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md). Mỗi đề xuất
> bám theo nguyên tắc kiến trúc mục tiêu: **frontend = hiển thị + tương tác người dùng**,
> **backend = quản trị + lưu dữ liệu quản trị**, **agent = thực thi tác vụ + quản trị code trên
> dev-server**. Case nào có xung đột/cần quyết định trước khi sửa → tách sang
> [decisions-needed.md](./decisions-needed.md), không đưa giải pháp 1 chiều ở đây. Kế hoạch
> thực thi tổng hợp (thứ tự làm) xem
> [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md).

## Nhóm A — Profile / RBAC

### A1. `profile.getUser` không tồn tại (bug)

**Nơi sửa: frontend.** `useProfile.ts` đổi tên gọi từ `profile.getUser` →
`profile.getUserProfile` (khớp đúng method thật). Không đụng backend — backend đã đúng.

### A2. `profile.listDepts` không tồn tại (bug)

**Nơi sửa: cả backend lẫn frontend, theo nguyên tắc phân lớp.** Đây là dữ liệu quản trị (danh
sách phòng ban) → backend phải là nơi cung cấp. Thêm method `profile.listDepts` vào
`backend/src/main/profile/profile-rpc-handler.ts` (query `orca_departments`, gate
`requireAdmin()` như các method sửa company/dept khác). Frontend (`CompanyProfileAdmin.tsx`/
`DeptProfileAdmin.tsx`) giữ nguyên cách gọi, chỉ chờ backend có method thật.

### A3. Trang Admin build ra nhưng không serve được (routing bug)

**Nơi sửa: backend.** `http-server.ts`'s check `path.startsWith('/admin')` đang bắt luôn cả
`/admin-index.html`/`/admin` trước khi tới static-file fallback. Sửa: tách rõ 2 nhánh — giữ
`/admin/api/*` route vào Express (đúng như hiện tại, đây là quản trị dữ liệu, thuộc backend),
nhưng `/admin` (không có `/api`) và `/admin-index.html` phải rơi qua static-file serve (hiển
thị, đáng lẽ thuộc phần "frontend" của nguyên tắc — nhưng vì đây chỉ là serve file tĩnh, việc
sửa route nằm ở backend's HTTP server).

### A4. `DeptProfileAdmin.tsx` không được import vào router Admin

**Nơi sửa: frontend.** Thêm `DeptProfileAdmin` vào `AdminApp.tsx`'s route table (hiện chỉ có
`CompanyProfileAdmin` cho `/profile`).

### A5. 5 bản `OrcaProfile`/profile-types khác nhau

**Nơi sửa: chủ yếu frontend, dữ liệu chuẩn giữ ở backend.** Theo nguyên tắc "backend lưu dữ liệu
quản trị", `backend/src/main/profile/OrcaProfile.ts` là **nguồn chuẩn duy nhất**. Việc cần làm:
1. Xoá 2 bản chết (`frontend/src/main/profile/OrcaProfile.ts`,
   `agent/src/main/profile/OrcaProfile.ts`) và 1 bản mồ côi (`frontend/src/shared/
   profile-types.ts`) — 0 importer, xoá an toàn.
2. Sửa `frontend/src/renderer/src/types/profile-types.ts` (bản UI thật đang dùng) khớp đúng
   field/enum của backend — đổi `disallowedCmds`→`disallowedCommands`,
   `sessionTimeoutHours`→`maxSessionHours`, bỏ `integrations`/`fleet`/`require2FA` (không có
   backend support) hoặc thêm các field đó vào backend nếu tính năng thật sự cần (xem
   [decisions-needed.md](./decisions-needed.md)).

### A6. `Team` chưa tồn tại như entity thật

**Nơi sửa: backend (schema + RPC), frontend (UI thêm mới).** Backend: thêm bảng metadata `Team`
(tái dùng `orca_team_members` đã có sẵn làm bảng nối), thêm RPC `team.create/addMember/
removeMember/list`. Frontend: thêm UI quản lý Team (đặt trong trang Admin — quản trị tổ chức
thuộc "backend quản trị dữ liệu", UI của nó vẫn là frontend hiển thị). Chi tiết schema đề xuất ở
[user-profile-team-department-rbac.md](./user-profile-team-department-rbac.md) mục 5.2.

## Nhóm B — Project / OrcaProject

### B1. `project.agentSpawn` luôn lỗi `AGENT_SPAWNER_NOT_AVAILABLE`

**Nơi sửa: backend.** `server-bootstrap.ts` gọi `createProjectMethods(projectService,
getUserRole)` thiếu tham số thứ 3. Sửa: khởi tạo `ProfileAwareAgentSpawner` **trước** khi đăng
ký project methods (đổi thứ tự trong `server-bootstrap.ts`), truyền vào làm tham số thứ 3.

### B2. `WorkspaceContext.switchProject()` gọi `git.status` sai tham số

**Nơi sửa: frontend.** Đổi `git.status({projectId})` → `git.status({worktree: <worktreeId>})`
đúng schema thật (`GitStatusParams extends WorktreeSelector`). Cần có `worktreeId` thật trước —
phụ thuộc quyết định nguồn `currentWorktree` (xem
[project-workspace-f38-doc-vs-code.md](./project-workspace-f38-doc-vs-code.md) mục 3, và
[decisions-needed.md](./decisions-needed.md)).

### B3. `workspace.listFiles` không tồn tại

**Nơi sửa: frontend, đổi tên gọi.** Method thật gần nhất là `workspace.refreshFileTree` — đổi
`WorkspaceContext.tsx` gọi đúng tên này thay vì tên tưởng tượng `workspace.listFiles`.

### B4. Cụm UI Workspace/Git/CodeReview (~14+18 component) mồ côi

**Nơi sửa: frontend (mount hoặc xoá), quyết định trước khi làm** — xem
[decisions-needed.md](./decisions-needed.md) mục "F38 release hay shelve".

### B5. `WorkspaceContextV6` cạnh tranh, chưa từng dùng

**Nơi sửa: frontend.** Nếu không có kế hoạch chuyển sang V6 (chưa thấy bằng chứng), xoá
`WorkspaceContextV6.tsx` + `WorkspaceContextBridge.ts` + flag `__ORCA_WORKSPACE_V6__` — code
chết, không ai bật flag để dùng.

### B6. `UNAUTHENTICATED` cho `project.*` RPC ở phiên web

**Nơi sửa: backend.** Đây là lỗi tầng transport — session token của phiên WebSocket không được
forward vào `ctx.userId` cho nhánh gọi `project.*`. Cần trace lại chuỗi
`dispatcher.ts`→session router→`RpcContext` để tìm chính xác điểm rớt `userId` (không thuộc
phạm vi audit này, cần điều tra riêng bằng log thật khi bug tái diễn).

## Nhóm C — Task / OrcaTask

### C1. 7 chỗ UI gọi sai tên RPC method (`tasks.*` thay vì `task.*`, `task.runAgent` thay vì `task.execute`)

**Nơi sửa: frontend.** Sửa tên gọi tại `TaskPromptEditor.tsx`, `TaskDetail.tsx`, `useTask.ts` —
đổi đúng theo bảng trong
[task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 4. Không đụng backend — backend đã đúng, đã có sẵn 18 method thật.

### C2. `TaskGraphPanel.tsx` là stub trống

**Nơi sửa: frontend.** Viết implementation thật (hiện chỉ có comment "full implementation in
TASK-V5-07"), dùng đúng `frontend/src/shared/task-types.ts` (bản khớp backend), không dùng bản
lệch `renderer/src/types/task-types.ts`.

### C3. 2 bản `OrcaTask` type không tương thích dùng lẫn trong 1 cụm UI

**Nơi sửa: frontend.** Xoá `frontend/src/renderer/src/types/task-types.ts` (bản lệch), migrate
toàn bộ `TaskDetail.tsx`/`TaskPromptEditor.tsx`/`TaskAIDecompose.tsx`/`TaskDAGView.tsx` sang
import `@shared/task-types` (bản khớp backend) — cùng 1 sửa như C2.

### C4. Field liên kết cross-system (`worktreeId`/`agentSessionId`/`workflowExecutionId`) chưa tồn tại trên `OrcaTask`

**Nơi sửa: backend (schema mới) + backend (service logic nối ①②③).** Xem đề xuất chi tiết ở
[task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 9.2 — cần quyết định trước (2 con đường "chạy task" độc lập hiện có), xem
[decisions-needed.md](./decisions-needed.md).

### C5. `TaskGrantService` độc lập với `ProjectMember` — 2 hệ RBAC không chia sẻ code

**Nơi sửa: backend, cần quyết định trước khi hợp nhất** (rủi ro cao nếu làm vội — 2 hệ đã chạy
thật, có dữ liệu thật). Xem [decisions-needed.md](./decisions-needed.md).

## Nhóm D — Orchestration / Automations

### D1. 🐛 `AutomationService` không bao giờ khởi tạo trong `backend/src` — scheduler không chạy

**Nơi sửa: backend.** Thêm đúng theo pattern `desktop/src/main/index.ts:1810` vào
`backend/src/main/server-bootstrap.ts`: khởi tạo `new AutomationService(store, {...})`, gọi
`automationService.start()`, và gọi `setAutomationService(automationService)` (setter đã tồn
tại sẵn ở `orca-runtime.ts:589`, chỉ chưa ai gọi). Đây là bug ĐƠN GIẢN NHẤT trong toàn bộ audit
— sửa xong, `automation.runNow` hết throw và scheduler `rrule` bắt đầu chạy thật trên server.

### D2. `WorkflowMonitor.tsx` là stub mồ côi — không ai điều khiển được `WorkflowOrchestrator` thật

**Nơi sửa: frontend.** Viết implementation thật cho `WorkflowMonitor.tsx`, gọi đúng RPC
`workflow.*` (đã có thật, đang chạy trên backend). Cần quyết định trước: mount ở đâu (cùng cụm
`WorkspaceLayout` orphaned, hay 1 chỗ độc lập)? Xem
[decisions-needed.md](./decisions-needed.md).

### D3. `OrchestrationPane.tsx`/`OrchestrationSkillPromptDialog.tsx` trùng tên gây nhầm lẫn

**Nơi sửa: frontend, đổi tên (không phải bug chức năng).** 2 component này hoạt động đúng chức
năng thật của chúng (cài CLI `orca`) — chỉ tên gọi dễ nhầm với `WorkflowOrchestrator`/
`coordinator.ts`. Đề xuất đổi tên thành `OrcaCliInstallPane.tsx`/tương tự để tránh nhầm lẫn cho
người đọc code sau này — việc nhỏ, không khẩn.

### D4. Pipeline Source→Plan→Execute (Task ↔ Orchestration coordinator) chưa tồn tại

**Nơi sửa: backend.** Xem
[task-automation-orchestration-integration.md](./task-automation-orchestration-integration.md)
mục 9.2/9.4 — cần quyết định thiết kế trước (2 con đường "chạy task" hiện có), xem
[decisions-needed.md](./decisions-needed.md).

## Bảng tổng hợp theo nơi sửa (đối chiếu nguyên tắc kiến trúc mục tiêu)

| Nơi sửa | Case |
|---|---|
| **Backend** (quản trị + dữ liệu) | A2, A3, A6 (schema+RPC), B1, B6, C4 (schema), D1, D4 |
| **Frontend** (hiển thị + tương tác) | A1, A4, A5, A6 (UI), B2, B3, B4, B5, C1, C2, C3, D2, D3 |
| **Agent** (thực thi + quản trị code trên dev-server) | Không có bug nào audit ra ở tầng này — `agent/` package hiện đúng vai trò thụ động (thực thi lệnh, không tự quyết định logic nghiệp vụ), khớp nguyên tắc mục tiêu, không cần sửa gì |

**Quan sát**: `agent/` package hiện đã khớp đúng nguyên tắc "thực thi tác vụ, không quản trị" —
mọi access-control/business-logic đều nằm ở `backend/` trước khi dispatch xuống agent (xác nhận
ở audit mục A6, B7). Đây là điểm kiến trúc **đang đúng**, không cần thay đổi khi tiếp tục xây
các phần còn thiếu.
