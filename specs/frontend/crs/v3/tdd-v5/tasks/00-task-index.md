# Frontend v5.0 — Task Index

**Date:** 2026-07-28  
**Source:** [`../solutions/`](../solutions/)  
**Workspace:** `src/renderer/src/`

> Mỗi task file = 1 đơn vị AI thực thi độc lập.  
> Thứ tự thực thi phải theo cột **Order** (dependency chain).

---

## Task List

| Order | Task File | Domain | Est. Files | Tests |
|-------|-----------|--------|-----------|-------|
| 1 | [TASK-V5-01.md](./TASK-V5-01.md) | Shared Types & Store Registration | 2 | 0 |
| 2 | [TASK-V5-02.md](./TASK-V5-02.md) | WorkspaceContext + WorkspaceSlice | 5 | 12 |
| 3 | [TASK-V5-03.md](./TASK-V5-03.md) | WorkspaceLayout + ProjectSwitcher UI | 6 | 13 |
| 4 | [TASK-V5-04.md](./TASK-V5-04.md) | Profile Slice + useProfile Hook | 3 | 9 |
| 5 | [TASK-V5-05.md](./TASK-V5-05.md) | ProfileEditor + ProfileSourceBadge | 5 | 16 |
| 6 | [TASK-V5-06.md](./TASK-V5-06.md) | AI Provider Slice + useAIProviders | 3 | 5 |
| 7 | [TASK-V5-07.md](./TASK-V5-07.md) | CredentialInput (Security Critical) | 2 | 7 |
| 8 | [TASK-V5-08.md](./TASK-V5-08.md) | ProviderList + ProviderForm UI | 4 | 15 |
| 9 | [TASK-V5-09.md](./TASK-V5-09.md) | File Explorer — ExplorerPanel + Hook | 5 | 11 |
| 10 | [TASK-V5-10.md](./TASK-V5-10.md) | File Explorer — FileViewer + Search | 3 | 9 |
| 11 | [TASK-V5-11.md](./TASK-V5-11.md) | Git Slice + useGit Hook | 3 | 10 |
| 12 | [TASK-V5-12.md](./TASK-V5-12.md) | GitPanel + StagingArea + CommitForm | 5 | 16 |
| 13 | [TASK-V5-13.md](./TASK-V5-13.md) | DiffViewer + BranchManager | 3 | 9 |
| 14 | [TASK-V5-14.md](./TASK-V5-14.md) | Task Slice + useTasks Hook | 3 | 9 |
| 15 | [TASK-V5-15.md](./TASK-V5-15.md) | TaskGraph + TaskTreeView + TaskCard | 5 | 14 |
| 16 | [TASK-V5-16.md](./TASK-V5-16.md) | TaskDetail + TaskAIDecompose | 3 | 11 |
| 17 | [TASK-V5-17.md](./TASK-V5-17.md) | Install npm deps + Workflow Slice | 2 | 0 |
| 18 | [TASK-V5-18.md](./TASK-V5-18.md) | DAGPreview (React Flow) | 2 | 5 |
| 19 | [TASK-V5-19.md](./TASK-V5-19.md) | WorkflowBuilder + StepList (dnd-kit) | 5 | 9 |
| 20 | [TASK-V5-20.md](./TASK-V5-20.md) | ExecutionMonitor + Streaming | 4 | 10 |
| 21 | [TASK-V5-21.md](./TASK-V5-21.md) | Admin SPA Integration (mount all) | 2 | 0 |
| 22 | [TASK-V5-22.md](./TASK-V5-22.md) | RPC Streaming Extension | 1 | 0 |

**Tổng:** 22 tasks | ~76 files mới | ≥ 175 tests

---

## Dependency Graph

```
TASK-01 (types)
  └─ TASK-02 (WorkspaceContext)
       ├─ TASK-03 (WorkspaceLayout UI)
       │    ├─ TASK-09 (File Explorer)
       │    │    └─ TASK-10 (FileViewer)
       │    ├─ TASK-11 (Git Slice)
       │    │    ├─ TASK-12 (GitPanel)
       │    │    └─ TASK-13 (DiffViewer)
       │    ├─ TASK-14 (Task Slice)
       │    │    ├─ TASK-15 (TaskGraph)
       │    │    └─ TASK-16 (TaskDetail)
       │    └─ TASK-17 (npm deps)
       │         ├─ TASK-18 (DAGPreview)
       │         ├─ TASK-19 (WorkflowBuilder)
       │         └─ TASK-20 (ExecutionMonitor)
       └─ TASK-22 (RPC Streaming) ← dùng bởi TASK-12, TASK-20
  └─ TASK-04 (Profile Slice)
       └─ TASK-05 (ProfileEditor)
  └─ TASK-06 (AI Provider Slice)
       ├─ TASK-07 (CredentialInput)
       └─ TASK-08 (ProviderList)
  └─ TASK-21 (Admin SPA) ← cuối cùng, mount tất cả
```

---

## Quy ước Task File

Mỗi task file tuân theo format:

```
# TASK-V5-XX: [Tên task]

## Mô tả
## Điều kiện tiên quyết (Prerequisites)
## Files cần tạo
## Files cần sửa (additive)
## Hướng dẫn implement
## Acceptance Criteria
## Test cases cần viết
```

---

## Nguyên tắc bất biến (AI phải tuân thủ)

1. **KHÔNG sửa:** `App.tsx`, `main.tsx`, `web-preload-api.ts`, `src/preload/index.ts`
2. **Additive-only:** mỗi thay đổi trên file existing phải là additive (thêm, không xóa)
3. **Lazy load:** tất cả component mới phải `React.lazy()` tại điểm mount
4. **Credential security:** API key KHÔNG bao giờ log hoặc store plaintext sau encrypt
5. **Test isolation:** mỗi test file `afterEach(() => cleanup())` nếu dùng React
6. **immer safe:** trong Zustand slice, không mutate array/object ngoài `set(s => { ... })`
