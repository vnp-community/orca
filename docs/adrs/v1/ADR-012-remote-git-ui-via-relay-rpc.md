# ADR-012 — Remote Git UI via Relay RPC (Not Local Git)

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-012 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-28 |
| **HLD Ref** | C3.12, C4.10 |
| **Code Ref** | `src/main/runtime/rpc/methods/git.ts` (extend), `src/relay/` (add git handlers) |
| **Feature Ref** | F39 |
| **Liên quan** | ADR-004 (Relay Binary), ADR-011 (WorkspaceContext) |

---

## Bối cảnh

Sau khi AI agent hoàn thành code changes trên dev server, developer cần thực hiện git workflow (stage, commit, push, PR). Hiện tại:
- Developer phải SSH vào server thủ công
- Hoặc dùng terminal tab trong Orca (nhưng cần nhớ git commands)
- Không có visual diff viewer
- Không tích hợp với Task status update

**Codebase hiện tại:**
- `src/main/runtime/rpc/methods/git.ts` — có một số git methods nhưng chủ yếu cho **local** git (Electron desktop mode)
- `src/relay/fs-handler-*.ts` — relay có file read nhưng không có dedicated git relay

---

## Quyết định

### Extend git.ts RPC cho Remote Git

```typescript
// src/main/runtime/rpc/methods/git.ts — extended
// NEW: all methods accept { cwd, devServerId } → relay to dev server

// Status
'git.status'    → relay.call('git.exec', { cwd, args: ['status', '--porcelain=v2', '--branch'] })

// Diff
'git.diff'      → relay.call('git.exec', { cwd, args: ['diff', ...(staged ? ['--staged'] : []), '--', file] })

// Stage/Unstage
'git.add'       → relay.call('git.exec', { cwd, args: ['add', ...files] })
'git.restore'   → relay.call('git.exec', { cwd, args: ['restore', ...(staged ? ['--staged'] : []), ...files] })

// Commit
'git.commit'    → relay.call('git.exec', { cwd, args: ['commit', '-m', message, '--author', author] })

// Push/Pull (streaming)
'git.push'      → relay.callStream('git.exec', { cwd, args: ['push', remote, branch], stream: true })
'git.pull'      → relay.callStream('git.exec', { cwd, args: ['pull', remote, branch], stream: true })
'git.fetch'     → relay.call('git.exec', { cwd, args: ['fetch', '--all'] })

// Branch
'git.branch.list'   → relay.call('git.exec', { cwd, args: ['branch', '-a', '-vv'] })
'git.branch.create' → relay.call('git.exec', { cwd, args: ['checkout', '-b', name, ...(from ? [from] : [])] })
'git.branch.delete' → relay.call('git.exec', { cwd, args: ['branch', '-d', name] })
'git.checkout'      → relay.call('git.exec', { cwd, args: ['checkout', branch] })
'git.merge'         → relay.call('git.exec', { cwd, args: ['merge', '--no-ff', branch] })
'git.stash'         → relay.call('git.exec', { cwd, args: ['stash', 'push', '-m', message] })
'git.stash.pop'     → relay.call('git.exec', { cwd, args: ['stash', 'pop'] })
'git.log'           → relay.call('git.exec', { cwd, args: ['log', '--oneline', '--graph', '--decorate', '-50'] })

// Worktree
'git.worktree.list'   → relay.call('git.exec', { cwd, args: ['worktree', 'list', '--porcelain'] })
'git.worktree.add'    → relay.call('git.exec', { cwd, args: ['worktree', 'add', path, branch] })
'git.worktree.remove' → relay.call('git.exec', { cwd, args: ['worktree', 'remove', path] })
```

### Relay Git Handler

```typescript
// src/relay/git-handler.ts (NEW)
// Generic git.exec handler — runs git commands on dev server

async function gitExecHandler(
  params: { cwd: string; args: string[]; stream?: boolean },
  context: RequestContext
): Promise<GitExecResult | AsyncGenerator<string>> {
  const { cwd, args, stream } = params

  if (stream) {
    // Yield output lines as they come (for push/pull progress)
    return spawnStream('git', args, { cwd })
  }

  const result = await execFile('git', args, { cwd, timeout: 30_000 })
  return { stdout: result.stdout, stderr: result.stderr, exitCode: 0 }
}
```

### AI Commit Message Flow

```typescript
// In CommitForm.tsx component
async function generateCommitMessage(): Promise<string> {
  // 1. Get staged diff
  const diff = await rpc.call('git.diff', { cwd, devServerId, staged: true })
  if (!diff.stdout || diff.stdout.length < 10) return ''

  // 2. Truncate if too large (max 8000 chars for context)
  const diffTruncated = diff.stdout.slice(0, 8000)

  // 3. Call AI
  const response = await rpc.call('ai.complete', {
    devServerId,
    prompt: COMMIT_MSG_SYSTEM_PROMPT + '\n\nDiff:\n' + diffTruncated,
    maxTokens: 200,
  })

  return response.text
}
```

### Streaming Push/Pull

```typescript
// In GitPanel.tsx
async function* pushWithProgress(): AsyncGenerator<string> {
  // rpc.callStream returns async iterator from server-side streaming RPC
  for await (const line of rpc.callStream('git.push', {
    cwd: currentWorktree.path,
    devServerId: project.devServerId,
    remote: 'origin',
    branch: gitStatus.branch,
  })) {
    yield line  // "Counting objects: 5, done."
  }
}
```

### PR Creation

```typescript
async function createPR(params: PRParams): Promise<PRResult> {
  // Try Category A (GitHub CLI on dev server) first
  const ghAvailable = await rpc.call('preflight.check', {
    devServerId, services: ['github']
  })

  if (ghAvailable.github.cli) {
    return rpc.call('github.pr.create', {
      devServerId,
      cwd: currentWorktree.path,
      ...params,
      ghConfigDir: `/home/${user.unixName}/.config/gh/${user.id}/`,
    })
  }

  // Fallback to Category B (API token from WebCredentialStore)
  const token = await rpc.call('credentials.get', { service: 'github' })
  return githubApiCreatePR({ ...params, token: token.access_token })
}
```

### Task Status Auto-advance

```typescript
// After git.commit, check message for task references
function checkTaskReferenceInCommitMessage(message: string): void {
  const matches = message.match(/#(TG-[\w-]+)/g) ?? []
  for (const match of matches) {
    const taskId = match.slice(1)  // remove '#'
    rpc.call('task.update', { taskId, status: 'done' })
  }
}

// WorkspaceContext event:
on('git.commit', ({ message }) => checkTaskReferenceInCommitMessage(message))
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **relay.call('git.exec') + streaming** ✅ | Reuse relay infrastructure; secure (git chạy trên server); flexible |
| Local git clone mirroring | Double bandwidth; sync complexity; stale data |
| isomorphic-git (browser) | Không access private repos cần SSH keys trên server |
| GitKraken/Sourcetree embed | External dependency; licensing |
| Terminal-only (existing) | No visual diff; poor UX; user phải remember commands |

---

## Hậu quả

**Tích cực:**
- Git chạy trực tiếp trên dev server → no sync lag
- SSH keys trên server được dùng cho push (user không cần expose private key)
- Push/pull progress streaming cho UX tốt
- AI commit/PR description tích hợp native

**Tiêu cực:**
- Relay cần hỗ trợ streaming (relay.callStream) — hiện tại có fs.stream nhưng cần verify cho git
- `git.exec` generic handler → SQL injection analogy: args phải validate whitelist (git subcommands)
- Large diffs (> 8000 chars) phải truncate cho AI → có thể miss context
- Windows: git path khác (Git for Windows), cần handle trong relay

---

## Security Constraints

```typescript
// Whitelist allowed git subcommands (prevent arbitrary shell injection)
const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash', 'log',
  'worktree', 'remote', 'tag'
])

function validateGitArgs(args: string[]): void {
  if (!ALLOWED_GIT_SUBCOMMANDS.has(args[0])) {
    throw new Error(`Disallowed git subcommand: ${args[0]}`)
  }
  // No shell expansion allowed: no &&, ||, ;, |, $, `
  for (const arg of args) {
    if (/[&|;$`]/.test(arg)) throw new Error(`Shell metacharacter in git arg: ${arg}`)
  }
}
```

---

## Trạng thái Implementation

⚠️ Partial (local git.ts có, relay git chưa)  
❌ Relay git-handler.ts chưa tạo  
❌ Streaming push/pull chưa  
❌ AI commit message generation chưa  
🎯 `src/relay/git-handler.ts`  
🎯 Extend `src/main/runtime/rpc/methods/git.ts` cho remote mode  
🎯 GitPanel.tsx, DiffViewer.tsx, CommitForm.tsx, BranchManager.tsx, PullRequestForm.tsx
