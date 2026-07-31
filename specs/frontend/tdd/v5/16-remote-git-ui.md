# TDD-FE-16: Remote Git UI

**Document:** TDD-FE-16 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-28
**Domain:** Git UI — GitPanel, DiffViewer, CommitForm, BranchManager, PullRequestForm
**Feature:** F39
**ADR:** ADR-012
**HLD Ref:** C3.12, C4.10
**Backend TDD:** TDD-20
**Source files (to create):**
- `src/renderer/src/components/workspace/git/GitPanel.tsx`
- `src/renderer/src/components/workspace/git/DiffViewer.tsx`
- `src/renderer/src/components/workspace/git/CommitForm.tsx`
- `src/renderer/src/components/workspace/git/BranchManager.tsx`
- `src/renderer/src/components/workspace/git/PullRequestForm.tsx`
- `src/renderer/src/components/workspace/git/StagingArea.tsx`
- `src/renderer/src/hooks/useGit.ts`

> **Status: ❌ TODO** — v5.0 proposed

---

## 1. GitPanel Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Git                                      Branch: main [▼] [⚡ Sync]      │
│                                           ↑ 2 commits  ↓ 0 commits       │
├──────────────────────────────────────────────────────────────────────────┤
│ TABS: [Changes (4)] [History] [Branches] [Pull Requests]                  │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Changes Tab:                                                            │
│  ─────────────────────────────────────────────────────────────────────  │
│  ▼ Staged (2)                        [Unstage All]                       │
│    ✓ M  src/auth/jwt.ts              [Unstage] [View Diff]               │
│    ✓ A  src/auth/middleware.ts       [Unstage] [View Diff]               │
│                                                                          │
│  ▼ Unstaged (2)                      [Stage All]                         │
│    ○ M  src/auth/types.ts            [Stage]   [View Diff]               │
│    ○ D  src/auth/old-auth.ts         [Stage]   [View Diff] [Restore]     │
│                                                                          │
│  ─────────────────────────────────────────────────────────────────────  │
│  Commit message                                                          │
│  [feat(auth): implement JWT middleware              ] [🤖 AI]             │
│                                                                          │
│  [Commit]  [Commit & Push]                                               │
│                                                                          │
│  ─────────────────────────────────────────────────────────────────────  │
│  DIFF VIEWER (selected file):                                            │
│  [DiffViewer: jwt.ts]                                                    │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 2. GitPanel Component

```typescript
// src/renderer/src/components/workspace/git/GitPanel.tsx

type GitTab = 'changes' | 'history' | 'branches' | 'pullrequests'

export function GitPanel() {
  const { project, gitStatus, refreshGitStatus, currentWorktree, emit } = useWorkspace()
  const { stagedFiles, unstagedFiles, stageAll, unstageAll, stageFile, unstageFile } = useGit()
  const [activeTab, setActiveTab] = useState<GitTab>('changes')
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [isPushing, setIsPushing] = useState(false)
  const [pushLines, setPushLines] = useState<string[]>([])

  const handlePush = async () => {
    if (!project || !currentWorktree) return
    setIsPushing(true)
    setPushLines([])
    try {
      // Streaming push
      const stream = rpc.callStream('git.push', {
        projectId: project.id,
        worktreePath: currentWorktree.path,
        remote: 'origin',
        branch: gitStatus?.branch ?? 'main',
      })
      for await (const line of stream) {
        setPushLines(prev => [...prev, line])
      }
      await refreshGitStatus()
      emit({ type: 'git.push', branch: gitStatus?.branch ?? 'main', remote: 'origin' })
      toast.success('Push complete')
    } catch (err) {
      toast.error('Push failed: ' + (err as Error).message)
    } finally {
      setIsPushing(false)
    }
  }

  return (
    <div className="git-panel flex flex-col h-full">
      <GitPanelHeader
        gitStatus={gitStatus}
        onSync={handlePush}
        isSyncing={isPushing}
      />

      <Tabs value={activeTab} onValueChange={setActiveTab as any}>
        <TabsList>
          <TabsTrigger value="changes">
            Changes {(stagedFiles.length + unstagedFiles.length) > 0 && (
              <Badge className="ml-1">{stagedFiles.length + unstagedFiles.length}</Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="history">History</TabsTrigger>
          <TabsTrigger value="branches">Branches</TabsTrigger>
          <TabsTrigger value="pullrequests">Pull Requests</TabsTrigger>
        </TabsList>

        <TabsContent value="changes" className="flex-1 overflow-y-auto">
          <StagingArea
            staged={stagedFiles}
            unstaged={unstagedFiles}
            onStageFile={stageFile}
            onUnstageFile={unstageFile}
            onStageAll={stageAll}
            onUnstageAll={unstageAll}
            onSelectFile={setSelectedFile}
          />
          <CommitForm onCommitted={() => { refreshGitStatus(); setSelectedFile(null) }} />
          {selectedFile && <DiffViewer filePath={selectedFile} worktreePath={currentWorktree?.path} />}
        </TabsContent>

        <TabsContent value="history">
          <GitLog projectId={project?.id} worktreePath={currentWorktree?.path} />
        </TabsContent>

        <TabsContent value="branches">
          <BranchManager projectId={project?.id} worktreePath={currentWorktree?.path} />
        </TabsContent>

        <TabsContent value="pullrequests">
          <PullRequestList projectId={project?.id} />
        </TabsContent>
      </Tabs>

      {isPushing && (
        <div className="push-progress p-3 bg-muted rounded-b border-t">
          <p className="text-xs font-medium mb-1">Pushing...</p>
          <pre className="text-xs font-mono overflow-auto max-h-20">
            {pushLines.join('\n')}
          </pre>
        </div>
      )}
    </div>
  )
}
```

---

## 3. DiffViewer — Monaco

```typescript
// src/renderer/src/components/workspace/git/DiffViewer.tsx
// Uses Monaco Editor in diff mode (read-only)

interface DiffViewerProps {
  filePath: string
  worktreePath?: string
  staged?: boolean   // true = diff HEAD vs staged; false = diff staged vs working
}

export function DiffViewer({ filePath, worktreePath, staged = false }: DiffViewerProps) {
  const { project } = useWorkspace()
  const [originalContent, setOriginalContent] = useState('')
  const [modifiedContent, setModifiedContent] = useState('')
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    if (!project || !worktreePath) return
    setIsLoading(true)

    Promise.all([
      // Original: HEAD version
      rpc.call('git.exec', {
        projectId: project.id,
        worktreePath,
        args: ['show', `HEAD:${filePath}`],
      }).then(r => (r as any).stdout).catch(() => ''),

      // Modified: current working tree version
      rpc.call('fs.readFile', {
        projectId: project.id,
        path: `${worktreePath}/${filePath}`,
        encoding: 'utf-8',
      }).then(r => (r as any).content).catch(() => ''),
    ]).then(([original, modified]) => {
      setOriginalContent(original)
      setModifiedContent(modified)
    }).finally(() => setIsLoading(false))
  }, [filePath, worktreePath, project])

  const language = detectLanguage(filePath)

  if (isLoading) return <Skeleton className="h-40" />

  return (
    <div className="diff-viewer border rounded overflow-hidden" style={{ height: 400 }}>
      <div className="diff-viewer-header flex items-center gap-2 px-3 py-1 bg-muted border-b text-xs">
        <FileCode size={12} />
        <span className="font-mono">{filePath}</span>
        <Badge variant="outline" className="ml-auto">{language}</Badge>
      </div>
      <MonacoDiffEditor
        original={originalContent}
        modified={modifiedContent}
        language={language}
        options={{
          readOnly: true,
          renderSideBySide: true,
          minimap: { enabled: false },
          fontSize: 12,
          scrollBeyondLastLine: false,
        }}
        theme="vs-dark"
        height={360}
      />
    </div>
  )
}
```

---

## 4. CommitForm with AI Assist

```typescript
// src/renderer/src/components/workspace/git/CommitForm.tsx

export function CommitForm({ onCommitted }: { onCommitted: () => void }) {
  const { project, currentWorktree, emit } = useWorkspace()
  const [message, setMessage] = useState('')
  const [isGenerating, setIsGenerating] = useState(false)
  const [isCommitting, setIsCommitting] = useState(false)

  const generateAIMessage = async () => {
    if (!project || !currentWorktree) return
    setIsGenerating(true)
    try {
      const result = await rpc.call('git.generateCommitMessage', {
        projectId: project.id,
        worktreePath: currentWorktree.path,
      }) as string
      setMessage(result)
    } catch (err: any) {
      if (err.code === 'GIT_NO_STAGED_CHANGES') {
        toast.error('No staged changes — stage files first')
      } else {
        toast.error('Failed to generate message')
      }
    } finally {
      setIsGenerating(false)
    }
  }

  const commit = async (push = false) => {
    if (!message.trim()) { toast.error('Enter a commit message'); return }
    if (!project || !currentWorktree) return
    setIsCommitting(true)
    try {
      await rpc.call('git.commit', {
        projectId: project.id,
        worktreePath: currentWorktree.path,
        message: message.trim(),
      })
      emit({ type: 'git.commit', hash: '', message: message.trim(), branch: '' })
      setMessage('')
      onCommitted()
      if (push) {
        // Fire push (streaming) separately
        toast.info('Pushing...')
      }
      toast.success('Committed')
    } finally {
      setIsCommitting(false)
    }
  }

  return (
    <div className="commit-form p-3 border-t space-y-2">
      <div className="relative">
        <Textarea
          value={message}
          onChange={e => setMessage(e.target.value)}
          placeholder="Commit message (e.g. feat(auth): add JWT middleware)"
          className="pr-8 resize-none text-sm font-mono"
          rows={3}
          maxLength={500}
        />
        <Button
          variant="ghost"
          size="icon"
          className="absolute right-1 top-1 h-6 w-6"
          onClick={generateAIMessage}
          disabled={isGenerating}
          title="Generate commit message with AI"
        >
          {isGenerating ? <Loader2 size={12} className="animate-spin" /> : <Sparkles size={12} />}
        </Button>
      </div>
      <div className="flex gap-2">
        <Button
          className="flex-1"
          onClick={() => commit(false)}
          disabled={isCommitting || !message.trim()}
        >
          {isCommitting ? <Loader2 className="animate-spin" /> : null}
          Commit
        </Button>
        <Button
          variant="outline"
          onClick={() => commit(true)}
          disabled={isCommitting || !message.trim()}
        >
          Commit & Push
        </Button>
      </div>
    </div>
  )
}
```

---

## 5. BranchManager

```typescript
// src/renderer/src/components/workspace/git/BranchManager.tsx

export function BranchManager({ projectId, worktreePath }: BranchManagerProps) {
  const [branches, setBranches] = useState<GitBranch[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [newBranchName, setNewBranchName] = useState('')

  useEffect(() => {
    rpc.call('git.branch.list', { projectId, worktreePath })
      .then(r => setBranches((r as any).branches))
      .finally(() => setIsLoading(false))
  }, [projectId, worktreePath])

  const checkout = async (branch: string) => {
    await rpc.call('git.checkout', { projectId, worktreePath, branch })
    toast.success(`Switched to ${branch}`)
  }

  const createBranch = async () => {
    await rpc.call('git.branch.create', { projectId, worktreePath, name: newBranchName })
    await checkout(newBranchName)
    setNewBranchName('')
  }

  // Layout: current branch highlighted, remote/local sections
  return (
    <div className="branch-manager p-3 space-y-3">
      <div className="flex gap-2">
        <Input
          value={newBranchName}
          onChange={e => setNewBranchName(e.target.value)}
          placeholder="New branch name..."
          className="text-sm"
        />
        <Button size="sm" onClick={createBranch} disabled={!newBranchName.trim()}>
          <GitBranch size={14} className="mr-1" /> Create
        </Button>
      </div>
      <BranchList branches={branches} onCheckout={checkout} isLoading={isLoading} />
    </div>
  )
}
```

---

## 6. useGit Hook

```typescript
// src/renderer/src/hooks/useGit.ts

export function useGit() {
  const { project, currentWorktree, gitStatus, refreshGitStatus } = useWorkspace()

  const parseStatus = (status: GitStatus) => ({
    stagedFiles: status.files.filter(f => f.staged),
    unstagedFiles: status.files.filter(f => !f.staged),
  })

  const { stagedFiles, unstagedFiles } = parseStatus(gitStatus ?? { files: [] } as any)

  const stageFile = useCallback(async (filePath: string) => {
    await rpc.call('git.add', { projectId: project!.id, worktreePath: currentWorktree!.path, files: [filePath] })
    await refreshGitStatus()
  }, [project, currentWorktree, refreshGitStatus])

  const unstageFile = useCallback(async (filePath: string) => {
    await rpc.call('git.restore', { projectId: project!.id, worktreePath: currentWorktree!.path, files: [filePath], staged: true })
    await refreshGitStatus()
  }, [project, currentWorktree, refreshGitStatus])

  const stageAll = useCallback(async () => {
    await rpc.call('git.add', { projectId: project!.id, worktreePath: currentWorktree!.path, files: ['.'] })
    await refreshGitStatus()
  }, [project, currentWorktree, refreshGitStatus])

  const unstageAll = useCallback(async () => {
    await rpc.call('git.restore', { projectId: project!.id, worktreePath: currentWorktree!.path, files: ['.'], staged: true })
    await refreshGitStatus()
  }, [project, currentWorktree, refreshGitStatus])

  return { stagedFiles, unstagedFiles, stageFile, unstageFile, stageAll, unstageAll }
}
```

---

## 7. Test Coverage

```
src/renderer/src/components/workspace/git/__tests__/
├── GitPanel.test.tsx
│   ├── renders branch name from gitStatus
│   ├── push button calls git.push rpc
│   ├── push streaming lines displayed in progress area
│   └── after push: refreshGitStatus and emit('git.push')
├── DiffViewer.test.tsx
│   ├── loads HEAD and working tree content
│   ├── isLoading shows skeleton
│   └── detects language from file extension
├── CommitForm.test.tsx
│   ├── commits with message → git.commit called
│   ├── empty message → error toast (no commit)
│   ├── AI button calls generateCommitMessage
│   ├── GIT_NO_STAGED_CHANGES → stage error toast
│   ├── after commit: message cleared, onCommitted called
│   └── emit('git.commit') fired
├── BranchManager.test.tsx
│   ├── lists branches on mount
│   ├── createBranch → git.branch.create + checkout
│   └── checkout → git.checkout called
└── hooks/__tests__/useGit.test.ts
    ├── stageFile → git.add + refreshGitStatus
    ├── unstageFile → git.restore --staged + refreshGitStatus
    └── stageAll passes '.' as files arg
```

**Target:** ≥ 30 tests

---

## Addendum: HLD Cross-References (v5.0 — 2026-07-30)

> **Nguồn:** [HLD C3.12](../../../docs/hld/v1/C3-components.md), [HLD C4.10](../../../docs/hld/v1/C4-code.md), [web-server-architecture.md §10.7](../../../docs/hld/web-server-architecture.md)

### git.* — Exact Backend Commands (từ HLD C4.10)

Backend relay đến Dev Server, execute các lệnh sau:

```typescript
'git.status'      → exec('git status --porcelain=v2 --branch', { cwd })
'git.diff'        → exec('git diff [--staged] [--] [file]', { cwd })
'git.add'         → exec('git add <files>', { cwd })
'git.restore'     → exec('git restore [--staged] <files>', { cwd })
'git.commit'      → exec(`git commit -m "${msg}" --author="${name} <${email}>"`, { cwd })
'git.push'        → execStream('git push origin <branch>', { cwd })
                    // streams: { type: 'stdout'|'stderr', data: string }
                    //          { type: 'progress', pct: 0..100 }
                    //          { type: 'done', exitCode: 0 }
'git.pull'        → execStream('git pull origin <branch>', { cwd })
'git.fetch'       → exec('git fetch --all --prune', { cwd })
'git.branch.list' → exec('git branch -a -vv --format=%(refname:short)..%(upstream:short)..%(push:short)', { cwd })
'git.branch.create' → exec('git checkout -b <name> [from]', { cwd })
'git.branch.delete' → exec('git branch -d <name>', { cwd })
'git.merge'       → exec('git merge --no-ff <branch>', { cwd })
'git.stash'       → exec('git stash push [-m <msg>]', { cwd })
'git.stash.pop'   → exec('git stash pop', { cwd })
'git.log'         → exec('git log --oneline --graph --decorate --abbrev-commit -50', { cwd })
```

### git.push Stream Pattern (Frontend)

```typescript
// hooks/useGit.ts — streaming push/pull
async function* streamPush(cwd: string, branch: string) {
  const stream = rpc.callStream('git.push', { cwd, remote: 'origin', branch })
  for await (const chunk of stream) {
    yield chunk
    // chunk types: 'stdout' | 'stderr' | 'progress' | 'done'
  }
}

// SyncPanel component usage:
// const progress = useRef(0)
// for await (const chunk of streamPush(cwd, branch)) {
//   if (chunk.type === 'progress') setProgress(chunk.pct)
//   if (chunk.type === 'stdout')  appendOutput(chunk.data)
//   if (chunk.type === 'done')    setDone(chunk.exitCode === 0)
// }
```

### AI Commit Message — Flow (từ HLD)

```typescript
// CommitForm: click 🤖 AI button
// 1. RPC: git.diff({ cwd, staged: true }) → diff string
// 2. RPC: ai.complete({ prompt: buildCommitPrompt(diff) })
//         → LLM on Dev Server (via relay)
//         → stream back tokens → progressively update textarea
// 3. User can edit generated message before committing
//    Format convention: 'type(scope): description'
```

### Pull Request via gh CLI (từ HLD)

```
PullRequestForm → click "Create PR"
    │
    ├── RPC: git.pr.create({ title, body, base, draft?, reviewers? })
    │
    ├── Backend relay exec:
    │     GH_CONFIG_DIR=~/.config/gh/<userId>/
    │     gh pr create --title "<title>" --body "<body>" --base <base>
    │         [--draft] [--reviewer <reviewer>,...]
    │
    └── Response: { url, number, status }
        → Frontend: open PR URL in BrowserPane hoặc copy to clipboard
```

### GitStatus — Data Shape (từ HLD)

```typescript
interface GitStatus {
  branch: string           // current branch
  upstream?: string        // tracking branch
  ahead: number            // commits ahead of upstream
  behind: number           // commits behind upstream
  staged: StatusEntry[]
  unstaged: StatusEntry[]
  untracked: string[]
  conflicted: string[]
}

interface StatusEntry {
  path: string
  status: 'M' | 'A' | 'D' | 'R' | 'C' | '?'  // git porcelain codes
  oldPath?: string  // for rename (R)
}
```
