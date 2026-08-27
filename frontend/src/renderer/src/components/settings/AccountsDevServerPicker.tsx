// AccountsDevServerPicker.tsx — TASK-023's dev-server picker for accounts.*.
// A tenant can own 0..N live infra-fleet-service connections; accounts.*
// (select/remove Claude/Codex accounts, accounts.subscribe) has no way to
// guess which one the user means, so the user picks explicitly here. The
// pick is persisted as a local UI preference (accounts-dev-server-connection.ts)
// and resolved to a live connectionId by runtime-provider-accounts-client.ts
// at call time — this component only surfaces the picker and the resulting
// connected/disconnected state so AccountsPane can disable account actions
// with a clear reason instead of silently attempting a doomed RPC.
import { useEffect, useState } from 'react'
import type { GlobalSettings } from '../../../../shared/types'
import { getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import {
  getPreferredAccountsDevServerId,
  listAccountsDevServers,
  resolveAccountsDevServerConnection,
  setPreferredAccountsDevServerId,
  type AccountsDevServerOption
} from '../../runtime/accounts-dev-server-connection'
import { Label } from '../ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select'
import { translate } from '@/i18n/i18n'

type AccountsDevServerPickerProps = {
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>
  // Why a callback, not a returned value: AccountsPane needs this component's
  // resolved readiness to gate account select/remove controls that live
  // elsewhere in that file's JSX tree.
  onReadyChange: (ready: boolean, reason: string | null) => void
}

type ConnectionCheckState = 'checking' | 'connected' | 'disconnected' | 'error'

export function AccountsDevServerPicker({
  settings,
  onReadyChange
}: AccountsDevServerPickerProps): React.JSX.Element | null {
  const target = getActiveRuntimeTarget(settings)
  const environmentId = target.kind === 'environment' ? target.environmentId : null
  const [devServers, setDevServers] = useState<AccountsDevServerOption[]>([])
  const [devServersLoading, setDevServersLoading] = useState(true)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [connectionState, setConnectionState] = useState<ConnectionCheckState>('checking')
  const [connectionError, setConnectionError] = useState<string | null>(null)

  useEffect(() => {
    if (target.kind !== 'environment') {
      return
    }
    let cancelled = false
    setDevServersLoading(true)
    listAccountsDevServers(target)
      .then((list) => {
        if (cancelled) {
          return
        }
        setDevServers(list)
        const preferred = getPreferredAccountsDevServerId(target.environmentId)
        setSelectedId(preferred && list.some((server) => server.id === preferred) ? preferred : null)
      })
      .catch(() => {
        if (!cancelled) {
          setDevServers([])
        }
      })
      .finally(() => {
        if (!cancelled) {
          setDevServersLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
    // Why: re-list only when the active environment changes, not on every
    // render — `target` is a fresh object each render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [environmentId])

  useEffect(() => {
    if (target.kind !== 'environment') {
      return
    }
    if (!selectedId) {
      setConnectionState('disconnected')
      onReadyChange(
        false,
        translate(
          'components.settings.AccountsDevServerPicker.pickReason',
          'Pick a dev server to manage its accounts.'
        )
      )
      return
    }
    let cancelled = false
    setConnectionState('checking')
    setConnectionError(null)
    resolveAccountsDevServerConnection(target, selectedId)
      .then((resolution) => {
        if (cancelled) {
          return
        }
        if (resolution.connected) {
          setConnectionState('connected')
          onReadyChange(true, null)
        } else {
          setConnectionState('disconnected')
          onReadyChange(
            false,
            translate(
              'components.settings.AccountsDevServerPicker.notConnectedReason',
              'This dev server is not currently connected.'
            )
          )
        }
      })
      .catch((error: unknown) => {
        if (cancelled) {
          return
        }
        const message = String((error as Error)?.message ?? error)
        setConnectionState('error')
        setConnectionError(message)
        onReadyChange(false, message)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId])

  if (target.kind !== 'environment' || !environmentId) {
    return null
  }

  const handleChange = (id: string): void => {
    setPreferredAccountsDevServerId(environmentId, id)
    setSelectedId(id)
  }

  return (
    <section className="space-y-3 scroll-mt-6">
      <div className="space-y-1">
        <h3 className="text-sm font-semibold">
          {translate('components.settings.AccountsDevServerPicker.title', 'Dev server')}
        </h3>
        <p className="text-xs text-muted-foreground">
          {translate(
            'components.settings.AccountsDevServerPicker.description',
            "Choose which dev server's Claude and Codex accounts to manage."
          )}
        </p>
      </div>

      {devServersLoading ? (
        <p className="text-xs text-muted-foreground">
          {translate('components.settings.AccountsDevServerPicker.loading', 'Loading dev servers…')}
        </p>
      ) : devServers.length === 0 ? (
        <div className="rounded-md border border-dashed border-border/70 px-3 py-4 text-xs text-muted-foreground">
          {translate(
            'components.settings.AccountsDevServerPicker.empty',
            'No dev servers available. Add one before managing its accounts.'
          )}
        </div>
      ) : (
        <div className="space-y-2">
          <Label>
            {translate('components.settings.AccountsDevServerPicker.selectLabel', 'Dev server')}
          </Label>
          <Select value={selectedId ?? undefined} onValueChange={handleChange}>
            <SelectTrigger className="w-full">
              <SelectValue
                placeholder={translate(
                  'components.settings.AccountsDevServerPicker.placeholder',
                  'Select a dev server'
                )}
              />
            </SelectTrigger>
            <SelectContent>
              {devServers.map((server) => (
                <SelectItem key={server.id} value={server.id}>
                  {server.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {selectedId && connectionState === 'disconnected' ? (
            <p className="text-xs text-destructive">
              {translate(
                'components.settings.AccountsDevServerPicker.notConnected',
                'This dev server is not currently connected. Accounts cannot be managed until it reconnects.'
              )}
            </p>
          ) : null}
          {connectionState === 'error' ? (
            <p className="text-xs text-destructive">
              {translate(
                'components.settings.AccountsDevServerPicker.checkFailed',
                'Could not check this dev server’s connection: {{value0}}',
                { value0: connectionError ?? '' }
              )}
            </p>
          ) : null}
        </div>
      )}
    </section>
  )
}
