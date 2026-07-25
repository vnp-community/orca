// ProvisionProgressPanel — real-time per-server provisioning status (CR-003, TASK-003-D)
import {
  CheckCircle,
  XCircle,
  Loader,
  Clock,
  UploadCloud,
  MinusCircle
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Progress } from '@/components/ui/progress'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { translate } from '@/i18n/i18n'
import type {
  ProvisioningSession,
  ProvisioningServerEntry
} from '@/store/slices/provisioning'

export function ProvisionProgressPanel({
  session
}: {
  session: ProvisioningSession
}): React.JSX.Element {
  const finishedCount = session.servers.filter((s) =>
    ['done', 'error', 'skipped'].includes(s.status)
  ).length
  const doneCount = session.servers.filter((s) => s.status === 'done').length
  const failCount = session.servers.filter((s) => s.status === 'error').length
  const total = session.servers.length
  const progress = total > 0 ? Math.round((finishedCount / total) * 100) : 0

  return (
    <div className="space-y-4">
      {/* ── Overall progress ── */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {translate('fleet.provision.inProgress', 'Provisioning in progress...')}
          </span>
          <span className="font-medium tabular-nums">{progress}%</span>
        </div>
        <Progress value={progress} className="h-2" />
        <div className="flex gap-4 text-xs text-muted-foreground">
          <span className="text-green-600">✓ {doneCount} done</span>
          {failCount > 0 && (
            <span className="text-destructive">✗ {failCount} failed</span>
          )}
          <span>{total - finishedCount} remaining</span>
        </div>
      </div>

      {/* ── Per-server status list ── */}
      <ScrollArea className="max-h-[280px] pr-2">
        <div className="space-y-1">
          {session.servers.map((entry) => (
            <ProvisionServerStatusRow key={entry.serverId} entry={entry} />
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}

// ─── Per-server row ────────────────────────────────────────────────────────────

type StatusConfig = {
  icon: React.ReactNode
  label: string
  textClass: string
}

function getStatusConfig(entry: ProvisioningServerEntry): StatusConfig {
  switch (entry.status) {
    case 'pending':
      return {
        icon: <Clock className="size-4" />,
        label: translate('fleet.provision.status.pending', 'Waiting...'),
        textClass: 'text-muted-foreground'
      }
    case 'connecting':
      return {
        icon: <Loader className="size-4 animate-spin" />,
        label: translate('fleet.provision.status.connecting', 'Connecting...'),
        textClass: 'text-blue-500'
      }
    case 'deploying-relay':
      return {
        icon: <UploadCloud className="size-4" />,
        label: translate('fleet.provision.status.deploying', 'Deploying relay...'),
        textClass: 'text-yellow-500'
      }
    case 'done':
      return {
        icon: <CheckCircle className="size-4" />,
        label: translate('fleet.provision.status.done', 'Ready'),
        textClass: 'text-green-500'
      }
    case 'error':
      return {
        icon: <XCircle className="size-4" />,
        label: entry.error ?? translate('fleet.provision.status.error', 'Error'),
        textClass: 'text-destructive'
      }
    case 'skipped':
      return {
        icon: <MinusCircle className="size-4" />,
        label: translate('fleet.provision.status.skipped', 'Skipped'),
        textClass: 'text-muted-foreground'
      }
    default: {
      // Why: TypeScript exhaustiveness check — all statuses handled above.
      void (entry.status satisfies never)
      return { icon: null, label: '', textClass: '' }
    }
  }
}

function ProvisionServerStatusRow({
  entry
}: {
  entry: ProvisioningServerEntry
}): React.JSX.Element {
  const config = getStatusConfig(entry)

  return (
    <div className="flex items-center gap-2.5 rounded px-1.5 py-1.5">
      {/* Status icon */}
      <span className={cn('flex-shrink-0', config.textClass)}>{config.icon}</span>

      {/* Server info */}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{entry.label}</p>
        <p className={cn('truncate text-xs', config.textClass)}>{config.label}</p>
      </div>

      {/* Relay version badge for done status */}
      {entry.status === 'done' && entry.relayVersion && (
        <Badge variant="secondary" className="flex-shrink-0 font-mono text-xs">
          v{entry.relayVersion}
        </Badge>
      )}
    </div>
  )
}
