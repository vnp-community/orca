# TASK-FE-PW-002-C: Wire tab "Linked Projects" vào `ProjectSettings.tsx` + resolve `currentUserRole`

**Domain:** project-workspace
**Solution Ref:** SOL-FE-PW-002 Bước 3
**Priority:** 🔴 P0
**Estimated:** 1.5 giờ
**Status:** ✅ DONE (2026-09-01)

---

## Mục tiêu

Thêm tab thứ 4 vào `ProjectSettings.tsx` render `LinkedProjectsManager`, và resolve
`currentUserRole` của user đang xem project — giá trị này **chưa tồn tại** ở component hiện tại.

---

## ⚠️ Cảnh báo bắt buộc trước khi bắt đầu

`ProjectSettings.tsx` được render bởi `ProjectSwitcher.tsx`, tiêu thụ `useWorkspace()` — theo
CodeGraph, `useWorkspace()` có **51 caller**. Task này **chỉ thêm 1 tab mới + 1 lệnh gọi RPC cục
bộ trong `ProjectSettings.tsx`**, KHÔNG đổi `WorkspaceContextValue`/`switchProject()` — nên nằm
ngoài vùng ảnh hưởng của 51 caller đó. Nếu trong lúc code phát sinh nhu cầu sửa
`WorkspaceContextValue` (ví dụ để expose `currentUserRole` toàn cục thay vì resolve cục bộ), DỪNG
LẠI và chạy `gitnexus impact --target WorkspaceContextValue --direction upstream` trước khi tiếp
tục, đúng quy tắc bắt buộc của dự án.

---

## Files cần sửa

1. `frontend/src/renderer/src/components/project/ProjectSettings.tsx`

---

## Các bước thực thi

### Bước 1: Resolve `currentUserRole` cục bộ trong `ProjectSettings.tsx` — ĐÃ XÁC NHẬN

Đã xác nhận `project.getMember` (số ít) **không tồn tại** như RPC (grep 0 kết quả, khớp nghi ngờ
ban đầu). Nguồn "tôi là ai" ở client: `store/slices/auth.ts`'s `AuthSlice.currentUser` — populate
qua `GET /auth/me` lúc bootstrap (`checkSession()`), đã sẵn có trong `useAppStore`, không cần xây
gì mới. Cách đã implement, dùng `project.getMembers` (đã xác nhận real) + lọc theo
`currentUser.id`:

```typescript
const [currentUserRole, setCurrentUserRole] = useState<ProjectMember['role'] | null>(null)

useEffect(() => {
  if (!open) {return}
  const target = getActiveRuntimeTarget(useAppStore.getState().settings)
  const myUserId = useAppStore.getState().currentUser?.id
  if (!myUserId) {
    setCurrentUserRole(null)
    return
  }
  callRuntimeRpc<ProjectMember[]>(target, 'project.getMembers', { projectId })
    .then(members => {
      const me = members.find(m => m.userId === myUserId)
      setCurrentUserRole(me?.role ?? null)
    })
    .catch(() => setCurrentUserRole(null))
}, [open, projectId])
```

`ProjectMember['role']` là `'owner' | 'member'` (đã sửa từ giả định `'viewer'` ban đầu — xem
TASK-FE-PW-002-A).

### Bước 2: Thêm tab

```tsx
import { LinkedProjectsManager } from './LinkedProjectsManager'

<TabsList>
  <TabsTrigger value="general" data-testid="tab-general">General</TabsTrigger>
  <TabsTrigger value="members" data-testid="tab-members">Members</TabsTrigger>
  <TabsTrigger value="repos" data-testid="tab-repos">Repos</TabsTrigger>
  <TabsTrigger value="linked" data-testid="tab-linked">Linked Projects</TabsTrigger>
</TabsList>

{/* ... 3 TabsContent hiện có, giữ nguyên ... */}

<TabsContent value="linked" className="py-2">
  {currentUserRole ? (
    <LinkedProjectsManager orcaProjectId={projectId} currentUserRole={currentUserRole} />
  ) : (
    <p className="p-4 text-sm text-muted-foreground">Loading…</p>
  )}
</TabsContent>
```

---

## Verify

```bash
grep -n "tab-linked\|LinkedProjectsManager" frontend/src/renderer/src/components/project/ProjectSettings.tsx
```

Test (mở rộng `ProjectSettings.test.tsx` hiện có):
- Tab "Linked Projects" render đúng, mở lên gọi `project.getMembers` rồi truyền đúng role xuống `LinkedProjectsManager`.
- Không phá vỡ 3 tab hiện có (regression guard — chạy lại toàn bộ test file này).

## Depends on
TASK-FE-PW-002-A, TASK-FE-PW-002-B

## Blocking
TASK-FE-PW-002-D

---

## Kết quả thực tế (2026-09-01)

Implement đúng như Bước 1 đã xác nhận ở trên + tab thứ 4 wire đúng spec. 2 test mới trong
`ProjectSettings.test.tsx` (resolve role thành `'owner'` khi có membership, `'none'` khi không) —
pass, cộng 6 test cũ vẫn pass (8/8 tổng). `tsc --noEmit`: 0 lỗi mới. `gitnexus impact upstream`
trên `ProjectSettings`: LOW risk, 0 upstream caller theo callgraph tĩnh (render qua JSX từ
`ProjectSwitcher` là dynamic-dispatch, không phải `CALLS` edge — đã xác nhận qua
`codegraph_explore` trước khi sửa). Không đụng `WorkspaceContextValue`/`switchProject()` — đúng
cảnh báo bắt buộc ở đầu file này.
