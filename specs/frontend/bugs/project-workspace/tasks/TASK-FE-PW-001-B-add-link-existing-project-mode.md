# TASK-FE-PW-001-B: Thêm chế độ "Link Existing Project" vào `CreateProjectDialog`

**Domain:** project-workspace
**Solution Ref:** SOL-FE-PW-001 Bước 2 + 3
**Priority:** 🟡 P1
**Estimated:** 2 giờ
**Status:** ✅ DONE (2026-09-01)

**Kết quả thực tế:** `CreateProjectDialog.tsx` giờ có 2 mode qua `Tabs` thật (`new-repo`/`link`),
`handleSubmit()` rẽ nhánh đúng như spec — mode `link` gọi `project.create` rồi
`orcaProjects.linkSourceProject`, **không** gọi `repo.add`/`project.rebindDevServer` (đã có test
assert `not.toHaveBeenCalledWith` cho cả 2 method này). Nút submit disable đúng theo từng mode.
Test mới: 4 test trong `describe('link existing project mode')` (disable logic, submit thành
công, submit lỗi FORBIDDEN) — pass. Toàn bộ 6 test cũ của file này vẫn pass nguyên vẹn (regression
guard). Ghi chú kỹ thuật: phải thêm mock riêng cho `../../ui/tabs` trong test (Radix Tabs thật
không đổi tab một cách đáng tin cậy dưới `fireEvent.click` trong happy-dom — cùng lý do
`ProfileEditor.test.tsx` cũng mock `ui/tabs`), không nằm trong dự tính ban đầu của solution.

---

## Mục tiêu

Cho phép tạo 1 `OrcaProject` bằng cách **link 1 Project đã có sẵn** (sidebar chính) thay vì luôn
phải nhập path để tạo Repo Go-native mới.

---

## Files cần sửa

1. `frontend/src/renderer/src/components/project/CreateProjectDialog.tsx`
2. `frontend/src/renderer/src/components/project/__tests__/CreateProjectDialog.test.tsx`

---

## Các bước thực thi

### Bước 1: Thêm state chọn mode + Project

```typescript
type DialogMode = 'new-repo' | 'link'
const [mode, setMode] = useState<DialogMode>('new-repo')
const myProjects = useAppStore(s => s.projects)
const [selectedProjectId, setSelectedProjectId] = useState('')
```

### Bước 2: Bọc form hiện tại trong `Tabs`

```tsx
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'

<Tabs value={mode} onValueChange={v => setMode(v as DialogMode)}>
  <TabsList>
    <TabsTrigger value="new-repo" data-testid="cp-mode-new-repo">New Repo</TabsTrigger>
    <TabsTrigger value="link" data-testid="cp-mode-link">Link Existing Project</TabsTrigger>
  </TabsList>

  <TabsContent value="new-repo">
    {/* Dev Server + Repo Path fields hiện có — di chuyển nguyên vào đây, KHÔNG đổi logic */}
  </TabsContent>

  <TabsContent value="link">
    <div className="grid gap-1.5 py-2">
      <Label htmlFor="cp-link-project">Project của bạn</Label>
      <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
        <SelectTrigger id="cp-link-project" data-testid="cp-link-project-select">
          <SelectValue placeholder="Chọn 1 Project" />
        </SelectTrigger>
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

### Bước 3: Nhánh submit riêng cho `mode === 'link'`

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
      await callRuntimeRpc(target, 'orcaProjects.linkSourceProject', {
        orcaProjectId: project.id, projectId: selectedProjectId,
      })
      onCreated(project); onOpenChange(false); resetForm()
    } catch (err) {
      setError(describeError(err, 'Failed to create project.'))
    } finally {
      setSubmitting(false)
    }
    return
  }
  // ... nhánh 'new-repo' hiện tại — giữ nguyên toàn bộ code cũ ...
}
```

### Bước 4: Cập nhật điều kiện disable nút Submit

```tsx
<Button
  type="submit"
  disabled={
    submitting || !name.trim() ||
    (mode === 'new-repo' ? (!devServerId || !repoPath.trim()) : !selectedProjectId)
  }
>
  {submitting ? 'Creating…' : 'Create Project'}
</Button>
```

### Bước 5 (tuỳ chọn, hoàn tất liên kết với TASK-FE-PW-001-A)

Trong cảnh báo trùng lặp ở TASK-FE-PW-001-A, bật nút chuyển mode:

```tsx
<button type="button" className="underline" onClick={() => setMode('link')}>
  Link Project có sẵn thay vào đó?
</button>
```

---

## Verify

```bash
grep -n "orcaProjects.linkSourceProject" frontend/src/renderer/src/components/project/CreateProjectDialog.tsx
grep -n "cp-mode-link" frontend/src/renderer/src/components/project/CreateProjectDialog.tsx
```

Test:
- Mode `link`: chọn 1 Project, submit → gọi đúng `project.create` rồi `orcaProjects.linkSourceProject`, **KHÔNG** gọi `repo.add`/`project.rebindDevServer`.
- Mode `new-repo`: hành vi cũ không đổi (regression guard — chạy lại toàn bộ test cũ của file này).
- Nút submit disable đúng theo từng mode.

## Depends on
TASK-FE-PW-002-A (cần type `LinkSourceProjectParam` — tuy TypeScript không bắt buộc phải có type
này mới compile được lệnh `callRuntimeRpc` generic, nhưng nên làm sau để có type-safety đầy đủ,
tránh gõ tay object literal không kiểm tra)

## Blocking
Không có
