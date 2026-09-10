# BUG-BE-PROJECT-002: `orcaProjects.linkSourceProject`/`unlinkSourceProject`/`getProjectData`/`list` hoàn chỉnh ở backend, có test — nhưng KHÔNG có UI nào gọi tới

**Phát hiện:** 2026-09-01, từ câu hỏi "cấu hình kết nối `OrcaProjectSourceProject` ở giao diện thế
nào?" — xác nhận bằng grep (`0` kết quả) cho `linkSourceProject`/`unlinkSourceProject`/
`orcaProjects.list`/`orcaProjects.getProjectData` trong toàn bộ `frontend/src` và `desktop/src`.

## Triệu chứng thật

Backend đầy đủ, đang chạy thật, có test:
- `backend/src/main/project/OrcaProjectSourceProjectService.ts` — `linkProject`/`unlinkProject`/
  `listSourceProjects`/`listOrcaProjectsForOwner`, bảng SQL thật `orca_project_source_projects`.
- `backend/src/main/project/orca-project-sharing-rpc-handler.ts` — 4 RPC method:
  `orcaProjects.linkSourceProject`, `.unlinkSourceProject`, `.getProjectData`, `.list`.
- Test: `backend/src/main/project/__tests__/orca-project-sharing-rpc-handler.test.ts`.
- Audit trail sẵn có: `Tracers.orcaProjectSharingFlow` ghi `orcaProjectId`/`actingUserId`/
  `ownerUserId`/`projectId` cho mỗi lần đọc chéo-user.

Nhưng: không có nút, không có dialog, không có màn hình nào trong app gọi 4 method này. Tính năng
chỉ dùng được qua RPC trực tiếp (Postman/CLI/test) — người dùng thật không tiếp cận được.

## Root cause

Comment trong `orca-project-sharing-rpc-handler.ts`: *"Spread into ALL_RPC_METHODS at bootstrap
(wiring done by the Wave 3 integration agent)"* — cho thấy đây nhiều khả năng là kết quả của 1
đợt tích hợp backend-only theo kế hoạch nhiều phase, và phase UI tương ứng chưa tới lượt/chưa
được lên lịch, không hẳn là "quên hoàn toàn". Đây là pattern đã lặp lại nhiều lần trong codebase
này (xem `docs/guides/authorization/asset-hierarchy-and-permission-model.md` mục 7: RBAC UI cho
OrcaProject/Task cũng cùng tình trạng "backend xong, UI mồ côi").

## Mức độ ảnh hưởng

- Đây là cơ chế **DUY NHẤT** để đưa 1 Project đã có sẵn (đa-host, sidebar chính) vào phạm vi chia
  sẻ RBAC của OrcaProject — không có nó, lý do OrcaProject tồn tại (xem
  [Project vs OrcaProject](../authorization/asset-hierarchy-and-permission-model.md#project-vs-orcaproject--vì-sao-tồn-tại-cả-2-và-có-nên-gộp-không))
  chỉ áp dụng được cho repo tạo mới hoàn toàn qua `repo.add` (xem
  [BUG-BE-PROJECT-001](./bug-be-project-001-project-orcaproject-no-bridge-ux.md)), không áp dụng
  được cho phần lớn dữ liệu người dùng đã có sẵn.
- Không có cách nào từ UI để: (a) xem Project nào đang được share vào 1 OrcaProject, (b) share
  thêm 1 Project, (c) gỡ share. Toàn bộ vòng đời phải làm thủ công qua RPC.

## Đề xuất fix

**1. Thiết kế UI — "Linked Projects" panel trong Project Settings của OrcaProject**

Vị trí đề xuất: 1 tab mới trong màn hình Project Settings hiện có của OrcaProject (nơi
`ProjectSwitcher.tsx`/`WorkspaceContext.tsx` đã dẫn tới), chỉ hiện cho user đã là member (mọi
role đều xem được danh sách, chỉ owner/admin thấy nút Unlink theo đúng RBAC đã có ở
`orca-project-sharing-rpc-handler.ts`):

```
┌─ Linked Projects ────────────────────────────────────────┐
│  Backend (owned by you)                    [Unlink]       │
│  Mobile App (owned by nguyen.van.a)        [Unlink]*      │
│                                                            │
│  [ + Link a Project ]                                     │
└────────────────────────────────────────────────────────────┘
* nút Unlink chỉ hiện nếu currentUser.role ∈ {owner} hoặc currentUser là global admin —
  khớp đúng `requireOwnerOrAdmin` đã enforce ở backend, KHÔNG suy luận lại RBAC ở frontend.
```

- Load dữ liệu: `orcaProjects.list()` — đã trả sẵn `sourceProjects` cho mỗi OrcaProject, không
  cần RPC mới.
- "Link a Project": dropdown chọn 1 trong các `Project` của chính user hiện tại (lấy từ store
  `repos.ts`, đã có ở client — không cần fetch thêm) → gọi `orcaProjects.linkSourceProject
  ({orcaProjectId, projectId})`. Không cho nhập tay `projectId`/`ownerUserId` — luôn chọn từ danh
  sách project CỦA CHÍNH USER hiện tại, khớp đúng ràng buộc backend (`ownerUserId` luôn = `ctx.
  userId`, không nhận từ client).
- "Unlink": confirm dialog ("Members will no longer be able to view this Project's files") →
  `orcaProjects.unlinkSourceProject({orcaProjectId, projectId})`.

**2. Đọc dữ liệu chéo-user khi user mở 1 Project đã link**

`WorkspaceContext.tsx`'s `switchProject()` hiện chỉ gọi `project.get`/`workspace.refreshFileTree`
— đây là dữ liệu của OrcaProject-native (Repo Go-native). Cần thêm nhánh: nếu `projectId` đang mở
thực ra là 1 **linked source Project** (không phải OrcaProject gốc), gọi
`orcaProjects.getProjectData({orcaProjectId, projectId})` thay vì `project.get`, và hiển thị kết
quả (`{ project, repos, worktreeMeta }`) qua đúng UI file explorer/git panel hiện có — cần xác
định rõ contract phân biệt "đang xem OrcaProject gốc" vs "đang xem 1 linked Project" ở tầng
routing/state trước khi code (tránh nhầm lẫn 2 loại `project` khác shape trong cùng 1 field
`WorkspaceContextValue.project`).

**3. Test**

- Unit test cho UI mới, theo đúng pattern đã có ở
  `frontend/src/renderer/src/components/project/__tests__/CreateProjectDialog.test.tsx`.
- Bổ sung 1 test tích hợp (nếu môi trường cho phép) mô phỏng đúng luồng "A link → B xem qua
  `getProjectData`" từ UI xuống RPC thật — hiện chỉ có test ở tầng RPC handler
  (`orca-project-sharing-rpc-handler.test.ts`), chưa có test nào phủ đường đi từ click chuột tới
  kết quả hiển thị. Đây là đường có rủi ro rò rỉ dữ liệu chéo-user nếu `filterOwnerProjectData()`
  bị sửa sai sau này mà không có test UI bảo vệ.

## Việc CHƯA làm

Chưa code bất kỳ component/UI nào ở trên — đây là bug report + đề xuất thiết kế, chờ xác nhận
phạm vi (đặc biệt mục 2, cần quyết định rõ contract `WorkspaceContextValue.project` trước khi
đụng vào — ảnh hưởng ≥51 caller của `useWorkspace()` theo CodeGraph, cần chạy `gitnexus impact`
trên `switchProject`/`WorkspaceContextValue` trước khi sửa, đúng quy tắc bắt buộc của dự án).

## Spec thực thi đầy đủ — ✅ ĐÃ FIX phần link/unlink/list (2026-09-01), ⚠️ đọc chéo-user chưa làm

Bug này đã được tách thành spec chi tiết (bám theo `specs/frontend/tdd`, cụ thể TDD-FE-12 §4
ProjectSettings), có solution + task thực thi từng bước, **đã code xong và có test pass cho phần
link/unlink/xem danh sách**, lưu tại `specs/frontend/bugs/project-workspace/`:
- Bug: `BUG-FE-PW-002-orcaproject-source-project-sharing-no-ui.md`
- Solution: `solutions/SOL-FE-PW-002-linked-projects-tab-in-project-settings.md` (phạm vi:
  link/unlink/xem danh sách — KHÔNG cover phần đọc-chéo-user `getProjectData`, xem mục "Không làm
  ở solution này" trong file đó)
- Tasks: `tasks/TASK-FE-PW-002-A` (types) → `TASK-FE-PW-002-B` (component) →
  `TASK-FE-PW-002-C` (wire vào ProjectSettings) → `TASK-FE-PW-002-D` (tests)
