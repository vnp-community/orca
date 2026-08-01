# TASK-TM-001: Fix workspace init relative path

**Priority:** 🟡 MEDIUM — Terminal starts in wrong directory  
**Effort:** ~10 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-TM-002  
**Solution ref:** [SOLUTION-terminal-management.md](../solutions/SOLUTION-terminal-management.md)

## Bước 1 — Tìm vị trí relative path bug

```bash
grep -rn "cwd\|workdir\|repoPath\|relative" src/main/runtime/rpc/methods/terminal.ts | grep -i "worktree\|init" | head -10
```

## Thay đổi

Tìm đoạn code tạo terminal với `cwd`:

```typescript
// TRƯỚC (relative path):
const cwd = worktree.path  // có thể là relative: '../worktrees/feature'

// SAU (absolute path):
import { resolve } from 'node:path'
const cwd = resolve(project.repoPath, worktree.path)
// Hoặc nếu worktree.path phải absolute:
if (!worktree.path.startsWith('/')) {
  throw new Error(`Worktree path must be absolute: ${worktree.path}`)
}
```

## Verification

```bash
pnpm tsc --noEmit
# Test: create terminal với worktree path → terminal cwd là absolute path
```
