// FileSearchPanel.tsx — Debounced file search panel (TASK-V5-10)
import { useEffect, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { toRuntimeWorktreeSelector } from '../../runtime/runtime-worktree-selector'
import { useAppStore } from '../../store'
import { Input } from '../ui/input'
import { FileIcon } from 'lucide-react'
import type { SearchResult } from '../../../../shared/types'

type FileSearchPanelProps = {
  onSelect: (filePath: string) => void
}

export function FileSearchPanel({ onSelect }: FileSearchPanelProps) {
  const { currentWorktree } = useWorkspace()
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<string[]>([])
  const [loading, setLoading] = useState(false)

  // Why (same crash class as GitPanel.tsx's push): this used to call the
  // nonexistent 'fs.grep' with a {projectId, cwd, pattern} shape — the real
  // method is 'files.search' and requires a {worktree, query} selector
  // (backend/src/main/runtime/rpc/methods/files.ts).
  useEffect(() => {
    if (!currentWorktree || query.length < 2) {
      setResults([])
      return
    }

    const timer = setTimeout(async () => {
      setLoading(true)
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        const found = await callRuntimeRpc<SearchResult>(target, 'files.search', {
          worktree: toRuntimeWorktreeSelector(currentWorktree.id),
          query,
          maxResults: 30
        })
        setResults(found.files.map((f) => f.relativePath))
      } catch {
        setResults([])
      } finally {
        setLoading(false)
      }
    }, 200) // 200ms debounce

    return () => clearTimeout(timer)
  }, [currentWorktree, query])

  return (
    <div className="file-search-panel p-2 space-y-2" data-testid="file-search-panel">
      <Input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search files..."
        autoFocus
        data-testid="search-input"
      />
      {loading && <div className="text-xs text-muted-foreground px-2">Searching...</div>}
      <div className="space-y-0.5">
        {results.map((path) => (
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
