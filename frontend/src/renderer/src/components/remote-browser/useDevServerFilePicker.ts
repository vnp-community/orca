// Browsing state/logic for DevServerFilePickerDialog. Split out of the
// dialog component so the dialog file stays focused on markup.
//
// Reuses the already-relay-wired `devServer.browseDir` RPC (backend/src/main/
// runtime/rpc/methods/dev-server.ts, itself calling the agent's existing
// fs.readDir/fs.stat) — the same listing endpoint RemoteFileBrowser.tsx's
// devServerId mode already uses — so no new directory-listing plumbing is
// introduced here.
import { useCallback, useEffect, useRef, useState } from 'react'

export type DevServerPickerEntry = { name: string; isDirectory: boolean }

export type DevServerFilePickerMode = 'directory' | 'file'

type UseDevServerFilePickerArgs = {
  devServerId: string | null
  open: boolean
  initialPath?: string
  mode: DevServerFilePickerMode
  /** Lowercase, no leading dot (e.g. ['png', 'jpg']). Ignored in 'directory' mode. */
  extensions?: string[]
}

type UseDevServerFilePickerResult = {
  currentPath: string
  entries: DevServerPickerEntry[]
  loading: boolean
  error: string | null
  navigate: (path: string) => void
  navigateUp: () => void
  joinCurrent: (name: string) => string
}

function extensionOf(name: string): string {
  const dotIndex = name.lastIndexOf('.')
  return dotIndex === -1 ? '' : name.slice(dotIndex + 1).toLowerCase()
}

function parentPath(path: string): string {
  if (path === '/' || path === '') {
    return '/'
  }
  return path.replace(/\/[^/]+\/?$/, '') || '/'
}

export function useDevServerFilePicker({
  devServerId,
  open,
  initialPath = '~',
  mode,
  extensions
}: UseDevServerFilePickerArgs): UseDevServerFilePickerResult {
  const [currentPath, setCurrentPath] = useState('')
  const [rawEntries, setRawEntries] = useState<DevServerPickerEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // Why: invalidates in-flight browseDir responses from a picker that was
  // closed (or re-opened against a different path) before they resolved.
  const genRef = useRef(0)

  const load = useCallback(
    async (path: string) => {
      if (!devServerId) {
        return
      }
      const gen = ++genRef.current
      setLoading(true)
      setError(null)
      try {
        const result = await window.api.devServer!.browseDir!({ id: devServerId, path })
        if (gen !== genRef.current) {
          return
        }
        setCurrentPath(result.resolvedPath)
        setRawEntries(result.entries)
      } catch (err) {
        if (gen !== genRef.current) {
          return
        }
        setError(err instanceof Error ? err.message : String(err))
        setRawEntries([])
      } finally {
        if (gen === genRef.current) {
          setLoading(false)
        }
      }
    },
    [devServerId]
  )

  useEffect(() => {
    if (open && devServerId) {
      void load(initialPath)
    } else {
      // Why: bump the generation so a load kicked off while open cannot land
      // after the picker closes (or before it reopens on the same devServerId).
      genRef.current++
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- initialPath is a snapshot taken once per open, not a live dependency.
  }, [open, devServerId, load])

  const navigate = useCallback((path: string) => void load(path), [load])

  const navigateUp = useCallback(() => {
    if (!currentPath || currentPath === '/') {
      return
    }
    void load(parentPath(currentPath))
  }, [currentPath, load])

  const joinCurrent = useCallback(
    (name: string) => (currentPath === '/' ? `/${name}` : `${currentPath}/${name}`),
    [currentPath]
  )

  const allowedExtensions =
    mode === 'file' && extensions && extensions.length > 0
      ? new Set(extensions.map((extension) => extension.toLowerCase()))
      : null

  const entries = rawEntries.filter((entry) => {
    if (entry.isDirectory) {
      return true
    }
    if (mode === 'directory') {
      return false
    }
    return !allowedExtensions || allowedExtensions.has(extensionOf(entry.name))
  })

  return { currentPath, entries, loading, error, navigate, navigateUp, joinCurrent }
}
