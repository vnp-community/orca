import { Button } from '@/components/ui/button'

// ─── Types ────────────────────────────────────────────────────────────────────

type DirectoryEntry = {
  name: string
  path: string
  isDirectory: boolean
  isGitRepo: boolean
}

type Props = {
  entry: DirectoryEntry
  onNavigate: () => void
  onSelect: () => void
}

// ─── Component ───────────────────────────────────────────────────────────────

/**
 * A single row in the RemoteDirectoryBrowser listing.
 * Git repositories are visually distinguished from plain directories.
 */
export function RemoteDirectoryEntry({ entry, onNavigate, onSelect }: Props) {
  return (
    <div
      className={`remote-dir-entry${entry.isGitRepo ? ' remote-dir-entry--git' : ''}`}
      role="listitem"
    >
      <button
        type="button"
        className="remote-dir-entry__name"
        onClick={onNavigate}
        title={entry.path}
      >
        <span aria-hidden="true">{entry.isGitRepo ? '📂' : '📁'}</span>
        {entry.name}
        {entry.isGitRepo && (
          <span
            className="remote-dir-entry__git-dot"
            title="Git repository"
            aria-label="Git repository"
          />
        )}
      </button>
      <Button variant="ghost" size="sm" onClick={onSelect}>
        Select
      </Button>
    </div>
  )
}
