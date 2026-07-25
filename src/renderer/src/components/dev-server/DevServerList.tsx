import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useDevServers } from '../../store/slices/dev-servers'
import { DevServerCard } from './DevServerCard'
import { AddDevServerDialog } from './AddDevServerDialog'
import { Button } from '@/components/ui/button'

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Full list of configured dev servers with add/remove/connect actions.
 * Used in Settings → Dev Servers panel.
 */
export function DevServerList() {
  const devServers = useDevServers()
  const [addDialogOpen, setAddDialogOpen] = useState(false)

  return (
    <div className="dev-server-list">
      <div className="dev-server-list__header">
        <h3 className="dev-server-list__title">Dev Servers</h3>
        <Button
          id="add-dev-server-btn"
          size="sm"
          onClick={() => setAddDialogOpen(true)}
        >
          <Plus className="mr-1 size-4" />
          Add Server
        </Button>
      </div>

      {devServers.length === 0 ? (
        <div className="dev-server-list__empty">
          <p>No dev servers configured.</p>
          <p className="text-sm text-muted-foreground">
            Add a server to run agents on a remote machine.
          </p>
        </div>
      ) : (
        <div className="dev-server-list__items">
          {devServers.map((server) => (
            <DevServerCard key={server.id} server={server} />
          ))}
        </div>
      )}

      <AddDevServerDialog
        open={addDialogOpen}
        onOpenChange={setAddDialogOpen}
      />
    </div>
  )
}
