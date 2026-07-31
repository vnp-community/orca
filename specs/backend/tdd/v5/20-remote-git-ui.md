# TDD-20: Remote Git UI

**Document:** TDD-20 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Remote Git — relay-based git operations, streaming, AI commit msg, PR creation
**Feature:** F39
**ADR:** ADR-012
**HLD Ref:** C3.12, C4.10
**Source files (to create/extend):**
- `src/relay/git-handler.ts` (NEW)
- `src/main/runtime/rpc/methods/git.ts` (EXTEND for remote)
- `src/renderer/src/components/workspace/git/GitPanel.tsx`
- `src/renderer/src/components/workspace/git/CommitForm.tsx`
- `src/renderer/src/components/workspace/git/DiffViewer.tsx`
- `src/renderer/src/components/workspace/git/BranchManager.tsx`
- `src/renderer/src/components/workspace/git/PullRequestForm.tsx`

> **Status: ❌ TODO** — v5.0 proposed; extends existing local git.ts for remote via relay

---

## 1. Mục tiêu

Cho phép developer thực hiện full git workflow trực tiếp trên dev server qua relay — stage, diff, commit (with AI message), push/pull (streaming), branch management, và tạo PR — mà không cần SSH vào server thủ công.

---

## 2. git-handler — Relay Binary (Dev Server)

```typescript
// src/relay/git-handler.ts (runs on Dev Server as part of relay binary)

import { execFile, spawn } from 'node:child_process'
import { promisify } from 'node:util'
const execFileAsync = promisify(execFile)

/** Whitelist of allowed git subcommands (prevents command injection) */
const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',
])

const SHELL_METACHARACTERS = /[&|;$`]/

function validateGitArgs(args: string[]): void {
  if (args.length === 0) throw new Error('GIT_NO_SUBCOMMAND')
  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new Error(`GIT_DISALLOWED_SUBCOMMAND: ${args[0]}`)
  }
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new Error(`GIT_SHELL_METACHARACTER_IN_ARG: ${arg}`)
    }
  }
}

export const gitHandlers = {
  'git.exec': async (params: {
    cwd: string
    args: string[]
    timeout?: number
  }): Promise<{ stdout: string; stderr: string; exitCode: number }> => {
    validateGitArgs(params.args)
    try {
      const result = await execFileAsync('git', params.args, {
        cwd: params.cwd,
        timeout: params.timeout ?? 30_000,
        maxBuffer: 10 * 1024 * 1024,  // 10MB
      })
      return { stdout: result.stdout, stderr: result.stderr, exitCode: 0 }
    } catch (err: any) {
      return {
        stdout: err.stdout ?? '',
        stderr: err.stderr ?? err.message,
        exitCode: err.code ?? 1,
      }
    }
  },

  'git.execStream': (params: {
    cwd: string
    args: string[]
  }): AsyncGenerator<string> => {
    validateGitArgs(params.args)
    return (async function* () {
      const child = spawn('git', params.args, { cwd: params.cwd, stdio: 'pipe' })
      for await (const chunk of child.stdout) {
        yield (chunk as Buffer).toString('utf-8')
      }
      await new Promise<void>((resolve, reject) => {
        child.on('close', (code) => code === 0 ? resolve() : reject(new Error(`git exit ${code}`)))
      })
    })()
  },
}
```

---

## 3. Extend git.ts RPC — Remote Mode

```typescript
// src/main/runtime/rpc/methods/git.ts (EXTENDED)
// All methods accept { projectId, worktreePath } in addition to local file ops

// NEW remote methods (via relay):
'git.status'           // relay: git status --porcelain=v2 --branch
'git.diff'             // relay: git diff [--staged] [--] <file>
'git.add'              // relay: git add <files>
'git.restore'          // relay: git restore [--staged] <files>
'git.commit'           // relay: git commit -m <msg>
'git.push'             // relay stream: git push <remote> <branch>
'git.pull'             // relay stream: git pull <remote> <branch>
'git.fetch'            // relay: git fetch --all
'git.branch.list'      // relay: git branch -a -vv
'git.branch.create'    // relay: git checkout -b <name> [from]
'git.branch.delete'    // relay: git branch -d <name>
'git.checkout'         // relay: git checkout <branch>
'git.merge'            // relay: git merge --no-ff <branch>
'git.stash'            // relay: git stash push -m <msg>
'git.stash.pop'        // relay: git stash pop
'git.log'              // relay: git log --oneline --graph --decorate -50
'git.show'             // relay: git show <hash>
'git.worktree.list'    // relay: git worktree list --porcelain
'git.worktree.add'     // relay: git worktree add <path> <branch>
'git.worktree.remove'  // relay: git worktree remove <path>

// AI-assisted methods:
'git.generateCommitMessage'  // relay: git diff --staged → LLM → commit message
'git.generatePRDescription'  // relay: git log main..HEAD → LLM → PR body

// PR creation:
'git.pr.create'        // relay: gh pr create (CLI) OR GitHub API token
'git.pr.list'          // relay: gh pr list OR GitHub API
```

---

## 4. AI Commit Message Generation

```typescript
// In git.ts RPC handler

async function generateCommitMessage(params: {
  projectId: string
  worktreePath: string
  userId: string
}): Promise<string> {
  const relay = await router.getRelayForProject(params.projectId, params.userId)
  const project = await projectService.get(params.projectId)!

  // Get staged diff
  const diffResult = await relay.call('git.exec', {
    cwd: params.worktreePath,
    args: ['diff', '--staged', '--stat', '-p'],
  }) as { stdout: string }

  if (!diffResult.stdout || diffResult.stdout.length < 10) {
    throw new Error('GIT_NO_STAGED_CHANGES')
  }

  // Truncate large diffs (max 8000 chars)
  const diffTruncated = diffResult.stdout.slice(0, 8000)

  const account = await providerResolver.resolve({
    devServerId: project.devServerId,
    projectId: project.id,
    userId: params.userId,
  })

  const response = await relay.call('ai.complete', {
    accountId: account.id,
    prompt: COMMIT_MSG_PROMPT + '\n\nDiff:\n```\n' + diffTruncated + '\n```',
    maxTokens: 200,
    temperature: 0.1,
  }) as { text: string }

  return response.text.trim()
}

const COMMIT_MSG_PROMPT = `Write a concise git commit message for the following diff.
Format: <type>(<scope>): <description>
Types: feat|fix|docs|style|refactor|test|chore
Max 72 characters for the first line.
Do NOT include any explanation, just the commit message.`
```

---

## 5. PR Creation — Dual Strategy

```typescript
// In git.ts RPC handler

async function createPR(params: {
  projectId: string
  worktreePath: string
  userId: string
  title: string
  body: string
  base: string
  draft?: boolean
}): Promise<{ prUrl: string; prNumber: number }> {
  const relay = await router.getRelayForProject(params.projectId, params.userId)
  const project = await projectService.get(params.projectId)!

  // Strategy A: GitHub CLI (if available on dev server)
  const preflight = await relay.call('preflight.check', {
    services: ['github-cli']
  }) as { githubCli: boolean }

  if (preflight.githubCli) {
    const result = await relay.call('git.exec', {
      cwd: params.worktreePath,
      args: [
        'gh', 'pr', 'create',
        '--title', params.title,
        '--body', params.body,
        '--base', params.base,
        ...(params.draft ? ['--draft'] : []),
        '--json', 'url,number',
      ],
    }) as { stdout: string }
    const { url, number } = JSON.parse(result.stdout)
    return { prUrl: url, prNumber: number }
  }

  // Strategy B: GitHub API token from WebCredentialStore
  const credential = await credentialService.get(params.userId, 'github')
  if (!credential) throw new Error('GITHUB_NO_CREDENTIAL')

  const response = await fetch('https://api.github.com/repos/.../pulls', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${credential.access_token}`,
      Accept: 'application/vnd.github+json',
    },
    body: JSON.stringify({ title: params.title, body: params.body, base: params.base }),
  })
  const pr = await response.json()
  return { prUrl: pr.html_url, prNumber: pr.number }
}
```

---

## 6. Task Status Auto-advance

```typescript
// After git.commit RPC is called:
async function onCommitComplete(
  commitMessage: string,
  projectId: string,
  userId: string
): Promise<void> {
  // Parse task references: #TG-xxx or closes #TG-xxx
  const taskRefs = [...commitMessage.matchAll(/#(TG-[\w-]+)/g)]
    .map(m => m[1])

  for (const taskRef of taskRefs) {
    const task = await taskService.findByRef(taskRef)
    if (task && task.projectId === projectId) {
      const hasPerm = await grantService.resolvePermission(userId, task.id)
      if (hasPerm && PERMISSION_LEVELS[hasPerm] >= PERMISSION_LEVELS['edit']) {
        await taskService.update(task.id, { status: 'review' })
        // Add activity comment
        await taskService.addComment(task.id, userId, `Commit: ${commitMessage}`, 'activity')
      }
    }
  }
}
```

---

## 7. Error Handling

| Scenario | Error code |
|---------|-----------|
| Disallowed git subcommand | `GIT_DISALLOWED_SUBCOMMAND` — 400 |
| Shell metacharacter in arg | `GIT_SHELL_METACHARACTER_IN_ARG` — 400 |
| No staged changes for commit msg | `GIT_NO_STAGED_CHANGES` — 400 |
| Git push rejected (non-fast-forward) | `GIT_PUSH_REJECTED` — passthrough stderr |
| Git merge conflict | `GIT_MERGE_CONFLICT` — return conflict files list |
| GitHub PR creation API error | `GITHUB_API_ERROR` — passthrough response |
| No GitHub credential | `GITHUB_NO_CREDENTIAL` — 401 |
| Diff too large (>10MB) | `GIT_DIFF_TOO_LARGE` — suggest staging fewer files |

---

## 8. Test Coverage

```
src/relay/__tests__/
├── git-handler.test.ts
│   ├── validateGitArgs: allowed subcommand OK
│   ├── validateGitArgs: disallowed → GIT_DISALLOWED_SUBCOMMAND
│   ├── validateGitArgs: metacharacter in arg → error
│   ├── git.exec: success → stdout returned
│   ├── git.exec: failure → non-zero exitCode returned (not thrown)
│   └── git.execStream: yields lines (mock child process)
src/main/runtime/rpc/methods/__tests__/
├── git-remote.test.ts
│   ├── git.status: relay.call invoked with correct args
│   ├── git.diff: staged=true → --staged flag passed
│   ├── git.add: files list passed
│   ├── git.commit: message passed
│   ├── git.push: streaming relay.callStream invoked
│   ├── git.generateCommitMessage: diff sent to AI → message returned
│   ├── git.generateCommitMessage: empty diff → GIT_NO_STAGED_CHANGES
│   ├── git.pr.create: gh CLI available → relay exec invoked
│   └── git.pr.create: no CLI → API fallback with token
src/main/task/__tests__/
├── task-commit-advance.test.ts
│   ├── commit with #TG-123 → task status → review
│   ├── commit with no task ref → no status change
│   └── commit with ref but no edit perm → no status change
```

**Target:** ≥ 35 tests; git-handler tested in complete isolation from Orca Server
