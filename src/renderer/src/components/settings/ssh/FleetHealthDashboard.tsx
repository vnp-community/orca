// FleetHealthDashboard — fleet health monitoring panel (CR-005, TASK-005-C)
import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useFleetHealthPolling } from '@/hooks/useFleetHealthPolling'
import { FleetSummaryCard } from './FleetSummaryCard'
import { FleetAlertStrip } from './FleetAlertStrip'
import { FleetHealthTable } from './FleetHealthTable'

function formatRelativeTime(ts: number): string {
  const diffSec = Math.round((Date.now() - ts) / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.round(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m ago`
  return `${Math.round(diffMin / 60)}h ago`
}

export function FleetHealthDashboard(): React.JSX.Element {
  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  // Why: connectionStates is a Map<string, SshConnectionState> — use .get() in child components
  const connectionStates = useAppStore((s) => s.sshConnectionStates)
  const healthMetrics = useAppStore((s) => s.serverHealthMetrics)
  const lastCheck = useAppStore((s) => s.lastFleetHealthCheck)
  const alerts = useAppStore((s) => s.fleetAlerts.filter((a) => !a.dismissed))

  // Enable polling while this dashboard is mounted
  useFleetHealthPolling(true)

  // Summary stats derived from Map.get()
  const summary = useMemo(() => {
    let connected = 0
    let error = 0
    for (const target of sshTargets) {
      const status = connectionStates.get(target.id)?.status
      if (status === 'connected') connected++
      else if (status === 'error' || status === 'reconnection-failed' || status === 'auth-failed') error++
    }
    return {
      total: sshTargets.length,
      connected,
      error,
      disconnected: sshTargets.length - connected - error
    }
  }, [sshTargets, connectionStates])

  return (
    <div className="space-y-4">
      {/* Alerts strip — only visible when there are undismissed alerts */}
      {alerts.length > 0 && <FleetAlertStrip alerts={alerts} />}

      {/* Summary stat cards */}
      <div className="grid grid-cols-4 gap-3">
        <FleetSummaryCard
          label={translate('fleet.health.total', 'Total')}
          value={summary.total}
          variant="default"
        />
        <FleetSummaryCard
          label={translate('fleet.health.connected', 'Connected')}
          value={summary.connected}
          variant="success"
        />
        <FleetSummaryCard
          label={translate('fleet.health.disconnected', 'Offline')}
          value={summary.disconnected}
          variant="warning"
        />
        <FleetSummaryCard
          label={translate('fleet.health.error', 'Error')}
          value={summary.error}
          variant="destructive"
        />
      </div>

      {/* Last poll info + manual refresh */}
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {lastCheck ? (
          <span>
            {translate('fleet.health.lastCheck', 'Last checked')}: {formatRelativeTime(lastCheck)}
          </span>
        ) : (
          <span>{translate('fleet.health.checking', 'Checking...')}</span>
        )}
        <Button
          variant="ghost"
          size="sm"
          className="h-5 px-2 text-xs"
          onClick={() => void window.api.ssh.refreshFleetHealth?.()}
        >
          {translate('fleet.health.refresh', 'Refresh now')}
        </Button>
      </div>

      {/* Per-server health table */}
      <FleetHealthTable
        targets={sshTargets}
        connectionStates={connectionStates}
        healthMetrics={healthMetrics}
      />
    </div>
  )
}
