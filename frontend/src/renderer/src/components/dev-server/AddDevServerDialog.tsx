import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { useAddDevServer } from '../../hooks/useAddDevServer'
import type { DevServerConnectionType } from '../../../../shared/dev-server-types'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called after a server is added and connected. */
  onAdded?: (serverId: string) => void
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Modal dialog for adding a new dev server.
 * Supports test-connection before committing.
 */
export function AddDevServerDialog({ open, onOpenChange, onAdded }: Props) {
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [connectionType, setConnectionType] = useState<DevServerConnectionType>('relay-ssh')
  const { state, testResult, testConnection, addAndConnect, reset } = useAddDevServer()

  const handleClose = () => {
    reset()
    setName('')
    setHost('')
    setConnectionType('relay-ssh')
    onOpenChange(false)
  }

  const handleTest = async () => {
    await testConnection({ name, connectionType, wsUrl: host })
  }

  const handleAdd = async () => {
    const server = await addAndConnect({ name, connectionType, wsUrl: host })
    if (server) {
      onAdded?.(server.id)
      handleClose()
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add Dev Server</DialogTitle>
          <DialogDescription>Connect your local machine or a remote workstation.</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Name */}
          <div className="space-y-1">
            <label htmlFor="add-ds-name" className="text-sm font-medium">
              Name
            </label>
            <Input
              id="add-ds-name"
              placeholder="MacBook Pro M3"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          {/* Connection type */}
          <div className="space-y-1">
            <label className="text-sm font-medium">Connection Type</label>
            <Select
              value={connectionType}
              onValueChange={(v) => setConnectionType(v as DevServerConnectionType)}
            >
              <SelectTrigger id="add-ds-connection-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="relay-ssh">SSH Relay</SelectItem>
                <SelectItem value="relay-websocket">WebSocket (Orca → dev server)</SelectItem>
                <SelectItem value="direct-websocket">WebSocket (dev server → Orca)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Host */}
          <div className="space-y-1">
            <label htmlFor="add-ds-host" className="text-sm font-medium">
              {connectionType === 'relay-ssh' ? 'SSH Host / Alias' : 'WebSocket URL'}
            </label>
            <Input
              id="add-ds-host"
              placeholder={
                connectionType === 'relay-ssh' ? 'user@dev.example.com' : 'ws://localhost:6799'
              }
              value={host}
              onChange={(e) => setHost(e.target.value)}
            />
          </div>

          {/* Test result */}
          {testResult && (
            <div
              className={`rounded-md p-3 text-sm ${
                testResult.ok
                  ? 'bg-green-50 text-green-800 dark:bg-green-950 dark:text-green-200'
                  : 'bg-destructive/10 text-destructive'
              }`}
            >
              {testResult.ok
                ? `✓ Connected — ${String(testResult.platform)} · Node ${testResult.nodeVersion}`
                : `✗ ${testResult.error}`}
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={handleClose}>
            Cancel
          </Button>
          <Button
            id="add-ds-test-btn"
            variant="secondary"
            onClick={() => void handleTest()}
            disabled={!host || state === 'testing'}
          >
            {state === 'testing' ? (
              <>
                <Loader2 className="mr-1 size-4 animate-spin" />
                Testing…
              </>
            ) : (
              'Test Connection'
            )}
          </Button>
          <Button
            id="add-ds-confirm-btn"
            onClick={() => void handleAdd()}
            disabled={!testResult?.ok || state === 'connecting'}
          >
            {state === 'connecting' ? (
              <>
                <Loader2 className="mr-1 size-4 animate-spin" />
                Connecting…
              </>
            ) : (
              'Add Server'
            )}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
