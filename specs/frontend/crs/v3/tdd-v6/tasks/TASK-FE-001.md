# TASK-FE-001: Install Required Dependencies

**Task ID:** TASK-FE-001
**Phase:** 0 — Prerequisites
**Priority:** P0 (MUST run first)
**Solution Ref:** SOL-FE-V6-004, SOL-FE-V6-005, SOL-FE-V6-006, SOL-FE-V6-007
**Estimated effort:** 15 minutes
**Dependencies:** None
**Status:** ✅ COMPLETED — 2026-07-30

---

## Objective

Install 2 npm packages required for DAG visualization and code editor features. These packages are shared across multiple modules.

---

## Context

- `@xyflow/react` — React Flow v12, used for Task DAG view (TDD-FE-15) and Workflow DAG preview (TDD-FE-14)
- `@monaco-editor/react` — Monaco Editor for React, used for DiffViewer (TDD-FE-16) and FileViewer (TDD-FE-17)

---

## Execution Results

### Package manager
- **pnpm** detected via `pnpm-lock.yaml`

### Installed packages
```
@xyflow/react: 12.11.2  (newly installed)
@monaco-editor/react: 4.7.0  (already installed)
monaco-editor: 0.55.1  (already installed)
```

### shadcn/ui resizable
```
resizable.tsx: CREATED — src/renderer/src/components/ui/resizable.tsx
```
- Installed via `npx shadcn@latest add resizable --yes`
- Uses `react-resizable-panels` as the underlying primitive

---

## Acceptance Criteria

- [x] `@xyflow/react` is importable in a TypeScript file without error
- [x] `@monaco-editor/react` is importable in a TypeScript file without error
- [x] `src/renderer/src/components/ui/resizable.tsx` exists
- [x] `package.json` contains all 3 dependencies

---

## Output

```
@xyflow/react: 12.11.2
@monaco-editor/react: 4.7.0
resizable.tsx: CREATED
```
