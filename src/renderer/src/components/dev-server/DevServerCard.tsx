import { Loader2, Trash2, Plug, PlugZap } from 'lucide-react'
import { useState } from 'react'
import type { DevServer } from '../../../../shared/dev-server-types'
import { DevServerStatusBadge } from './DevServerStatusBadge'
import { Button } from '@/components/ui/button'
import { useAppStore } from '../../store'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  server: DevServer
  /** Show connect/disconnect controls. Default: true */
  showActions?: boolean
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Card displaying a single dev server with status, connection controls, and
 * a remove button.
 */
export function DevServerCard({ server, showActions = true }: Props) {
  const [connecting, setConnecting] = useState(false)
  const updateDevServerStatus = useAppStore((s) => s.updateDevServerStatus)
  const removeDevServer = useAppStore((s) => s.removeDevServer)

  const handleConnect = async () => {
    setConnecting(true)
    try {
      await window.api.devServer.connect(server.id)
    } finally {
      setConnecting(false)
    }
  }

  const handleDisconnect = async () => {
    setConnecting(true)
    try {
      await window.api.devServer.disconnect(server.id)
    } finally {
      setConnecting(false)
    }
  }

  const handleRemove = async () => {
    await window.api.devServer.remove(server.id)
    removeDevServer(server.id)
  }

  return (
    <div className="dev-server-card" data-server-id={server.id}>
      {/* Header */}
      <div className="dev-server-card__header">
        <div className="dev-server-card__info">
          <span className="dev-server-card__name">{server.name}</span>
          <DevServerStatusBadge status={server.status} platform={server.platform} />
        </div>

        {showActions && (
          <div className="dev-server-card__actions">
            {server.status === 'connected' ? (
              <Button
                variant="ghost"
                size="sm"
                id={`disconnect-btn-${server.id}`}
                onClick={() => void handleDisconnect()}
                disabled={connecting}
                title="Disconnect"
              >
                {connecting ? <Loader2 className="size-4 animate-spin" /> : <PlugZap className="size-4" />}
              </Button>
            ) : (
              <Button
                variant="ghost"
                size="sm"
                id={`connect-btn-${server.id}`}
                onClick={() => void handleConnect()}
                disabled={connecting || server.status === 'connecting'}
                title="Connect"
              >
                {connecting ? <Loader2 className="size-4 animate-spin" /> : <Plug className="size-4" />}
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              id={`remove-btn-${server.id}`}
              onClick={() => void handleRemove()}
              title="Remove"
            >
              <Trash2 className="size-4 text-destructive" />
            </Button>
          </div>
        )}
      </div>

      {/* Meta */}
      <div className="dev-server-card__meta">
        {server.wsUrl && (
          <span className="dev-server-card__url">{server.wsUrl}</span>
        )}
        {server.lastError && (
          <span className="dev-server-card__error">{server.lastError}</span>
        )}
        {server.arch && server.nodeVersion && (
          <span className="dev-server-card__runtime">
            {server.arch} · Node {server.nodeVersion}
          </span>
        )}
      </div>
    </div>
  )
}
