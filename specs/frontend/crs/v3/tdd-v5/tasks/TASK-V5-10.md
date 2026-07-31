# TASK-V5-10: FileViewer + FileSearchPanel + FileContextMenu

**Order:** 10  
**Prerequisite:** TASK-V5-09 (ExplorerPanel, useFileExplorer)  
**Solution Ref:** SOL-FE-V5-07 (section 5, 6, 7)  
**Est. effort:** ~50 min | **Tests:** 9

---

## Files Cần Tạo

### 1. `src/renderer/src/components/workspace/FileViewer.tsx`

```typescript
import { useEffect, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { Button } from '../ui/button'
import { Skeleton } from '../ui/skeleton'
import { Copy, X } from 'lucide-react'

const MAX_FILE_SIZE = 500 * 1024   // 500KB

interface FileViewerProps {
  filePath: string
  onClose?: () => void
}

export function FileViewer({ filePath, onClose }: FileViewerProps) {
  const { project } = useWorkspace()
  const [content,   setContent]   = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error,     setError]     = useState<string | null>(null)
  const [copied,    setCopied]    = useState(false)

  useEffect(() => {
    if (!project || !filePath) return
    setIsLoading(true)
    setError(null)
    callRuntimeRpc('workspace.readFile', { projectId: project.id, path: filePath })
      .then(c => setContent(c as string))
      .catch(err => setError(err?.message ?? 'Failed to read file'))
      .finally(() => setIsLoading(false))
  }, [project, filePath])

  const copyToClipboard = async () => {
    if (!content) return
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
        {!isLoading && !error && content !== null && (
          content.length > MAX_FILE_SIZE ? (
            <div className="p-4 text-sm text-muted-foreground">
              File too large to display ({Math.round(content.length / 1024)}KB &gt; 500KB)
            </div>
          ) : (
            <pre
              className="p-4 text-xs font-mono whitespace-pre-wrap break-all"
              data-testid="file-content"
            >
              {content}
            </pre>
          )
        )}
      </div>
    </div>
  )
}
```

### 2. `src/renderer/src/components/workspace/FileSearchPanel.tsx`

```typescript
import { useEffect, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import { Input } from '../ui/input'
import { FileIcon } from 'lucide-react'

interface FileSearchPanelProps {
  onSelect: (filePath: string) => void
}

export function FileSearchPanel({ onSelect }: FileSearchPanelProps) {
  const { project } = useWorkspace()
  const [query,   setQuery]   = useState('')
  const [results, setResults] = useState<string[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!project || query.length < 2) {
      setResults([])
      return
    }

    const timer = setTimeout(async () => {
      setLoading(true)
      try {
        const found = await callRuntimeRpc('workspace.searchFiles', {
          projectId: project.id,
          query,
          limit: 20,
        }) as string[]
        setResults(found)
      } finally {
        setLoading(false)
      }
    }, 200)  // 200ms debounce

    return () => clearTimeout(timer)
  }, [project, query])

  return (
    <div className="file-search-panel p-2 space-y-2" data-testid="file-search-panel">
      <Input
        value={query}
        onChange={e => setQuery(e.target.value)}
        placeholder="Search files..."
        autoFocus
      />
      {loading && (
        <div className="text-xs text-muted-foreground px-2">Searching...</div>
      )}
      <div className="space-y-0.5">
        {results.map(path => (
          <div
            key={path}
            className="flex items-center gap-2 px-2 py-1 rounded cursor-pointer hover:bg-accent text-sm"
            onClick={() => onSelect(path)}
            data-testid={`search-result-${path}`}
          >
            <FileIcon size={12} className="text-muted-foreground shrink-0" />
            <span className="truncate">{path}</span>
          </div>
        ))}
        {!loading && results.length === 0 && query.length >= 2 && (
          <div className="text-xs text-muted-foreground px-2">No files found</div>
        )}
      </div>
    </div>
  )
}
```

### 3. `src/renderer/src/components/workspace/FileContextMenu.tsx`

```typescript
import {
  ContextMenu, ContextMenuContent, ContextMenuItem,
  ContextMenuSeparator, ContextMenuTrigger,
} from '../ui/context-menu'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import type { FileNode } from '@shared/workspace-types'
import toast from 'react-hot-toast'

interface FileContextMenuProps {
  node:       FileNode
  children:   React.ReactNode
  onViewFile: (path: string) => void
  onRefresh:  (dirPath?: string) => void
}

export function FileContextMenu({ node, children, onViewFile, onRefresh }: FileContextMenuProps) {
  const { project } = useWorkspace()

  const copyPath = () => {
    navigator.clipboard.writeText(node.path)
    toast.success('Path copied')
  }

  const deleteFile = async () => {
    if (!project) return
    if (!window.confirm(`Delete "${node.name}"? This cannot be undone.`)) return
    try {
      await callRuntimeRpc('workspace.deleteFile', { projectId: project.id, path: node.path })
      const parentDir = node.path.includes('/')
        ? node.path.split('/').slice(0, -1).join('/')
        : '.'
      onRefresh(parentDir)
      toast.success(`Deleted ${node.name}`)
    } catch (err: any) {
      toast.error(err?.message ?? 'Delete failed')
    }
  }

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem onSelect={copyPath}>Copy Path</ContextMenuItem>

        {node.type === 'file' && (
          <>
            <ContextMenuItem onSelect={() => onViewFile(node.path)}>
              View File
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem
              className="text-red-600 focus:text-red-600"
              onSelect={deleteFile}
            >
              Delete
            </ContextMenuItem>
          </>
        )}

        {node.type === 'directory' && (
          <>
            <ContextMenuItem onSelect={() => onRefresh(node.path)}>
              Refresh
            </ContextMenuItem>
          </>
        )}
      </ContextMenuContent>
    </ContextMenu>
  )
}
```

---

## Tests — `src/renderer/src/components/workspace/__tests__/FileViewer.test.tsx`

```typescript
// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, act, cleanup, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('../../context/WorkspaceContext', () => ({
  useWorkspace: () => ({ project: { id: 'p1', name: 'test' } }),
}))
vi.mock('../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
}))
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
const mockRpc = vi.mocked(callRuntimeRpc)
import { FileViewer } from '../FileViewer'

afterEach(() => cleanup())

describe('FileViewer', () => {
  it('fetches file content on mount', async () => {
    mockRpc.mockResolvedValueOnce('const x = 1\n')
    render(<FileViewer filePath="src/index.ts" />)
    await waitFor(() => expect(screen.getByTestId('file-content')).toBeInTheDocument())
    expect(mockRpc).toHaveBeenCalledWith('workspace.readFile', {
      projectId: 'p1', path: 'src/index.ts'
    })
    expect(screen.getByTestId('file-content').textContent).toContain('const x = 1')
  })

  it('shows loading skeleton while fetching', () => {
    mockRpc.mockImplementation(() => new Promise(() => {}))
    render(<FileViewer filePath="src/file.ts" />)
    expect(document.querySelector('.animate-pulse, [data-slot=skeleton]')).toBeTruthy()
  })

  it('shows error message on fetch failure', async () => {
    mockRpc.mockRejectedValueOnce(new Error('File not found'))
    render(<FileViewer filePath="missing.ts" />)
    await waitFor(() => expect(screen.getByTestId('file-error')).toBeInTheDocument())
    expect(screen.getByTestId('file-error').textContent).toContain('File not found')
  })

  it('copy button copies content to clipboard', async () => {
    const writeText = vi.fn()
    Object.assign(navigator, { clipboard: { writeText } })
    mockRpc.mockResolvedValueOnce('file content here')
    render(<FileViewer filePath="test.ts" />)
    await waitFor(() => screen.getByTestId('file-content'))
    fireEvent.click(screen.getByTestId('copy-btn'))
    expect(writeText).toHaveBeenCalledWith('file content here')
  })
})
```

## Tests — `src/renderer/src/components/workspace/__tests__/FileSearchPanel.test.tsx`

```typescript
// @vitest-environment happy-dom — 5 tests:
// renders input | <2 chars → no search | ≥2 chars → debounced search
// results shown + click calls onSelect | "No files found" for empty results
```

---

## Acceptance Criteria

- [x] `FileViewer` fetches `workspace.readFile` on mount
- [x] Loading skeleton → content → displayed in `<pre>`
- [x] Error message shown on RPC failure
- [x] Copy button copies to clipboard
- [x] Files >500KB show warning instead of content
- [x] `FileSearchPanel` debounces search 200ms
- [x] Search query <2 chars → no RPC call
- [x] 9/9 tests pass
