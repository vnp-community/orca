import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { useDevServers } from '@/store/slices/dev-servers'
import { useAddDevServer } from '@/hooks/useAddDevServer'
import type { DevServerConnectionType } from '../../../../shared/dev-server-types'
import { translate } from '@/i18n/i18n'

const DEV_SERVER_CONNECTION_TYPE_LABELS: Record<DevServerConnectionType, string> = {
  'relay-ssh': 'SSH Relay',
  'relay-websocket': 'WebSocket (dev server → Orca)',
  'direct-websocket': 'WebSocket (Orca → dev server)'
}

/** Onboarding action for the 'connect-dev-server' setup step — the machine a
 *  user picks here decides which repos/worktrees/agent runtimes show up in
 *  every later step, so this is deliberately the first step in the checklist
 *  (see FEATURE_WALL_SETUP_STEPS). Reuses useAddDevServer(), the same headless
 *  hook AddDevServerDialog.tsx drives, restyled inline (no dialog chrome) to
 *  match this checklist's Tailwind conventions. */
export function FeatureWallConnectDevServerAction(): React.JSX.Element {
  const devServers = useDevServers()
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [connectionType, setConnectionType] = useState<DevServerConnectionType>('relay-ssh')
  const { state, testResult, testConnection, addAndConnect } = useAddDevServer()

  if (devServers.length > 0) {
    return (
      <div className="max-w-3xl space-y-3">
        {devServers.map((server) => (
          <div
            key={server.id}
            className="flex items-center justify-between rounded-lg border border-border px-4 py-3"
          >
            <div className="min-w-0">
              <div className="truncate text-sm font-medium text-foreground">{server.name}</div>
              <div className="text-xs text-muted-foreground">
                {DEV_SERVER_CONNECTION_TYPE_LABELS[server.connectionType]}
              </div>
            </div>
            <span
              className={cn(
                'shrink-0 rounded-full border px-2.5 py-1 text-xs font-medium',
                server.status === 'connected'
                  ? 'border-green-500/45 bg-green-500/10 text-green-600 dark:text-green-300'
                  : 'border-border bg-muted/30 text-muted-foreground'
              )}
            >
              {server.status}
            </span>
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className="max-w-md space-y-4">
      <div className="space-y-1">
        <label htmlFor="setup-dev-server-name" className="text-sm font-medium text-foreground">
          {translate(
            'auto.components.feature.wall.FeatureWallSetupChecklist.devServerName',
            'Name'
          )}
        </label>
        <Input
          id="setup-dev-server-name"
          placeholder="MacBook Pro M3"
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
      </div>
      <div className="space-y-1">
        <label className="text-sm font-medium text-foreground">
          {translate(
            'auto.components.feature.wall.FeatureWallSetupChecklist.devServerConnectionType',
            'Connection Type'
          )}
        </label>
        <Select
          value={connectionType}
          onValueChange={(value) => setConnectionType(value as DevServerConnectionType)}
        >
          <SelectTrigger id="setup-dev-server-connection-type">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(DEV_SERVER_CONNECTION_TYPE_LABELS).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1">
        <label htmlFor="setup-dev-server-host" className="text-sm font-medium text-foreground">
          {connectionType === 'relay-ssh'
            ? translate(
                'auto.components.feature.wall.FeatureWallSetupChecklist.devServerSshHost',
                'SSH Host / Alias'
              )
            : translate(
                'auto.components.feature.wall.FeatureWallSetupChecklist.devServerWsUrl',
                'WebSocket URL'
              )}
        </label>
        <Input
          id="setup-dev-server-host"
          placeholder={
            connectionType === 'relay-ssh' ? 'user@dev.example.com' : 'ws://localhost:6799'
          }
          value={host}
          onChange={(event) => setHost(event.target.value)}
        />
      </div>
      {testResult ? (
        <div
          className={cn(
            'rounded-md p-3 text-sm',
            testResult.ok
              ? 'bg-green-50 text-green-800 dark:bg-green-950 dark:text-green-200'
              : 'bg-destructive/10 text-destructive'
          )}
        >
          {testResult.ok
            ? `✓ Connected — ${String(testResult.platform)} · Node ${testResult.nodeVersion}`
            : `✗ ${testResult.error}`}
        </div>
      ) : null}
      <div className="flex gap-2">
        <Button
          type="button"
          variant="secondary"
          disabled={!host || state === 'testing'}
          onClick={() => void testConnection({ name, connectionType, wsUrl: host })}
        >
          {state === 'testing' ? (
            <>
              <Loader2 className="mr-1 size-4 animate-spin" />
              {translate(
                'auto.components.feature.wall.FeatureWallSetupChecklist.devServerTesting',
                'Testing…'
              )}
            </>
          ) : (
            translate(
              'auto.components.feature.wall.FeatureWallSetupChecklist.devServerTest',
              'Test Connection'
            )
          )}
        </Button>
        <Button
          type="button"
          disabled={!testResult?.ok || state === 'connecting'}
          onClick={() => void addAndConnect({ name, connectionType, wsUrl: host })}
        >
          {state === 'connecting' ? (
            <>
              <Loader2 className="mr-1 size-4 animate-spin" />
              {translate(
                'auto.components.feature.wall.FeatureWallSetupChecklist.devServerConnecting',
                'Connecting…'
              )}
            </>
          ) : (
            translate(
              'auto.components.feature.wall.FeatureWallSetupChecklist.devServerAdd',
              'Add Server'
            )
          )}
        </Button>
      </div>
    </div>
  )
}
