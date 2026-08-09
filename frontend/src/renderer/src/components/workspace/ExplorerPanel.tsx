// ExplorerPanel.tsx — Full file explorer panel implementation (TASK-V5-09)
import { useFileExplorer } from '../../hooks/useFileExplorer'
import { FileTreeNode } from './FileTreeNode'
import { Button } from '../ui/button'
import { RefreshCw, Search } from 'lucide-react'

export function ExplorerPanel() {
  const {
    fileTree, expandedDirs, selectedPath, isOffline,
    project, toggleDir, openFile, openContextMenu, refresh,
  } = useFileExplorer()

  if (!project) {
    return <div className="p-3 text-sm text-muted-foreground">No project selected</div>
  }

  return (
    <div className="explorer-panel flex flex-col h-full" data-testid="explorer-panel">
      {/* Header */}
      <div className="flex items-center justify-between px-2 py-1.5 border-b">
        <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Explorer
        </span>
        <div className="flex gap-1">
          <Button size="icon" variant="ghost" className="h-6 w-6" aria-label="Search files">
            <Search size={12} />
          </Button>
          <Button
            size="icon"
            variant="ghost"
            className="h-6 w-6"
            onClick={() => refresh()}
            aria-label="Refresh"
            data-testid="refresh-btn"
          >
            <RefreshCw size={12} />
          </Button>
        </div>
      </div>

      {isOffline && (
        <div className="px-2 py-1 text-xs text-yellow-700 bg-yellow-50 border-b">
          Offline — file operations unavailable
        </div>
      )}

      {/* Tree */}
      <div className="flex-1 overflow-y-auto py-1">
        {fileTree ? (
          <FileTreeNode
            node={fileTree}
            depth={0}
            isExpanded={expandedDirs.has(fileTree.path)}
            isSelected={selectedPath === fileTree.path}
            onToggle={toggleDir}
            onSelect={openFile}
            onContextMenu={openContextMenu}
          />
        ) : (
          <div className="p-3 text-sm text-muted-foreground">Loading...</div>
        )}
      </div>
    </div>
  )
}
