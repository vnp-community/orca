// FleetAlertStrip — dismissable alert bar for fleet disconnect notifications (CR-005, TASK-005-D)
import { AlertTriangle, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import type { FleetAlert } from '@/store/slices/ssh'

function formatRelativeTime(ts: number): string {
  const diff = Math.round((Date.now() - ts) / 1000)
  if (diff < 60) {return `${diff}s ago`}
  if (diff < 3600) {return `${Math.round(diff / 60)}m ago`}
  return `${Math.round(diff / 3600)}h ago`
}

export function FleetAlertStrip({
  alerts
}: {
  alerts: FleetAlert[]
}): React.JSX.Element {
  const dismissAlert = useAppStore((s) => s.dismissFleetAlert)

  // Show max 3 alerts; indicate overflow count
  const visible = alerts.slice(0, 3)
  const overflow = alerts.length - visible.length

  return (
    <div className="space-y-1.5">
      {visible.map((alert) => (
        <div
          key={alert.id}
          className="flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"
        >
          <AlertTriangle className="size-4 flex-shrink-0 text-destructive" />
          <p className="flex-1 text-sm text-destructive">{alert.message}</p>
          <span className="flex-shrink-0 text-xs text-muted-foreground">
            {formatRelativeTime(alert.timestamp)}
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="size-6 flex-shrink-0 text-muted-foreground hover:text-foreground"
            onClick={() => dismissAlert(alert.id)}
          >
            <X className="size-3" />
            <span className="sr-only">
              {translate('fleet.alert.dismiss', 'Dismiss alert')}
            </span>
          </Button>
        </div>
      ))}
      {overflow > 0 && (
        <p className="text-center text-xs text-muted-foreground">
          {translate('fleet.alert.moreAlerts', `+${overflow} more alerts`)}
        </p>
      )}
    </div>
  )
}
