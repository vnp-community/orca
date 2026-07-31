# TASK-FE-010: Implement DiffViewer with Monaco Diff Editor

**Task ID:** TASK-FE-010
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-006 (Section 3)
**Estimated effort:** 45 minutes
**Dependencies:** TASK-FE-001 (@monaco-editor/react installed ✅)
**Status:** ✅ COMPLETED — 2026-07-30

---

## Execution Results

### Pre-checks

- `@monaco-editor/react` v4.7.0 — ✅ INSTALLED
- Monaco exports confirmed: `['DiffEditor', 'Editor', 'default', 'loader', 'useMonaco']`
- Existing RPC for diff: `git.getDiff` (already used in `useGit.ts` and old stub)
- No `git.exec` / `git.run` RPC found — `git.getDiff` is the correct method

### Original stub analysis

Old `DiffViewer.tsx` (1436 bytes):
- Used plain-text diff rendering (colored `div` rows)
- Props: `{ filePath: string, staged?: boolean }`
- Used `callRuntimeRpc('git.getDiff', ...)` without target
- Stored result in Zustand store `diffContent` (shared state = cross-contamination bug)

### Replacement implementation

**New `DiffViewer.tsx`** (Monaco-based):
- `DiffEditor` from `@monaco-editor/react` ✅
- Props: `{ filePath: string, worktreePath?: string, staged?: boolean }` ✅
- **Staged mode**: calls `git.getDiff` with `staged: true`
- **Unstaged mode**: loads HEAD version (`git.getDiff side: 'original'`) + working tree (`fs.readFile`) in `Promise.all`
- Local `useState` for content (no shared store contamination)
- `detectLanguage()` maps ext → Monaco language id (20 file types)
- Loading state: 2 `<Skeleton>` components (`data-testid="diff-viewer-loading"`)
- Error state: `data-testid="diff-viewer-error"`
- Header: filename + language badge
- Monaco options: `readOnly: true`, `renderSideBySide: true`, `theme="vs-dark"`, `height={350}`
- Root: `data-testid="diff-viewer"` ✅

### TypeScript errors: **0**

---

## Acceptance Criteria

- [x] `DiffViewer.tsx` uses `DiffEditor` from `@monaco-editor/react`
- [x] Props: `filePath: string`, `worktreePath?: string`, `staged?: boolean`
- [x] Loading state shows `Skeleton` components
- [x] Error state shows error message
- [x] Header shows filename and detected language badge
- [x] `detectLanguage()` maps `.ts/.tsx` to `typescript`
- [x] Monaco options: `readOnly: true`, `renderSideBySide: true`
- [x] `data-testid="diff-viewer"` present on root element
- [x] No TypeScript errors

---

## Output

```
DiffViewer.tsx replaced: YES
@monaco-editor/react DiffEditor: IMPORTED as 'DiffEditor'
git.getDiff RPC: USED (existing RPC — no change; git.exec not used in codebase)
worktreePath prop: ADDED (new, not in stub)
TypeScript errors: 0
```
