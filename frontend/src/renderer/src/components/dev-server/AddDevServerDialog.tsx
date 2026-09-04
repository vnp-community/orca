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
import type { AgentKind, DevServerConnectionType } from '../../../../shared/dev-server-types'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called after a server is added and connected. */
  onAdded?: (serverId: string) => void
  /**
   * CR-DS-009 / TASK-EMU-012c: preselects the agent kind. Callers that only
   * ever add Mobile Emulator Agents (e.g. a future dedicated entry point in
   * MobileEmulatorSettingsPane) pass 'mobile-emulator' so the dialog opens
   * straight to that mode instead of making the user switch the Select.
   */
  initialKind?: AgentKind
}

const AGENT_KIND_LABELS: Record<AgentKind, string> = {
  'dev-server': 'Dev Server (code, git, terminals)',
  'mobile-emulator': 'Mobile Emulator Agent (Android/iOS device control)'
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Modal dialog for adding a new dev server OR Mobile Emulator Agent — both
 * register through the same DevServer registry, distinguished only by
 * `kind` (CR-DS-009 / TASK-EMU-007). Supports test-connection before
 * committing.
 */
export function AddDevServerDialog({ open, onOpenChange, onAdded, initialKind }: Props) {
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [connectionType, setConnectionType] = useState<DevServerConnectionType>('relay-ssh')
  const [kind, setKind] = useState<AgentKind>(initialKind ?? 'dev-server')
  const { state, testResult, testConnection, addAndConnect, reset } = useAddDevServer()

  const handleClose = () => {
    reset()
    setName('')
    setHost('')
    setConnectionType('relay-ssh')
    setKind(initialKind ?? 'dev-server')
    onOpenChange(false)
  }

  const handleTest = async () => {
    await testConnection({ name, connectionType, wsUrl: host, kind })
  }

  const handleAdd = async () => {
    const server = await addAndConnect({ name, connectionType, wsUrl: host, kind })
    if (server) {
      onAdded?.(server.id)
      handleClose()
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {kind === 'mobile-emulator' ? 'Add Mobile Emulator Agent' : 'Add Dev Server'}
          </DialogTitle>
          <DialogDescription>
            {kind === 'mobile-emulator'
              ? "Connect a machine with Android Studio and/or Xcode installed — often your own laptop, independent from the dev server running this project's code."
              : 'Connect your local machine or a remote workstation.'}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          {/* Agent kind — CR-DS-009: same registry, distinguished by kind */}
          <div className="space-y-1">
            <label className="text-sm font-medium">Type</label>
            <Select value={kind} onValueChange={(v) => setKind(v as AgentKind)}>
              <SelectTrigger id="add-ds-kind">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(AGENT_KIND_LABELS) as AgentKind[]).map((k) => (
                  <SelectItem key={k} value={k}>
                    {AGENT_KIND_LABELS[k]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Name */}
          <div className="space-y-1">
            <label htmlFor="add-ds-name" className="text-sm font-medium">
              Name
            </label>
            <Input
              id="add-ds-name"
              placeholder={kind === 'mobile-emulator' ? 'My MacBook (Xcode)' : 'MacBook Pro M3'}
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

          {/* direct-websocket run instructions — CR-DS-009 / TASK-EMU-012c:
              no published binary to curl|bash yet (no npm publish / GitHub
              release for emulator/ or agent/), so this shows the real
              build-from-source command instead of fabricating an installer.
              AgentTokenPanel (unwired to a live token event for either
              agent kind — a pre-existing gap, not introduced here) would
              replace this once that gap is closed. */}
          {connectionType === 'direct-websocket' ? (
            <div className="rounded-md border bg-muted/50 p-3 text-xs text-muted-foreground space-y-1">
              <p className="font-medium text-foreground">
                Run on {kind === 'mobile-emulator' ? 'the machine with Android Studio/Xcode' : 'the dev server'}:
              </p>
              <pre className="whitespace-pre-wrap break-all font-mono">
                {kind === 'mobile-emulator'
                  ? 'cd emulator && pnpm install && node build.mjs\nORCA_BACKEND_URL=<url> ORCA_AGENT_TOKEN=<token> node out/emulator.js'
                  : 'cd agent && pnpm install && node build.mjs\nORCA_URL=<url> AGENT_TOKEN=<token> node out/agent.js'}
              </pre>
            </div>
          ) : null}

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
              kind === 'mobile-emulator' ? 'Add Agent' : 'Add Server'
            )}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
