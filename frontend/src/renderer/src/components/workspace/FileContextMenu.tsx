// FileContextMenu.tsx — Right-click context menu for file tree nodes (TASK-V5-10)
import {
  ContextMenu, ContextMenuContent, ContextMenuItem,
  ContextMenuSeparator, ContextMenuTrigger,
} from '../ui/context-menu'
import { useWorkspace } from '../../context/WorkspaceContext'
import { callRuntimeRpc } from '../../runtime/runtime-rpc-client'
import type { FileNode } from '@shared/workspace-types'
import { toast } from 'sonner'

type FileContextMenuProps = {
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
    if (!project) {return}
    if (!window.confirm(`Delete "${node.name}"? This cannot be undone.`)) {return}
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
