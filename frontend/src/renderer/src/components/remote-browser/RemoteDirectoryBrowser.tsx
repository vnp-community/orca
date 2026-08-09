import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useRemoteDirectoryBrowser } from '../../hooks/useRemoteDirectoryBrowser'
import { RemoteDirectoryEntry } from './RemoteDirectoryEntry'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  devServerId: string
  onSelect: (path: string) => void
}

// ─── Component ───────────────────────────────────────────────────────────────

/**
 * Remote filesystem browser for selecting a directory on a dev server.
 * Supports breadcrumb navigation, up-traversal, and manual path entry.
 */
export function RemoteDirectoryBrowser({ devServerId, onSelect }: Props) {
  const { currentPath, entries, loading, error, platform, navigate, navigateUp } =
    useRemoteDirectoryBrowser(devServerId)
  const [manualPath, setManualPath] = useState('')

  return (
    <div className="remote-dir-browser">
      {/* Toolbar */}
      <div className="remote-dir-browser__toolbar">
        <Button
          variant="ghost"
          size="sm"
          onClick={navigateUp}
          disabled={!currentPath}
          aria-label="Navigate up"
        >
          ↑ Up
        </Button>
        <code className="remote-dir-browser__path">{currentPath ?? '…'}</code>
      </div>

      {/* Manual path input */}
      <div className="remote-dir-browser__manual">
        <Input
          id="manual-path-input"
          placeholder={platform === 'win32' ? 'C:\\path\\to\\project' : '/home/user/projects'}
          value={manualPath}
          onChange={(e) => setManualPath(e.target.value)}
        />
        <Button
          id="manual-path-add-btn"
          onClick={() => onSelect(manualPath)}
          disabled={!manualPath}
        >
          Use this path
        </Button>
      </div>

      {loading && (
        <div className="remote-dir-browser__loading">
          <Loader2 className="animate-spin" />
          Loading…
        </div>
      )}
      {error && <p className="remote-dir-browser__error">{error}</p>}

      {/* Directory listing */}
      <div className="remote-dir-browser__list" role="list">
        {entries.map((entry) => (
          <RemoteDirectoryEntry
            key={entry.path}
            entry={entry}
            onNavigate={() => void navigate(entry.path)}
            onSelect={() => onSelect(entry.path)}
          />
        ))}
        {!loading && entries.length === 0 && (
          <p className="remote-dir-browser__empty">No directories found</p>
        )}
      </div>
    </div>
  )
}
