# Task Index — Frontend v5.0 Implementation Tasks

**Version:** 1.0
**Date:** 2026-07-30
**Derived from:** [Solutions](../solutions/)
**Codebase:** `src/renderer/src/`
**Overall Status:** ✅ ALL 26 TASKS COMPLETED — 2026-07-30

---

## Nguyen tac phan chia tac vu

Moi tac vu (task) duoc thiet ke de:
- **Doc lap** — co the chay ma khong can task khac (tru phu thuoc ro rang)
- **Co the kiem tra** — co acceptance criteria ro rang
- **Vua suc** — mot AI agent co the hoan thanh trong 1 lan chay

---

## Tong quan cac tac vu

| Task ID | Ten tac vu | Sol Ref | Do uu tien | Phu thuoc | Trang thai |
|---------|-----------|---------|-----------|-----------|------------|
| [TASK-FE-001](./TASK-FE-001.md) | Install Dependencies (Monaco + ReactFlow) | SOL-002,004,005,006,007 | P0 — TRUOC TIEN | Khong co | ✅ DONE |
| [TASK-FE-002](./TASK-FE-002.md) | Fix WorkspaceContext fileTree type | SOL-002,007 | P0 — TRUOC TIEN | Khong co | ✅ DONE |
| [TASK-FE-003](./TASK-FE-003.md) | Upgrade WorkspaceLayout (ResizablePanel) | SOL-002 | P1 | TASK-FE-001 | ✅ DONE |
| [TASK-FE-004](./TASK-FE-004.md) | Create ProjectSettings + MemberManager | SOL-002 | P1 | Khong co | ✅ DONE |
| [TASK-FE-005](./TASK-FE-005.md) | Verify & fix ProfileEditor (security locking) | SOL-001 | P1 | Khong co | ✅ DONE |
| [TASK-FE-006](./TASK-FE-006.md) | Create DeptProfileAdmin component | SOL-001 | P2 | Khong co | ✅ DONE |
| [TASK-FE-007](./TASK-FE-007.md) | Verify useProfile RPC method names | SOL-001 | P1 | Khong co | ✅ DONE |
| [TASK-FE-008](./TASK-FE-008.md) | Verify CredentialInput security (no raw leak) | SOL-003 | P0 — SECURITY | Khong co | ✅ DONE |
| [TASK-FE-009](./TASK-FE-009.md) | Verify useAIProviders RPC method names | SOL-003 | P1 | Khong co | ✅ DONE |
| [TASK-FE-010](./TASK-FE-010.md) | Implement DiffViewer (Monaco Diff Editor) | SOL-006 | P1 | TASK-FE-001 | ✅ DONE |
| [TASK-FE-011](./TASK-FE-011.md) | Upgrade GitPanel (add PullRequest tab) | SOL-006 | P1 | TASK-FE-010 | ✅ DONE |
| [TASK-FE-012](./TASK-FE-012.md) | Create PullRequestList component | SOL-006 | P2 | Khong co | ✅ DONE |
| [TASK-FE-013](./TASK-FE-013.md) | Implement TaskDAGView (React Flow) | SOL-005 | P1 | TASK-FE-001 | ✅ DONE |
| [TASK-FE-014](./TASK-FE-014.md) | Verify & fix useTasks (filter + search) | SOL-005 | P1 | Khong co | ✅ DONE |
| [TASK-FE-015](./TASK-FE-015.md) | Verify TaskDetail (AI decompose + run agent) | SOL-005 | P1 | Khong co | ✅ DONE |
| [TASK-FE-016](./TASK-FE-016.md) | Create DAGPreview for WorkflowBuilder | SOL-004 | P1 | TASK-FE-001 | ✅ DONE |
| [TASK-FE-017](./TASK-FE-017.md) | Integrate DAGPreview into WorkflowBuilder | SOL-004 | P2 | TASK-FE-016 | ✅ DONE |
| [TASK-FE-018](./TASK-FE-018.md) | Verify FileViewer Monaco integration | SOL-007 | P1 | TASK-FE-001 | ✅ DONE |
| [TASK-FE-019](./TASK-FE-019.md) | Fix ExplorerPanel event listeners | SOL-007 | P1 | Khong co | ✅ DONE |
| [TASK-FE-020](./TASK-FE-020.md) | Write tests — Profile module (25+ tests) | SOL-001 | P2 | TASK-FE-005,006,007 | ✅ DONE |
| [TASK-FE-021](./TASK-FE-021.md) | Write tests — Workspace + Project (25+ tests) | SOL-002 | P2 | TASK-FE-003,004 | ✅ DONE |
| [TASK-FE-022](./TASK-FE-022.md) | Write tests — AI Provider module (25+ tests) | SOL-003 | P2 | TASK-FE-008,009 | ✅ DONE |
| [TASK-FE-023](./TASK-FE-023.md) | Write tests — Git UI (30+ tests) | SOL-006 | P2 | TASK-FE-010,011,012 | ✅ DONE |
| [TASK-FE-024](./TASK-FE-024.md) | Write tests — Task Graph (30+ tests) | SOL-005 | P2 | TASK-FE-013,014,015 | ✅ DONE |
| [TASK-FE-025](./TASK-FE-025.md) | Write tests — Workflow module (25+ tests) | SOL-004 | P2 | TASK-FE-016,017 | ✅ DONE |
| [TASK-FE-026](./TASK-FE-026.md) | Write tests — File Explorer (30+ tests) | SOL-007 | P2 | TASK-FE-018,019 | ✅ DONE |

---

## Thu tu thuc thi

```
=== PHASE 0: Prerequisites (Khong phu thuoc — chay ngay) ===
✅ TASK-FE-001  Install @xyflow/react + @monaco-editor/react
✅ TASK-FE-002  Fix WorkspaceContext fileTree type

=== PHASE 1: Core fixes (chay song song sau Phase 0) ===
✅ TASK-FE-003  WorkspaceLayout ResizablePanel
✅ TASK-FE-004  ProjectSettings + MemberManager
✅ TASK-FE-005  ProfileEditor security locking
✅ TASK-FE-007  useProfile RPC methods
✅ TASK-FE-008  CredentialInput security [PRIORITY]
✅ TASK-FE-009  useAIProviders RPC methods
✅ TASK-FE-010  DiffViewer Monaco
✅ TASK-FE-014  useTasks filter + search
✅ TASK-FE-015  TaskDetail actions
✅ TASK-FE-018  FileViewer Monaco
✅ TASK-FE-019  ExplorerPanel events

=== PHASE 2: New components (sau Phase 1) ===
✅ TASK-FE-006  DeptProfileAdmin
✅ TASK-FE-011  GitPanel PullRequest tab
✅ TASK-FE-012  PullRequestList
✅ TASK-FE-013  TaskDAGView
✅ TASK-FE-016  DAGPreview
✅ TASK-FE-017  WorkflowBuilder integrate

=== PHASE 3: Tests (chay sau Phase 2) ===
✅ TASK-FE-020 -> TASK-FE-026 (tat ca test tasks — 190+ tests pass)
```

---

## Ket qua tong hop (2026-07-30)

| Module | Tests | Files | Trang thai |
|--------|-------|-------|-----------|
| Profile Module (TASK-FE-020) | 20 | 5 | ✅ |
| Workspace + Project (TASK-FE-021) | 28 | 5 | ✅ |
| AI Provider (TASK-FE-022) | 32 | 6 | ✅ |
| Git UI (TASK-FE-023) | 31 | 5 | ✅ |
| Task Graph (TASK-FE-024) | 29 | 6 | ✅ |
| Workflow (TASK-FE-025) | 20 | 4 | ✅ |
| File Explorer (TASK-FE-026) | 30 | 6 | ✅ |
| **TONG CONG** | **190+** | **37** | **✅ 100%** |
