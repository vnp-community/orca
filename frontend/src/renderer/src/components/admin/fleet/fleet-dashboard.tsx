// src/renderer/src/components/admin/fleet/fleet-dashboard.tsx
// BUG-FE-FLEET-001-D: Fleet health dashboard — list servers, show health metrics, manage alerts
// Integrates useFleetHealthPolling and FleetImportDialog

import { useState, useCallback } from 'react'
import {
  Server, AlertTriangle, CheckCircle, XCircle,
  RefreshCw, Upload, Loader2, X, WifiOff
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import { useAppStore } from '../../../store'
import { useShallow } from 'zustand/react/shallow'
import { useFleetHealthPolling } from '../../../hooks/use-fleet-health-polling'
import { FleetImportDialog } from './fleet-import-dialog'
import type { SshTarget, FleetAlert, ServerHealthMetrics } from '../../../store/slices/ssh'

/** Native JS alternative to date-fns formatDistanceToNow — no external dependency */
function timeAgo(date: Date | number): string {
  const fmt = new Intl.RelativeTimeFormat('en', { numeric: 'auto' })
  const diffMs = (date instanceof Date ? date.getTime() : date) - Date.now()
  const diffSec = Math.round(diffMs / 1000)
  if (Math.abs(diffSec) < 60)  {return fmt.format(diffSec, 'second')}
  const diffMin = Math.round(diffSec / 60)
  if (Math.abs(diffMin) < 60)  {return fmt.format(diffMin, 'minute')}
  const diffHr  = Math.round(diffMin / 60)
  if (Math.abs(diffHr)  < 24)  {return fmt.format(diffHr, 'hour')}
  return fmt.format(Math.round(diffHr / 24), 'day')
}

// ─── Server Health Row ────────────────────────────────────────────────────────

function HealthIndicator({ metrics }: { metrics: ServerHealthMetrics | undefined }) {
  if (!metrics) {return <Badge variant="secondary" className="text-[10px]">Unknown</Badge>}

  if (!metrics.isReachable) {
    return (
      <div className="flex items-center gap-1">
        <XCircle size={12} className="text-red-500" />
        <span className="text-xs text-red-500">Offline</span>
      </div>
    )
  }

  const isWarning = (metrics.cpuPercent ?? 0) > 85 || (metrics.memPercent ?? 0) > 90
  return (
    <div className="flex items-center gap-2">
      <CheckCircle size={12} className={isWarning ? 'text-yellow-500' : 'text-green-500'} />
      {metrics.cpuPercent !== undefined && (
        <span className="text-[10px] text-muted-foreground">CPU {metrics.cpuPercent}%</span>
      )}
      {metrics.memPercent !== undefined && (
        <span className="text-[10px] text-muted-foreground">MEM {metrics.memPercent}%</span>
      )}
    </div>
  )
}

function ServerRow({ target, metrics }: { target: SshTarget; metrics: ServerHealthMetrics | undefined }) {
  return (
    <div className="flex items-center gap-3 py-2 px-3 hover:bg-muted/40 rounded-md">
      <Server size={14} className="text-muted-foreground shrink-0" />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{target.label ?? target.id}</p>
        {target.hostname && (
          <p className="text-[10px] text-muted-foreground">{target.hostname}</p>
        )}
      </div>
      <HealthIndicator metrics={metrics} />
      {metrics?.relayVersion && (
        <span className="text-[10px] text-muted-foreground">v{metrics.relayVersion}</span>
      )}
    </div>
  )
}

// ─── Alert Banner ─────────────────────────────────────────────────────────────

function AlertBanner({ alert, onDismiss }: { alert: FleetAlert; onDismiss: () => void }) {
  const icons = {
    disconnected: <WifiOff  size={12} className="text-red-500 shrink-0" />,
    error:        <XCircle  size={12} className="text-red-500 shrink-0" />,
    'relay-outdated': <AlertTriangle size={12} className="text-yellow-500 shrink-0" />,
  }

  return (
    <div className={cn(
      'flex items-center gap-2 rounded-md px-3 py-2 text-xs',
      alert.type === 'relay-outdated'
        ? 'bg-yellow-500/10 border border-yellow-500/20'
        : 'bg-red-500/10 border border-red-500/20'
    )}>
      {icons[alert.type] ?? <AlertTriangle size={12} className="shrink-0" />}
      <span className="flex-1">{alert.message}</span>
      <span className="text-muted-foreground shrink-0">
        {timeAgo(alert.timestamp)}
      </span>
      <button onClick={onDismiss} className="shrink-0 hover:opacity-70">
        <X size={12} />
      </button>
    </div>
  )
}

// ─── Main Dashboard ───────────────────────────────────────────────────────────

export function FleetDashboard() {
  const [importOpen, setImportOpen] = useState(false)

  const {
    sshTargets,
    serverHealthMetrics,
    fleetAlerts,
    lastFleetHealthCheck,
    dismissFleetAlert,
    clearDismissedAlerts,
  } = useAppStore(
    useShallow(s => ({
      sshTargets:           s.sshTargets,
      serverHealthMetrics:  s.serverHealthMetrics,
      fleetAlerts:          s.fleetAlerts,
      lastFleetHealthCheck: s.lastFleetHealthCheck,
      dismissFleetAlert:    s.dismissFleetAlert,
      clearDismissedAlerts: s.clearDismissedAlerts,
    }))
  )

  const { isPolling, checkNow } = useFleetHealthPolling({ autoStart: true })

  const activeAlerts = fleetAlerts.filter(a => !a.dismissed)
  const onlineCount  = sshTargets.filter(t => serverHealthMetrics[t.id]?.isReachable).length

  return (
    <TooltipProvider>
      <div className="fleet-dashboard space-y-4 p-4">

        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Fleet</h2>
            <p className="text-xs text-muted-foreground">
              {sshTargets.length} servers
              {sshTargets.length > 0 && ` · ${onlineCount} online`}
              {lastFleetHealthCheck && (
                <> · checked {timeAgo(lastFleetHealthCheck)}</>
              )}
            </p>
          </div>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              className="h-7 gap-1 text-xs"
              onClick={() => setImportOpen(true)}
            >
              <Upload size={12} />
              Import
            </Button>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={checkNow}
                  disabled={isPolling}
                >
                  {isPolling
                    ? <Loader2 size={14} className="animate-spin" />
                    : <RefreshCw size={14} />
                  }
                </Button>
              </TooltipTrigger>
              <TooltipContent>Refresh health metrics</TooltipContent>
            </Tooltip>
          </div>
        </div>

        {/* Alerts */}
        {activeAlerts.length > 0 && (
          <div className="space-y-1">
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs font-medium text-muted-foreground">Alerts ({activeAlerts.length})</span>
              {activeAlerts.length > 1 && (
                <button
                  onClick={clearDismissedAlerts}
                  className="text-[10px] text-muted-foreground hover:text-foreground"
                >
                  Dismiss all
                </button>
              )}
            </div>
            {activeAlerts.slice(0, 5).map(alert => (
              <AlertBanner
                key={alert.id}
                alert={alert}
                onDismiss={() => dismissFleetAlert(alert.id)}
              />
            ))}
          </div>
        )}

        {/* Server List */}
        <div className="rounded-lg border">
          {sshTargets.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-10 gap-2">
              <Server size={32} className="text-muted-foreground" />
              <p className="text-sm text-muted-foreground">No servers in fleet</p>
              <Button
                size="sm"
                variant="outline"
                className="text-xs mt-2"
                onClick={() => setImportOpen(true)}
              >
                <Upload size={12} className="mr-1" />
                Import fleet config
              </Button>
            </div>
          ) : (
            <div className="divide-y">
              {sshTargets.map(target => (
                <ServerRow
                  key={target.id}
                  target={target}
                  metrics={serverHealthMetrics[target.id]}
                />
              ))}
            </div>
          )}
        </div>

        <FleetImportDialog open={importOpen} onOpenChange={setImportOpen} />
      </div>
    </TooltipProvider>
  )
}
