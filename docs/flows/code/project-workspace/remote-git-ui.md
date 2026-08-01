# Remote Git UI Flow — F39 Remote Git UI

> **Scope**: Luồng xem file explorer, git status, diff, commit, push, tạo PR — tất cả qua relay đến Dev Server
>
> **Key files**:
> - [`src/renderer/src/components/workspace/GitPanel.tsx`](../../src/renderer/src/components/workspace/GitPanel.tsx) — Git status, diff, commit UI
> - [`src/renderer/src/components/workspace/ExplorerPanel.tsx`](../../src/renderer/src/components/workspace/ExplorerPanel.tsx) — File tree
> - [`src/renderer/src/components/workspace/DiffViewer.tsx`](../../src/renderer/src/components/workspace/DiffViewer.tsx) — Unified diff với syntax highlight
> - [`src/renderer/src/components/workspace/CommitForm.tsx`](../../src/renderer/src/components/workspace/CommitForm.tsx) — Commit message + AI generate
> - [`src/main/runtime/rpc/methods/git.ts`](../../src/main/runtime/rpc/methods/git.ts) — Git RPC methods (relay)
> - [relay: `src/agent/git/git-engine.ts`](../../src/agent/git/git-engine.ts) — Git execution on Dev Server
> - **Feature**: [F39 Remote Git UI](../features/F39-remote-git-ui.md)
> - **Business Logic**: [BL-PW-03](../logic/project-workspace/BL-PW-03-remote-git-operations.md), [BL-CR-04](../logic/code-review/BL-CR-04-generate-commit-message.md), [BL-CR-05](../logic/code-review/BL-CR-05-tao-pull-request.md)

---

## 1. Tổng quan Data Flow

```
Browser UI (GitPanel)           Orca Server          Dev Server (relay)
       │                             │                      │
       │ git.status()                │                      │
       │────────────────────────────►│                      │
       │                             │ relay.call('git.status', { cwd })
       │                             │─────────────────────►│
       │                             │                      │ execFile('git status --porcelain=v2 --branch')
       │                             │◄─────────────────────│ GitStatus object
       │◄────────────────────────────│                      │
       │ Render: M:12 A:2 D:1        │                      │
```

---

## 2. File Explorer (ExplorerPanel)

### 2.1 Initial Load

```typescript
// Triggered bởi WorkspaceContext.switchProject() → fs.readDir

// RPC: fs.readDir
// Server calls: relay.call('fs.readDir', { path: repoPath, depth: 2 })
// Dev Server relay executes:
async readDir(path: string, depth: number): Promise<FileTreeNode> {
  const entries = await fs.readdir(path, { withFileTypes: true })
  return {
    path,
    name: basename(path),
    type: 'directory',
    children: await Promise.all(
      entries.map(async entry => {
        if (entry.isDirectory()) {
          if (depth > 1 && !IGNORED_DIRS.has(entry.name)) {
            return readDir(join(path, entry.name), depth - 1)
          }
          return { path: join(path, entry.name), name: entry.name, type: 'directory' }
        }
        const stat = await fs.stat(join(path, entry.name))
        return {
          path: join(path, entry.name), name: entry.name,
          type: 'file', size: stat.size,
          gitStatus: gitStatusMap.get(join(path, entry.name))  // M/A/D badges
        }
      })
    )
  }
}
```

### 2.2 File Read (RemoteFileViewer)

```
User click file "src/core/validator.ts"
    │
    ▼ RPC: fs.readFile({ path: '/srv/vnp/src/core/validator.ts' })
    │
    ├── relay.call('fs.readFile', { path, maxSize: 5 * 1024 * 1024 })
    │   → Dev Server: fs.readFile(path)
    │   → Check size ≤ 5MB
    │   → return { content: string, encoding: 'utf8' }
    │
    └── RemoteFileViewer: Monaco editor (read-only, syntax highlight)
        file extension → Monaco language detection
```

### 2.3 File Search

```
User types in FileSearchPanel: "validateSignature"
    │
    ▼ RPC: fs.grep({ pattern: 'validateSignature', cwd: repoPath, includes: ['*.ts'] })
    │
    ├── relay.call('fs.grep', { ... })
    │   → Dev Server: spawn('grep', ['-rn', '--include=*.ts', 'validateSignature', cwd])
    │   → limit: 30 results
    │   → return [{ file, line, content }]
    │
    └── FileSearchPanel: list results with file:line + snippet
        Click → open RemoteFileViewer at that line
```

---

## 3. Git Status Panel

### 3.1 git.status

```typescript
// Triggered mỗi 5s (polling) + sau mỗi agent.complete event

// RPC: git.status
// Server: relay.call('git.status', { cwd: worktreePath })
// Dev Server:
async getStatus(cwd: string): Promise<GitStatus> {
  const output = await execFile('git', ['status', '--porcelain=v2', '--branch'], { cwd })

  return parsePortcelainV2(output)
  // → {
  //     branch: 'feat/ecdsa-validation',
  //     ahead: 3,
  //     behind: 0,
  //     staged: [{ path, status: 'M' }],
  //     unstaged: [{ path, status: 'M' }],
  //     untracked: [{ path }],
  //   }
}
```

### 3.2 Diff Viewer

```
User click file in staged/unstaged list
    │
    ▼ RPC: git.diff({ cwd, file, staged: true })
    │
    ├── relay.call('git.diff', { cwd, file, staged })
    │   → Dev Server: execFile('git', ['diff', '--staged', '--', file], { cwd })
    │   → return: unified diff string
    │
    └── DiffViewer component:
        → parse diff → syntax-highlighted hunks
        → + lines (green), - lines (red), @@ headers
        → Monaco DiffEditor (side-by-side option)
```

---

## 4. Stage / Unstage Operations

```typescript
// Stage files
'git.add'      → relay: execFile('git', ['add', ...files], { cwd })
'git.restore'  → relay: execFile('git', ['restore', '--staged', ...files], { cwd })

// Discard changes
'git.restore'  // (no --staged) → relay: git restore <file>

// Stash
'git.stash'    → relay: execFile('git', ['stash', 'push', '-m', msg], { cwd })
'git.stash.pop'→ relay: execFile('git', ['stash', 'pop'], { cwd })
```

---

## 5. AI Commit Message Generation

```
User: [Stage All] → [AI: Generate commit message]
    │
    ├── RPC: git.add({ cwd, files: ['.'] })
    │   → relay: git add .
    │
    ├── RPC: git.generateCommitMessage({ cwd })
    │   │
    │   ├── relay.call('git.diff', { cwd, staged: true })
    │   │   → diff string (max 4000 chars truncated)
    │   │
    │   ├── ProviderResolver.resolve(userId, projectId)
    │   │   → API key
    │   │
    │   ├── LLM call:
    │   │   system: "You are a commit message generator. Follow conventional commits."
    │   │   user: "Generate commit message for:\n{diff}"
    │   │   → response: "feat(crypto): add ECDSA signature validation with SEC1 encoding"
    │   │
    │   └── return { message: string }
    │
    └── CommitForm: pre-filled với AI message (user editable)
```

---

## 6. Commit

```typescript
// CommitForm: user reviews message → [Commit]
// RPC: git.commit({ cwd, message, author? })

// Server:
async commit(cwd: string, message: string, author?: GitAuthor): Promise<CommitResult> {
  const args = ['commit', '-m', message]

  // Inject user identity (per-user git author for multi-user server)
  if (author) {
    args.push(`--author=${author.name} <${author.email}>`)
  }

  const { stdout } = await execFile('git', args, { cwd })
  // stdout: "[feat/ecdsa-validation abc1234] feat(crypto): add ECDSA..."
  return { hash: extractHash(stdout), message }
}

// Sau commit: refresh git.status
// GitPanel: ahead count +1, staged list empty
```

---

## 7. Push with Progress Stream

```
User: [Push]
    │
    ▼ RPC: git.push({ cwd, remote: 'origin', branch })
    │
    ├── relay.callStream('git.push', { cwd, remote, branch })
    │   │
    │   │ Dev Server streams stdout:
    │   │ → { type: 'git.push.progress', line: 'Enumerating objects: 15, done.' }
    │   │ → { type: 'git.push.progress', line: 'Counting objects: 100% (15/15), done.' }
    │   │ → { type: 'git.push.progress', line: 'Writing objects: 100% (8/8), 4.2 KiB' }
    │   │ → { type: 'git.push.progress', line: 'To github.com:org/repo.git' }
    │   │ → { type: 'git.push.progress', line: '   abc1234..def5678  feat/ecdsa-validation → origin/feat/ecdsa-validation' }
    │   │ → { type: 'git.push.done', success: true, exitCode: 0 }
    │   │
    │   └── GitPanel: stream each line vào "Push Progress" panel
    │
    └── On done:
        refresh git.status → ahead=0, behind=0
        WorkspaceContext.emit('git.push', { branch })
        → if PR exists → refresh PR status via GitHub API
```

---

## 8. Branch Management

```typescript
// BranchManager component
'git.branch.list'   // → [{ name, isRemote, isHead, commit, behind, ahead }]
'git.branch.create' // (name, from?) → checkout -b
'git.branch.delete' // (name, force?) → git branch [-D] name
'git.branch.switch' // (name) → git checkout name

// Git Log
'git.log'           // → [{hash, subject, author, date, refs}] (last 50)
'git.fetch'         // → fetch --all --prune (background)
'git.pull'          // → stream progress như push
'git.merge'         // (branch, noFF?) → stream output

// Worktree management (in workspace)
'git.worktree.add'  // (branchName, basePath) → create new worktree
'git.worktree.list' // → [{ path, branch, isMain }]
'git.worktree.remove' // (path) → git worktree remove
```

---

## 9. Pull Request Flow

```
Developer: [Create PR] (sau khi push thành công)
    │
    ├── UI: PullRequestForm
    │   ├── Auto-fill: title từ last commit message
    │   ├── Auto-fill: base branch = project.defaultBranch
    │   │
    │   └── [AI: Generate PR Description]
    │       → relay: git log origin/main..HEAD --format="- %s%n%b"
    │       → LLM call → PR description markdown
    │
    ├── User edits: title, description, reviewers, labels
    │
    ├── RPC: git.createPR({
    │     cwd: worktreePath,
    │     title, body, base,
    │     reviewers: ['maya@co.com'],
    │     labels: ['enhancement'],
    │   })
    │
    │   Dev Server:
    │   → execFile('gh', ['pr', 'create',
    │       '--title', title,
    │       '--body', body,
    │       '--base', base,
    │       '--reviewer', 'maya@co.com',
    │     ])
    │   → uses GH_CONFIG_DIR = userDataPath/gh-config (user isolation)
    │   → return: { prUrl: 'https://github.com/org/repo/pull/42' }
    │
    └── TaskService.update(taskId, { prUrl, status: 'review' })
        GitPanel: shows PR link + status badge
```

---

## 10. Conflict Resolution

```
git.merge() trả về exit code 1 (conflict)
    │
    ├── GitPanel shows ConflictPanel:
    │   ┌─────────────────────────────────────────────┐
    │   │  ⚠️ 3 files have merge conflicts             │
    │   │  - src/core/validator.ts                    │
    │   │  - src/crypto/keys.ts                       │
    │   │  - tests/validator.test.ts                  │
    │   │                                             │
    │   │  [AI Resolve]  [Resolve Manually]           │
    │   └─────────────────────────────────────────────┘
    │
    ├── [AI Resolve]:
    │   → Với mỗi conflict file:
    │     relay: fs.readFile → content với <<<, ===, >>> markers
    │     LLM call: "Resolve this git conflict: {content}"
    │     → resolved content
    │     relay: fs.writeFile(path, resolved)
    │   → relay: git.add(conflictFiles)
    │   → relay: git.commit({ message: 'Merge: resolve conflicts' })
    │
    └── [Resolve Manually]:
        → Open file in Monaco (read-write mode)
        → User edits conflict markers manually
        → [Mark Resolved] → git.add → git.commit
```

---

## 11. RPC Methods — git.* & fs.*

```typescript
// Git operations (all via relay)
'git.status'           // { cwd } → GitStatus
'git.diff'             // { cwd, file?, staged? } → string
'git.add'              // { cwd, files } → void
'git.restore'          // { cwd, files, staged? } → void
'git.commit'           // { cwd, message, author? } → CommitResult
'git.push'             // { cwd, remote, branch } → stream
'git.pull'             // { cwd, remote?, branch? } → stream
'git.fetch'            // { cwd } → void
'git.branch.list'      // { cwd } → BranchInfo[]
'git.branch.create'    // { cwd, name, from? } → void
'git.branch.delete'    // { cwd, name, force? } → void
'git.branch.switch'    // { cwd, name } → void
'git.log'              // { cwd, limit? } → CommitInfo[]
'git.merge'            // { cwd, branch, noFF? } → stream
'git.stash'            // { cwd, message? } → void
'git.stash.pop'        // { cwd } → void
'git.worktree.add'     // { repoPath, branch, basePath } → { worktreePath }
'git.worktree.list'    // { repoPath } → WorktreeInfo[]
'git.worktree.remove'  // { path } → void
'git.createPR'         // { cwd, title, body, base, reviewers?, labels? } → { prUrl }
'git.generateCommitMessage' // { cwd } → { message }

// File system (via relay)
'fs.readDir'           // { path, depth } → FileTreeNode
'fs.readFile'          // { path } → { content, encoding }
'fs.stat'              // { path } → FileStat
'fs.glob'              // { pattern, cwd } → string[]
'fs.grep'              // { pattern, cwd, includes? } → SearchResult[]
```

---

## 12. Git User Isolation (Multi-User)

```typescript
// Mỗi user có GH_CONFIG_DIR riêng → gh CLI dùng credentials của user đó
// src/agent/git/git-user-isolation.ts

function buildGitEnv(userId: string, userDataPath: string): Record<string, string> {
  const userGhConfigDir = path.join(userDataPath, 'users', userId, 'gh-config')
  const userGlabConfigDir = path.join(userDataPath, 'users', userId, 'glab-config')

  return {
    GH_CONFIG_DIR: userGhConfigDir,   // gh CLI: dùng token của user này
    GLAB_CONFIG_DIR: userGlabConfigDir, // glab CLI: dùng token của user này

    // Git author identification
    GIT_AUTHOR_NAME:    ctx.userName,
    GIT_AUTHOR_EMAIL:   ctx.userEmail,
    GIT_COMMITTER_NAME: ctx.userName,
    GIT_COMMITTER_EMAIL: ctx.userEmail,
  }
}
```

---

## 13. Cross-References

| Resource | Mô tả |
|---|---|
| [project-workspace-switch.md](./project-workspace-switch.md) | Workspace phải ready trước khi dùng Git UI |
| [task-agent-execution.md](./task-agent-execution.md) | Agent complete → GitPanel refresh |
| [relay-management.md](./relay-management.md) | Tất cả git calls qua SSH relay |
| [ai-provider-credential.md](./ai-provider-credential.md) | API key cho AI commit message/PR description |
| **HLD C1 Flow 10** | Task → Agent → Git → PR end-to-end |
| **HLD C4.10** | Remote Git Push with Progress data flow |
| **HLD C3.3** | Relay Components (git-handler) |
| **F39 Remote Git UI** | Feature spec |
| **BL-PW-03** | Remote git operations business logic |
| **BL-CR-04** | AI commit message |
| **BL-CR-05** | Create PR |
