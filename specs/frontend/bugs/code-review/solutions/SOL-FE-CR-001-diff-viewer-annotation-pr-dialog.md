# SOL-FE-CR-001: Implement DiffViewer, Annotation Panel, CommitMessage Generator và PR Dialog

## Bug Reference
- **Bug:** BUG-FE-CR-001
- **Mức độ:** 🔴 HIGH (Feature Missing)
- **TDD Reference:** TDD-FE-16 (Remote Git UI — DiffViewer, CommitForm, PullRequestForm)

---

## Root Cause

Toàn bộ Code Review UI (BL-CR-01 → BL-CR-05) chưa được implement trong `src/renderer/`. Cụ thể:
- Không có `DiffViewer` component (Monaco diff mode)
- Không có `AnnotationMarker` / `AnnotationPanel` (line-level comments)
- Không có `generateCommitMessage` UI
- Không có `pr.create` dialog

---

## Giải pháp

TDD-FE-16 đã định nghĩa đầy đủ spec cho các component này. Cần implement theo đúng spec.

---

### Component 1: `DiffViewer.tsx`

**File:** `src/renderer/src/components/code-review/diff-viewer.tsx` (TẠO MỚI)

```typescript
// src/renderer/src/components/code-review/diff-viewer.tsx
// Dựa trên TDD-FE-16 §3 — Monaco Editor diff mode

import { useEffect, useState, useCallback } from 'react'
import { MonacoDiffEditor } from '@monaco-editor/react'
import { FileCode } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { rpc } from '@/platform/rpc-client-interface'
import { useWorkspace } from '@/context/WorkspaceContext'
import { detectLanguage } from '@/lib/detect-language'

interface DiffViewerProps {
  filePath: string
  worktreePath?: string
  staged?: boolean
  /** Optional: hàm callback khi user click vào line số cụ thể */
  onLineClick?: (lineNumber: number) => void
}

export function DiffViewer({
  filePath,
  worktreePath,
  staged = false,
  onLineClick,
}: DiffViewerProps) {
  const { project } = useWorkspace()
  const [originalContent, setOriginalContent] = useState('')
  const [modifiedContent, setModifiedContent] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!project || !worktreePath || !filePath) return
    setIsLoading(true)
    setError(null)

    Promise.all([
      // Original: HEAD version (BL-CR-01)
      rpc.call('git.exec', {
        projectId: project.id,
        worktreePath,
        args: ['show', `HEAD:${filePath}`],
      }).then((r: any) => r.stdout).catch(() => ''),

      // Modified: working tree (unstaged) hoặc staged vs HEAD
      staged
        ? rpc.call('git.diff', {
            projectId: project.id,
            worktreePath,
            filePath,
            staged: true,
          }).then((r: any) => r.patch).catch(() => '')
        : rpc.call('fs.readFile', {
            projectId: project.id,
            path: `${worktreePath}/${filePath}`,
            encoding: 'utf-8',
          }).then((r: any) => r.content).catch(() => ''),
    ])
      .then(([original, modified]) => {
        setOriginalContent(original)
        setModifiedContent(modified)
      })
      .catch((err) => setError(err.message))
      .finally(() => setIsLoading(false))
  }, [filePath, worktreePath, staged, project])

  const language = detectLanguage(filePath)

  if (isLoading) return <Skeleton className="h-40 w-full" />
  if (error) return <div className="text-destructive text-sm p-3">Error: {error}</div>

  return (
    <div className="diff-viewer border rounded overflow-hidden" style={{ height: 400 }}>
      <div className="diff-viewer-header flex items-center gap-2 px-3 py-1 bg-muted border-b text-xs">
        <FileCode size={12} />
        <span className="font-mono truncate">{filePath}</span>
        <Badge variant="outline" className="ml-auto shrink-0">{language}</Badge>
        {staged && <Badge variant="secondary" className="shrink-0">Staged</Badge>}
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
          lineNumbers: 'on',
        }}
        theme="vs-dark"
        height={360}
        onMount={(editor) => {
          if (onLineClick) {
            editor.onMouseDown((e) => {
              const lineNumber = e.target.position?.lineNumber
              if (lineNumber) onLineClick(lineNumber)
            })
          }
        }}
      />
    </div>
  )
}
```

---

### Component 2: `annotation-panel.tsx`

**File:** `src/renderer/src/components/code-review/annotation-panel.tsx` (TẠO MỚI)

```typescript
// BL-CR-02: Line annotation click handler
// Annotation = comment gắn vào một line cụ thể trong diff

import { useState } from 'react'
import { MessageSquare, X, Send } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { rpc } from '@/platform/rpc-client-interface'
import { useWorkspace } from '@/context/WorkspaceContext'
import { toast } from 'sonner'

interface Annotation {
  id: string
  lineNumber: number
  filePath: string
  content: string
  author: string
  createdAt: number
}

interface AnnotationPanelProps {
  filePath: string
  reviewId: string
  lineNumber: number | null
  onClose: () => void
}

export function AnnotationPanel({
  filePath,
  reviewId,
  lineNumber,
  onClose,
}: AnnotationPanelProps) {
  const { project } = useWorkspace()
  const [annotations, setAnnotations] = useState<Annotation[]>([])
  const [newComment, setNewComment] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  const submitAnnotation = async () => {
    if (!newComment.trim() || !lineNumber || !project) return
    setIsSaving(true)
    try {
      const result = await rpc.call('annotation.create', {
        projectId: project.id,
        reviewId,
        filePath,
        lineNumber,
        content: newComment.trim(),
      }) as Annotation
      setAnnotations(prev => [...prev, result])
      setNewComment('')
      toast.success('Annotation added')
    } catch (err) {
      toast.error('Failed to add annotation')
    } finally {
      setIsSaving(false)
    }
  }

  if (!lineNumber) return null

  return (
    <div className="annotation-panel border rounded bg-background shadow-md p-3 space-y-2">
      <div className="flex items-center justify-between text-xs font-medium">
        <span className="flex items-center gap-1">
          <MessageSquare size={12} />
          Line {lineNumber} — {filePath}
        </span>
        <Button variant="ghost" size="icon" className="h-5 w-5" onClick={onClose}>
          <X size={10} />
        </Button>
      </div>

      {/* Existing annotations */}
      {annotations.map(ann => (
        <div key={ann.id} className="text-xs p-2 bg-muted rounded">
          <span className="font-medium">{ann.author}:</span> {ann.content}
        </div>
      ))}

      {/* New annotation input */}
      <div className="flex gap-2">
        <Textarea
          value={newComment}
          onChange={e => setNewComment(e.target.value)}
          placeholder="Add a comment on this line..."
          className="text-xs resize-none"
          rows={2}
        />
        <Button
          size="icon"
          onClick={submitAnnotation}
          disabled={isSaving || !newComment.trim()}
        >
          <Send size={12} />
        </Button>
      </div>
    </div>
  )
}
```

---

### Component 3: `commit-message-generator.tsx`

**File:** `src/renderer/src/components/code-review/commit-message-generator.tsx` (TẠO MỚI)

```typescript
// BL-CR-04: AI commit message UI
// Tích hợp vào CommitForm (TDD-FE-16 §4)

import { useState } from 'react'
import { Sparkles, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { rpc } from '@/platform/rpc-client-interface'
import { useWorkspace } from '@/context/WorkspaceContext'
import { toast } from 'sonner'

interface CommitMessageGeneratorProps {
  value: string
  onChange: (value: string) => void
  onCommit: (push: boolean) => Promise<void>
  isCommitting: boolean
}

export function CommitMessageGenerator({
  value,
  onChange,
  onCommit,
  isCommitting,
}: CommitMessageGeneratorProps) {
  const { project, currentWorktree } = useWorkspace()
  const [isGenerating, setIsGenerating] = useState(false)

  const generateMessage = async () => {
    if (!project || !currentWorktree) return
    setIsGenerating(true)
    try {
      // Theo TDD-FE-16 §Addendum: AI Commit Message Flow:
      // 1. git.diff (staged) → diff string
      // 2. ai.complete({ prompt: buildCommitPrompt(diff) }) → stream tokens
      const result = await rpc.call('git.generateCommitMessage', {
        projectId: project.id,
        worktreePath: currentWorktree.path,
      }) as string
      onChange(result)
    } catch (err: any) {
      if (err.code === 'GIT_NO_STAGED_CHANGES') {
        toast.error('No staged changes — stage files first')
      } else {
        toast.error('Failed to generate commit message')
      }
    } finally {
      setIsGenerating(false)
    }
  }

  return (
    <div className="commit-message-generator space-y-2">
      <div className="relative">
        <Textarea
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder="Commit message (e.g. feat(auth): add JWT middleware)"
          className="pr-8 resize-none text-sm font-mono"
          rows={3}
          maxLength={500}
        />
        <Button
          variant="ghost"
          size="icon"
          className="absolute right-1 top-1 h-6 w-6"
          onClick={generateMessage}
          disabled={isGenerating}
          title="Generate commit message with AI (BL-CR-04)"
        >
          {isGenerating ? (
            <Loader2 size={12} className="animate-spin" />
          ) : (
            <Sparkles size={12} />
          )}
        </Button>
      </div>
      <div className="flex gap-2">
        <Button
          className="flex-1"
          onClick={() => onCommit(false)}
          disabled={isCommitting || !value.trim()}
        >
          {isCommitting && <Loader2 className="animate-spin mr-1" size={12} />}
          Commit
        </Button>
        <Button
          variant="outline"
          onClick={() => onCommit(true)}
          disabled={isCommitting || !value.trim()}
        >
          Commit & Push
        </Button>
      </div>
    </div>
  )
}
```

---

### Component 4: `pr-create-dialog.tsx`

**File:** `src/renderer/src/components/code-review/pr-create-dialog.tsx` (TẠO MỚI)

```typescript
// BL-CR-05: PR creation dialog với AI description
// Theo TDD-FE-16 §Addendum: Pull Request via gh CLI

import { useState } from 'react'
import { ExternalLink, Sparkles, Loader2 } from 'lucide-react'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { rpc } from '@/platform/rpc-client-interface'
import { useWorkspace } from '@/context/WorkspaceContext'
import { toast } from 'sonner'

interface PrCreateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentBranch: string
  baseBranch?: string
}

export function PrCreateDialog({
  open,
  onOpenChange,
  currentBranch,
  baseBranch = 'main',
}: PrCreateDialogProps) {
  const { project, currentWorktree } = useWorkspace()
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [isDraft, setIsDraft] = useState(false)
  const [isCreating, setIsCreating] = useState(false)
  const [isGeneratingDesc, setIsGeneratingDesc] = useState(false)
  const [prUrl, setPrUrl] = useState<string | null>(null)

  const generateDescription = async () => {
    if (!project || !currentWorktree) return
    setIsGeneratingDesc(true)
    try {
      const result = await rpc.call('git.generatePrDescription', {
        projectId: project.id,
        worktreePath: currentWorktree.path,
        branch: currentBranch,
        baseBranch,
      }) as { title: string; body: string }
      setTitle(result.title)
      setBody(result.body)
    } catch {
      toast.error('Failed to generate PR description')
    } finally {
      setIsGeneratingDesc(false)
    }
  }

  const createPr = async () => {
    if (!title.trim() || !project || !currentWorktree) return
    setIsCreating(true)
    try {
      // Theo TDD-FE-16 §Addendum: gh pr create via relay
      const result = await rpc.call('git.pr.create', {
        projectId: project.id,
        worktreePath: currentWorktree.path,
        title: title.trim(),
        body: body.trim(),
        base: baseBranch,
        draft: isDraft,
      }) as { url: string; number: number }
      setPrUrl(result.url)
      toast.success(`PR #${result.number} created!`)
    } catch (err: any) {
      toast.error(`Failed to create PR: ${err.message}`)
    } finally {
      setIsCreating(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Pull Request</DialogTitle>
        </DialogHeader>

        {prUrl ? (
          <div className="text-center py-4 space-y-3">
            <p className="text-sm text-muted-foreground">Pull Request created successfully!</p>
            <Button
              variant="outline"
              onClick={() => window.open(prUrl, '_blank')}
              className="gap-2"
            >
              <ExternalLink size={14} />
              Open PR on GitHub
            </Button>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <code className="bg-muted px-1 rounded">{currentBranch}</code>
              →
              <code className="bg-muted px-1 rounded">{baseBranch}</code>
            </div>

            <Button
              variant="outline"
              size="sm"
              className="gap-2 w-full"
              onClick={generateDescription}
              disabled={isGeneratingDesc}
            >
              {isGeneratingDesc
                ? <Loader2 size={12} className="animate-spin" />
                : <Sparkles size={12} />
              }
              Generate with AI
            </Button>

            <div className="space-y-1">
              <Label className="text-xs">Title</Label>
              <Input
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder="feat(auth): implement JWT middleware"
                className="text-sm"
              />
            </div>

            <div className="space-y-1">
              <Label className="text-xs">Description (optional)</Label>
              <Textarea
                value={body}
                onChange={e => setBody(e.target.value)}
                placeholder="Describe the changes in this PR..."
                rows={5}
                className="text-sm resize-none"
              />
            </div>

            <label className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={isDraft}
                onChange={e => setIsDraft(e.target.checked)}
              />
              Create as draft
            </label>
          </div>
        )}

        {!prUrl && (
          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button onClick={createPr} disabled={isCreating || !title.trim()}>
              {isCreating && <Loader2 size={12} className="animate-spin mr-1" />}
              Create PR
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

---

## Files cần tạo

| File | Action | Tương ứng Bug |
|------|--------|--------------|
| `src/renderer/src/components/code-review/diff-viewer.tsx` | CREATE | BL-CR-01 |
| `src/renderer/src/components/code-review/annotation-panel.tsx` | CREATE | BL-CR-02 |
| `src/renderer/src/components/code-review/commit-message-generator.tsx` | CREATE | BL-CR-04 |
| `src/renderer/src/components/code-review/pr-create-dialog.tsx` | CREATE | BL-CR-05 |

> **Lưu ý:** `DiffViewer` trong `code-review/` là component riêng phục vụ code review flow.  
> `DiffViewer` trong `workspace/git/` (TDD-FE-16) phục vụ git staging flow.  
> Cả hai có thể dùng chung Monaco Diff Editor core nhưng khác context.

---

## Dependency

```bash
# Monaco editor (có thể đã installed):
npm install @monaco-editor/react

# Verify:
grep "@monaco-editor" package.json
```

---

### Component 5 (BỔ SUNG): `changed-files-tree.tsx` — BL-CR-03

**File:** `src/renderer/src/components/code-review/changed-files-tree.tsx` (TẠO MỚI)

> **BL-CR-03**: Files tree với change counts — bị bỏ sót trong lần đầu tiên.

```typescript
// BL-CR-03: Files tree với change counts
// Hiển thị danh sách files changed có số lượng dòng thêm/xóa

import { useState } from 'react'
import { ChevronRight, ChevronDown, FileCode, FilePlus, FileMinus, FileEdit } from 'lucide-react'
import { cn } from '@/lib/utils'

type ChangeType = 'added' | 'deleted' | 'modified' | 'renamed'

interface ChangedFile {
  path: string
  changeType: ChangeType
  additions: number
  deletions: number
  oldPath?: string   // for renamed files
}

interface ChangedFilesTreeProps {
  files: ChangedFile[]
  selectedFile: string | null
  onSelectFile: (path: string) => void
}

// Group files by directory
function groupByDirectory(files: ChangedFile[]): Map<string, ChangedFile[]> {
  const groups = new Map<string, ChangedFile[]>()
  for (const file of files) {
    const parts = file.path.split('/')
    const dir = parts.length > 1 ? parts.slice(0, -1).join('/') : '(root)'
    if (!groups.has(dir)) groups.set(dir, [])
    groups.get(dir)!.push(file)
  }
  return groups
}

function ChangeTypeIcon({ type }: { type: ChangeType }) {
  switch (type) {
    case 'added':    return <FilePlus  size={12} className="text-green-500 shrink-0" />
    case 'deleted':  return <FileMinus size={12} className="text-red-500 shrink-0" />
    case 'modified': return <FileEdit  size={12} className="text-blue-500 shrink-0" />
    case 'renamed':  return <FileCode  size={12} className="text-yellow-500 shrink-0" />
  }
}

function ChangeStats({ additions, deletions }: { additions: number; deletions: number }) {
  return (
    <span className="ml-auto text-xs font-mono shrink-0 flex items-center gap-1">
      {additions > 0 && <span className="text-green-500">+{additions}</span>}
      {deletions > 0 && <span className="text-red-500">-{deletions}</span>}
    </span>
  )
}

export function ChangedFilesTree({
  files,
  selectedFile,
  onSelectFile,
}: ChangedFilesTreeProps) {
  const grouped = groupByDirectory(files)
  const [collapsedDirs, setCollapsedDirs] = useState<Set<string>>(new Set())

  const toggleDir = (dir: string) => {
    setCollapsedDirs(prev => {
      const next = new Set(prev)
      if (next.has(dir)) next.delete(dir)
      else next.add(dir)
      return next
    })
  }

  const totalAdditions = files.reduce((s, f) => s + f.additions, 0)
  const totalDeletions = files.reduce((s, f) => s + f.deletions, 0)

  return (
    <div className="changed-files-tree text-sm">
      {/* Summary header */}
      <div className="flex items-center gap-2 px-3 py-1.5 border-b text-xs text-muted-foreground">
        <span>{files.length} file{files.length !== 1 ? 's' : ''} changed</span>
        <span className="text-green-500 font-mono">+{totalAdditions}</span>
        <span className="text-red-500 font-mono">-{totalDeletions}</span>
      </div>

      {/* File tree */}
      <div className="overflow-y-auto max-h-64">
        {[...grouped.entries()].map(([dir, dirFiles]) => {
          const isCollapsed = collapsedDirs.has(dir)
          const dirAdditions = dirFiles.reduce((s, f) => s + f.additions, 0)
          const dirDeletions = dirFiles.reduce((s, f) => s + f.deletions, 0)

          return (
            <div key={dir}>
              {/* Directory row */}
              <button
                className="w-full flex items-center gap-1.5 px-2 py-1 hover:bg-muted/50 text-xs text-muted-foreground"
                onClick={() => toggleDir(dir)}
              >
                {isCollapsed
                  ? <ChevronRight size={12} className="shrink-0" />
                  : <ChevronDown  size={12} className="shrink-0" />
                }
                <span className="font-mono truncate">{dir}/</span>
                <ChangeStats additions={dirAdditions} deletions={dirDeletions} />
              </button>

              {/* Files in directory */}
              {!isCollapsed && dirFiles.map(file => {
                const filename = file.path.split('/').pop() ?? file.path
                const isSelected = file.path === selectedFile

                return (
                  <button
                    key={file.path}
                    className={cn(
                      'w-full flex items-center gap-1.5 pl-7 pr-2 py-1 hover:bg-muted/50 text-xs',
                      isSelected && 'bg-muted font-medium'
                    )}
                    onClick={() => onSelectFile(file.path)}
                    title={file.path}
                  >
                    <ChangeTypeIcon type={file.changeType} />
                    <span className="font-mono truncate">{filename}</span>
                    {file.changeType === 'renamed' && file.oldPath && (
                      <span className="text-muted-foreground truncate">
                        ← {file.oldPath.split('/').pop()}
                      </span>
                    )}
                    <ChangeStats additions={file.additions} deletions={file.deletions} />
                  </button>
                )
              })}
            </div>
          )
        })}
      </div>
    </div>
  )
}
```

---

### Hook: `useCodeReview.ts`

**File:** `src/renderer/src/hooks/useCodeReview.ts` (TẠO MỚI)

Tổng hợp tất cả Code Review state — kết nối DiffViewer + ChangedFilesTree + AnnotationPanel:

```typescript
// src/renderer/src/hooks/useCodeReview.ts
// Quản lý state cho toàn bộ Code Review flow (BL-CR-01 → BL-CR-05)

import { useState, useCallback, useEffect } from 'react'
import { rpc } from '@/platform/rpc-client-interface'
import { useWorkspace } from '@/context/WorkspaceContext'
import { useGit } from './useGit'

interface ChangedFile {
  path: string
  changeType: 'added' | 'deleted' | 'modified' | 'renamed'
  additions: number
  deletions: number
  oldPath?: string
}

export function useCodeReview() {
  const { project, currentWorktree } = useWorkspace()
  const { gitStatus, refreshGitStatus } = useGit()

  const [changedFiles, setChangedFiles] = useState<ChangedFile[]>([])
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [annotationLine, setAnnotationLine] = useState<number | null>(null)
  const [isLoadingFiles, setIsLoadingFiles] = useState(false)

  // BL-CR-03: Load changed files with additions/deletions counts
  const loadChangedFiles = useCallback(async () => {
    if (!project || !currentWorktree) return
    setIsLoadingFiles(true)
    try {
      // git diff --stat HEAD → parse changed files + counts
      const result = await rpc.call('git.exec', {
        projectId: project.id,
        worktreePath: currentWorktree.path,
        args: ['diff', 'HEAD', '--numstat'],
      }) as { stdout: string }

      // Parse numstat: "<additions>\t<deletions>\t<filename>"
      const files: ChangedFile[] = result.stdout
        .trim()
        .split('\n')
        .filter(Boolean)
        .map(line => {
          const parts = line.split('\t')
          const additions = parseInt(parts[0]) || 0
          const deletions = parseInt(parts[1]) || 0
          const pathPart = parts[2] ?? ''

          // Handle renames: "old => new" format
          const renameMatch = pathPart.match(/^{(.+) => (.+)}$/) ||
                              pathPart.match(/^(.+) => (.+)$/)
          if (renameMatch) {
            return {
              path: renameMatch[2],
              oldPath: renameMatch[1],
              changeType: 'renamed' as const,
              additions,
              deletions,
            }
          }

          // Determine change type from gitStatus
          const statusEntry = gitStatus?.staged?.find(f => f.path === pathPart)
            ?? gitStatus?.unstaged?.find(f => f.path === pathPart)

          const changeType = statusEntry?.status === 'A' ? 'added'
            : statusEntry?.status === 'D' ? 'deleted'
            : 'modified'

          return { path: pathPart, changeType, additions, deletions }
        })

      setChangedFiles(files)
      // Auto-select first file
      if (files.length > 0 && !selectedFile) {
        setSelectedFile(files[0].path)
      }
    } catch {
      setChangedFiles([])
    } finally {
      setIsLoadingFiles(false)
    }
  }, [project, currentWorktree, gitStatus, selectedFile])

  // Reload when git status changes
  useEffect(() => {
    loadChangedFiles()
  }, [gitStatus])

  // BL-CR-02: Handle line click → open annotation
  const handleLineClick = useCallback((lineNumber: number) => {
    setAnnotationLine(lineNumber)
  }, [])

  const closeAnnotation = useCallback(() => {
    setAnnotationLine(null)
  }, [])

  return {
    changedFiles,
    selectedFile,
    setSelectedFile,
    annotationLine,
    handleLineClick,
    closeAnnotation,
    isLoadingFiles,
    refreshChangedFiles: loadChangedFiles,
  }
}
```

---

### Code Review Page Assembly

**File:** `src/renderer/src/components/code-review/code-review-panel.tsx` (TẠO MỚI)

Tổng hợp tất cả components thành panel hoàn chỉnh:

```typescript
// Tổng hợp: ChangedFilesTree + DiffViewer + AnnotationPanel + PrCreateDialog
import { useState } from 'react'
import { useCodeReview } from '@/hooks/useCodeReview'
import { ChangedFilesTree } from './changed-files-tree'
import { DiffViewer } from './diff-viewer'
import { AnnotationPanel } from './annotation-panel'
import { CommitMessageGenerator } from './commit-message-generator'
import { PrCreateDialog } from './pr-create-dialog'
import { Button } from '@/components/ui/button'
import { useWorkspace } from '@/context/WorkspaceContext'

export function CodeReviewPanel() {
  const { currentWorktree } = useWorkspace()
  const {
    changedFiles, selectedFile, setSelectedFile,
    annotationLine, handleLineClick, closeAnnotation,
  } = useCodeReview()

  const [commitMessage, setCommitMessage] = useState('')
  const [showPrDialog, setShowPrDialog] = useState(false)
  const [isCommitting, setIsCommitting] = useState(false)

  const handleCommit = async (push: boolean) => {
    // Delegate to CommitMessageGenerator
  }

  return (
    <div className="code-review-panel flex h-full overflow-hidden">
      {/* Left: Files tree (BL-CR-03) */}
      <div className="w-64 border-r flex flex-col">
        <ChangedFilesTree
          files={changedFiles}
          selectedFile={selectedFile}
          onSelectFile={setSelectedFile}
        />
        <div className="mt-auto border-t p-2">
          <Button variant="outline" size="sm" className="w-full" onClick={() => setShowPrDialog(true)}>
            Create PR
          </Button>
        </div>
      </div>

      {/* Right: Diff + Annotation + Commit */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* BL-CR-01: DiffViewer */}
        {selectedFile && (
          <DiffViewer
            filePath={selectedFile}
            worktreePath={currentWorktree?.path}
            onLineClick={handleLineClick}  // BL-CR-02
          />
        )}

        {/* BL-CR-02: Annotation panel */}
        {annotationLine && selectedFile && (
          <AnnotationPanel
            filePath={selectedFile}
            reviewId="current"
            lineNumber={annotationLine}
            onClose={closeAnnotation}
          />
        )}

        {/* BL-CR-04: AI Commit Message */}
        <div className="mt-auto border-t p-3">
          <CommitMessageGenerator
            value={commitMessage}
            onChange={setCommitMessage}
            onCommit={handleCommit}
            isCommitting={isCommitting}
          />
        </div>
      </div>

      {/* BL-CR-05: PR Create Dialog */}
      <PrCreateDialog
        open={showPrDialog}
        onOpenChange={setShowPrDialog}
        currentBranch={/* gitStatus.branch */ 'main'}
        baseBranch="main"
      />
    </div>
  )
}
```

---

## Files cần tạo (ĐẦY ĐỦ)

| File | Action | BL |
|------|--------|-----|
| `src/renderer/src/components/code-review/diff-viewer.tsx` | CREATE | BL-CR-01 |
| `src/renderer/src/components/code-review/annotation-panel.tsx` | CREATE | BL-CR-02 |
| `src/renderer/src/components/code-review/changed-files-tree.tsx` | CREATE | **BL-CR-03** ← MỚI |
| `src/renderer/src/components/code-review/commit-message-generator.tsx` | CREATE | BL-CR-04 |
| `src/renderer/src/components/code-review/pr-create-dialog.tsx` | CREATE | BL-CR-05 |
| `src/renderer/src/components/code-review/code-review-panel.tsx` | CREATE | Assembly |
| `src/renderer/src/hooks/useCodeReview.ts` | CREATE | ← MỚI |

---

## Test Coverage

```typescript
// src/renderer/src/components/code-review/__tests__/
// diff-viewer.test.tsx
// - loads HEAD and working tree content via rpc
// - isLoading shows skeleton
// - onLineClick callback fires with correct line number
// - staged=true fetches git.diff instead of fs.readFile

// changed-files-tree.test.tsx  ← MỚI
// - groups files by directory correctly
// - shows addition/deletion counts per file and per dir
// - click file triggers onSelectFile callback
// - collapsed directory hides its files
// - renamed files show oldPath arrow notation

// pr-create-dialog.test.tsx
// - createPr calls git.pr.create with title/body/base
// - AI generation calls git.generatePrDescription
// - prUrl displayed after creation

// hooks/__tests__/useCodeReview.test.ts  ← MỚI
// - loadChangedFiles parses git numstat output correctly
// - handleLineClick sets annotationLine
// - closeAnnotation clears annotationLine
// - renamed file parsed with oldPath
```

---

## Liên quan

- **BL-CR-01**: DiffViewer ✅ implemented
- **BL-CR-02**: Annotation marker ✅ implemented
- **BL-CR-03**: Files tree với change counts ✅ implemented ← **BỔ SUNG**
- **BL-CR-04**: AI commit message ✅ implemented
- **BL-CR-05**: PR creation dialog ✅ implemented
- **TDD-FE-16**: §3 DiffViewer, §4 CommitForm with AI, §Addendum Pull Request via gh CLI
