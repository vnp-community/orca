# BUG-FE-PW-001: `CreateProjectDialog` không phát hiện repo trùng với sidebar hiện có, và không có lựa chọn "link 1 Project đã có sẵn"

## Mức độ: 🟡 MEDIUM

## Trạng thái: ✅ FIXED (2026-09-01) — xem [SOL-FE-PW-001](./solutions/SOL-FE-PW-001-duplicate-repo-detection-and-link-existing-project.md)

## Tóm tắt

TDD-FE-12 (§2 ProjectSwitcher, §4 ProjectSettings) mô tả `OrcaProject` như 1 lớp project-level
riêng biệt, nhưng không hề đặt câu hỏi: khi user tạo 1 `OrcaProject` mới bằng cách nhập lại 1
`repoPath` **đã có sẵn** trong sidebar Project/Repo chính (mô hình cũ, per-user, đa-host — xem
`docs/guides/authorization/asset-hierarchy-and-permission-model.md` mục 1.2/1.3), 2 luồng dữ liệu
này có liên hệ gì với nhau không.

Thực tế grep + đọc trực tiếp `CreateProjectDialog.tsx`:

```typescript
// frontend/src/renderer/src/components/project/CreateProjectDialog.tsx (handleSubmit)
const project = await callRuntimeRpc<OrcaProject>(target, 'project.create', {
  name: name.trim(), description: description.trim() || undefined, visibility,
})
await callRuntimeRpc(target, 'project.rebindDevServer', { projectId: project.id, newDevServerId: devServerId })
await callRuntimeRpc(target, 'repo.add', { projectId: project.id, url: repoPath.trim(), displayName: name.trim() })
```

`repo.add` luôn tạo **1 Repo mới hoàn toàn** (Go-native, `orca.project.v1.Repo`, FK
`projectId → OrcaProject.id`) — không có bước nào kiểm tra `repoPath` đã tồn tại trong
`useAppStore(s => s.repos)` (sidebar chính) hay chưa, và không có lựa chọn nào khác ngoài "nhập
path để tạo Repo mới".

## Thực tế trong UI

1. User đã có Repo `my-service` (`/home/dev/my-service`, host `dev-01`) trong sidebar chính.
2. User mở "Project Switcher" → "Create New Project" (component thật, `App.tsx` →
   `ProjectSwitcher.tsx` → `CreateProjectDialog.tsx` — xác nhận không phải code chết).
3. Chọn dev server `dev-01`, nhập lại đúng path `/home/dev/my-service`, bấm Create.
4. Kết quả: **2 bản ghi độc lập** cùng trỏ 1 thư mục vật lý — sidebar chính không đổi gì, OrcaProject
   mới không "biết" gì về lịch sử worktree/terminal đã có trên repo đó. Dialog **không cảnh báo gì**.

## Ảnh hưởng

1. Người dùng dễ tưởng đây là bug ("tôi add rồi sao Project mới không thấy?").
2. Giá trị RBAC cốt lõi của OrcaProject (chia sẻ project đang làm việc — xem "Project vs
   OrcaProject" trong `docs/guides/authorization/asset-hierarchy-and-permission-model.md`) không
   áp dụng được cho phần lớn dữ liệu người dùng ĐÃ CÓ SẴN, chỉ áp dụng cho repo tạo mới hoàn toàn.
3. Không có cách nào từ dialog này để chọn "dùng lại 1 Project có sẵn" thay vì luôn phải tạo Repo
   Go-native mới — cách duy nhất để nối 2 hệ (`orcaProjects.linkSourceProject`) không xuất hiện ở
   đâu trong UI (xem [BUG-FE-PW-002](./BUG-FE-PW-002-orcaproject-source-project-sharing-no-ui.md)).

## Root cause

- `CreateProjectDialog.tsx` chỉ implement 1 nhánh duy nhất ("tạo Repo Go-native mới qua `repo.add`")
  — không đọc `useAppStore(s => s.repos)` (đã có sẵn ở client, không cần RPC mới) để so khớp
  `path`/`executionHostId` trước khi submit.
- TDD-FE-12 không đặc tả nhánh "link" nào cả — tài liệu chỉ mô tả luồng tạo mới. `orcaProjects.*`
  (bảng `orca_project_source_projects`, xem
  `docs/guides/bugs/bug-be-project-001-project-orcaproject-no-bridge-ux.md`) là phần backend được
  thêm sau ("Wave 3 integration agent"), không có TDD nào theo sau ghi nhận nó ở tầng frontend —
  đây là lý do sâu xa vì sao chưa ai build UI cho nhánh link.

## Liên quan

- **TDD-FE-12** §2 (ProjectSwitcher), §4 (ProjectSettings) — không đặc tả nhánh "link"
- **BUG-FE-PW-002** — cần fix trước/song song, vì lựa chọn "link" phụ thuộc UI đã có ở đó
- `docs/guides/authorization/asset-hierarchy-and-permission-model.md` mục "Project vs OrcaProject", mục 5
- `docs/guides/bugs/bug-be-project-001-project-orcaproject-no-bridge-ux.md` (bản ghi nhận gốc, phạm vi rộng hơn — bug này là phần frontend cụ thể có thể fix độc lập)
