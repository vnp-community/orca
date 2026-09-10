# FE-TASK-003: Gate tab Git đằng sau `currentWorktree` (tái dùng `NoWorktreeSelected`)

**Domain:** project-workspace
**Solution Ref:** FE-SOL-001 Bước 3
**Priority:** 🔴 P0
**Estimated:** 10 phút
**Status:** ✅ DONE (2026-09-06)

**Kết quả thực tế:** Đúng như kế hoạch. Phát sinh: `WorkspaceLayout.test.tsx`'s test cũ `'"git"
tab active → GitPanel renders'` giả định render vô điều kiện (mock `currentWorktree: null` mặc
định) — tách thành 2 test case (`currentWorktree` có/không) để phản ánh đúng hành vi mới, giống
pattern đã có sẵn cho tab `agent`.

---

## Mục tiêu

`WorkspaceLayout.tsx` render `<GitPanel />` không điều kiện cho tab `'git'`, trong khi tab
`'agent'` liền kề đã có sẵn pattern đúng. Áp dụng lại pattern đó cho tab Git.

## Files cần sửa

1. `frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx`

## Các bước thực thi

```tsx
{activeTab === 'git' && (
  currentWorktree ? <GitPanel /> : <NoWorktreeSelected />
)}
```

`NoWorktreeSelected` đã export trong cùng file, không cần import mới.

## Verify

```bash
grep -n "no-worktree-selected" frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx
cd frontend && npx tsc --noEmit -p .
```

## Depends on
Không có (độc lập với FE-TASK-001/002, nhưng nên làm cùng lượt vì cùng file/solution)

## Blocking
Không có
