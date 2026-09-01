# TASK-FE-PW-001-A: Cảnh báo trùng repo trong `CreateProjectDialog`

**Domain:** project-workspace
**Solution Ref:** SOL-FE-PW-001 Bước 1
**Priority:** 🟡 P1
**Estimated:** 45 phút
**Status:** ✅ DONE (2026-09-01)

**Kết quả thực tế:** Implemented cùng lúc với TASK-FE-PW-001-B trong 1 lần sửa
`CreateProjectDialog.tsx` (2 task chia sẻ cùng file, tách task chỉ để dễ review). Cảnh báo +
nút "Link an existing Project instead?" (điều kiện hiện khi `myProjects.length > 0`) đã có trong
JSX thật, đúng `data-testid="cp-duplicate-repo-warning"`. `existingRepos`/`myProjects` đọc qua
`useAppStore.getState()` (không phải hook `useAppStore(selector)`) để khớp đúng convention hiện
có trong `components/project/*` và không phá mock `useAppStore` trong test hiện có (mock cũ chỉ
định nghĩa `{ getState }`, không callable). Test: 3 test mới trong
`CreateProjectDialog.test.tsx` (`describe('duplicate repo warning')`) — pass. `tsc --noEmit`: 0
lỗi mới. `gitnexus impact upstream` trên `CreateProjectDialog`: LOW risk (1 caller:
`ProjectSwitcher`).

---

## Mục tiêu

Khi user nhập `repoPath` + chọn `devServerId` trùng với 1 Repo đã có trong sidebar chính, hiện
cảnh báo inline — không chặn submit, chỉ minh bạch hoá hệ quả (2 hệ dữ liệu sẽ độc lập).

---

## Files cần sửa

1. `frontend/src/renderer/src/components/project/CreateProjectDialog.tsx`
2. `frontend/src/renderer/src/components/project/__tests__/CreateProjectDialog.test.tsx`

---

## Các bước thực thi

### Bước 1: Đọc `state.repos` từ store

```typescript
import { useAppStore } from '../../store'
// Trong component:
const existingRepos = useAppStore(s => s.repos)
```

### Bước 2: Hàm so khớp trùng lặp

```typescript
function findDuplicateRepo(repos: typeof existingRepos, path: string, devServerId: string) {
  const normalizedPath = path.trim().replace(/\/+$/, '')
  if (!normalizedPath || !devServerId) {return undefined}
  return repos.find(r =>
    r.path.replace(/\/+$/, '') === normalizedPath &&
    r.executionHostId === `devServer:${devServerId}`
  )
}
```

### Bước 3: Render cảnh báo

Đặt ngay dưới field "Repo Path" hiện có trong JSX:

```tsx
{(() => {
  const dup = findDuplicateRepo(existingRepos, repoPath, devServerId)
  return dup ? (
    <p className="text-xs text-amber-600" data-testid="cp-duplicate-repo-warning">
      Repo này đã có trong sidebar của bạn ({dup.displayName}). Tạo Project mới ở đây sẽ KHÔNG
      liên kết với dữ liệu đó — chúng sẽ độc lập.
    </p>
  ) : null
})()}
```

(Nút "Link Project có sẵn thay vào đó?" trong cảnh báo là phần của TASK-FE-PW-001-B — không thêm
ở task này nếu B chưa xong, để tránh dẫn tới 1 tab chưa tồn tại.)

---

## Verify

```bash
grep -n "cp-duplicate-repo-warning" frontend/src/renderer/src/components/project/CreateProjectDialog.tsx
```

Test:
- Nhập path trùng 1 repo trong `state.repos` cùng `devServerId` → cảnh báo hiện.
- Nhập path không trùng, hoặc trùng path nhưng khác `devServerId` → cảnh báo KHÔNG hiện.
- Submit vẫn thành công bình thường dù có cảnh báo (không chặn).

## Depends on
Không có

## Blocking
TASK-FE-PW-001-B (nút "Link" trong cảnh báo cần tab `link` đã tồn tại)
