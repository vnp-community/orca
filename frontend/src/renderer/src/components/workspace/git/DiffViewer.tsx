// DiffViewer.tsx — Side-by-side diff viewer using Monaco DiffEditor (TDD-FE-16, TASK-FE-010)
// TASK-FE-010: Replaced plain-text stub with full Monaco DiffEditor implementation
import { useEffect, useState } from 'react'
import { DiffEditor } from '@monaco-editor/react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../../runtime/runtime-rpc-client'
import { toRuntimeWorktreeSelector } from '../../../runtime/runtime-worktree-selector'
import { useAppStore } from '../../../store'
import { Skeleton } from '../../ui/skeleton'
import { Badge } from '../../ui/badge'
import { FileCode } from 'lucide-react'
import { Tracers } from '../../../../../shared/trace/tracers'
import type { GitDiffResult } from '../../../../../shared/types'

type DiffViewerProps = {
  filePath: string
  worktreePath?: string
  staged?: boolean
}

const LANGUAGE_MAP: Record<string, string> = {
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  py: 'python',
  go: 'go',
  rs: 'rust',
  css: 'css',
  scss: 'scss',
  less: 'less',
  json: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  md: 'markdown',
  mdx: 'markdown',
  html: 'html',
  xml: 'xml',
  svg: 'xml',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  sql: 'sql',
  prisma: 'prisma'
}

function detectLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_MAP[ext] ?? 'plaintext'
}

export function DiffViewer({ filePath, staged = false }: DiffViewerProps) {
  const { currentWorktree } = useWorkspace()
  const [originalContent, setOriginal] = useState('')
  const [modifiedContent, setModified] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Why (crash reported by user, same contract bug as GitPanel.tsx's push):
  // this used to call the nonexistent 'git.getDiff' with a {projectId} shape
  // via two separate hand-rolled branches (one of which also called the
  // nonexistent 'fs.readFile'). The real 'git.diff' RPC
  // (backend/src/main/runtime/rpc/methods/git.ts) takes a {worktree, filePath,
  // staged, compareAgainstHead?} selector and already returns both
  // originalContent/modifiedContent in one call — no need to fetch each side
  // separately.
  useEffect(() => {
    if (!currentWorktree || !filePath) {
      return
    }
    setIsLoading(true)
    setError(null)

    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const span = Tracers.codeReviewDiffFlow.start({ filePath, staged, mode: target.kind })

    callRuntimeRpc<GitDiffResult>(target, 'git.diff', {
      worktree: toRuntimeWorktreeSelector(currentWorktree.id),
      filePath,
      staged
    })
      .then((diff) => {
        setOriginal(diff.originalContent)
        setModified(diff.modifiedContent)
        span.ok({ staged, kind: diff.kind })
      })
      .catch((err) => {
        setError(err?.message ?? 'Failed to load diff')
        span.fail(err, { staged })
      })
      .finally(() => setIsLoading(false))
  }, [filePath, currentWorktree, staged])

  const language = detectLanguage(filePath)

  if (isLoading) {
    return (
      <div className="diff-viewer" data-testid="diff-viewer-loading">
        <Skeleton className="h-8 w-full rounded-none" />
        <Skeleton className="h-64 w-full rounded-none" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-3 text-xs text-destructive" data-testid="diff-viewer-error">
        Failed to load diff: {error}
      </div>
    )
  }

  return (
    <div className="diff-viewer" data-testid="diff-viewer">
      {/* Header bar */}
      <div className="diff-viewer-header flex items-center gap-2 px-3 py-1 bg-muted border-b text-xs">
        <FileCode size={12} />
        <span className="font-mono flex-1 truncate">{filePath}</span>
        <Badge variant="outline" className="text-xs shrink-0">
          {language}
        </Badge>
      </div>

      {/* Monaco side-by-side diff */}
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
          lineNumbers: 'on'
        }}
        theme="vs-dark"
        height={350}
      />
    </div>
  )
}
