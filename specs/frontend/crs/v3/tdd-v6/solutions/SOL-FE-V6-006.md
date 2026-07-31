# SOL-FE-V6-006: Remote Git UI (TDD-FE-16)

**Solution ID:** SOL-FE-V6-006
**TDD Ref:** [TDD-FE-16](../../../../tdd/v5/16-remote-git-ui.md)
**Feature:** F39 | **ADR:** ADR-012 | **HLD Ref:** C3.12, C4.10
**Date:** 2026-07-30
**Status:** ✅ COMPLETED — 2026-07-30

---

## 1. Phan tich code hien co

### 1.1 Da ton tai (KHONG viet lai)

| File | Size | Nhan xet |
|------|------|---------|
| `components/workspace/git/GitPanel.tsx` | 2258 bytes (64 lines) | SKELETON — thieu PullRequest tab + streaming push |
| `components/workspace/git/StagingArea.tsx` | 3831 bytes | Co san — day du theo TDD |
| `components/workspace/git/CommitForm.tsx` | 2376 bytes | Co san — co AI assist |
| `components/workspace/git/BranchManager.tsx` | 2517 bytes | Co san — day du |
| `components/workspace/git/GitHistory.tsx` | 1394 bytes | Co san |
| `components/workspace/git/PullRequestForm.tsx` | 8662 bytes | Co san — day du |
| `hooks/useGit.ts` | 3767 bytes | Co san — day du |

### 1.2 Stub can hoan thien

| File | Size | Van de |
|------|------|-------|
| `components/workspace/git/DiffViewer.tsx` | 1436 bytes | STUB — chua co Monaco editor |

### 1.3 Chua ton tai (CAN TAO MOI)

| File | TDD Ref | Do uu tien |
|------|---------|-----------|
| `components/workspace/git/PullRequestList.tsx` | Section 2 (tab PR) | HIGH |
| Test files trong `__tests__/` | Section 7 | HIGH |

---

## 2. Dependency Required: @monaco-editor/react

**TRUOC KHI implement DiffViewer, can install:**

```bash
# Kiem tra xem da co chua
ls node_modules/@monaco-editor 2>/dev/null && echo "INSTALLED" || echo "NOT INSTALLED"

# Neu chua co:
npm install @monaco-editor/react
# hoac
pnpm add @monaco-editor/react
```

**Tai sao can @monaco-editor/react:**
- `DiffViewer` su dung Monaco Diff Editor (side-by-side diff)
- `FileViewer` (TDD-FE-17) cung su dung Monaco Editor (read-only)
- Monaco la editor chinh cua VS Code — rich syntax highlighting

---

## 3. Giai phap — DiffViewer Full Implementation

**REPLACE stub voi full implementation:**

```typescript
// MODIFY: src/renderer/src/components/workspace/git/DiffViewer.tsx
// Hien la stub 1436 bytes — can integrate Monaco Diff Editor

import { useEffect, useState } from 'react'
import { DiffEditor } from '@monaco-editor/react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { useAppStore } from '@/store'
import { Skeleton } from '@/components/ui/skeleton'
import { FileCode } from 'lucide-react'
import { Badge } from '@/components/ui/badge'

interface DiffViewerProps {
  filePath: string
  worktreePath?: string
  staged?: boolean
}

const LANGUAGE_MAP: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript',
  js: 'javascript', jsx: 'javascript',
  py: 'python', go: 'go', rs: 'rust',
  css: 'css', scss: 'scss',
  json: 'json', yaml: 'yaml', yml: 'yaml',
  md: 'markdown', html: 'html',
  sh: 'shell', bash: 'shell',
}

function detectLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_MAP[ext] ?? 'plaintext'
}

export function DiffViewer({ filePath, worktreePath, staged = false }: DiffViewerProps) {
  const { project } = useWorkspace()
  const [originalContent, setOriginalContent] = useState('')
  const [modifiedContent, setModifiedContent] = useState('')
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    if (!project || !filePath) return
    setIsLoading(true)

    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const cwd = worktreePath ?? '.'

    Promise.all([
      // Original: HEAD version
      callRuntimeRpc<{ stdout: string }>(target, 'git.exec', {
        projectId: project.id,
        cwd,
        args: ['show', `HEAD:${filePath}`],
      }).then(r => r.stdout).catch(() => ''),
      // Modified: current working tree
      callRuntimeRpc<{ content: string }>(target, 'fs.readFile', {
        projectId: project.id,
        path: filePath,
        encoding: 'utf-8',
      }).then(r => r.content).catch(() => ''),
    ]).then(([original, modified]) => {
      setOriginalContent(original)
      setModifiedContent(modified)
    }).finally(() => setIsLoading(false))
  }, [filePath, worktreePath, project, staged])

  const language = detectLanguage(filePath)

  if (isLoading) return <Skeleton className="h-40 rounded-none" />

  return (
    <div className="diff-viewer border-t" data-testid="diff-viewer">
      <div className="diff-viewer-header flex items-center gap-2 px-3 py-1 bg-muted border-b text-xs">
        <FileCode size={12} />
        <span className="font-mono flex-1 truncate">{filePath}</span>
        <Badge variant="outline" className="text-xs">{language}</Badge>
      </div>
      <DiffEditor
        original={originalContent}
        modified={modifiedContent}
        language={language}
        options={{
          readOnly: true,
          renderSideBySide: true,
          minimap: { enabled: false },
          fontSize: 12,
          scrollBeyondLastLine: false,
          wordWrap: 'off',
        }}
        theme="vs-dark"
        height={350}
      />
    </div>
  )
}
```

---

## 4. Giai phap — GitPanel Upgrade

**MODIFY:** `src/renderer/src/components/workspace/git/GitPanel.tsx`

**Gap hien tai (64 lines):**
1. Chi co 3 tabs: changes, history, branches — **thieu tab "Pull Requests"**
2. Khong co streaming push UI

**Bo sung Push Requests tab va streaming:**

```typescript
// MODIFY: src/renderer/src/components/workspace/git/GitPanel.tsx
// Them tab 'pullrequests' + streaming push display

import { useState, lazy, Suspense } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { useGit } from '../../../hooks/useGit'
import { StagingArea } from './StagingArea'
import { CommitForm } from './CommitForm'
import { toast } from 'sonner'
import { Loader2 } from 'lucide-react'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { useAppStore } from '@/store'

const GitHistory     = lazy(() => import('./GitHistory').then(m => ({ default: m.GitHistory })))
const BranchManager  = lazy(() => import('./BranchManager').then(m => ({ default: m.BranchManager })))
const DiffViewer     = lazy(() => import('./DiffViewer').then(m => ({ default: m.DiffViewer })))
const PullRequestList = lazy(() => import('./PullRequestList').then(m => ({ default: m.PullRequestList })))

type GitTab = 'changes' | 'history' | 'branches' | 'pullrequests'

export function GitPanel() {
  const { gitStatus, project, emit, refreshGitStatus } = useWorkspace()
  const { getDiff } = useGit()
  const [activeTab, setActiveTab] = useState<GitTab>('changes')
  const [selectedDiff, setSelectedDiff] = useState<string | null>(null)
  const [isPushing, setIsPushing] = useState(false)
  const [pushLines, setPushLines] = useState<string[]>([])

  const handlePush = async () => {
    if (!project || !gitStatus) return
    setIsPushing(true)
    setPushLines([])
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      // Streaming push
      // Note: callRuntimeRpc khong ho tro streaming hien tai
      // Dung pattern polling hoac batch call
      await callRuntimeRpc(target, 'git.push', {
        projectId: project.id,
        branch: gitStatus.branch ?? 'main',
        remote: 'origin',
      })
      await refreshGitStatus()
      emit('git.push', { branch: gitStatus.branch ?? 'main' })
      toast.success('Push complete')
    } catch (err: any) {
      toast.error('Push failed: ' + err.message)
    } finally {
      setIsPushing(false)
    }
  }

  const handleViewDiff = async (path: string) => {
    setSelectedDiff(path)
    await getDiff(path)
  }

  const TABS: { id: GitTab; label: string }[] = [
    { id: 'changes', label: 'Changes' },
    { id: 'history', label: 'History' },
    { id: 'branches', label: 'Branches' },
    { id: 'pullrequests', label: 'Pull Requests' },
  ]

  return (
    <div className="git-panel flex flex-col h-full" data-testid="git-panel">
      {/* Header: branch + sync button */}
      <div className="flex items-center gap-2 px-3 py-2 border-b text-sm">
        <span className="font-mono text-xs">{gitStatus?.branch ?? '(no branch)'}</span>
        {gitStatus && (
          <span className="text-xs text-muted-foreground">
            +{gitStatus.ahead ?? 0} -{gitStatus.behind ?? 0}
          </span>
        )}
        <button
          onClick={handlePush}
          disabled={isPushing}
          className="ml-auto text-xs px-2 py-1 border rounded hover:bg-accent"
        >
          {isPushing ? <Loader2 size={10} className="animate-spin inline mr-1" /> : null}
          Sync
        </button>
      </div>

      {/* Tab bar */}
      <div className="flex border-b text-sm">
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            data-testid={`git-tab-${tab.id}`}
            className={`px-3 py-2 text-xs border-b-2 ${
              activeTab === tab.id
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground'
            }`}
          >
            {tab.label}
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
                <div className="border-t">
                  <DiffViewer filePath={selectedDiff} />
                </div>
              )}
            </div>
          )}
          {activeTab === 'history' && <GitHistory />}
          {activeTab === 'branches' && <BranchManager />}
          {activeTab === 'pullrequests' && <PullRequestList />}
        </Suspense>
      </div>

      {/* Push progress */}
      {isPushing && pushLines.length > 0 && (
        <div className="push-progress p-2 bg-muted border-t text-xs font-mono">
          {pushLines.slice(-5).join('\n')}
        </div>
      )}
    </div>
  )
}
```

---

## 5. Giai phap — PullRequestList Component

**File moi:** `src/renderer/src/components/workspace/git/PullRequestList.tsx`

```typescript
// NEW: src/renderer/src/components/workspace/git/PullRequestList.tsx
// Liet ke PR tu gh CLI qua backend relay

import { useState, useEffect, useCallback } from 'react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { useAppStore } from '@/store'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ExternalLink, GitPullRequest } from 'lucide-react'
import { toast } from 'sonner'

interface PullRequest {
  number: number
  title: string
  state: 'open' | 'closed' | 'merged'
  url: string
  author: string
  baseBranch: string
  headBranch: string
  createdAt: string
  reviewDecision?: 'approved' | 'changes_requested' | 'review_required'
}

export function PullRequestList() {
  const { project } = useWorkspace()
  const [prs, setPrs] = useState<PullRequest[]>([])
  const [isLoading, setIsLoading] = useState(true)

  const loadPRs = useCallback(async () => {
    if (!project) return
    setIsLoading(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const result = await callRuntimeRpc<PullRequest[]>(target, 'git.pr.list', {
        projectId: project.id,
        state: 'open',
      })
      setPrs(result)
    } catch {
      toast.error('Failed to load pull requests')
    } finally {
      setIsLoading(false)
    }
  }, [project])

  useEffect(() => { loadPRs() }, [loadPRs])

  if (!project) return <div className="p-3 text-xs text-muted-foreground">No project selected</div>
  if (isLoading) return <div className="p-3 text-xs text-muted-foreground">Loading pull requests...</div>
  if (prs.length === 0) return (
    <div className="p-6 text-center">
      <GitPullRequest size={24} className="mx-auto mb-2 text-muted-foreground opacity-40" />
      <p className="text-sm text-muted-foreground">No open pull requests</p>
    </div>
  )

  return (
    <div className="pr-list divide-y" data-testid="pr-list">
      {prs.map(pr => (
        <div key={pr.number} className="pr-item px-3 py-2 hover:bg-accent/50">
          <div className="flex items-start gap-2">
            <GitPullRequest size={14} className="mt-0.5 text-green-600 shrink-0" />
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium truncate">{pr.title}</p>
              <p className="text-xs text-muted-foreground">
                #{pr.number} opened by {pr.author} — {pr.headBranch} -> {pr.baseBranch}
              </p>
            </div>
            <a href={pr.url} target="_blank" rel="noopener noreferrer">
              <Button variant="ghost" size="icon" className="h-6 w-6 shrink-0">
                <ExternalLink size={10} />
              </Button>
            </a>
          </div>
          {pr.reviewDecision && (
            <Badge
              variant="outline"
              className={`mt-1 text-xs ${
                pr.reviewDecision === 'approved' ? 'text-green-600' :
                pr.reviewDecision === 'changes_requested' ? 'text-red-600' : 'text-yellow-600'
              }`}
            >
              {pr.reviewDecision.replace('_', ' ')}
            </Badge>
          )}
        </div>
      ))}
    </div>
  )
}
```

---

## 6. useGit Hook Verification

**KIEM TRA:** `hooks/useGit.ts` (3767 bytes)

**Phan can co theo TDD-FE-16:**

```typescript
// Tat ca cac operations trong useGit can:
// 1. Lay project + currentWorktree tu useWorkspace()
// 2. Call callRuntimeRpc voi dung RPC method
// 3. Goi refreshGitStatus() sau moi thay doi

// RPC method names can verify:
'git.add'     // stage files
'git.restore' // unstage (--staged) hoac discard
'git.commit'  // commit voi message + author
'git.push'    // push len origin (co the streaming)
'git.pull'    // pull tu origin
'git.fetch'   // fetch --all
'git.diff'    // get diff string
```

---

## 7. Test Plan

**Target:** >= 30 tests

```
src/renderer/src/components/workspace/git/__tests__/
├── GitPanel.test.tsx                (6+ tests)
│   ├── renders 4 tabs: changes, history, branches, pullrequests
│   ├── renders branch name from gitStatus
│   ├── Sync button calls git.push RPC
│   ├── switching tabs shows correct content
│   ├── DiffViewer shown when file selected
│   └── isPushing shows loader on Sync button
├── DiffViewer.test.tsx              (4+ tests)
│   ├── fetches HEAD and working tree content on mount
│   ├── shows Skeleton while isLoading=true
│   ├── renders Monaco DiffEditor with original/modified
│   └── detects language from file extension (.ts => typescript)
├── CommitForm.test.tsx              (6+ tests)
│   ├── commits with message => git.commit called
│   ├── empty message => toast.error, NOT commit
│   ├── AI button calls git.generateCommitMessage or equivalent
│   ├── after commit: message cleared
│   ├── after commit: onCommitted callback fired
│   └── emit('git.commit') fired
├── PullRequestList.test.tsx         (4+ tests)
│   ├── fetches PR list on mount via git.pr.list
│   ├── renders PR items with title + number
│   ├── empty list => shows empty state
│   └── external link button has correct href
└── hooks/__tests__/useGit.test.ts   (6+ tests)
    ├── stageFile => git.add + refreshGitStatus
    ├── unstageFile => git.restore --staged + refreshGitStatus
    ├── stageAll => git.add files=['.']
    ├── unstageAll => git.restore files=['.'] staged=true
    ├── getDiff => calls git.diff
    └── commit => git.commit with message + author
```

---

## 8. Phu thuoc va Thu tu

**Prerequisite:** `@monaco-editor/react` phai duoc install

**Cach kiem tra va install:**
```bash
cat package.json | grep monaco  # kiem tra
npm install @monaco-editor/react  # neu chua co
```

**Luu y boi sung:**
- Monaco la heavy library (~10MB). Dam bao DiffViewer duoc `lazy()` loaded
- `DiffEditor` component tu `@monaco-editor/react` khac voi `MonacoDiffEditor` — kiem tra exact import
- Worker bundling: co the can config trong `vite.config` cho Monaco workers
