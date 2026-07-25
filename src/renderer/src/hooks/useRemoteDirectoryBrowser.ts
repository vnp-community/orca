import { useState, useCallback, useEffect } from 'react'

// ─── Types ────────────────────────────────────────────────────────────────────

type DirectoryEntry = {
  name: string
  path: string
  isDirectory: boolean
  isGitRepo: boolean
}

type UseRemoteDirectoryBrowserResult = {
  currentPath: string | null
  entries: DirectoryEntry[]
  loading: boolean
  error: string | null
  platform: NodeJS.Platform | null
  navigate: (path: string) => Promise<void>
  navigateUp: () => void
}

// ─── Hook ─────────────────────────────────────────────────────────────────────

export function useRemoteDirectoryBrowser(
  devServerId: string | null
): UseRemoteDirectoryBrowserResult {
  const [currentPath, setCurrentPath] = useState<string | null>(null)
  const [entries, setEntries] = useState<DirectoryEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [platform, setPlatform] = useState<NodeJS.Platform | null>(null)

  const navigate = useCallback(
    async (path: string) => {
      if (!devServerId) return
      setLoading(true)
      setError(null)
      try {
        const result = await window.api.repo.listRemoteDirectory({
          devServerId,
          path,
          includeGitStatus: true,
        })
        setCurrentPath(path)
        setEntries(result.entries)
        setPlatform(result.platform)
      } catch (err) {
        setError((err as Error).message)
      } finally {
        setLoading(false)
      }
    },
    [devServerId]
  )

  // Navigate up — platform-aware separator
  const navigateUp = useCallback(() => {
    if (!currentPath || !platform) return
    const sep = platform === 'win32' ? '\\' : '/'
    const parts = currentPath.split(sep).filter(Boolean)
    if (parts.length <= 1) return
    parts.pop()
    const parent = (platform === 'win32' ? '' : '/') + parts.join(sep)
    void navigate(parent)
  }, [currentPath, platform, navigate])

  // Init: navigate to workspaceDir or default home
  useEffect(() => {
    if (!devServerId) return
    const { devServers } = (window as any).__store?.getState?.() ?? { devServers: [] }
    const ds = (devServers as Array<{ id: string; workspaceDir: string | null; platform: NodeJS.Platform | null }>)?.find(
      (d) => d.id === devServerId
    )
    const defaultPath = ds?.workspaceDir ?? (ds?.platform === 'win32' ? 'C:\\Users' : '/home')
    void navigate(defaultPath)
  }, [devServerId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { currentPath, entries, loading, error, platform, navigate, navigateUp }
}
