# TASK-FE-PW-002-A: Thêm types cho `orcaProjects.*` sharing RPC

**Domain:** project-workspace
**Solution Ref:** SOL-FE-PW-002 Bước 1
**Priority:** 🔴 P0 — prerequisite cho mọi task tiếp theo (B, C) và cho TASK-FE-PW-001-B
**Estimated:** 20 phút
**Status:** ✅ DONE (2026-09-01)

**Kết quả thực tế:** 4 type thêm đúng như spec vào `frontend/src/renderer/src/types/
workspace-types.ts`, đặt cạnh `OrcaProject`/`ProjectMember`. `tsc --noEmit`: 0 lỗi mới.

**Phát hiện lệch so với giả định ban đầu (đã sửa các task/solution phụ thuộc):** `ProjectMember`
thật ở file này chỉ có `role: 'owner' | 'member'` — **không có `'viewer'`** (comment trong code:
khớp đúng `project.project_members`'s DB CHECK constraint và `project.rego`'s action_roles). Các
task B/C dùng `'owner' | 'member' | null`, không phải `'owner' | 'member' | 'viewer' | null` như
phác thảo ban đầu trong SOL-FE-PW-002.

---

## Mục tiêu

Thêm type definitions khớp đúng shape RPC thật (`SourceProjectRef` ở
`backend/src/main/project/OrcaProjectSourceProjectService.ts`) để các component dùng
`callRuntimeRpc<T>` có type-safety, không gõ tay object literal không kiểm tra.

---

## Files cần sửa

1. `frontend/src/renderer/src/types/workspace-types.ts`

---

## Các bước thực thi

### Bước 1: Thêm types

```typescript
// Khớp SourceProjectRef ở backend/src/main/project/OrcaProjectSourceProjectService.ts:33-36
export type SourceProjectRef = {
  ownerUserId: string
  projectId: string
}

// Khớp params của orcaProjects.linkSourceProject (orca-project-sharing-rpc-handler.ts:147-149)
export type LinkSourceProjectParam = {
  orcaProjectId: string
  projectId: string
}

// Khớp params của orcaProjects.unlinkSourceProject (orca-project-sharing-rpc-handler.ts:165-167)
export type UnlinkSourceProjectParam = {
  orcaProjectId: string
  projectId: string
}

// Khớp return shape của orcaProjects.list (orca-project-sharing-rpc-handler.ts:246-260)
export type OrcaProjectListItemWithSources = {
  orcaProject: OrcaProject
  sourceProjects: SourceProjectRef[]
}
```

Đặt cạnh `OrcaProject`/`ProjectMember` type hiện có trong cùng file — không tạo file mới, tránh
phân mảnh thêm types cho cùng domain.

---

## Verify

```bash
grep -n "SourceProjectRef\|LinkSourceProjectParam\|UnlinkSourceProjectParam\|OrcaProjectListItemWithSources" \
  frontend/src/renderer/src/types/workspace-types.ts
```

`tsc --noEmit` sạch (0 lỗi mới) sau khi thêm — types thuần, không có logic runtime nên rủi ro
regression gần như bằng 0.

## Depends on
Không có

## Blocking
TASK-FE-PW-002-B, TASK-FE-PW-002-C, TASK-FE-PW-001-B
