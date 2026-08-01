# F39 — Remote Git UI

| Trường | Giá trị |
|--------|---------|
| **ID** | F39 |
| **Tên** | Remote Git UI |
| **Ưu tiên** | P0 |
| **Trạng thái** | 🚧 Phát triển |
| **Phiên bản** | v5.0+ |
| **ADR References** | ADR-012 |
| **HLD References** | C3.12, C4.10 |

---

## Mô tả

Người dùng có thể thực hiện toàn bộ quy trình **git** trực tiếp trên **dev server** (trong thư mục repo của project) thông qua giao diện đồ họa — mà **không cần SSH terminal thủ công**. Bao gồm: xem status, visual diff, stage/unstage, commit, push, pull, branch management, merge, và tạo Pull Request.

---

## Vấn đề cần giải quyết

- Agent thực hiện code changes trên dev server → developer cần SSH vào server để commit/push thủ công
- Không có visual diff giữa worktrees trên server
- Không có branch management UI
- Developer phải nhớ cú pháp git, dễ sai lệnh khi thao tác từ xa

---

## Git Tab Layout

```
Git (vnp-blc-backend / dev-alpha.internal)
Worktree: [feature/auth-bcrypt ▼]  Branch: feature/auth-bcrypt → main

┌─── Changes ─────────────────────────────────┐
│ Modified (3):                                │
│  ● M  src/auth/auth-manager.ts    [Diff] [+]│
│  ● M  src/auth/auth.test.ts       [Diff] [+]│
│  ● A  src/auth/bcrypt-utils.ts    [Diff] [+]│
│                                             │
│ Staged (1):                                 │
│  ✓ M  package.json                      [-] │
│                                             │
│ [Stage All] [Unstage All]                   │
└─────────────────────────────────────────────┘

┌─── Commit ──────────────────────────────────┐
│ Message:                                    │
│ ┌─────────────────────────────────────────┐ │
│ │ feat(auth): implement bcrypt 12-round   │ │
│ │ password hashing                        │ │
│ └─────────────────────────────────────────┘ │
│ [🤖 AI: Generate message]                   │
│ [✅ Commit]  [✅ Commit & Push]              │
└─────────────────────────────────────────────┘

┌─── Sync ────────────────────────────────────┐
│ Remote: origin/feature/auth-bcrypt          │
│ ↑ 2 to push   ↓ 0 to pull                  │
│ [🔼 Push]  [🔽 Pull]  [🔄 Fetch]            │
└─────────────────────────────────────────────┘

┌─── Branches ────────────────────────────────┐
│ Local:  feature/auth-bcrypt (current) ★     │
│         main                                │
│         develop                             │
│ Remote: origin/main, origin/develop         │
│ [+ New Branch] [🔀 Merge into...] [🗑 Delete]│
└─────────────────────────────────────────────┘
```

---

## Visual Diff Viewer

```
Diff: src/auth/auth-manager.ts
┌─────────────────────────────────────────────────────────────────┐
│ ─ async hashPassword(plain: string): Promise<string> {          │
│ -   return createHash('sha256').update(plain).digest('hex')     │
│ +   const saltRounds = 12                                       │
│ +   return bcrypt.hash(plain, saltRounds)                       │
│   }                                                             │
│                                                                 │
│ ─ async verifyPassword(plain: string, hash: string): Promise<boolean> { │
│ -   return createHash('sha256').update(plain).digest('hex') === hash     │
│ +   return bcrypt.compare(plain, hash)                          │
│   }                                                             │
│                                                                 │
│ [Stage this file]  [Discard changes]  [Open in Explorer]        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Tính năng chi tiết

### Git Status (real-time)

```typescript
// Poll mỗi 5s khi Git tab active, hoặc sau mỗi agent completion
async function loadGitStatus(cwd: string): Promise<GitStatus> {
  const raw = await relay.call('git.status', { cwd, format: 'porcelain-v2' })
  return parseGitPorcelain(raw)
  // Returns: { modified, added, deleted, untracked, staged, ahead, behind }
}
```

### Stage / Unstage

```
[Stage file] → relay.call('git.add', { cwd, files: ['src/auth/auth-manager.ts'] })
[Unstage]    → relay.call('git.restore', { cwd, files, staged: true })
[Stage All]  → relay.call('git.add', { cwd, files: ['.'] })
```

### Commit

```
User nhập message → [Commit]
    │
    ├── relay.call('git.commit', { cwd, message, author: user.gitAuthor })
    │     gitAuthor = { name: user.name, email: user.email }  // from user profile
    │
    ├── On success: reload git status
    │
    └── Show: "✅ Committed: abc1234 — feat(auth): implement bcrypt..."
```

### AI Commit Message Generation

```
User → [🤖 AI: Generate message]
    │
    ├── relay.call('git.diff', { cwd, staged: true })  → diff text
    │
    ├── AI call: (provider from resolved profile)
    │   prompt = "Generate a conventional commit message for this diff:\n{{diff}}"
    │   model = profile.agent.preferredModel
    │
    └── Fill commit message textarea với generated text (editable)
```

### Push / Pull

```
[Push] → relay.call('git.push', { cwd, remote: 'origin', branch: currentBranch })
         Progress stream: "Counting objects... Compressing... Writing..."
         → relay streams push output line-by-line

[Pull] → relay.call('git.pull', { cwd, remote: 'origin', branch, rebase: false })
         Conflict detection: nếu MERGE_CONFLICT → show conflict files
```

### Branch Management

```
[+ New Branch] → input name → relay.call('git.branch.create', { cwd, name, from: 'main' })
                           → relay.call('git.checkout', { cwd, branch: name })

[🔀 Merge into main] → relay.call('git.merge', {
  cwd, sourceBranch: current, targetBranch: 'main', noFF: true
})

[🗑 Delete] → confirm → relay.call('git.branch.delete', { cwd, branch, force: false })
```

### Create Pull Request (GitHub/GitLab)

```
User → [Create PR]
    │
    ├── Preflight: check pushed commits, check token (WebCredentialStore)
    │
    ├── PR Form:
    │   Title: [AI-generated or manual]
    │   Base branch: [main ▼]
    │   Description: [AI-generated from commits + diff]
    │   Reviewers: [@lead.dev]  (from project settings)
    │   Draft: [ ]
    │
    ├── relay.call('github.createPR', { ... }) — via CLI token on dev server
    │   OR WebCredentialStore token → direct GitHub API
    │
    └── Show: "✅ PR #42 created — https://github.com/org/repo/pull/42"
              [Open in browser]  [Copy link]
```

### Worktree Switcher (per project)

```
Worktree: [feature/auth-bcrypt ▼]
    │
    ├── List: all worktrees of this project on dev server
    │   relay.call('git.worktree.list', { repoPath })
    │
    ├── Switch → update all Git tab data for selected worktree path
    │
    └── [+ New Worktree]: branch name → create + switch
```

---

## Luồng người dùng tích hợp

```
1. Agent hoàn thành tác vụ bcrypt
   → Agent tab: "✅ Done — 3 files modified"
   → Click [→ Go to Git]

2. Git tab tự động refresh:
   → 3 files modified hiện ra (auth-manager.ts, auth.test.ts, bcrypt-utils.ts)

3. User click [Diff] từng file → review changes trong visual diff

4. Click [Stage All] → all files staged

5. Click [🤖 AI: Generate message]
   → AI generates: "feat(auth): implement bcrypt 12-round password hashing\n\nReplace SHA-256 with bcrypt for secure password storage.\nAll unit tests updated."

6. User review/edit message → [Commit & Push]

7. Click [Create PR] → PR form pre-filled → submit
   → PR #42 created ✅

8. Task linked to this worktree → auto-advance task status to 'review'
```

---

## Tiêu chí chấp nhận

- [ ] Git status: modified/added/deleted/untracked/staged list (refresh mỗi 5s)
- [ ] Visual diff viewer (unified diff, syntax highlighted)
- [ ] Stage / Unstage file hoặc tất cả
- [ ] Discard changes (git restore)
- [ ] Commit với manual message
- [ ] AI commit message generation từ staged diff
- [ ] Push + pull với progress stream
- [ ] Conflict detection sau pull → list conflict files
- [ ] Branch list (local + remote)
- [ ] New branch (create + checkout)
- [ ] Merge branch với no-ff
- [ ] Delete branch (local)
- [ ] Worktree switcher per project (list + switch + create)
- [ ] Create Pull Request UI (GitHub/GitLab)
- [ ] AI PR description generation
- [ ] Git log (last 20 commits) với branch graph
- [ ] Async push/pull progress stream → real-time UI update

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Git panel | `src/renderer/src/components/workspace/GitPanel.tsx` |
| Diff viewer | `src/renderer/src/components/workspace/DiffViewer.tsx` |
| Branch manager | `src/renderer/src/components/workspace/BranchManager.tsx` |
| Commit form | `src/renderer/src/components/workspace/CommitForm.tsx` |
| PR form | `src/renderer/src/components/workspace/PullRequestForm.tsx` |
| Git log | `src/renderer/src/components/workspace/GitLog.tsx` |
| Git RPC methods | `src/main/runtime/rpc/methods/git.ts` (extended) |
| Relay git commands | `src/main/ssh/relay-git-bridge.ts` |
