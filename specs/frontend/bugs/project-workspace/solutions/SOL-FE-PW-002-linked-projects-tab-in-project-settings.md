# SOL-FE-PW-002: Thêm tab "Linked Projects" vào `ProjectSettings` — UI cho `orcaProjects.*` sharing API

> **✅ ĐÃ IMPLEMENT (2026-09-01)** — xem TASK-FE-PW-002-A/B/C/D để biết kết quả thực tế. 2 điểm
> lệch so với bản nháp dưới đây: (1) `ProjectMember.role` thật chỉ có `'owner' | 'member'`, không
> có `'viewer'` như giả định ban đầu; (2) `currentUserRole` resolve qua `project.getMembers` +
> `AuthSlice.currentUser.id` (đã có sẵn, không phải xây mới) — mục "Không làm ở solution này" bên
> dưới (đọc chéo-user qua `getProjectData`) **vẫn CHƯA làm**, đúng như phạm vi đã khoanh.

## Bug Reference
- **Bug:** BUG-FE-PW-002
- **Mức độ:** 🔴 HIGH
- **TDD Reference:** TDD-FE-12 §4 (ProjectSettings Dialog — cần bổ sung tab thứ 4, TDD chưa đặc tả)
- **Pattern tham chiếu:** `MemberManager.tsx` (component RPC-driven đã có, cùng thư mục, cùng
  parent `ProjectSettings.tsx`) — solution này build 1 component **mới, cùng shape**, không sửa
  `MemberManager.tsx`.

---

## Root Cause

4 RPC method (`orcaProjects.linkSourceProject`/`unlinkSourceProject`/`getProjectData`/`list`)
hoàn chỉnh ở backend, không component nào gọi tới. `ProjectSettings.tsx` đã có sẵn 3 tab
(`General`/`Members`/`Repos`) theo đúng pattern "mỗi tab = 1 component RPC-driven riêng biệt" —
chỉ cần thêm tab thứ 4 theo đúng pattern đó, không cần thiết kế mới.

---

## Giải pháp

### Bước 1 — Types cho `orcaProjects.*` RPC

**File:** `frontend/src/renderer/src/types/workspace-types.ts` (cùng nơi định nghĩa `OrcaProject`/
`ProjectMember` đã dùng ở `MemberManager.tsx`/`CreateProjectDialog.tsx`)

```typescript
// Khớp SourceProjectRef ở backend/src/main/project/OrcaProjectSourceProjectService.ts
export type SourceProjectRef = {
  ownerUserId: string
  projectId: string
}

export type LinkSourceProjectParam = {
  orcaProjectId: string
  projectId: string
}

export type UnlinkSourceProjectParam = {
  orcaProjectId: string
  projectId: string
}

export type OrcaProjectListItemWithSources = {
  orcaProject: OrcaProject
  sourceProjects: SourceProjectRef[]
}
```

Không cần types cho `getProjectData` ở solution này — đó là phần đọc-chéo-user để **xem nội dung**
1 Project đã link (hiển thị file explorer/git status), phạm vi rộng hơn (đụng
`WorkspaceContextValue.project`, cần chạy `gitnexus impact` riêng trước khi đổi — xem mục "Không
làm ở đây" cuối file). Solution này chỉ cover **link/unlink/xem danh sách đã link**.

### Bước 2 — Component `LinkedProjectsManager.tsx` (component mới, mirror `MemberManager.tsx`)

**File:** `frontend/src/renderer/src/components/project/LinkedProjectsManager.tsx` (TẠO MỚI)

```tsx
// LinkedProjectsManager.tsx — quản lý orca_project_source_projects links (BUG-FE-PW-002 fix)
import { useState, useEffect, useCallback } from 'react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table'
import { Button } from '../ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { callRuntimeRpc, getActiveRuntimeTarget, RuntimeRpcCallError } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import type { SourceProjectRef, OrcaProjectListItemWithSources } from '../../types/workspace-types'
import { toast } from 'sonner'
import { Link2, Trash2 } from 'lucide-react'

// Cùng pattern message-mapping với MemberManager.tsx/CreateProjectDialog.tsx.
function describeError(err: unknown, fallback: string): string {
  const message = err instanceof RuntimeRpcCallError || err instanceof Error ? err.message : ''
  if (/^FORBIDDEN/i.test(message) || message === 'UNAUTHENTICATED') {
    return 'You do not have permission to do that.'
  }
  return message || fallback
}

export function LinkedProjectsManager({
  orcaProjectId,
  currentUserRole,        // 'owner' | 'member' | 'viewer' — truyền từ ProjectSettings, KHÔNG tự suy luận lại RBAC ở đây
}: {
  orcaProjectId: string
  currentUserRole: 'owner' | 'member' | 'viewer'
}) {
  const [sourceProjects, setSourceProjects] = useState<SourceProjectRef[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [linking, setLinking] = useState(false)

  // Project của chính user hiện tại — sidebar chính, đã có sẵn ở client (không cần RPC mới).
  const myProjects = useAppStore(s => s.projects)
  const canUnlink = currentUserRole === 'owner'   // khớp requireOwnerOrAdmin ở backend — chỉ để ẩn/hiện nút, backend vẫn là nguồn chân lý

  const load = useCallback(async () => {
    setIsLoading(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // orcaProjects.list() trả TẤT CẢ OrcaProject của caller — lọc đúng cái đang xem.
      const all = await callRuntimeRpc<OrcaProjectListItemWithSources[]>(target, 'orcaProjects.list', null)
      const mine = all.find(item => item.orcaProject.id === orcaProjectId)
      setSourceProjects(mine?.sourceProjects ?? [])
    } catch {
      toast.error('Failed to load linked projects')
    } finally {
      setIsLoading(false)
    }
  }, [orcaProjectId])

  useEffect(() => { load() }, [load])

  const linkProject = async () => {
    if (!selectedProjectId) {return}
    setLinking(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'orcaProjects.linkSourceProject', {
        orcaProjectId, projectId: selectedProjectId,
      })
      setSelectedProjectId('')
      toast.success('Project linked')
      await load()
    } catch (err) {
      toast.error(describeError(err, 'Failed to link project'))
    } finally {
      setLinking(false)
    }
  }

  const unlinkProject = async (projectId: string) => {
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      await callRuntimeRpc(target, 'orcaProjects.unlinkSourceProject', { orcaProjectId, projectId })
      setSourceProjects(prev => prev.filter(s => s.projectId !== projectId))
      toast.success('Project unlinked')
    } catch (err) {
      toast.error(describeError(err, 'Failed to unlink project'))
    }
  }

  // Không cho chọn lại 1 Project đã link rồi — tránh gọi linkSourceProject dư thừa (idempotent ở
  // backend nên không lỗi, nhưng dropdown gọn hơn nếu lọc trước).
  const linkedIds = new Set(sourceProjects.map(s => s.projectId))
  const linkableProjects = myProjects.filter(p => !linkedIds.has(p.id))

  return (
    <div className="linked-projects-manager" data-testid="linked-projects-manager">
      <div className="flex items-end gap-2 pb-3" data-testid="link-project-form">
        <div className="flex-1 grid gap-1.5">
          <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
            <SelectTrigger data-testid="link-project-select">
              <SelectValue placeholder={linkableProjects.length === 0 ? 'No projects to link' : 'Choose a Project'} />
            </SelectTrigger>
            <SelectContent>
              {linkableProjects.map(p => (
                <SelectItem key={p.id} value={p.id}>{p.displayName}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Button
          type="button" size="icon" disabled={linking || !selectedProjectId}
          onClick={linkProject} data-testid="link-project-submit" aria-label="Link project"
        >
          <Link2 size={14} />
        </Button>
      </div>

      {isLoading ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="linked-loading">Loading…</div>
      ) : sourceProjects.length === 0 ? (
        <div className="p-4 text-sm text-muted-foreground" data-testid="linked-empty">
          No projects linked yet.
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Project</TableHead>
              <TableHead>Owner</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {sourceProjects.map(s => {
              const label = myProjects.find(p => p.id === s.projectId)?.displayName ?? s.projectId
              return (
                <TableRow key={s.projectId} data-testid={`linked-row-${s.projectId}`}>
                  <TableCell><p className="font-medium text-sm">{label}</p></TableCell>
                  <TableCell><p className="text-xs text-muted-foreground">{s.ownerUserId}</p></TableCell>
                  <TableCell>
                    {canUnlink ? (
                      <Button
                        variant="ghost" size="icon" className="h-7 w-7"
                        onClick={() => unlinkProject(s.projectId)}
                        data-testid={`unlink-project-${s.projectId}`}
                      >
                        <Trash2 size={12} />
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
```

### Bước 3 — Wire vào `ProjectSettings.tsx`

**File:** `frontend/src/renderer/src/components/project/ProjectSettings.tsx` (MODIFY)

```tsx
import { LinkedProjectsManager } from './LinkedProjectsManager'
// ...
<TabsList>
  <TabsTrigger value="general" data-testid="tab-general">General</TabsTrigger>
  <TabsTrigger value="members" data-testid="tab-members">Members</TabsTrigger>
  <TabsTrigger value="repos" data-testid="tab-repos">Repos</TabsTrigger>
  <TabsTrigger value="linked" data-testid="tab-linked">Linked Projects</TabsTrigger>
</TabsList>
{/* ... 3 TabsContent hiện có giữ nguyên ... */}
<TabsContent value="linked" className="py-2">
  {/* currentUserRole: ProjectSettings chưa có prop này hôm nay. CHƯA XÁC NHẬN có RPC riêng nào
      trả role của chính user hiện tại (project.getMember là method nội bộ của ProjectService.ts,
      không có bằng chứng được đăng ký thành RPC) — xem TASK-FE-PW-002-C cho cách resolve an toàn
      bằng project.getMembers() (đã xác nhận real) + lọc theo userId hiện tại. */}
  <LinkedProjectsManager orcaProjectId={projectId} currentUserRole={currentUserRole} />
</TabsContent>
```

**Lưu ý quan trọng:** `ProjectSettings.tsx` hiện **không có** sẵn `currentUserRole` của user đang
xem (chỉ có `project` từ `useWorkspace()`, không có role). Cần 1 bước nhỏ resolve role trước khi
render tab này — xem TASK-FE-PW-002-C.

---

## Files cần tạo/sửa

| File | Action | Ghi chú |
|------|--------|---------|
| `frontend/src/renderer/src/types/workspace-types.ts` | MODIFY | Thêm `SourceProjectRef`, `LinkSourceProjectParam`, `UnlinkSourceProjectParam`, `OrcaProjectListItemWithSources` |
| `frontend/src/renderer/src/components/project/LinkedProjectsManager.tsx` | CREATE | Component mới, mirror `MemberManager.tsx` |
| `frontend/src/renderer/src/components/project/ProjectSettings.tsx` | MODIFY | Thêm tab thứ 4 + resolve `currentUserRole` |
| `frontend/src/renderer/src/components/project/__tests__/LinkedProjectsManager.test.tsx` | CREATE | Theo pattern `MemberManager.test.tsx` |

---

## Verification

```bash
# 1. Grep verify component mới gọi đúng 3 RPC (list dùng chung với load ban đầu):
grep -n "orcaProjects\." frontend/src/renderer/src/components/project/LinkedProjectsManager.tsx

# 2. Grep verify tab đã wire:
grep -n "tab-linked" frontend/src/renderer/src/components/project/ProjectSettings.tsx

# 3. Test (Vitest, theo pattern MemberManager.test.tsx):
# - load(): gọi orcaProjects.list, lọc đúng orcaProjectId, hiển thị đúng sourceProjects
# - link: chọn 1 Project trong dropdown (chỉ project CHƯA link), gọi linkSourceProject, reload list
# - unlink: chỉ hiện nút khi currentUserRole==='owner', gọi unlinkSourceProject, xoá khỏi UI ngay
# - empty state khi chưa link Project nào
# - lỗi FORBIDDEN/UNAUTHENTICATED hiển thị đúng message thân thiện (describeError)
```

---

## Không làm ở solution này (phạm vi riêng, cần plan/impact-analysis trước)

- **Đọc dữ liệu chéo-user khi user mở 1 Project đã link** (gọi `orcaProjects.getProjectData` và
  hiển thị qua file explorer/git panel) — đụng `WorkspaceContextValue.project`
  (`WorkspaceContext.tsx`), có **51 caller** theo CodeGraph. Bắt buộc chạy `gitnexus impact
  --target switchProject` (và/hoặc `WorkspaceContextValue`) trước khi đổi bất kỳ gì ở đây, theo
  đúng quy tắc bắt buộc của dự án (CLAUDE.md — "MUST run impact analysis before editing any
  symbol"). Đề xuất tách thành 1 solution riêng (SOL-FE-PW-003) sau khi solution này đã merge và
  ổn định.
- **Resolve `currentUserRole` cho `ProjectSettings.tsx`** — `project.getMember(projectId, userId)`
  chỉ xác nhận là method NỘI BỘ của `ProjectService.ts` (dùng trong `assertAccess()`), **chưa xác
  nhận** có được đăng ký thành RPC method độc lập gọi được từ client hay không. Cách chắc chắn
  hoạt động: dùng `project.getMembers(projectId)` (RPC đã xác nhận real, dùng ở `MemberManager.tsx`)
  rồi lọc theo userId hiện tại. Một cách khác là mở rộng `WorkspaceContextValue` để expose sẵn —
  cách này đụng vào cùng bề mặt rủi ro 51-caller ở trên nên KHÔNG chọn ở đây. Xem TASK-FE-PW-002-C
  cho chi tiết + việc còn cần xác nhận (nguồn "userId hiện tại" ở phía client).
