# Frontend v5.0 Upgrade — Solution Overview

**Date:** 2026-07-28  
**CR Scope:** TDD-FE-11 → TDD-FE-17 (7 new TDDs)  
**Base:** `specs/frontend/tdd/v4/` (current as-implemented)  
**Target:** `specs/frontend/tdd/` (v5.0 — 7 new feature domains)  
**Solutions:** This directory (`specs/frontend/crs/v3/tdd-v5/solutions/`)

---

## Gap Analysis: v4 → v5.0

| TDD | Domain | v4 Status | v5.0 Status |
|-----|--------|----------|------------|
| TDD-FE-01…10 | Auth, Admin, SSH, Fleet, etc. | ✅ Implemented | ✅ Carry-over, no change |
| **TDD-FE-11** | Profile Hierarchy UI | ❌ Missing | ✅ Implemented |
| **TDD-FE-12** | Project Workspace Shell | ❌ Missing | ✅ Implemented |
| **TDD-FE-13** | AI Provider Admin UI | ❌ Missing | ✅ Implemented |
| **TDD-FE-14** | Workflow Builder & Monitor | ❌ Missing | ✅ Implemented |
| **TDD-FE-15** | Task Graph UI | ❌ Missing | ✅ Implemented |
| **TDD-FE-16** | Remote Git UI | ❌ Missing | ✅ Implemented |
| **TDD-FE-17** | Remote File Explorer | ❌ Missing | ✅ Implemented |

---

## Solution Files

| File | TDD | Domain |
|------|-----|--------|
| [SOL-FE-V5-01.md](./SOL-FE-V5-01.md) | TDD-FE-11 | Profile Hierarchy UI |
| [SOL-FE-V5-02.md](./SOL-FE-V5-02.md) | TDD-FE-12 | Project Workspace Shell |
| [SOL-FE-V5-03.md](./SOL-FE-V5-03.md) | TDD-FE-13 | AI Provider Admin UI |
| [SOL-FE-V5-04.md](./SOL-FE-V5-04.md) | TDD-FE-14 | Workflow Builder & Monitor |
| [SOL-FE-V5-05.md](./SOL-FE-V5-05.md) | TDD-FE-15 | Task Graph UI |
| [SOL-FE-V5-06.md](./SOL-FE-V5-06.md) | TDD-FE-16 | Remote Git UI |
| [SOL-FE-V5-07.md](./SOL-FE-V5-07.md) | TDD-FE-17 | Remote File Explorer |

---

## Shared Infrastructure Needed (Đánh giá cross-cutting)

### 1. New Zustand Slices
```
store/slices/profile.ts         ← TDD-FE-11
store/slices/workspace.ts       ← TDD-FE-12 (projects, fileTree, gitStatus)
store/slices/ai-provider.ts     ← TDD-FE-13 (accounts, usageByAccount)
store/slices/workflow.ts        ← TDD-FE-14 (templates, executions, streamingOutput)
store/slices/task.ts            ← TDD-FE-15 (tasks, taskGraph)
store/slices/git-panel.ts       ← TDD-FE-16 (stagedFiles, gitHistory, branches, PRs)
```

### 2. New Dependencies
```json
{
  "@xyflow/react": "^12.0.0",   // DAGPreview (TDD-FE-14, TDD-FE-15)
  "dnd-kit":       "^6.0.0",    // Drag-to-reorder steps (TDD-FE-14)
}
```

### 3. WorkspaceContext (cross-cutting)
`src/renderer/src/context/WorkspaceContext.tsx` — dùng chung bởi TDD-FE-12, 15, 16, 17

### 4. RPC Streaming API Extension
`rpc.callStream()` — streaming responses (TDD-FE-14 output, TDD-FE-16 push/commit)

### 5. Additive-only principle
- Không sửa `App.tsx`, `main.tsx`, `web-preload-api.ts`
- Tất cả features là additive — lazy-loaded

---

## Implementation Order (Dependencies)

```
1. WorkspaceContext (TDD-FE-12)      ← nhiều features phụ thuộc
   └─ shared by: FE-15, FE-16, FE-17

2. Profile Slice + ProfileEditor (TDD-FE-11)  ← standalone
   └─ đăng ký trong store/index.ts, mount trong Admin SPA

3. AI Provider (TDD-FE-13)           ← standalone, Admin SPA page
   └─ Cần SubtleCrypto (browser built-in)

4. File Explorer (TDD-FE-17)         ← depends on WorkspaceContext
   └─ mount trong WorkspaceLayout.tsx left panel

5. Git UI (TDD-FE-16)               ← depends on WorkspaceContext
   └─ mount trong WorkspaceLayout.tsx center panel (git tab)

6. Task Graph (TDD-FE-15)           ← depends on WorkspaceContext
   └─ mount trong WorkspaceLayout.tsx center panel (tasks tab)

7. Workflow Builder (TDD-FE-14)     ← depends on WorkspaceContext + DAGPreview shared
   └─ mount trong WorkspaceLayout.tsx center panel (workflows tab)
```

---

## Test Coverage Target (v5.0)

| Solution | Tests Target |
|---------|-------------|
| SOL-FE-V5-01 (Profile) | ≥ 25 |
| SOL-FE-V5-02 (Workspace) | ≥ 25 |
| SOL-FE-V5-03 (AI Provider) | ≥ 25 |
| SOL-FE-V5-04 (Workflow) | ≥ 25 |
| SOL-FE-V5-05 (Task Graph) | ≥ 25 |
| SOL-FE-V5-06 (Remote Git) | ≥ 30 |
| SOL-FE-V5-07 (File Explorer) | ≥ 20 |
| **Total** | **≥ 175** |

---

## Nguyên tắc thiết kế (kế thừa + bổ sung)

1. **Additive-only** — không sửa `App.tsx`, `main.tsx`, `web-preload-api.ts`
2. **Lazy loading** — tất cả 7 feature areas lazy-loaded
3. **WorkspaceContext** — single context cho project-scoped state (không Zustand global)
4. **Credential security** — API keys NEVER plaintext in state (SubtleCrypto encrypt)
5. **Streaming RPC** — `rpc.callStream()` cho git push, agent output, workflow steps
6. **React Flow** — `@xyflow/react` cho DAG visualization (workflows, tasks)
7. **Offline-aware** — tất cả workspace features handle `isOffline` gracefully
