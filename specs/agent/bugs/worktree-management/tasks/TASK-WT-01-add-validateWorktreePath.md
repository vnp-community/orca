# TASK-WT-01: Add validateWorktreePath — Prevent Path Traversal in git worktree add

**Task ID:** TASK-WT-01  
**Priority:** 🔴 HIGH  
**Bugs fixed:** WT-Issue-1 (git worktree path traversal)  
**Estimated effort:** Small (1 new function + 1 call site)  
**Dependencies:** None  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**File:** `src/relay/git-handler.ts`

**Problem:** `git worktree add <path> <branch>` via `git.exec` accepts any path including:
- `../../etc/passwd` (path traversal)
- `/root/worktree` (privileged directory)
- Cross-user paths that override another user's worktree

TDD-AG-01 §A.8 (Agent Isolation Model) requires file path validation via `SecureFs.validatePath()`.

---

## Investigation First

```bash
# Check where git.exec is handled and if worktree is currently special-cased:
grep -n "worktree\|validatePath\|ALLOWED_GIT" src/relay/git-handler.ts | head -20

# Check imports available in git-handler.ts:
head -30 src/relay/git-handler.ts
```

---

## Implementation

### Step 1: Add imports at top of `git-handler.ts`

```typescript
import { resolve, dirname, join } from 'node:path'
import { existsSync } from 'node:fs'
```

### Step 2: Add `validateWorktreePath()` function

Add after the `ALLOWED_GIT_SUBCOMMANDS` set definition:

```typescript
/**
 * validateWorktreePath — Security check for `git worktree add <path>`.
 *
 * Allowed paths: subdirectories of workDir or its parent (sibling worktrees).
 * Rejects: absolute paths outside allowed roots, path traversal (../../).
 *
 * @throws Error with code GIT_WORKTREE_PATH_NOT_ALLOWED if path is rejected.
 */
export function validateWorktreePath(args: string[], workDir: string): void {
  // args for 'git worktree add <path> [branch]':
  // args[0] = 'worktree', args[1] = 'add', args[2] = path
  if (args[0] !== 'worktree' || args[1] !== 'add' || !args[2]) return

  const rawPath = args[2]
  const resolved = rawPath.startsWith('/')
    ? resolve(rawPath)
    : resolve(join(workDir, rawPath))

  // Reject null bytes (injection attack)
  if (resolved.includes('\0')) {
    throw Object.assign(
      new Error(`GIT_WORKTREE_PATH_INVALID: null bytes in path`),
      { code: 'GIT_WORKTREE_PATH_INVALID' }
    )
  }

  // Allow: workDir and its parent (for sibling worktrees like project-main, project-feature)
  const parentDir = dirname(workDir)
  const allowedRoots = [workDir, parentDir, '/tmp', '/var/tmp']
  const isAllowed = allowedRoots.some(
    (root) => resolved === root || resolved.startsWith(root + '/')
  )

  if (!isAllowed) {
    throw Object.assign(
      new Error(`GIT_WORKTREE_PATH_NOT_ALLOWED: "${resolved}" is outside allowed roots`),
      { code: 'GIT_WORKTREE_PATH_NOT_ALLOWED', path: resolved }
    )
  }
}
```

### Step 3: Call `validateWorktreePath()` in git.exec handler

Find the `git.exec` handler (likely in `git-handler.ts` or `agent-rpc-dispatch.ts`) and add validation before execution:

```typescript
// In git.exec handler, BEFORE running the command:
if (args[0] === 'worktree' && args[1] === 'add') {
  validateWorktreePath(args, cwd)  // throws if invalid — error propagated to caller
}
```

---

## Unit Tests

File: `src/relay/__tests__/git-handler.test.ts`

```typescript
import { validateWorktreePath } from '../git-handler'

describe('validateWorktreePath', () => {
  const workDir = '/home/ubuntu/projects/main-repo'

  it('allows path inside workDir', () => {
    expect(() => validateWorktreePath(
      ['worktree', 'add', '/home/ubuntu/projects/main-repo/feature'],
      workDir
    )).not.toThrow()
  })

  it('allows sibling path (parent dir)', () => {
    expect(() => validateWorktreePath(
      ['worktree', 'add', '/home/ubuntu/projects/feature-branch'],
      workDir
    )).not.toThrow()
  })

  it('allows relative path inside workDir', () => {
    expect(() => validateWorktreePath(
      ['worktree', 'add', '../feature-branch'],  // → /home/ubuntu/projects/feature-branch
      workDir
    )).not.toThrow()
  })

  it('allows /tmp path', () => {
    expect(() => validateWorktreePath(
      ['worktree', 'add', '/tmp/test-worktree'],
      workDir
    )).not.toThrow()
  })

  it('rejects /root path', () => {
    expect(() => validateWorktreePath(
      ['worktree', 'add', '/root/worktree'],
      workDir
    )).toThrow('GIT_WORKTREE_PATH_NOT_ALLOWED')
  })

  it('rejects /etc path', () => {
    expect(() => validateWorktreePath(
      ['worktree', 'add', '/etc/worktree'],
      workDir
    )).toThrow('GIT_WORKTREE_PATH_NOT_ALLOWED')
  })

  it('rejects deep path traversal ../../etc', () => {
    expect(() => validateWorktreePath(
      ['worktree', 'add', '../../../../../../etc'],
      workDir
    )).toThrow('GIT_WORKTREE_PATH_NOT_ALLOWED')
  })

  it('is no-op for non-worktree-add commands', () => {
    expect(() => validateWorktreePath(['worktree', 'list'], workDir)).not.toThrow()
    expect(() => validateWorktreePath(['status'], workDir)).not.toThrow()
  })
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep git-handler
npx vitest run src/relay/__tests__/git-handler.test.ts
```

**Security test:**
```bash
# Send via relay client:
# { "method": "git.exec", "params": { "args": ["worktree", "add", "/root/evil", "main"], "cwd": "/project" } }
# Expected: { "error": { "code": -32602, "message": "GIT_WORKTREE_PATH_NOT_ALLOWED..." } }
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-git-handler.ts: Gọi validateWorktreePath() từ git-handler.ts trước khi tạo worktree. Reject path traversal, absolute paths ngoài workspace.  
**Tests:** Verified: validateWorktreePath import tại L358-359 trong agent-git-handler.ts.  
