import { useCallback, useEffect, useState } from 'react'
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
import { useAppStore } from '@/store'
import { useAddDevServer } from '../../hooks/useAddDevServer'
import { useDevServers } from '../../store/slices/dev-servers-selectors'
import type { DevServer, DevServerConnectionType } from '../../../../shared/dev-server-types'
import type { SshTarget } from '../../../../shared/ssh-types'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  /** Called when user clicks Continue with ≥1 connected server */
  onNext: () => void
  /** Called when user clicks Skip */
  onSkip: () => void
}

// Why: matches AddDevServerDialog.tsx/FeatureWallConnectDevServerAction.tsx's
// established label set — a fixed 2-option list here silently hid
// direct-websocket, one of DevServerConnectionType's 3 real values.
//
// Direction fix: these two labels previously described the opposite of what
// each mode actually does. Per agent/src/relay/agent-connection-relay.ts
// ("Relay-websocket connection mode: Orca Server connects inbound to
// Agent") and agent-connection-direct.ts ("Direct-websocket connection
// mode: Agent connects outbound to Orca Server") — relay-websocket is Orca
// dialing OUT to the dev server (the dev server only listens);
// direct-websocket is the dev server dialing INTO Orca. Confirmed identical
// in backend-go's devserveragent/session.go and the old TS backend's
// dev-server-relay-bridge.ts/ws-handshake.ts.
const DEV_SERVER_CONNECTION_TYPE_LABELS: Record<DevServerConnectionType, string> = {
  'relay-ssh': 'SSH Relay',
  'relay-websocket': 'WebSocket (Orca → dev server)',
  'direct-websocket': 'WebSocket (dev server → Orca)'
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Onboarding wizard step for adding a remote dev server.
 * Supports relay-ssh, relay-websocket, and direct-websocket connection types.
 *
 * Shows existing connected servers so users who already have a server
 * registered can skip the add form and continue. Layout/primitives mirror
 * FeatureWallConnectDevServerAction.tsx (the other place this same "add a
 * dev server" form lives) rather than raw <label>/<select>/<input> — see
 * docs/STYLEGUIDE.md's form-anatomy section.
 */
export function DevServerStep({ onNext, onSkip }: Props) {
  const { state, testResult, testConnection, addAndConnect, reset } = useAddDevServer()
  const devServers = useDevServers()
  const connectedServers = devServers.filter((ds) => ds.status === 'connected')
  const isAdmin = useAppStore((s) => s.currentUser?.role === 'admin')
  const updateSettings = useAppStore((s) => s.updateSettings)

  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [sshTargetId, setSshTargetId] = useState('')
  // Why direct-websocket by default: this is how a real dev-server agent
  // actually connects (the agent dials into Orca — see deploy/agent) —
  // relay-websocket/relay-ssh require Orca to reach the dev server first,
  // which is the less common path for this deployment's users.
  const [connectionType, setConnectionType] = useState<DevServerConnectionType>('direct-websocket')

  // Why a separate list for direct-websocket: those dev servers are already
  // connected via an agent and admin-approved (CR-DS-006/007/008) — the
  // user picks from what they're allowed to see (their department's grants,
  // or every dev server if they're admin) instead of typing a WebSocket URL
  // that doesn't apply to this mode (the dev server dials Orca, not the
  // other way around).
  const [selectableDevServers, setSelectableDevServers] = useState<DevServer[]>([])
  const [selectableDevServersLoading, setSelectableDevServersLoading] = useState(false)
  const [selectableDevServersError, setSelectableDevServersError] = useState(false)
  const [selectedDevServerId, setSelectedDevServerId] = useState('')

  // Why: relay-ssh picks from already-configured SSH targets (Settings →
  // SSH Hosts) instead of free-typing a target id — see
  // devServer.listSshTargets's doc comment in backend-go/services/
  // api-gateway/internal/adapter/wscompat/channels.go for the wire contract.
  const [sshTargets, setSshTargets] = useState<SshTarget[]>([])
  const [sshTargetsLoading, setSshTargetsLoading] = useState(false)
  const [sshTargetsError, setSshTargetsError] = useState(false)

  // Why: onboarding sits above body-level portals, so every select menu in
  // this step must portal into the overlay to stay clickable — same fix as
  // NotificationStep.tsx/WindowsTerminalStep.tsx's setSelectPortalHost.
  const [selectPortalRoot, setSelectPortalRoot] = useState<HTMLElement | null>(null)
  const setSelectPortalHost = useCallback((node: HTMLDivElement | null) => {
    setSelectPortalRoot(node?.closest<HTMLElement>('[data-onboarding-overlay]') ?? node)
  }, [])

  useEffect(() => {
    if (connectionType !== 'relay-ssh' || sshTargets.length > 0 || sshTargetsLoading) {
      return
    }
    setSshTargetsLoading(true)
    setSshTargetsError(false)
    window.api.devServer
      .listSshTargets()
      .then((targets) => setSshTargets(targets))
      .catch(() => setSshTargetsError(true))
      .finally(() => setSshTargetsLoading(false))
    // sshTargets.length/sshTargetsLoading intentionally excluded — this
    // effect's own guard reads them to decide "already have/fetching data",
    // not to re-trigger on their change.
  }, [connectionType]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (connectionType !== 'direct-websocket') {
      return
    }
    let cancelled = false
    setSelectableDevServersLoading(true)
    setSelectableDevServersError(false)
    // Why the role branch: admin sees every dev server so they can pick
    // any one to test/onboard with; a regular user only sees what their
    // department has been granted — devServer.list() is unfiltered
    // (includes pending_approval/rejected/other connection types), so
    // filter to what's actually usable here either way.
    const fetchServers = isAdmin ? window.api.devServer.list() : window.api.devServer.listForUser()
    fetchServers
      .then((servers) => {
        if (cancelled) {
          return
        }
        setSelectableDevServers(
          servers.filter(
            (s) =>
              s.connectionType === 'direct-websocket' &&
              (s.approvalStatus ?? 'approved') === 'approved'
          )
        )
      })
      .catch(() => {
        if (!cancelled) {
          setSelectableDevServersError(true)
        }
      })
      .finally(() => {
        if (!cancelled) {
          setSelectableDevServersLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [connectionType, isAdmin])

  const selectedSshTargetId = connectionType === 'relay-ssh' ? sshTargetId : undefined
  const canSubmit = connectionType === 'relay-ssh' ? Boolean(sshTargetId) : Boolean(host)

  const handleUseSelectedDevServer = useCallback(() => {
    if (!selectedDevServerId) {
      return
    }
    updateSettings({ activeDevServerId: selectedDevServerId })
      .then(() => onNext())
      .catch((err) => {
        // updateSettings itself never rejects (swallows to console.error) —
        // this only guards against onNext() throwing synchronously inside
        // the .then(), so a failure here is never silent.
        console.error('[DevServerStep] failed to continue with selected dev server:', err)
      })
  }, [selectedDevServerId, updateSettings, onNext])

  const handleTestConnection = async () => {
    await testConnection({
      name: name || 'Dev Server',
      connectionType,
      wsUrl: connectionType === 'relay-ssh' ? undefined : host,
      sshTargetId: selectedSshTargetId
    })
  }

  const handleAddServer = async () => {
    try {
      await addAndConnect({
        name: name || 'Dev Server',
        connectionType,
        wsUrl: connectionType === 'relay-ssh' ? undefined : host,
        sshTargetId: selectedSshTargetId
      })
    } catch {
      // error state is handled by useAddDevServer
    }
  }

  return (
    <div ref={setSelectPortalHost} className="max-w-md space-y-6" data-testid="dev-server-step">
      {/* Existing connected servers */}
      {connectedServers.length > 0 && (
        <div className="space-y-3">
          <p className="text-sm font-medium text-foreground">Connected dev servers</p>
          <div className="space-y-2">
            {connectedServers.map((ds) => (
              <div
                key={ds.id}
                className="flex items-center justify-between rounded-lg border border-border px-4 py-3"
              >
                <span className="truncate text-sm font-medium text-foreground">{ds.name}</span>
                <span className="shrink-0 rounded-full border border-green-500/45 bg-green-500/10 px-2.5 py-1 text-xs font-medium text-green-600 dark:text-green-300">
                  {ds.platform ?? ds.status}
                </span>
              </div>
            ))}
          </div>
          <Button type="button" id="dev-server-continue-btn" onClick={onNext}>
            Continue
          </Button>
        </div>
      )}

      {/* Add new server form */}
      <div className="space-y-4">
        <p className="text-sm font-medium text-foreground">
          {connectionType === 'direct-websocket'
            ? 'Pick a dev server'
            : connectedServers.length > 0
              ? 'Or add another dev server'
              : 'Add a dev server'}
        </p>

        <div className="space-y-1">
          <label
            htmlFor="dev-server-connection-type"
            className="text-sm font-medium text-foreground"
          >
            Connection type
          </label>
          <Select
            value={connectionType}
            onValueChange={(value) => {
              setConnectionType(value as DevServerConnectionType)
              reset()
            }}
          >
            <SelectTrigger id="dev-server-connection-type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent portalContainer={selectPortalRoot}>
              {Object.entries(DEV_SERVER_CONNECTION_TYPE_LABELS).map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {/* Why hidden for direct-websocket: picking an existing, already-
            named dev server has no use for a free-text name field. */}
        {connectionType !== 'direct-websocket' && (
          <div className="space-y-1">
            <label htmlFor="dev-server-name" className="text-sm font-medium text-foreground">
              Server name
            </label>
            <Input
              id="dev-server-name"
              placeholder="e.g. My MacBook Pro"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
        )}

        {connectionType === 'relay-ssh' ? (
          <div className="space-y-1">
            <label htmlFor="dev-server-host" className="text-sm font-medium text-foreground">
              SSH target
            </label>
            {sshTargets.length === 0 && !sshTargetsLoading ? (
              <p className="text-xs text-muted-foreground">
                {sshTargetsError
                  ? 'Could not load SSH targets — check your connection and try again.'
                  : 'No SSH targets configured yet. Add one in Settings → SSH Hosts, then come back here.'}
              </p>
            ) : (
              <Select
                value={sshTargetId}
                onValueChange={(value) => {
                  setSshTargetId(value)
                  reset()
                }}
                disabled={sshTargetsLoading}
              >
                <SelectTrigger id="dev-server-host">
                  <SelectValue
                    placeholder={
                      sshTargetsLoading ? 'Loading SSH targets…' : 'Select an SSH target'
                    }
                  />
                </SelectTrigger>
                <SelectContent portalContainer={selectPortalRoot}>
                  {sshTargets.map((target) => (
                    <SelectItem key={target.id} value={target.id}>
                      {target.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        ) : connectionType === 'direct-websocket' ? (
          <div className="space-y-1">
            <label htmlFor="dev-server-select" className="text-sm font-medium text-foreground">
              Dev server
            </label>
            {selectableDevServers.length === 0 && !selectableDevServersLoading ? (
              <p className="text-xs text-muted-foreground">
                {selectableDevServersError
                  ? 'Could not load dev servers — check your connection and try again.'
                  : isAdmin
                    ? 'No approved dev servers connected yet. Approve one in Settings → Admin console.'
                    : 'No dev servers available for your department yet. Ask an admin to grant access, or file a request from Settings.'}
              </p>
            ) : (
              <Select
                value={selectedDevServerId}
                onValueChange={setSelectedDevServerId}
                disabled={selectableDevServersLoading}
              >
                <SelectTrigger id="dev-server-select">
                  <SelectValue
                    placeholder={
                      selectableDevServersLoading ? 'Loading dev servers…' : 'Select a dev server'
                    }
                  />
                </SelectTrigger>
                <SelectContent portalContainer={selectPortalRoot}>
                  {selectableDevServers.map((server) => (
                    <SelectItem key={server.id} value={server.id}>
                      {server.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        ) : (
          <div className="space-y-1">
            <label htmlFor="dev-server-host" className="text-sm font-medium text-foreground">
              WebSocket URL
            </label>
            <Input
              id="dev-server-host"
              placeholder="ws://devserver.local:6799"
              value={host}
              onChange={(e) => {
                setHost(e.target.value)
                reset()
              }}
            />
          </div>
        )}

        {/* Test result feedback — not applicable to direct-websocket: those
            dev servers are already connected via an agent, nothing to test. */}
        {connectionType !== 'direct-websocket' && testResult && (
          <div
            className={cn(
              'rounded-md p-3 text-sm',
              testResult.ok
                ? 'bg-green-50 text-green-800 dark:bg-green-950 dark:text-green-200'
                : 'bg-destructive/10 text-destructive'
            )}
          >
            {testResult.ok
              ? `✓ Connected — ${testResult.platform} · Node ${testResult.nodeVersion}`
              : `✗ ${testResult.error}${testResult.ok === false && testResult.hint ? ` (${testResult.hint})` : ''}`}
          </div>
        )}

        {connectionType !== 'direct-websocket' && state === 'error' && !testResult && (
          <p className="text-sm text-destructive">Failed to connect. Please check your settings.</p>
        )}

        {connectionType === 'direct-websocket' ? (
          <Button
            type="button"
            id="use-dev-server-btn"
            disabled={!selectedDevServerId}
            onClick={handleUseSelectedDevServer}
          >
            Use this dev server
          </Button>
        ) : (
          <div className="flex gap-2">
            <Button
              type="button"
              id="test-connection-btn"
              variant="secondary"
              disabled={!canSubmit || state === 'testing' || state === 'connecting'}
              onClick={() => void handleTestConnection()}
            >
              {state === 'testing' ? (
                <>
                  <Loader2 className="mr-1 size-4 animate-spin" />
                  Testing…
                </>
              ) : (
                'Test connection'
              )}
            </Button>
            <Button
              type="button"
              id="add-server-btn"
              disabled={!testResult?.ok || state === 'connecting' || state === 'testing'}
              onClick={() => void handleAddServer()}
            >
              {state === 'connecting' ? (
                <>
                  <Loader2 className="mr-1 size-4 animate-spin" />
                  Connecting…
                </>
              ) : (
                'Add dev server'
              )}
            </Button>
          </div>
        )}
      </div>

      {/* Skip */}
      <Button type="button" id="dev-server-skip-btn" variant="ghost" onClick={onSkip}>
        Skip for now
      </Button>
    </div>
  )
}
