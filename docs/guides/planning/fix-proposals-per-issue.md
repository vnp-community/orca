# Đề xuất sửa cho từng bug/case cụ thể

**Cập nhật:** 2026-08-13

> Nguồn: [audit-backend-agent-2026-08-13.md](./audit-backend-agent-2026-08-13.md). Case nào có
> xung đột/cần quyết định trước khi sửa → tách sang
> [decisions-needed.md](./decisions-needed.md), không đưa giải pháp 1 chiều ở đây. Kế hoạch
> thực thi tổng hợp (thứ tự làm) xem
> [roadmap-orca-project-task-rbac.md](./roadmap-orca-project-task-rbac.md).

## Nguyên tắc kiến trúc — luồng dữ liệu 1 chiều, không đảo ngược

**Backend và agent xử lý toàn bộ luồng nghiệp vụ (business logic), trả kết quả cho frontend qua
API (RPC). Frontend chỉ gọi API và hiển thị — không tự tính toán/quyết định nghiệp vụ.**

```
agent (thực thi lệnh thật trên dev-server: PTY, git, fs, spawn agent CLI)
   │  kết quả thô
   ▼
backend (nghiệp vụ: validate, tính progress/permission/trạng thái, lưu DB, quản trị)
   │  API (RPC) — CHỈ trả dữ liệu đã xử lý xong, sẵn sàng hiển thị
   ▼
frontend (gọi API, render đúng những gì API trả về — KHÔNG tự tính toán lại)
```

Áp dụng cụ thể khi review từng đề xuất bên dưới:

- **Sai tên/tham số RPC** (A1, B2, B3, C1) → sửa ở frontend là ĐÚNG, vì đây chỉ là gọi đúng tên
  API có sẵn — không phải thêm nghiệp vụ vào frontend.
- **Method/field/entity chưa tồn tại** (A2, A6, C4) → PHẢI thêm ở backend trước, frontend chỉ
  gọi sau khi backend đã có — không tự tính toán tạm ở frontend để "chạy được trước".
- **Type contract lệch nhau** (A5, C3) → nguồn chuẩn LUÔN là backend's type
  (`backend/src/shared/*`, `backend/src/main/*/*.ts`). Đề xuất bổ sung: cân nhắc để frontend
  **import trực tiếp** type từ `backend/src/shared/` (nếu build pipeline cho phép) thay vì duy
  trì bản sao tay ở `frontend/src/shared/`/`frontend/src/renderer/src/types/` — đây chính là
  nguyên nhân gốc của cả 5 bản `OrcaProfile` và 2 bản `OrcaTask` lệch nhau đã audit ra. Nếu
  không thể import thẳng (build/bundle constraint), tối thiểu cần 1 test tự động so khớp 2 phía
  (kiểu "contract test"), không để lệch âm thầm như đã xảy ra.
- **UI component viết mới/hoàn thiện** (C2, D2) → chỉ được **gọi API + render** — không tính
  `progressPercent` phía client (đã có `task.recalculateProgress` ở backend), không tự resolve
  quyền phía client (đã có `task.resolvePermission`/`TaskGrantService` ở backend). Đã kiểm tra:
  code frontend hiện tại **không có** vi phạm kiểu này (không tìm thấy tính toán progress hay
  resolve permission phía client) — giữ nguyên tắc này khi viết code mới cho C2/D2.

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
quản trị", `backend/src/main/profile/OrcaProfile.ts` là **nguồn chuẩn duy nhất**.

**✅ Đã làm trong Giai đoạn 1**:
1. Xoá `frontend/src/shared/profile-types.ts` (0 importer thật, xác nhận an toàn).
2. Sửa `frontend/src/renderer/src/types/profile-types.ts` (bản UI thật đang dùng) khớp đúng
   field/enum của backend — đổi `disallowedCmds`→`disallowedCommands`,
   `sessionTimeoutHours`→`maxSessionHours`, `agent.trustPreset` khớp enum backend, di chuyển
   `agent.approvedModels`→`security.approvedModels`.

**✅ Đã xoá (2026-08-13, follow-up round 2)**: `frontend/src/main/profile/OrcaProfile.ts` và
`agent/src/main/profile/OrcaProfile.ts`, cùng 2 file vệ tinh cô lập theo (`frontend/src/main/
profile/{ProfileService,ProfileResolver}.ts` — 0 importer ngoài chính OrcaProfile.ts, xác nhận
cả cụm 3 file là 1 đảo chết hoàn toàn tách biệt). Cách làm: tạo
`frontend/src/shared/resolved-profile-type.ts` — mirror type thủ công của
`backend/src/main/profile/OrcaProfile.ts` (comment giải thích lý do không import cross-package
được: TS project boundary của `frontend`/`agent` không cho `src/shared/` reach vào `src/main/`
hay sang package khác) — sửa `project-types.ts:50` trỏ vào đó thay vì inline-import vào file
main/ chết. Bên `agent`: `agent/src/shared/project-types.ts` tự nó hoá ra cũng 0 importer thật
(đã audit sai/bỏ sót — sửa nhầm coi nó là "còn sống" vì `ResolvedProfile` được dùng) → xoá thẳng
file này thay vì viết mirror, kéo theo xoá được `OrcaProfile.ts` phía agent luôn.

**Phát hiện thêm ngoài phạm vi 5 bản đã biết**: `agent/src/shared/profile-types.ts` — bản song
sinh của `frontend/src/shared/profile-types.ts` (đã xoá ở Giai đoạn 1) nhưng bị bỏ sót phía
`agent` khi đó (audit ban đầu chỉ kiểm `frontend`, không cross-check `agent`). Xác nhận 0 importer
thật, đã xoá. Tổng cộng thực ra là **6 bản divergent**, không phải 5.

Verify: tsc lỗi giảm thật (không phải noise cache) — frontend 973→962 (-11), agent 98→97 (-1),
so sánh qua `git worktree` + xoá `tsconfig.tsbuildinfo` cả 2 phía. Test suite: frontend 0
regression (90 fail pre-existing y hệt), agent 0 regression (3 fail/2 test fail pre-existing y
hệt, xác nhận qua worktree baseline). **1.9 nay đã 3/3, đóng hoàn toàn.**

`require2FA`: ✅ đã thêm vào backend `SecurityProfileSection` trong Giai đoạn 1 (xem
[decisions-needed.md](./decisions-needed.md) mục 6). `integrations`/`fleet`: ⏳ còn mở, chưa
quyết định — tạm giữ nguyên ở frontend.

### A6. `Team` chưa tồn tại như entity thật

**Nơi sửa: backend (schema + RPC), frontend (UI thêm mới).** Backend: thêm bảng metadata `Team`
(tái dùng `orca_team_members` đã có sẵn làm bảng nối), thêm RPC `team.create/addMember/
removeMember/list`. Frontend: thêm UI quản lý Team (đặt trong trang Admin — quản trị tổ chức
thuộc "backend quản trị dữ liệu", UI của nó vẫn là frontend hiển thị). Chi tiết schema đề xuất ở
[user-profile-team-department-rbac.md](../profile/user-profile-team-department-rbac.md) mục 5.2.

## Nhóm B — Project / OrcaProject

### B1. `project.agentSpawn` luôn lỗi `AGENT_SPAWNER_NOT_AVAILABLE`

**Nơi sửa: backend.** `server-bootstrap.ts` gọi `createProjectMethods(projectService,
getUserRole)` thiếu tham số thứ 3. Sửa: khởi tạo `ProfileAwareAgentSpawner` **trước** khi đăng
ký project methods (đổi thứ tự trong `server-bootstrap.ts`), truyền vào làm tham số thứ 3.

### B2. `WorkspaceContext.switchProject()` gọi `git.status` sai tham số

**Nơi sửa: frontend.** Đổi `git.status({projectId})` → `git.status({worktree: <worktreeId>})`
đúng schema thật (`GitStatusParams extends WorktreeSelector`). Cần có `worktreeId` thật trước —
phụ thuộc quyết định nguồn `currentWorktree` (xem
[project-workspace-f38-doc-vs-code.md](../project-workspace/project-workspace-f38-doc-vs-code.md) mục 3, và
[decisions-needed.md](./decisions-needed.md)).

### B3. `workspace.listFiles` không tồn tại

**Nơi sửa: frontend, đổi tên gọi.** Method thật gần nhất là `workspace.refreshFileTree` — đổi
`WorkspaceContext.tsx` gọi đúng tên này thay vì tên tưởng tượng `workspace.listFiles`.

### B4. Cụm UI Workspace/Git/CodeReview (~14+18 component) mồ côi

**Nơi sửa: frontend (mount hoặc xoá), quyết định trước khi làm** — xem
[decisions-needed.md](./decisions-needed.md) mục "F38 release hay shelve". Nếu quyết định mount
lại: audit từng component trong cụm theo đúng nguyên tắc luồng 1 chiều ở đầu tài liệu này trước
khi bật cho người dùng thật — cụm này viết ra trước khi nguyên tắc được xác lập, chưa chắc đã
tuân thủ (ví dụ cần kiểm tra `GitPanel.tsx`/`DiffViewer.tsx` có tự tính diff/merge-state phía
client hay gọi đúng `git.*` API cho việc đó).

### B5. `WorkspaceContextV6` cạnh tranh, chưa từng dùng

**✅ Quyết định (2026-08-13): giữ nguyên, không động tới trong đợt này.** Có kế hoạch hoàn
thiện V6 làm bản nâng cấp sau, nhưng chưa phải bây giờ — không xoá, không migrate. Khi hoàn
thiện F38 (B4), tiếp tục dùng V5 (`WorkspaceContext.tsx`, đang được `main.tsx` mount) làm nền,
không chuyển sang V6 trong đợt này.

### B6. `UNAUTHENTICATED` cho `project.*` RPC ở phiên web

**Nơi sửa: backend.** Đây là lỗi tầng transport — session token của phiên WebSocket không được
forward vào `ctx.userId` cho nhánh gọi `project.*`. Cần trace lại chuỗi
`dispatcher.ts`→session router→`RpcContext` để tìm chính xác điểm rớt `userId` (không thuộc
phạm vi audit này, cần điều tra riêng bằng log thật khi bug tái diễn).

## Nhóm C — Task / OrcaTask

### C1. 7 chỗ UI gọi sai tên RPC method (`tasks.*` thay vì `task.*`, `task.runAgent` thay vì `task.execute`)

**✅ Đã sửa trong Giai đoạn 1** tại `TaskPromptEditor.tsx`, `TaskDetail.tsx`, `useTask.ts` — theo
bảng trong
[task-automation-orchestration-integration.md](../task-automation/task-automation-orchestration-integration.md)
mục 4. Không đụng backend — backend đã đúng, đã có sẵn 18 method thật. Khi sửa, phát hiện thêm
2 việc ngoài phạm vi ban đầu: (a) một vài chỗ không chỉ sai TÊN mà còn sai SHAPE tham số (`task.update`
cần `{taskId, patch}` lồng nhau, không phải spread phẳng; `task.execute` không nhận `prompt` —
backend tự dựng prompt từ `promptTemplate`) — đã sửa cùng lúc; (b) `useTasks.ts` (số nhiều, khác
`useTask.ts` số ít) cũng gọi sai `tasks.list` (đúng phải `task.list`) — **chưa sửa**, ngoài phạm
vi được giao cho agent, cần làm ở đợt sau.

### C2. `TaskGraphPanel.tsx` là stub trống

**Nơi sửa: frontend.** Viết implementation thật (hiện chỉ có comment "full implementation in
TASK-V5-07"), dùng đúng `frontend/src/shared/task-types.ts` (bản khớp backend), không dùng bản
lệch `renderer/src/types/task-types.ts`. **Chỉ gọi `task.list`/`task.getChildren`/
`task.getSubtree` và render đúng `progressPercent`/`status` API trả về** — không tự tính lại
progress hay tự suy luận trạng thái cha từ trạng thái con phía client (backend đã có
`task.recalculateProgress` làm việc này).

### C3. 2 bản `OrcaTask` type không tương thích dùng lẫn trong 1 cụm UI

**✅ Đã làm trong Giai đoạn 1**: xoá `frontend/src/renderer/src/types/task-types.ts` (bản lệch),
migrate `TaskDetail.tsx`/`TaskPromptEditor.tsx`/`TaskAIDecompose.tsx`/`TaskDAGView.tsx` sang
import đường dẫn tương đối tới `frontend/src/shared/task-types.ts` (bản khớp backend).

**⚠️ Phát hiện mới khi thực thi**: đề xuất ban đầu ghi `TaskCard.tsx`/`TaskGraph.tsx`/
`TaskStatusBadge.tsx`/`TaskTreeView.tsx` "đã import đúng `@shared/task-types`" — sai. Alias
`@shared/task-types` **không hề resolve được ở bất kỳ đâu trong repo** (không có tsconfig/vite
path alias nào định nghĩa nó) — đây là lỗi tiền tồn tại, độc lập với Giai đoạn 1, ảnh hưởng đúng
4 file này. Chưa sửa (ngoài phạm vi đã giao cho agent, cần việc riêng: đổi alias `@shared/*`
sang đường dẫn tương đối, hoặc định nghĩa alias đó trong tsconfig/vite config).

### C4. Field liên kết cross-system (`worktreeId`/`agentSessionId`/`workflowExecutionId`) chưa tồn tại trên `OrcaTask`

**Nơi sửa: backend (schema mới) + backend (service logic nối ①②③).** Xem đề xuất chi tiết ở
[task-automation-orchestration-integration.md](../task-automation/task-automation-orchestration-integration.md)
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
`workflow.*` (đã có thật, đang chạy trên backend). **Chỉ hiển thị trạng thái/kết quả step API
trả về** — không tự suy luận step nào "sẵn sàng chạy tiếp" hay tự tính trạng thái tổng thể của
workflow phía client (đó là việc của `WorkflowOrchestrator`'s `buildWaves`/DAG logic ở backend).
Cần quyết định trước: mount ở đâu (cùng cụm `WorkspaceLayout` orphaned, hay 1 chỗ độc lập)? Xem
[decisions-needed.md](./decisions-needed.md).

### D3. `OrchestrationPane.tsx`/`OrchestrationSkillPromptDialog.tsx` trùng tên gây nhầm lẫn

**Nơi sửa: frontend, đổi tên (không phải bug chức năng).** 2 component này hoạt động đúng chức
năng thật của chúng (cài CLI `orca`) — chỉ tên gọi dễ nhầm với `WorkflowOrchestrator`/
`coordinator.ts`. Đề xuất đổi tên thành `OrcaCliInstallPane.tsx`/tương tự để tránh nhầm lẫn cho
người đọc code sau này — việc nhỏ, không khẩn.

### D4. Pipeline Source→Plan→Execute (Task ↔ Orchestration coordinator) chưa tồn tại

**Nơi sửa: backend.** Xem
[task-automation-orchestration-integration.md](../task-automation/task-automation-orchestration-integration.md)
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

**Xác nhận khớp yêu cầu**: `agent` thực thi (PTY/git/fs/spawn agent CLI) → `backend` xử lý
nghiệp vụ (validate, tính toán, lưu DB) và trả kết quả qua RPC (`profile.*`/`project.*`/
`task.*`/`workflow.*`/`orchestration.*`/`automation.*`) → `frontend` chỉ gọi các RPC này và
render, không tính toán lại. Toàn bộ 17 case ở trên đã được rà lại theo đúng chiều này — không
có case nào đề xuất đặt nghiệp vụ vào frontend hay đặt quản trị/lưu trữ vào agent.
