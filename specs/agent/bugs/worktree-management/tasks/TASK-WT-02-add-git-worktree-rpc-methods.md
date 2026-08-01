# TASK-WT-02: Add git.worktree.list and git.worktree.add RPC Methods

**Task ID:** TASK-WT-02  
**Priority:** 🔴 HIGH  
**Bugs fixed:** WT-Issue-4 (missing dedicated worktree RPC handlers)  
**Estimated effort:** Medium (add to dispatch + add helpers)  
**Dependencies:** TASK-WT-01 (validateWorktreePath must exist first)  
**Status:** ✅ DONE (2026-08-01)

---

## Context

**Files:**
- `src/relay/agent-rpc-dispatch.ts` — add `case 'git.worktree.list'` and `case 'git.worktree.add'`
- `src/relay/git-handler.ts` — add `parseWorktreePorcelain()` helper

**Why dedicated handlers instead of using `git.exec`?**
- `git.exec` gives no structured response — callers get raw text
- `git.worktree.list` returns typed `WorktreeInfo[]` array
- `git.worktree.add` validates path BEFORE executing (TASK-WT-01)
- TDD-AG-01 §A.10 explicitly specifies `git.worktree.list` and `git.worktree.add` as named methods

---

## Implementation

### Part 1: Add `parseWorktreePorcelain()` to `src/relay/git-handler.ts`

```typescript
export interface WorktreeInfo {
  path:       string
  head:       string      // commit SHA
  branch:     string      // branch name (without refs/heads/)
  bare:       boolean
  detached:   boolean
  prunable:   boolean
  locked:     boolean
  lockedReason?: string
}

/**
 * parseWorktreePorcelain — Parse `git worktree list --porcelain` output.
 *
 * Input example:
 *   worktree /home/ubuntu/project
 *   HEAD abc123
 *   branch refs/heads/main
 *
 *   worktree /home/ubuntu/project-feat
 *   HEAD def456
 *   branch refs/heads/feature/my-feature
 *   locked
 */
export function parseWorktreePorcelain(stdout: string): WorktreeInfo[] {
  const worktrees: WorktreeInfo[] = []
  let current: Partial<WorktreeInfo> | null = null

  const flush = (): void => {
    if (current?.path !== undefined) {
      worktrees.push({
        path:        current.path       ?? '',
        head:        current.head       ?? '',
        branch:      current.branch     ?? '',
        bare:        current.bare       ?? false,
        detached:    current.detached   ?? false,
        prunable:    current.prunable   ?? false,
        locked:      current.locked     ?? false,
        lockedReason: current.lockedReason,
      })
    }
    current = null
  }

  for (const rawLine of stdout.split('\n')) {
    const line = rawLine.trim()
    if (line === '') {
      flush()
      continue
    }
    if (line.startsWith('worktree ')) {
      flush()
      current = { path: line.slice('worktree '.length) }
    } else if (line.startsWith('HEAD ')     && current) { current.head    = line.slice('HEAD '.length) }
    else if (line.startsWith('branch ')   && current) { current.branch  = line.slice('branch '.length).replace('refs/heads/', '') }
    else if (line === 'bare'             && current) { current.bare     = true }
    else if (line === 'detached'         && current) { current.detached = true }
    else if (line.startsWith('prunable') && current) { current.prunable = true }
    else if (line === 'locked'           && current) { current.locked   = true }
    else if (line.startsWith('locked ')  && current) { current.locked = true; current.lockedReason = line.slice('locked '.length) }
  }
  flush()

  return worktrees
}
```

### Part 2: Add dispatch cases to `src/relay/agent-rpc-dispatch.ts`

Add after `case 'git.execStream'`:

```typescript
// ── git.worktree.list ─────────────────────────────────────────────────────────
// WT-Issue-4: Structured worktree listing per TDD-AG-01 §A.10.
case 'git.worktree.list': {
  const cwd = typeof rpc.params?.cwd === 'string' ? rpc.params.cwd : config.workDir

  try {
    const { execFile }              = await import('node:child_process')
    const { promisify }             = await import('node:util')
    const { parseWorktreePorcelain } = await import('./git-handler')
    const execAsync = promisify(execFile)

    const { stdout } = await execAsync('git', ['worktree', 'list', '--porcelain'], {
      cwd,
      timeout: 10_000,
    })

    const worktrees = parseWorktreePorcelain(stdout)
    log.info(`git.worktree.list: cwd=${cwd} count=${worktrees.length}`)
    return { jsonrpc: '2.0', id: rpc.id, result: { worktrees } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.list failed: ${msg}`)
  }
}

// ── git.worktree.add ──────────────────────────────────────────────────────────
// WT-Issue-4: Create a worktree with path validation per TASK-WT-01.
case 'git.worktree.add': {
  const cwd           = typeof rpc.params?.cwd    === 'string' ? rpc.params.cwd    : config.workDir
  const worktreePath  = typeof rpc.params?.path   === 'string' ? rpc.params.path   : ''
  const branch        = typeof rpc.params?.branch === 'string' ? rpc.params.branch : ''
  const createBranch  = rpc.params?.createBranch === true

  if (!worktreePath) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'git.worktree.add: path is required')
  if (!branch)       return makeError(rpc.id, AgentErrorCode.InvalidParams, 'git.worktree.add: branch is required')

  // Security: validate path before executing
  try {
    const { validateWorktreePath } = await import('./git-handler')
    validateWorktreePath(['worktree', 'add', worktreePath], cwd)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.InvalidParams, msg)
  }

  const args = createBranch
    ? ['worktree', 'add', '-b', branch, worktreePath]
    : ['worktree', 'add', worktreePath, branch]

  try {
    const { execFile } = await import('node:child_process')
    const { promisify } = await import('node:util')
    const { resolve, join } = await import('node:path')
    const execAsync = promisify(execFile)

    const { stdout, stderr } = await execAsync('git', args, { cwd, timeout: 30_000 })
    const resolvedPath = worktreePath.startsWith('/')
      ? resolve(worktreePath)
      : resolve(join(cwd, worktreePath))

    log.info(`git.worktree.add: path=${resolvedPath} branch=${branch}`)
    return {
      jsonrpc: '2.0', id: rpc.id,
      result: { worktreePath: resolvedPath, branch, createBranch, stdout, stderr },
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.add failed: ${msg}`)
  }
}

// ── git.worktree.remove ───────────────────────────────────────────────────────
case 'git.worktree.remove': {
  const cwd          = typeof rpc.params?.cwd    === 'string' ? rpc.params.cwd    : config.workDir
  const worktreePath = typeof rpc.params?.path   === 'string' ? rpc.params.path   : ''
  const force        = rpc.params?.force === true

  if (!worktreePath) return makeError(rpc.id, AgentErrorCode.InvalidParams, 'git.worktree.remove: path is required')

  try {
    const { validateWorktreePath } = await import('./git-handler')
    validateWorktreePath(['worktree', 'add', worktreePath], cwd)  // reuse path validation
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.InvalidParams, msg)
  }

  const args = force
    ? ['worktree', 'remove', '--force', worktreePath]
    : ['worktree', 'remove', worktreePath]

  try {
    const { execFile } = await import('node:child_process')
    const { promisify } = await import('node:util')
    const execAsync = promisify(execFile)
    await execAsync('git', args, { cwd, timeout: 10_000 })
    log.info(`git.worktree.remove: path=${worktreePath}`)
    return { jsonrpc: '2.0', id: rpc.id, result: { ok: true } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.remove failed: ${msg}`)
  }
}
```

---

## Wire Protocol

```json
// git.worktree.list
{ "jsonrpc": "2.0", "id": 1, "method": "git.worktree.list", "params": { "cwd": "/home/ubuntu/project" } }
// →
{ "jsonrpc": "2.0", "id": 1, "result": { "worktrees": [
    { "path": "/home/ubuntu/project", "head": "abc123", "branch": "main", "bare": false, "detached": false, "prunable": false, "locked": false }
]}}

// git.worktree.add (existing branch)
{ "jsonrpc": "2.0", "id": 2, "method": "git.worktree.add", "params": {
    "cwd": "/home/ubuntu/project", "path": "../project-feature", "branch": "feature/my-feature"
}}
// →
{ "jsonrpc": "2.0", "id": 2, "result": { "worktreePath": "/home/ubuntu/project-feature", "branch": "feature/my-feature", "createBranch": false }}

// git.worktree.add (new branch)
{ "jsonrpc": "2.0", "id": 3, "method": "git.worktree.add", "params": {
    "cwd": "/home/ubuntu/project", "path": "../project-bugfix", "branch": "bugfix/auth", "createBranch": true
}}
```

---

## Unit Tests to Add

File: `src/relay/__tests__/git-handler.test.ts`

```typescript
describe('parseWorktreePorcelain', () => {
  it('parses single worktree', () => {
    const stdout = 'worktree /home/ubuntu/project\nHEAD abc123\nbranch refs/heads/main\n'
    const result = parseWorktreePorcelain(stdout)
    expect(result).toHaveLength(1)
    expect(result[0].path).toBe('/home/ubuntu/project')
    expect(result[0].head).toBe('abc123')
    expect(result[0].branch).toBe('main')
    expect(result[0].bare).toBe(false)
  })

  it('parses multiple worktrees', () => {
    const stdout = [
      'worktree /main', 'HEAD aaa', 'branch refs/heads/main', '',
      'worktree /feat', 'HEAD bbb', 'branch refs/heads/feature/x', '',
    ].join('\n')
    const result = parseWorktreePorcelain(stdout)
    expect(result).toHaveLength(2)
    expect(result[1].branch).toBe('feature/x')
  })

  it('handles locked worktree', () => {
    const stdout = 'worktree /locked\nHEAD ccc\nbranch refs/heads/fix\nlocked reason: editing\n'
    const result = parseWorktreePorcelain(stdout)
    expect(result[0].locked).toBe(true)
    expect(result[0].lockedReason).toBe('reason: editing')
  })

  it('handles empty stdout', () => {
    expect(parseWorktreePorcelain('')).toHaveLength(0)
  })
})
```

---

## Verification

```bash
npx tsc --noEmit -p config/tsconfig.node.json 2>&1 | grep -E "git-handler|agent-rpc-dispatch"
npx vitest run src/relay/__tests__/git-handler.test.ts
```

---

## ✅ Completion Notes

**Completed:** 2026-08-01  
**Implementation:** agent-git-handler.ts: git.worktree.list (L308), git.worktree.add (L336), git.worktree.remove (L372). Dispatch routing từ agent-rpc-dispatch.ts.  
**Tests:** Verified: 'git.worktree.list: cwd=' log pattern. Full worktree lifecycle handlers.  
