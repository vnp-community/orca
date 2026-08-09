// useFileExplorer.ts — File explorer state and event subscriptions (TASK-V5-09)
import { useCallback, useEffect, useState } from 'react'
import { useWorkspace } from '../context/WorkspaceContext'

export function useFileExplorer() {
  const { project, fileTree, refreshFileTree, isOffline, on } = useWorkspace()

  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set())
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [viewingFile, setViewingFile]   = useState<string | null>(null)
  const [contextMenu, setContextMenu]   = useState<{
    x: number; y: number; node: any
  } | null>(null)

  // Subscribe to workspace events for auto-refresh
  useEffect(() => {
    const unsubs = [
      on('agent.complete', () => {
        refreshFileTree()
      }),
      on('files.changed', (payload) => {
        const paths = (payload as any)?.paths as string[] ?? []
        const parentDirs = [...new Set(
          paths.map(p => p.includes('/') ? p.split('/').slice(0, -1).join('/') : '.')
        )]
        parentDirs.forEach(dir => refreshFileTree(dir))
      }),
      on('git.commit', () => refreshFileTree()),
      on('worktree.switched', () => refreshFileTree()),
    ]
    return () => unsubs.forEach(u => u())
  }, [on, refreshFileTree])

  const toggleDir = useCallback(async (dirPath: string) => {
    setExpandedDirs(prev => {
      const s = new Set(prev)
      if (s.has(dirPath)) {
        s.delete(dirPath)
      } else {
        s.add(dirPath)
        // Lazy load children (don't await here — WorkspaceContext updates fileTree)
        refreshFileTree(dirPath)
      }
      return s
    })
  }, [refreshFileTree])

  const openFile = useCallback((filePath: string) => {
    setSelectedPath(filePath)
    setViewingFile(filePath)
  }, [])

  const openContextMenu = useCallback((
    e: React.MouseEvent,
    node: any
  ) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, node })
  }, [])

  const closeContextMenu = useCallback(() => setContextMenu(null), [])

  return {
    fileTree,
    expandedDirs,
    selectedPath,
    viewingFile,
    contextMenu,
    isOffline,
    project,
    toggleDir,
    openFile,
    openContextMenu,
    closeContextMenu,
    refresh: refreshFileTree,
  }
}
