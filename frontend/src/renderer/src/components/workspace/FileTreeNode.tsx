// FileTreeNode.tsx — Recursive file tree node component (TASK-V5-09)
import { ChevronDown, ChevronRight, FileIcon, FolderIcon, Loader2 } from 'lucide-react'
import type { FileNode } from '@shared/workspace-types'
import { cn } from '../../lib/utils'

type FileTreeNodeProps = {
  node:          FileNode
  depth:         number
  isExpanded:    boolean
  isSelected:    boolean
  onToggle:      (path: string) => void
  onSelect:      (path: string) => void
  onContextMenu: (e: React.MouseEvent, node: FileNode) => void
}

export function FileTreeNode({
  node, depth, isExpanded, isSelected, onToggle, onSelect, onContextMenu,
}: FileTreeNodeProps) {
  const indent = depth * 16

  const handleClick = () => {
    if (node.type === 'directory') {onToggle(node.path)}
    else {onSelect(node.path)}
  }

  return (
    <>
      <div
        className={cn(
          'flex items-center gap-1 py-0.5 pr-2 cursor-pointer select-none rounded-sm',
          isSelected && 'bg-accent',
          !isSelected && 'hover:bg-accent/50',
        )}
        style={{ paddingLeft: indent + 4 }}
        onClick={handleClick}
        onContextMenu={e => onContextMenu(e, node)}
        data-testid={`file-node-${node.path}`}
      >
        {node.type === 'directory' ? (
          <>
            {(node as any).isLoading
              ? <Loader2 size={12} className="animate-spin text-muted-foreground" />
              : (isExpanded
                ? <ChevronDown  size={12} className="text-muted-foreground" />
                : <ChevronRight size={12} className="text-muted-foreground" />
              )
            }
            <FolderIcon size={14} className="text-yellow-500 shrink-0" />
          </>
        ) : (
          <FileIcon size={14} className="ml-5 text-gray-400 shrink-0" />
        )}
        <span className="text-sm truncate">{node.name}</span>
        {node.size !== undefined && (
          <span className="ml-auto text-xs text-muted-foreground shrink-0">
            {formatFileSize(node.size)}
          </span>
        )}
      </div>

      {/* Render children recursively if expanded */}
      {node.type === 'directory' && isExpanded && node.children?.map(child => (
        <FileTreeNode
          key={child.path}
          node={child}
          depth={depth + 1}
          isExpanded={false}
          isSelected={false}
          onToggle={onToggle}
          onSelect={onSelect}
          onContextMenu={onContextMenu}
        />
      ))}
    </>
  )
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024)        {return `${bytes}B`}
  if (bytes < 1024 * 1024) {return `${(bytes / 1024).toFixed(1)}K`}
  return `${(bytes / (1024 * 1024)).toFixed(1)}M`
}
