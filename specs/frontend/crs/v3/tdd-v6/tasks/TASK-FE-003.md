# TASK-FE-003: Upgrade WorkspaceLayout with ResizablePanel

**Task ID:** TASK-FE-003
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-002 (Section 2)
**Estimated effort:** 30 minutes
**Dependencies:** TASK-FE-001 (resizable.tsx must exist)
**Status:** ✅ COMPLETED — 2026-07-30

---

## Objective

Replace the fixed-width CSS flex layout in `WorkspaceLayout.tsx` with `ResizablePanelGroup` from shadcn/ui. Add the missing right panel content, status bar, and terminal toggle.

---

## Execution Results

### Analysis of original file
- Line 38: `<div className="workspace-left w-56 ...">` — hardcoded `w-56`
- Line 47: `<GitPanel projectId={project.id} />` — wrong: GitPanel already uses `useWorkspace()` internally
- No status bar
- No terminal toggle
- Right panel was empty placeholder

### Changes made

**`WorkspaceLayout.tsx`** — Full replacement:
1. ✅ Replaced `<div className="workspace-left w-56">` with `<ResizablePanel defaultSize={20} minSize={15} maxSize={35}>`
2. ✅ Wrapped all 3 panels in `<ResizablePanelGroup orientation="horizontal">`
3. ✅ Added `<ResizableHandle />` separators between panels
4. ✅ Removed `projectId={project.id}` from `<GitPanel>` call
5. ✅ Added status bar at bottom with `data-testid="status-bar"`
6. ✅ Added terminal toggle button with `data-testid="toggle-terminal"`
7. ✅ Added right panel toggle with `data-testid="toggle-right-panel"`
8. ✅ Added `terminalVisible` state with conditional terminal panel render

### Bug found & fixed
- `ResizablePanelGroup` prop is `orientation="horizontal"` NOT `direction="horizontal"`
  (react-resizable-panels v2+ uses `orientation`, not `direction`)
- Import path: `@/components/ui/resizable` (using @ alias)

---

## Acceptance Criteria

- [x] `WorkspaceLayout.tsx` uses `ResizablePanelGroup` with 3 panels (left, center, right)
- [x] Left panel shows `ExplorerPanel` with `data-testid="panel-explorer"`
- [x] Center panel shows correct tab content (git/tasks/workflows)
- [x] Right panel is collapsible via "Hide Panel" / "Show Panel" button
- [x] Terminal panel appears/disappears via toggle button (`data-testid="toggle-terminal"`)
- [x] Status bar visible at bottom with `data-testid="status-bar"`
- [x] No TypeScript errors (task scope)

---

## Output

```
WorkspaceLayout updated: YES
ResizablePanelGroup prop: orientation (not direction — fixed bug)
ResizablePanel imported from: @/components/ui/resizable
GitPanel projectId prop removed: YES
TypeScript errors (task scope): 0
```
