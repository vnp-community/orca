import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RemoteDirectoryBrowser } from '../remote-browser/RemoteDirectoryBrowser'
import { useConnectedDevServers } from '../../store/slices/dev-servers-selectors'
import { useAppStore } from '../../store'
import { addRuntimeRepoRemote } from '../../runtime/runtime-repo-client'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  activeDevServerId: string | null
  onRepoAdded: (repoId: string) => void
}

type Mode = 'browse' | 'clone' | 'scan'

type ScannedRepo = {
  path: string
  name: string
}

// ─── Component ───────────────────────────────────────────────────────────────

/**
 * Onboarding step for adding a repository from a remote dev server.
 * Supports three modes: Browse filesystem, Clone URL, Scan for repos.
 */
export function AddRepoStep({ activeDevServerId, onRepoAdded }: Props) {
  const [mode, setMode] = useState<Mode>('browse')
  const [selectedServerId, setSelectedServerId] = useState<string | null>(activeDevServerId)
  const connectedServers = useConnectedDevServers()

  // Clone state
  const [cloneUrl, setCloneUrl] = useState('')
  const [cloning, setCloning] = useState(false)
  const [cloneError, setCloneError] = useState<string | null>(null)

  // Scan state
  const [scannedRepos, setScannedRepos] = useState<ScannedRepo[]>([])
  const [scanning, setScanning] = useState(false)
  const [selectedScanPaths, setSelectedScanPaths] = useState<Set<string>>(new Set())
  const [addingError, setAddingError] = useState<string | null>(null)

  const effectiveDevServerId = selectedServerId ?? activeDevServerId

  // ── No dev server ────────────────────────────────────────────────────────────

  if (!effectiveDevServerId) {
    return (
      <div className="add-repo-step__no-server">
        <p>Connect a dev server first to add repositories.</p>
        <Button variant="outline" onClick={() => window.history.back()}>
          Go back
        </Button>
      </div>
    )
  }

  // ── Dev server selector (multiple servers) ────────────────────────────────────

  const serverSelector = connectedServers.length > 1 && (
    <div className="add-repo-step__server-select">
      <label htmlFor="dev-server-select">Dev server</label>
      <select
        id="dev-server-select"
        value={effectiveDevServerId}
        onChange={(e) => setSelectedServerId(e.target.value)}
      >
        {connectedServers.map((ds) => (
          <option key={ds.id} value={ds.id}>
            {ds.name}
          </option>
        ))}
      </select>
    </div>
  )

  // ── Browse mode ────────────────────────────────────────────────────────────────

  const handleBrowseSelect = async (path: string) => {
    try {
      const result = await addRuntimeRepoRemote(useAppStore.getState().settings, {
        connectionId: effectiveDevServerId,
        remotePath: path
      })
      if ('error' in result) {
        setAddingError(result.error)
      } else {
        onRepoAdded(result.repo.id)
        // [CR-OB-009] Mark addedRepo per-server checklist
        useAppStore.getState().markServerChecklistItem(effectiveDevServerId, 'addedRepo')
      }
    } catch (err) {
      setAddingError((err as Error).message)
    }
  }

  // ── Clone mode ─────────────────────────────────────────────────────────────────

  const handleClone = async () => {
    setCloning(true)
    setCloneError(null)
    try {
      const repo = await window.api.repos.cloneRemote({
        connectionId: effectiveDevServerId,
        url: cloneUrl,
        destination: ''
      })
      onRepoAdded(repo.id)
    } catch (err) {
      setCloneError((err as Error).message)
    } finally {
      setCloning(false)
    }
  }

  // ── Scan mode ──────────────────────────────────────────────────────────────────

  const handleScan = async () => {
    setScanning(true)
    try {
      const repos = await window.api.repos.scanRemote({
        devServerId: effectiveDevServerId,
        rootPath: '/home'
      })
      setScannedRepos(repos)
    } catch {
      /* non-fatal */
    } finally {
      setScanning(false)
    }
  }

  const handleAddSelected = async () => {
    setAddingError(null)
    for (const path of selectedScanPaths) {
      try {
        const result = await addRuntimeRepoRemote(useAppStore.getState().settings, {
          connectionId: effectiveDevServerId,
          remotePath: path
        })
        if ('error' in result) {
          setAddingError(result.error)
          return
        }
        onRepoAdded(result.repo.id)
      } catch (err) {
        setAddingError((err as Error).message)
        return
      }
    }
  }

  // ── Render ─────────────────────────────────────────────────────────────────────

  return (
    <div className="add-repo-step">
      {serverSelector}

      {/* Mode tabs */}
      <div className="add-repo-step__tabs" role="tablist">
        {(['browse', 'clone', 'scan'] as Mode[]).map((m) => (
          <button
            key={m}
            type="button"
            role="tab"
            aria-selected={mode === m}
            id={`add-repo-tab-${m}`}
            className={`add-repo-step__tab${mode === m ? ' add-repo-step__tab--active' : ''}`}
            onClick={() => setMode(m)}
          >
            {m.charAt(0).toUpperCase() + m.slice(1)}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="add-repo-step__content">
        {mode === 'browse' && (
          <>
            <RemoteDirectoryBrowser
              devServerId={effectiveDevServerId}
              onSelect={(path) => void handleBrowseSelect(path)}
            />
            {addingError && <p className="add-repo-step__error">{addingError}</p>}
          </>
        )}

        {mode === 'clone' && (
          <div className="add-repo-step__clone">
            <label htmlFor="clone-url-input">Repository URL</label>
            <Input
              id="clone-url-input"
              placeholder="https://github.com/org/repo"
              value={cloneUrl}
              onChange={(e) => setCloneUrl(e.target.value)}
            />
            {cloneError && <p className="add-repo-step__error">{cloneError}</p>}
            <Button
              id="clone-btn"
              onClick={() => void handleClone()}
              disabled={!cloneUrl || cloning}
            >
              {cloning ? (
                <>
                  <Loader2 className="mr-2 size-4 animate-spin" />
                  Cloning…
                </>
              ) : (
                'Clone'
              )}
            </Button>
          </div>
        )}

        {mode === 'scan' && (
          <div className="add-repo-step__scan">
            <Button id="scan-btn" onClick={() => void handleScan()} disabled={scanning}>
              {scanning ? (
                <>
                  <Loader2 className="mr-2 size-4 animate-spin" />
                  Scanning…
                </>
              ) : (
                'Scan for git repos'
              )}
            </Button>

            {scannedRepos.length > 0 && (
              <>
                <ul className="add-repo-step__scan-list">
                  {scannedRepos.map((repo) => (
                    <li key={repo.path}>
                      <label>
                        <input
                          type="checkbox"
                          checked={selectedScanPaths.has(repo.path)}
                          onChange={(e) => {
                            const next = new Set(selectedScanPaths)
                            if (e.target.checked) {
                              next.add(repo.path)
                            } else {
                              next.delete(repo.path)
                            }
                            setSelectedScanPaths(next)
                          }}
                        />
                        {repo.name}
                        <span className="add-repo-step__scan-path">{repo.path}</span>
                      </label>
                    </li>
                  ))}
                </ul>
                <Button
                  id="add-selected-btn"
                  onClick={() => void handleAddSelected()}
                  disabled={selectedScanPaths.size === 0}
                >
                  Add selected repos ({selectedScanPaths.size})
                </Button>
                {addingError && <p className="add-repo-step__error">{addingError}</p>}
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
