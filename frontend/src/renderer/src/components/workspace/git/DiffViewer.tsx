// DiffViewer.tsx — Side-by-side diff viewer using Monaco DiffEditor (TDD-FE-16, TASK-FE-010)
// TASK-FE-010: Replaced plain-text stub with full Monaco DiffEditor implementation
import { useEffect, useState } from 'react'
import { DiffEditor } from '@monaco-editor/react'
import { useWorkspace } from '../../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'
import { useAppStore } from '../../../store'
import { getActiveRuntimeTarget } from '../../../runtime/runtime-rpc-client'
import { Skeleton } from '../../ui/skeleton'
import { Badge } from '../../ui/badge'
import { FileCode } from 'lucide-react'
import { Tracers } from '../../../../../shared/trace/tracers'

type DiffViewerProps = {
  filePath:      string
  worktreePath?: string
  staged?:       boolean
}

const LANGUAGE_MAP: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript',
  js: 'javascript', jsx: 'javascript',
  py: 'python',     go: 'go',         rs: 'rust',
  css: 'css',       scss: 'scss',     less: 'less',
  json: 'json',     yaml: 'yaml',     yml: 'yaml',
  md: 'markdown',   mdx: 'markdown',
  html: 'html',     xml: 'xml',       svg: 'xml',
  sh: 'shell',      bash: 'shell',    zsh: 'shell',
  sql: 'sql',       prisma: 'prisma',
}

function detectLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_MAP[ext] ?? 'plaintext'
}

export function DiffViewer({ filePath, worktreePath, staged = false }: DiffViewerProps) {
  const { project }                       = useWorkspace()
  const [originalContent, setOriginal]    = useState('')
  const [modifiedContent, setModified]    = useState('')
  const [isLoading, setIsLoading]         = useState(true)
  const [error, setError]                 = useState<string | null>(null)

  useEffect(() => {
    if (!project || !filePath) {return}
    setIsLoading(true)
    setError(null)

    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const span = Tracers.codeReviewDiffFlow.start({ filePath, staged, mode: target.kind })

    // Use git.getDiff if staged mode; otherwise load HEAD + working tree separately
    if (staged) {
      // Staged diff: show index vs HEAD
      callRuntimeRpc<string>(target, 'git.getDiff', {
        projectId: project.id,
        path: filePath,
        staged: true,
        traceId: span.id,
      })
        .then(diff => {
          // For staged diff we show the raw diff text as context; split at first @@
          const idx = diff.indexOf('@@')
          setOriginal(idx >= 0 ? diff.slice(0, idx) : '')
          setModified(diff)
          span.ok({ staged: true })
        })
        .catch(err => {
          setError(err?.message ?? 'Failed to load diff')
          span.fail(err, { staged: true })
        })
        .finally(() => setIsLoading(false))
    } else {
      // Unstaged diff: load HEAD version and working tree version side-by-side
      span.step('parallelFetch', { staged: false })
      Promise.all([
        // Original: HEAD version (empty string for new/untracked files)
        callRuntimeRpc<string>(target, 'git.getDiff', {
          projectId: project.id,
          path: filePath,
          staged: false,
          side: 'original',   // returns HEAD content
          traceId: span.id,
        }).catch(() => ''),

        // Modified: current working tree content
        callRuntimeRpc<{ content: string }>(target, 'fs.readFile', {
          projectId: project.id,
          path: filePath,
          encoding: 'utf-8',
          traceId: span.id,
        }).then(r => r.content).catch(() => ''),
      ])
        .then(([original, modified]) => {
          setOriginal(original)
          setModified(modified)
          span.ok({ staged: false })
        })
        .catch(err => {
          setError(err?.message ?? 'Failed to load diff')
          span.fail(err, { staged: false })
        })
        .finally(() => setIsLoading(false))
    }
  }, [filePath, worktreePath, project, staged])

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
        <Badge variant="outline" className="text-xs shrink-0">{language}</Badge>
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
          lineNumbers: 'on',
        }}
        theme="vs-dark"
        height={350}
      />
    </div>
  )
}
