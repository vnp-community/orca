# TASK-V5-12: GitPanel + StagingArea + CommitForm

**Order:** 12  
**Prerequisite:** TASK-V5-11 (useGit, git slice)  
**Solution Ref:** SOL-FE-V5-06 (section 2, 5)  
**Est. effort:** ~60 min | **Tests:** 16

---

## Files Cần Tạo

### 1. `src/renderer/src/components/workspace/git/StagingArea.tsx`

```typescript
import { useGit } from '../../../hooks/useGit'
import { Button } from '../../ui/button'
import { ChevronDown, ChevronRight, Plus, Minus, Eye } from 'lucide-react'
import { useState } from 'react'
import type { GitFileChange } from '../../../store/slices/git-panel'
import { cn } from '../../../utils'

const STATUS_LABELS: Record<string, string> = {
  M: 'Modified', A: 'Added', D: 'Deleted', R: 'Renamed', U: 'Untracked'
}

interface FileRowProps {
  file:       GitFileChange
  actionIcon: ReactNode
  onAction:   (path: string) => void
  onViewDiff: (path: string) => void
}

function FileRow({ file, actionIcon, onAction, onViewDiff }: FileRowProps) {
  return (
    <div
      className="flex items-center gap-1 px-2 py-0.5 text-sm hover:bg-accent/50 rounded-sm group"
      data-testid={`file-row-${file.path}`}
    >
      <span className="text-xs font-mono text-muted-foreground w-4 shrink-0">
        {file.status}
      </span>
      <span className="truncate flex-1">{file.path}</span>
      <div className="hidden group-hover:flex gap-1 shrink-0">
        <Button size="icon" variant="ghost" className="h-5 w-5" onClick={() => onViewDiff(file.path)}>
          <Eye size={10} />
        </Button>
        <Button
          size="icon"
          variant="ghost"
          className="h-5 w-5"
          onClick={() => onAction(file.path)}
          data-testid={`action-btn-${file.path}`}
        >
          {actionIcon}
        </Button>
      </div>
    </div>
  )
}

export function StagingArea({ onViewDiff }: { onViewDiff: (path: string) => void }) {
  const { stagedFiles, unstagedFiles, stageFile, unstageFile, stageAll, unstageAll } = useGit()
  const [stagedOpen,   setStagedOpen]   = useState(true)
  const [unstagedOpen, setUnstagedOpen] = useState(true)

  return (
    <div className="staging-area" data-testid="staging-area">
      {/* Staged */}
      <div className="staged-section">
        <div
          className="flex items-center justify-between px-2 py-1 cursor-pointer hover:bg-accent/30"
          onClick={() => setStagedOpen(v => !v)}
        >
          <div className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
            {stagedOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            Staged ({stagedFiles.length})
          </div>
          {stagedFiles.length > 0 && (
            <Button size="sm" variant="ghost" className="h-5 text-xs" onClick={e => { e.stopPropagation(); unstageAll() }}>
              Unstage All
            </Button>
          )}
        </div>
        {stagedOpen && stagedFiles.map(f => (
          <FileRow
            key={f.path}
            file={f}
            actionIcon={<Minus size={10} />}
            onAction={unstageFile}
            onViewDiff={onViewDiff}
          />
        ))}
      </div>

      {/* Unstaged */}
      <div className="unstaged-section mt-2">
        <div
          className="flex items-center justify-between px-2 py-1 cursor-pointer hover:bg-accent/30"
          onClick={() => setUnstagedOpen(v => !v)}
        >
          <div className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
            {unstagedOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            Unstaged ({unstagedFiles.length})
          </div>
          {unstagedFiles.length > 0 && (
            <Button size="sm" variant="ghost" className="h-5 text-xs" onClick={e => { e.stopPropagation(); stageAll() }}>
              Stage All
            </Button>
          )}
        </div>
        {unstagedOpen && unstagedFiles.map(f => (
          <FileRow
            key={f.path}
            file={f}
            actionIcon={<Plus size={10} />}
            onAction={stageFile}
            onViewDiff={onViewDiff}
          />
        ))}
      </div>
    </div>
  )
}
```

### 2. `src/renderer/src/components/workspace/git/CommitForm.tsx`

```typescript
import { useState } from 'react'
import { useGit } from '../../../hooks/useGit'
import { Button } from '../../ui/button'
import { Textarea } from '../../ui/textarea'
import { Bot, Loader2 } from 'lucide-react'

export function CommitForm() {
  const { commit, push, aiCommitMessage, stagedFiles, isPushing, isCommitting } = useGit()
  const [message, setMessage]   = useState('')
  const [aiLoading, setAiLoad]  = useState(false)

  const canCommit = message.trim().length > 0 && stagedFiles.length > 0

  const handleCommit = async () => {
    if (!canCommit) return
    await commit(message.trim())
    setMessage('')
  }

  const handleCommitAndPush = async () => {
    if (!canCommit) return
    await commit(message.trim())
    setMessage('')
    await push('HEAD')
  }

  const fillAIMessage = async () => {
    setAiLoad(true)
    try {
      const msg = await aiCommitMessage()
      setMessage(msg)
    } finally {
      setAiLoad(false)
    }
  }

  return (
    <div className="commit-form p-2 space-y-2 border-t" data-testid="commit-form">
      <div className="flex items-center gap-1">
        <Textarea
          value={message}
          onChange={e => setMessage(e.target.value)}
          placeholder="Commit message..."
          rows={2}
          className="resize-none text-sm flex-1"
          data-testid="commit-message-input"
        />
        <Button
          size="icon"
          variant="ghost"
          onClick={fillAIMessage}
          disabled={aiLoading}
          title="Generate commit message with AI"
          data-testid="ai-commit-btn"
        >
          {aiLoading ? <Loader2 size={14} className="animate-spin" /> : <Bot size={14} />}
        </Button>
      </div>

      <div className="flex gap-2">
        <Button
          size="sm"
          onClick={handleCommit}
          disabled={!canCommit || isCommitting}
          className="flex-1"
          data-testid="commit-btn"
        >
          {isCommitting ? <Loader2 size={12} className="animate-spin mr-1" /> : null}
          Commit
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={handleCommitAndPush}
          disabled={!canCommit || isCommitting || isPushing}
          className="flex-1"
          data-testid="commit-push-btn"
        >
          Commit & Push
        </Button>
      </div>
    </div>
  )
}
```

### 3. `src/renderer/src/components/workspace/git/GitHistory.tsx`

```typescript
import { useEffect } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useAppStore } from '../../../store'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import type { GitCommit } from '../../../store/slices/git-panel'

export function GitHistory() {
  const { project } = useWorkspace()
  const gitHistory  = useAppStore(s => s.gitHistory)

  useEffect(() => {
    if (!project) return
    callRuntimeRpc('git.getLog', { projectId: project.id, limit: 50 })
      .then(commits => {
        useAppStore.getState().setGitHistory(commits as GitCommit[])
      })
  }, [project])

  return (
    <div className="git-history p-2" data-testid="git-history">
      {gitHistory.map(commit => (
        <div key={commit.hash} className="py-2 border-b last:border-0">
          <div className="flex items-baseline gap-2">
            <code className="text-xs text-muted-foreground font-mono">{commit.shortHash}</code>
            <span className="text-sm truncate">{commit.message}</span>
          </div>
          <div className="text-xs text-muted-foreground mt-0.5">
            {commit.author} · {new Date(commit.date).toLocaleDateString()}
          </div>
        </div>
      ))}
      {gitHistory.length === 0 && (
        <div className="text-sm text-muted-foreground py-4 text-center">No commits yet</div>
      )}
    </div>
  )
}
```

### 4. `src/renderer/src/components/workspace/git/GitPanel.tsx`

```typescript
import { useState } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useGit } from '../../../hooks/useGit'
import { StagingArea } from './StagingArea'
import { CommitForm } from './CommitForm'
import { lazy, Suspense } from 'react'

const GitHistory    = lazy(() => import('./GitHistory').then(m => ({ default: m.GitHistory })))
const BranchManager = lazy(() => import('./BranchManager').then(m => ({ default: m.BranchManager })))
const DiffViewer    = lazy(() => import('./DiffViewer').then(m => ({ default: m.DiffViewer })))

type GitTab = 'changes' | 'history' | 'branches'

export function GitPanel() {
  const { gitStatus } = useWorkspace()
  const { getDiff }   = useGit()
  const [activeTab, setActiveTab] = useState<GitTab>('changes')
  const [selectedDiff, setSelectedDiff] = useState<string | null>(null)

  const handleViewDiff = async (path: string) => {
    setSelectedDiff(path)
    await getDiff(path)
  }

  return (
    <div className="git-panel flex flex-col h-full" data-testid="git-panel">
      {/* Tab bar */}
      <div className="flex border-b text-sm">
        {(['changes', 'history', 'branches'] as GitTab[]).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            data-testid={`git-tab-${tab}`}
            className={`px-4 py-2 capitalize border-b-2 ${
              activeTab === tab ? 'border-primary text-primary' : 'border-transparent text-muted-foreground'
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        <Suspense fallback={<div className="p-4 text-sm text-muted-foreground">Loading...</div>}>
          {activeTab === 'changes' && (
            <div>
              <StagingArea onViewDiff={handleViewDiff} />
              <CommitForm />
              {selectedDiff && (
                <div className="border-t h-64">
                  <DiffViewer filePath={selectedDiff} />
                </div>
              )}
            </div>
          )}
          {activeTab === 'history' && <GitHistory />}
          {activeTab === 'branches' && <BranchManager />}
        </Suspense>
      </div>
    </div>
  )
}
```

---

## Tests

```
__tests__/workspace/git/GitPanel.test.tsx        (5 tests)
__tests__/workspace/git/StagingArea.test.tsx     (6 tests)
__tests__/workspace/git/CommitForm.test.tsx      (5 tests)
```

Xem SOL-FE-V5-06 section 9 cho full test list.

---

## Acceptance Criteria

- [x] `GitPanel` renders Changes tab by default
- [x] Tab switching: changes/history/branches
- [x] `StagingArea` shows staged/unstaged files from store
- [x] Stage/Unstage buttons call correct hooks
- [x] `CommitForm` submit disabled when message empty or no staged files
- [x] AI button calls `aiCommitMessage` and sets textarea value
- [x] "Commit & Push" calls commit then push
- [x] 16/16 tests pass
