# TC-PW-003 — Remote Git UI Operations

**BL Reference:** BL-PW-03  
**Flow Reference:** docs/flows/logic/project-workspace.md + docs/features/F39-remote-git-ui.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Developer, Lead (Carlos especially — all via browser)

---

## TC-PW-003-01: Git status — modified/staged/untracked (porcelain-v2 parse)

**Priority:** P0

### Steps
1. On dev server: modify `auth.ts` (staged), add `newfile.ts` (untracked), delete `old.ts`
2. Open Git panel in workspace
3. Git status polled via `relay.call('git.status', { format: 'porcelain-v2' })`

### Expected Results
- Staged: `[auth.ts]`
- Untracked: `[newfile.ts]`
- Deleted: `[old.ts]`
- ahead/behind counts parsed from branch header
- Real-time poll refreshes mỗi 5s khi active

### Assertions
```
mockRelayGitStatus(`
# branch.head feature/auth
# branch.ab +2 -0
1 M. ... auth.ts
? newfile.ts
1 D. ... old.ts
`)
status = await rpc.call('git.status', { projectId })
assert status.staged.includes('auth.ts')
assert status.untracked.includes('newfile.ts')
assert status.deleted.includes('old.ts')
assert status.ahead === 2
assert status.behind === 0
```

---

## TC-PW-003-02: Stage file

**Priority:** P0

### Steps
1. `git.stage { file: 'auth.ts' }`

### Expected Results
- `relay.call('git.add', { cwd: worktreePath, files: ['auth.ts'] })`
- Git status refreshed: auth.ts moved from modified → staged

### Assertions
```
await rpc.call('git.stage', { projectId, file: 'auth.ts' })
addCall = capturedRelayCall('git.add')
assert addCall.args.files.includes('auth.ts')
// Status refreshed
status = await rpc.call('git.status', { projectId })
assert status.staged.includes('auth.ts')
assert !status.modified.includes('auth.ts')
```

---

## TC-PW-003-03: Stage all files

**Priority:** P0

### Steps
1. 3 modified files, 1 untracked
2. `git.stageAll`

### Expected Results
- `relay.call('git.add', { files: ['.'] })`
- All files staged

---

## TC-PW-003-04: Unstage file

**Priority:** P1

### Steps
1. File staged
2. `git.unstage { file: 'auth.ts' }`

### Expected Results
- `relay.call('git.restore', { cwd, files: ['auth.ts'], staged: true })`
- File back to unstaged/modified

---

## TC-PW-003-05: Discard changes — git restore với confirm

**Priority:** P1

### Steps
1. File modified (unstaged): `auth.ts`
2. User clicks [Discard Changes]
3. Confirm dialog shown → user confirms

### Expected Results
- `relay.call('git.restore', { cwd, files: ['auth.ts'] })` (no --staged flag)
- File reverts to HEAD
- Git status refreshed: auth.ts removed from modified list

### Assertions
```
// User confirms discard
await rpc.call('git.discardChanges', { projectId, file: 'auth.ts', confirmed: true })
restoreCall = capturedRelayCall('git.restore')
assert restoreCall.args.staged === undefined || restoreCall.args.staged === false
assert restoreCall.args.files.includes('auth.ts')
```

### Error Scenarios
| Scenario | Input | Expected |
|----------|-------|----------|
| User cancels confirm | confirmed: false | No action, file unchanged |
| File already committed | file: 'committed.ts' | 400 NO_CHANGES |

---

## TC-PW-003-06: Commit với manual message

**Priority:** P0

### Steps
1. Files staged
2. `git.commit { message: 'feat: add auth middleware', projectId }`

### Expected Results
- `relay.call('git.commit', { cwd, message: 'feat: add auth middleware', author: { name: user.name, email: user.email } })`
- Author from user profile (gitAuthor)
- Status: commit successful

### Assertions
```
await rpc.call('git.commit', { projectId, message: 'feat: add auth middleware' })
commitCall = capturedRelayCall('git.commit')
assert commitCall.args.message === 'feat: add auth middleware'
assert commitCall.args.author.email === currentUser.email
```

---

## TC-PW-003-07: AI commit message generation

**Priority:** P0

### Steps
1. Files staged (diff exists)
2. `git.generateCommitMessage { projectId }`
3. AI generates message from staged diff

### Expected Results
- `relay.call('git.diff', { staged: true })` → diff text (max 8000 chars)
- AI call với `COMMIT_MSG_PROMPT_TEMPLATE + diff`
- AI returns conventional commit message
- Message shown in commit dialog (editable)

### Assertions
```
mockStagedDiff('- old code\n+ new code')
mockAIResponse('feat(auth): implement bcrypt 12-round password hashing\n\nReplace SHA-256...')

result = await rpc.call('git.generateCommitMessage', { projectId })
assert result.message.startsWith('feat(auth):')
assert result.message.length > 20

diffCall = capturedRelayCall('git.diff')
assert diffCall.args.staged === true
```

---

## TC-PW-003-08: Push với progress stream

**Priority:** P0

### Steps
1. Commit exists, push to origin
2. `git.push { remote: 'origin', branch: 'feature/auth-bcrypt', projectId }`

### Expected Results
- `relay.callStream('git.push', ...)` with streaming
- Output lines streamed real-time: "Counting objects...", "Writing objects...", "Done."
- Each line emitted as SSE event
- Push complete: success event

### Assertions
```
lines = []
subscribeSSE('/api/workspace/git/push-events', line => lines.push(line))
await rpc.call('git.push', { projectId, branch: 'feature/auth-bcrypt' })
assert lines.some(l => l.includes('Counting objects'))
assert lines.some(l => l.includes('Writing objects'))
assert lines[lines.length - 1].type === 'git.push.complete'
```

---

## TC-PW-003-09: Pull

**Priority:** P0

### Steps
1. `git.pull { remote: 'origin', branch: 'main', projectId }`

### Expected Results
- `relay.call('git.pull', { cwd, remote: 'origin', branch: 'main' })`
- Result: "Fast-forward" or "Already up to date"
- Git status refreshed after pull

---

## TC-PW-003-10: Conflict detection sau pull

**Priority:** P1

### Steps
1. Local changes + remote changes → conflict
2. `git.pull`

### Expected Results
- pull output contains "CONFLICT"
- Error: `{ code: 'MERGE_CONFLICT', conflictFiles: ['auth.ts', 'auth.test.ts'] }`
- Conflict panel shown with files list
- Git status shows 'U' (unmerged) status for conflict files

### Assertions
```
mockRelayGitPull({ output: 'CONFLICT (content): Merge conflict in auth.ts' })
result = await rpc.call('git.pull', { projectId }).catch(e => e)
assert result.code === 'MERGE_CONFLICT'
assert result.conflictFiles.includes('auth.ts')
```

---

## TC-PW-003-11: AI conflict resolution

**Priority:** P1

### Preconditions
- auth.ts has conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`)

### Steps
1. User clicks [AI: Resolve conflict] on auth.ts
2. AI reads both sides, generates resolution
3. File written with resolved content

### Expected Results
- `relay.call('git.diff', { file: 'auth.ts' })` → conflict markers
- AI agent reads conflict, writes resolution
- `relay.call('git.add', { files: ['auth.ts'] })`
- Conflict resolved → auth.ts no longer in unmerged list

### Assertions
```
await rpc.call('git.aiResolveConflict', { projectId, file: 'auth.ts' })
diffCall = capturedRelayCall('git.diff')
assert diffCall.args.file === 'auth.ts'

// After AI resolves
status = await rpc.call('git.status', { projectId })
assert !status.unmerged.includes('auth.ts')
```

---

## TC-PW-003-12: Branch management — create, checkout, delete

**Priority:** P1

### Steps (create)
1. `git.createBranch { name: 'feature/new-ui', from: 'main', projectId }`

### Expected Results
- `relay.call('git.branch.create', { name: 'feature/new-ui', from: 'main' })`
- Checkout to new branch automatically

### Steps (delete)
1. `git.deleteBranch { branch: 'feature/old', force: false, projectId }`

### Expected Results
- `relay.call('git.branch.delete', { branch: 'feature/old', force: false })`
- If branch not merged: error `{ code: 'BRANCH_NOT_MERGED' }` (force=false)

---

## TC-PW-003-13: Git log — 50 commits với branch graph

**Priority:** P1

### Steps
1. `git.log { limit: 50, projectId }`

### Expected Results
- `relay.call('git.log', { cwd, format: '--oneline --graph --decorate -50' })`
- Returns 50 most recent commits
- Each commit: hash, author, date, message, branch refs

---

## TC-PW-003-14: Create Pull Request — GitHub CLI (Category A)

**Priority:** P0

### Preconditions
- gh CLI installed on dev server
- User has GitHub credentials via CLI

### Steps
1. Branch pushed to origin
2. `git.createPR { title: 'feat: auth', base: 'main', useAI: false, projectId }`

### Expected Results
- `relay.call('github.pr.create', { cwd, title, base, draft, ghConfigDir: '/home/dev/.config/gh/<userId>/' })`
- Returns PR URL
- Per-user ghConfigDir isolation (no credential sharing)

### Assertions
```
result = await rpc.call('git.createPR', { projectId, title: 'feat: auth', base: 'main' })
assert result.prUrl.includes('github.com')
assert result.prUrl.includes('/pull/')

prCall = capturedRelayCall('github.pr.create')
assert prCall.args.ghConfigDir.includes(currentUser.id)  // per-user isolation
```

---

## TC-PW-003-15: Create Pull Request — GitHub API token (Category B)

**Priority:** P1

### Preconditions
- gh CLI NOT installed
- GitHub API token in WebCredentialStore

### Steps
1. `git.createPR { title: 'feat: auth', base: 'main', projectId }`

### Expected Results
- No `relay.call('github.pr.create')` call
- Instead: `WebCredentialStore.get(userId, 'github', 'api_token')`
- Direct GitHub API call: `POST /repos/.../pulls`

### Assertions
```
mockGhCLINotInstalled()
mockWebCredentialStore({ token: 'ghp_xxx' })

result = await rpc.call('git.createPR', { projectId, title: 'feat: auth' })
assert capturedRelayCall('github.pr.create') === undefined
assert capturedGitHubApiCall.url.includes('/pulls')
```

---

## TC-PW-003-16: Create PR — No auth method available

**Priority:** P1

### Preconditions
- gh CLI NOT installed
- No GitHub token in CredentialStore

### Steps
1. `git.createPR { projectId }`

### Expected Results
- Error: `{ code: 'NO_GITHUB_AUTH', message: 'No GitHub CLI or API token available' }`

---

## TC-PW-003-17: AI PR description generation

**Priority:** P1

### Steps
1. `git.createPR { useAI: true, base: 'main', projectId }`
2. AI generates description from diff + commits

### Expected Results
- `relay.call('git.diff', { base: 'main' })` → diff
- `relay.call('git.log', { base: 'main' })` → commits list
- AI generates PR description
- PR form pre-filled with title + description

### Assertions
```
mockAIResponse('feat(auth): implement bcrypt...\n\nThis PR replaces SHA-256...')
result = await rpc.call('git.createPR', { projectId, base: 'main', useAI: true })
assert result.suggestedDescription.length > 50

diffCall = capturedRelayCall('git.diff')
assert diffCall.args.base === 'main'
```

---

## TC-PW-003-18: Stash push / pop

**Priority:** P1

### Steps (push)
1. Modified files (unstaged)
2. `git.stashPush { message: 'WIP: auth refactor', projectId }`

### Expected Results
- `relay.call('git.stash', { cwd, message: 'WIP: auth refactor' })`
- Working tree clean after stash

### Steps (pop)
1. `git.stashPop { projectId }`

### Expected Results
- `relay.call('git.stash.pop', { cwd })`
- Files restored from stash

---

## TC-PW-003-19: Worktree switcher per project

**Priority:** P1

### Steps
1. Project has 3 worktrees: [main, feature/auth, feature/ui]
2. `workspace.switch` → worktree selector shows all 3
3. Select `feature/auth`

### Expected Results
- `relay.call('git.worktree.list', { repoPath })` → list fetched
- Git tab switches to feature/auth context
- All git commands now use feature/auth's cwd
- File explorer shows feature/auth file tree

### Assertions
```
worktrees = await rpc.call('git.listWorktrees', { projectId })
assert worktrees.length === 3
assert worktrees.some(wt => wt.branch === 'feature/auth')

await rpc.call('workspace.switchWorktree', { projectId, worktreeId: authWorktree.id })
status = await rpc.call('git.status', { projectId })
assert status.branch === 'feature/auth'
```

---

*TC-PW-003 — Orca v5.0 — Updated 2026-08-01*
