// FileViewer.tsx — File content viewer with skeleton loading and clipboard copy (TASK-V5-10)
import { useEffect, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { toRuntimeWorktreeSelector } from '../../runtime/runtime-worktree-selector'
import { useAppStore } from '../../store'
import { Button } from '../ui/button'
import { Skeleton } from '../ui/skeleton'
import { Copy, X } from 'lucide-react'
import { Editor } from '@monaco-editor/react'

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
  json: 'json',
  yaml: 'yaml',
  yml: 'yaml',
  md: 'markdown',
  html: 'html',
  sh: 'shell',
  bash: 'shell',
  sql: 'sql'
}
function detectLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_MAP[ext] ?? 'plaintext'
}

const MAX_FILE_SIZE = 500 * 1024 // 500KB

type FileViewerProps = {
  filePath: string
  onClose?: () => void
}

type RuntimeFileReadResult = { content: string; truncated: boolean; byteLength: number }

export function FileViewer({ filePath, onClose }: FileViewerProps) {
  const { currentWorktree } = useWorkspace()
  const [content, setContent] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  // Why (same crash class as GitPanel.tsx's push): this used to call the
  // nonexistent 'workspace.readFile' with a {projectId} shape — the real
  // method is 'files.read' and requires a {worktree, relativePath} selector
  // (backend/src/main/runtime/rpc/methods/files.ts); it rejects binary paths
  // with a typed 'binary_file' error instead of returning null bytes.
  useEffect(() => {
    if (!currentWorktree || !filePath) {
      return
    }
    setIsLoading(true)
    setError(null)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<RuntimeFileReadResult>(target, 'files.read', {
      worktree: toRuntimeWorktreeSelector(currentWorktree.id),
      relativePath: filePath
    })
      .then((result) => {
        setContent(result.truncated ? `${result.content}\n\n[truncated]` : result.content)
      })
      .catch((err) => {
        if (err?.message === 'binary_file') {
          setContent('[Binary file — cannot display]')
        } else {
          setError(err?.message ?? 'Failed to read file')
        }
      })
      .finally(() => setIsLoading(false))
  }, [currentWorktree, filePath])

  const copyToClipboard = async () => {
    if (!content) {
      return
    }
    await navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const fileName = filePath.split('/').pop() ?? filePath

  return (
    <div className="file-viewer flex flex-col h-full" data-testid="file-viewer">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b bg-muted/30">
        <span className="text-sm font-medium truncate">{fileName}</span>
        <div className="flex gap-1 shrink-0">
          <Button size="sm" variant="ghost" onClick={copyToClipboard} data-testid="copy-btn">
            <Copy size={12} className="mr-1" />
            {copied ? 'Copied!' : 'Copy'}
          </Button>
          {onClose && (
            <Button size="icon" variant="ghost" className="h-7 w-7" onClick={onClose}>
              <X size={12} />
            </Button>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {isLoading && (
          <div className="p-4 space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-5/6" />
          </div>
        )}
        {error && (
          <div className="p-4 text-sm text-red-600" data-testid="file-error">
            {error}
          </div>
        )}
        {!isLoading &&
          !error &&
          content !== null &&
          (content.length > MAX_FILE_SIZE ? (
            <div className="p-4 text-sm text-muted-foreground">
              File too large to display ({Math.round(content.length / 1024)}KB &gt; 500KB)
            </div>
          ) : (
            <Editor
              value={content}
              language={detectLanguage(filePath)}
              options={{
                readOnly: true,
                minimap: { enabled: false },
                fontSize: 12,
                scrollBeyondLastLine: false,
                wordWrap: 'on',
                lineNumbers: 'on'
              }}
              theme="vs-dark"
              height="100%"
            />
          ))}
      </div>
    </div>
  )
}
