# TASK-044: Remote Git Handler + RPC

**Phase:** 7 — Workspace + Remote Git  
**Solution ref:** [SOL-V5-007](../solutions/SOL-V5-007-remote-git-ui.md) §2, §3  
**Prerequisite:** TASK-043, TASK-040 (TaskService RPC)  
**Status:** ✅ DONE

---

## Files cần tạo

### `src/relay/git-handler.ts` (relay-side — runs on Dev Server)

```typescript
const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',
])

const SHELL_METACHARACTERS = /[&|;$`]/

function validateGitArgs(args: string[]): void {
  if (args.length === 0) throw new Error('GIT_NO_SUBCOMMAND')
  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) throw new Error(`GIT_DISALLOWED_SUBCOMMAND: ${args[0]}`)
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) throw new Error(`GIT_SHELL_METACHARACTER_IN_ARG: ${arg}`)
  }
}

export const gitHandlers = {
  'git.exec': async (params: { cwd: string; args: string[]; timeout?: number }) => { ... },
  'git.execStream': (params: { cwd: string; args: string[] }) => { ... },
}
```

**Security rules:**
- Only whitelisted subcommands allowed
- Shell metacharacters (`&|;$\``) forbidden in args
- `execFile()` — NOT `exec()` (prevents shell injection)
- `maxBuffer: 10MB`, default timeout 30s

**Register in relay entry:** Add `gitHandlers` to relay handler registry.

### `src/main/runtime/rpc/methods/git-remote.ts` (server-side routing)

Implement `registerRemoteGitRpcMethods(router, aiProviderService, taskService, taskGrantService, dispatcher)`:

**Methods:**
- `git.status` → relay `git.exec { args: ['status', '--porcelain=v2', '--branch'] }`
- `git.diff` → relay `git.exec { args: ['diff', staged?'--staged':'', ...files] }`
- `git.add` → relay `git.exec { args: ['add', ...params.files] }`
- `git.restore` → relay `git.exec { args: ['restore', '--staged', ...params.files] }`
- `git.commit` → relay `git.exec { args: ['commit', '-m', params.message] }` + task auto-advance
- `git.push` → relay `git.execStream`
- `git.pull` → relay `git.execStream`
- `git.fetch` → relay `git.exec`
- `git.branch.list` → relay `git.exec { args: ['branch', '-a', '--format=...'] }`
- `git.branch.create` → relay `git.exec { args: ['branch', params.name] }`
- `git.branch.delete` → relay `git.exec { args: ['branch', '-d', params.name] }`
- `git.checkout` → relay `git.exec`
- `git.log` → relay `git.exec { args: ['log', '--oneline', '-20'] }`
- `git.worktree.list` → relay `git.exec { args: ['worktree', 'list', '--porcelain'] }`
- `git.worktree.add` → relay `git.exec { args: ['worktree', 'add', params.path, params.branch] }`
- `git.worktree.remove` → relay `git.exec { args: ['worktree', 'remove', params.path] }`
- `git.generateCommitMessage` → diff → AI → commit message
- `git.pr.create` → gh CLI or API

**Task auto-advance (in git.commit):**
Extract `#TG-xxx` refs from commit message → update task status to 'review' (non-fatal).

**Wire in bootstrap** (sau `rpcServer.start()`):
```typescript
const { registerRemoteGitRpcMethods } = await import('./runtime/rpc/methods/git-remote')
registerRemoteGitRpcMethods(projectRouter, aiProviderService, taskService, taskGrantService, rpcServer.dispatcher)
```

---

## Tests cần tạo

### `src/relay/__tests__/git-handler.test.ts` (≥ 12 tests)

1. `validateGitArgs`: `['status']` → OK
2. `validateGitArgs`: `['rm', '-rf', '/']` → GIT_DISALLOWED_SUBCOMMAND
3. `validateGitArgs`: `['commit', '-m', 'msg; evil']` → GIT_SHELL_METACHARACTER
4. `validateGitArgs`: empty array → GIT_NO_SUBCOMMAND
5. `git.exec`: success → `{ stdout, stderr, exitCode: 0 }`
6. `git.exec`: failure → `{ exitCode: 1 }` (no throw)
7. `git.exec`: timeout reached → returns error exitCode

### `src/main/runtime/rpc/methods/__tests__/git-remote.test.ts` (≥ 17 tests)

Use mocks for relay, projectRouter, taskService.

1. `git.status` → relay.call called with correct args
2. `git.diff` → staged=true adds `--staged`
3. `git.add` → files list passed to relay
4. `git.commit` → message passed correctly
5. `git.commit` → task auto-advance with `#TG-xxx`
6. `git.commit` → no task ref → no task update
7. `git.push` → relay.callStream used
8. `git.generateCommitMessage` → empty diff → GIT_NO_STAGED_CHANGES
9. `git.generateCommitMessage` → diff → AI called → returns message
10. `git.pr.create` → gh CLI path
11. Task auto-advance: has edit perm → status='review'
12. Task auto-advance: no perm → no change (non-fatal)

## Acceptance Criteria

- [x] `git-handler.ts` relay-side: security whitelist enforced
- [x] `git-remote.ts` server-side: all methods route to relay
- [x] `git.commit` task auto-advance
- [x] `git.generateCommitMessage` AI integration
- [x] ≥ 12 relay tests pass
- [x] ≥ 17 RPC tests pass
