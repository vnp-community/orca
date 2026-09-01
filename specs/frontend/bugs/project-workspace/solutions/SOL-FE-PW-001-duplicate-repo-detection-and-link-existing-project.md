# SOL-FE-PW-001: Cảnh báo trùng repo + thêm chế độ "Link Existing Project" vào `CreateProjectDialog`

> **✅ ĐÃ IMPLEMENT (2026-09-01)** — xem TASK-FE-PW-001-A/B để biết kết quả thực tế và các điểm
> lệch nhỏ so với bản nháp dưới đây (chủ yếu: đọc store qua `getState()` thay vì hook
> `useAppStore(selector)` để khớp convention có sẵn trong `components/project/*`).

## Bug Reference
- **Bug:** BUG-FE-PW-001
- **Mức độ:** 🟡 MEDIUM
- **TDD Reference:** TDD-FE-12 §2 (ProjectSwitcher / "Create New Project" entry point)
- **Phụ thuộc:** SOL-FE-PW-002 Bước 1 (types) phải xong trước Bước 2 của solution này

---

## Root Cause

`CreateProjectDialog.tsx` chỉ có 1 nhánh nộp form: `project.create` → `project.rebindDevServer` →
`repo.add`. Không kiểm tra `repoPath` đã tồn tại trong sidebar chính (`useAppStore(s => s.repos)`)
hay chưa, và không có lựa chọn nào khác để "dùng lại 1 Project có sẵn" thay vì luôn tạo Repo
Go-native mới.

---

## Giải pháp

### Bước 1 — Phát hiện trùng lặp (client-side, không cần RPC mới)

**File:** `frontend/src/renderer/src/components/project/CreateProjectDialog.tsx`

`state.repos` (Zustand store, `repos.ts` slice) đã có sẵn `path` + `executionHostId` cho mọi Repo
trong sidebar chính — dùng trực tiếp, không cần fetch thêm:

```typescript
import { useAppStore } from '../../store'

// Trong component, trước return:
const existingRepos = useAppStore(s => s.repos)

function findDuplicateRepo(path: string, devServerId: string) {
  const normalizedPath = path.trim().replace(/\/+$/, '')
  return existingRepos.find(r =>
    r.path.replace(/\/+$/, '') === normalizedPath &&
    r.executionHostId === `devServer:${devServerId}`
  )
}
```

Trong JSX, ngay dưới field "Repo Path", thêm cảnh báo inline (không chặn submit):

```tsx
{repoPath.trim() && devServerId && findDuplicateRepo(repoPath, devServerId) ? (
  <p className="text-xs text-amber-600" data-testid="cp-duplicate-repo-warning">
    Repo này đã có trong sidebar của bạn ({findDuplicateRepo(repoPath, devServerId)?.displayName}).
    Tạo Project mới ở đây sẽ KHÔNG liên kết với dữ liệu đó — chúng sẽ độc lập.{' '}
    <button type="button" className="underline" onClick={() => setMode('link')}>
      Link Project có sẵn thay vào đó?
    </button>
  </p>
) : null}
```

### Bước 2 — Thêm chế độ "Link Existing Project"

Tách form thành 2 chế độ bằng `Tabs` (component `../ui/tabs` đã dùng ở `ProjectSettings.tsx`,
tái dùng nguyên pattern, không cần thư viện mới):

```tsx
type DialogMode = 'new-repo' | 'link'
const [mode, setMode] = useState<DialogMode>('new-repo')

// Thêm state cho danh sách Project của chính user (từ sidebar, KHÔNG cần RPC mới):
const myProjects = useAppStore(s => s.projects)   // legacy Project[] — xem ghi chú ProjectSettings.tsx
const [selectedProjectId, setSelectedProjectId] = useState('')
```

```tsx
<Tabs value={mode} onValueChange={v => setMode(v as DialogMode)}>
  <TabsList>
    <TabsTrigger value="new-repo">New Repo</TabsTrigger>
    <TabsTrigger value="link">Link Existing Project</TabsTrigger>
  </TabsList>

  <TabsContent value="new-repo">
    {/* form hiện tại: Dev Server + Repo Path — GIỮ NGUYÊN */}
  </TabsContent>

  <TabsContent value="link">
    <div className="grid gap-1.5">
      <Label htmlFor="cp-link-project">Project của bạn</Label>
      <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
        <SelectTrigger id="cp-link-project"><SelectValue placeholder="Chọn 1 Project" /></SelectTrigger>
        <SelectContent>
          {myProjects.map(p => (
            <SelectItem key={p.id} value={p.id}>{p.displayName}</SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-xs text-muted-foreground">
        Chia sẻ 1 Project bạn đã có sẵn (đa-host) với các thành viên khác của OrcaProject này.
      </p>
    </div>
  </TabsContent>
</Tabs>
```

### Bước 3 — Nhánh submit riêng cho mode `link`

```typescript
async function handleSubmit(e: FormEvent) {
  e.preventDefault()
  if (mode === 'link') {
    if (!name.trim() || !selectedProjectId) {return}
    setSubmitting(true); setError(null)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const project = await callRuntimeRpc<OrcaProject>(target, 'project.create', {
        name: name.trim(), description: description.trim() || undefined, visibility,
      })
      // orcaProjects.linkSourceProject — xem SOL-FE-PW-002 Bước 1 cho type LinkSourceProjectParam
      await callRuntimeRpc(target, 'orcaProjects.linkSourceProject', {
        orcaProjectId: project.id,
        projectId: selectedProjectId,
      })
      onCreated(project)
      onOpenChange(false)
      resetForm()
    } catch (err) {
      setError(describeError(err, 'Failed to create project.'))
    } finally {
      setSubmitting(false)
    }
    return
  }
  // ... nhánh 'new-repo' hiện tại giữ nguyên, không đổi ...
}
```

**Lưu ý:** nhánh `link` **không** gọi `project.rebindDevServer` hay `repo.add` — Project được link
giữ nguyên devServer/repo path gốc của nó (đa-host), không gán lại 1 dev server duy nhất.

---

## Files cần sửa

| File | Action | Ghi chú |
|------|--------|---------|
| `frontend/src/renderer/src/components/project/CreateProjectDialog.tsx` | MODIFY | Thêm duplicate detection + tab mode `new-repo`/`link` |
| `frontend/src/renderer/src/components/project/__tests__/CreateProjectDialog.test.tsx` | MODIFY | Thêm test cho cảnh báo trùng + luồng `link` |

---

## Verification

```bash
# 1. Grep verify cảnh báo trùng đã thêm:
grep -n "cp-duplicate-repo-warning" frontend/src/renderer/src/components/project/CreateProjectDialog.tsx

# 2. Grep verify nhánh link gọi đúng RPC:
grep -n "orcaProjects.linkSourceProject" frontend/src/renderer/src/components/project/CreateProjectDialog.tsx

# 3. Test (Vitest):
# - hiển thị cảnh báo khi repoPath+devServerId trùng 1 repo trong store.repos
# - không hiển thị cảnh báo khi không trùng
# - mode 'link': submit gọi project.create rồi orcaProjects.linkSourceProject, KHÔNG gọi repo.add/rebindDevServer
# - mode 'new-repo': hành vi cũ không đổi (regression guard)
```

---

## Liên quan

- **TDD-FE-12** §2 — cần bổ sung addendum mô tả 2 mode của `CreateProjectDialog` sau khi fix
- **SOL-FE-PW-002** — cung cấp type `LinkSourceProjectParam` dùng ở Bước 3 trên
