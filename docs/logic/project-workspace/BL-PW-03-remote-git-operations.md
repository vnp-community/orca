# BL-PW-03 — Remote Git UI Operations

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PW-03 |
| **Tên** | Remote Git UI Operations |
| **Domain** | Project Workspace |
| **Actor** | Developer, Lead |
| **Priority** | P0 |

---

## Mô tả

Toàn bộ git workflow được thực hiện qua relay trên dev server. UI gọi relay RPC → relay chạy git commands trong thư mục repo/worktree → stream output về UI.

---

## Relay Git Command Map

```typescript
// relay.call('git.*', { cwd: worktreePath, ...args })

'git.status'        → git status --porcelain=v2 --branch
'git.diff'          → git diff [--staged] [-- file]
'git.add'           → git add <files>
'git.restore'       → git restore [--staged] <files>
'git.commit'        → git commit -m <message> --author="name <email>"
'git.push'          → git push origin <branch> (stream output)
'git.pull'          → git pull origin <branch> (stream output)
'git.fetch'         → git fetch --all
'git.branch.list'   → git branch -a -vv
'git.branch.create' → git checkout -b <branch> [<from>]
'git.branch.delete' → git branch -d|-D <branch>
'git.checkout'      → git checkout <branch>
'git.merge'         → git merge [--no-ff] <branch>
'git.rebase'        → git rebase <branch>
'git.log'           → git log --oneline --graph --decorate -50
'git.worktree.list' → git worktree list --porcelain
'git.worktree.add'  → git worktree add <path> <branch>
'git.worktree.remove' → git worktree remove <path>
'git.stash'         → git stash push -m <message>
'git.stash.pop'     → git stash pop
```

---

## Git Status Parsing

```typescript
function parseGitPorcelainV2(raw: string): GitStatus {
  const lines = raw.split('\n')
  const modified: string[] = [], added: string[] = [], deleted: string[] = []
  const untracked: string[] = [], staged: string[] = []
  let ahead = 0, behind = 0, branch = ''

  for (const line of lines) {
    if (line.startsWith('# branch.head')) branch = line.split(' ')[2]
    if (line.startsWith('# branch.ab')) {
      const [, a, b] = line.match(/\+(\d+) -(\d+)/) ?? []
      ahead = +a; behind = +b
    }
    if (line.startsWith('1 ')) {  // ordinary changed entry
      const xy = line.slice(2, 4)
      const file = line.slice(line.lastIndexOf('\t') + 1)
      if (xy[0] !== '.') staged.push(file)
      if (xy[1] === 'M') modified.push(file)
      if (xy[1] === 'A') added.push(file)
      if (xy[1] === 'D') deleted.push(file)
    }
    if (line.startsWith('?')) untracked.push(line.slice(3))
  }
  return { branch, ahead, behind, modified, added, deleted, untracked, staged }
}
```

---

## Commit Flow với AI Message

```
1. User clicks [AI: Generate message]
2. relay.call('git.diff', { cwd, staged: true })
   → staged diff text (max 8000 chars)
3. AIProviderResolver.resolve({ devServerId, ... })
4. relay.call('ai.complete', {
     prompt: COMMIT_MSG_PROMPT_TEMPLATE + diff,
     maxTokens: 200,
     accountId
   })
5. Return: "feat(auth): implement bcrypt 12-round password hashing\n\n..."
6. Fill commitMessageInput.value (editable by user)
```

---

## Push / Pull với Progress Stream

```typescript
async function* pushWithProgress(cwd: string, branch: string): AsyncGenerator<string> {
  // relay.callStream returns async iterator of output lines
  for await (const line of relay.callStream('git.push', {
    cwd, remote: 'origin', branch,
    verbose: true
  })) {
    yield line
    // "Counting objects: 5, done."
    // "Writing objects: 100% (5/5), 1.23 KiB | 1.23 MiB/s, done."
    // "To github.com:org/repo.git"
    // "   abc1234..def5678  feature/auth-bcrypt -> feature/auth-bcrypt"
  }
}
// UI: render each yielded line in progress panel
```

---

## Conflict Detection

```
After git.pull:
  IF output contains "CONFLICT":
    ├── relay.call('git.status') → files with 'U' (unmerged) status
    ├── Show conflict panel:
    │   "Merge conflicts in:"
    │   ├── src/auth/auth-manager.ts  [View conflict]
    │   └── src/auth/auth.test.ts     [View conflict]
    │
    ├── [View conflict] → open file with conflict markers:
    │   <<<<<<< HEAD
    │   return sha256(plain)
    │   =======
    │   return bcrypt.hash(plain, 12)
    │   >>>>>>> origin/main
    │
    ├── Options: [AI: Resolve conflict] → agent reads both sides + resolves
    │            [Open in terminal]
    │            [Abort merge]
    └── After resolution: relay.call('git.add') + commit
```

---

## PR Creation Flow

```typescript
async function createPullRequest(params: PRParams): Promise<PRResult> {
  // Method 1: GitHub CLI on dev server (Category A integration)
  if (hasGitHubCLI(devServerId)) {
    return relay.call('github.pr.create', {
      cwd: worktreePath,
      title: params.title,
      body: params.body,
      base: params.baseBranch,
      draft: params.draft,
      reviewers: params.reviewers,
      ghConfigDir: `/home/dev/.config/gh/${userId}/`,  // per-user isolation
    })
  }

  // Method 2: GitHub API via WebCredentialStore (Category B integration)
  const token = await WebCredentialStore.get(userId, 'github', 'api_token')
  return githubApi.createPR({ ...params, token })
}

// AI PR description generation:
const diff = await relay.call('git.diff', { cwd, base: params.baseBranch })
const commits = await relay.call('git.log', { cwd, base: params.baseBranch })
description = await ai.generate(PR_DESCRIPTION_PROMPT + diff + commits)
```

---

## Tiêu chí chấp nhận

- [ ] git status poll (5s) với porcelain-v2 parsing
- [ ] Visual diff: file-by-file, syntax highlighted, unified format
- [ ] Stage/Unstage individual files hoặc tất cả
- [ ] Discard changes (git restore) với confirm dialog
- [ ] Commit: manual message + AI generate
- [ ] Push: stream progress output
- [ ] Pull: detect conflicts → show conflict files
- [ ] AI conflict resolution: agent reads conflict markers + resolves
- [ ] Branch list (local + remote), create, checkout, delete
- [ ] Merge branch (no-ff)
- [ ] Stash push + pop
- [ ] Git log: last 50 commits, branch graph
- [ ] PR creation: GitHub CLI (Category A) hoặc API token (Category B)
- [ ] AI PR description generation từ diff + commits
- [ ] Worktree switcher: list + create + switch (via git.worktree.*)
