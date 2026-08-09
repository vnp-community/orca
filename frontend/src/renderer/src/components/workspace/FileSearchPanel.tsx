// FileSearchPanel.tsx — Debounced file search panel (TASK-V5-10)
import { useEffect, useState } from 'react'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import { Input } from '../ui/input'
import { FileIcon } from 'lucide-react'

type FileSearchPanelProps = {
  onSelect: (filePath: string) => void
}

export function FileSearchPanel({ onSelect }: FileSearchPanelProps) {
  const { project, currentWorktree } = useWorkspace()
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
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        const searchRoot = currentWorktree?.path ?? project.repoPath ?? '.'
        const found = await callRuntimeRpc(target, 'fs.grep', {
          projectId: project.id,
          cwd: searchRoot,
          pattern: query,
          maxResults: 30,
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
        data-testid="search-input"
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
