// SshTargetGroupRow — individual SSH target row for the fleet grouped list (CR-002, TASK-002-E)
// Why: Extracted as standalone component so it can be used independently from
// SshTargetGroupedList and tested in isolation.
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import type { SshTarget, SshConnectionState } from '../../../../../shared/ssh-types'

function statusColorClass(status: string): string {
  switch (status) {
    case 'connected':
      return 'bg-green-500'
    case 'connecting':
    case 'reconnecting':
    case 'deploying-relay':
      return 'bg-blue-500 animate-pulse'
    case 'error':
    case 'auth-failed':
    case 'reconnection-failed':
      return 'bg-destructive'
    default:
      return 'bg-muted-foreground/40'
  }
}

const ENV_BADGE_CLASS: Record<'development' | 'staging' | 'production', string> = {
  development: 'border-green-500/40 text-green-600 dark:text-green-400',
  staging: 'border-yellow-500/40 text-yellow-600 dark:text-yellow-400',
  production: 'border-red-500/40 text-red-600 dark:text-red-400'
}

type SshTargetGroupRowProps = {
  target: SshTarget
  /** Connection state from global store. undefined = disconnected/unknown. */
  connectionState: SshConnectionState | undefined
}

export function SshTargetGroupRow({
  target,
  connectionState
}: SshTargetGroupRowProps): React.JSX.Element {
  const status = connectionState?.status ?? 'disconnected'

  return (
    <div className="flex items-center gap-2 rounded px-2 py-1 text-xs hover:bg-muted/40">
      {/* Status dot */}
      <span
        className={cn('size-1.5 flex-shrink-0 rounded-full', statusColorClass(status))}
        title={translate(`ssh.status.${status}`, status)}
      />

      {/* Label */}
      <span
        className={cn(
          'flex-1 truncate font-medium',
          status !== 'connected' && 'text-muted-foreground'
        )}
      >
        {target.label || `${target.username}@${target.host}`}
      </span>

      {/* Team badge — only rendered when team is set */}
      {target.team && (
        <Badge variant="outline" className="h-4 flex-shrink-0 px-1.5 py-0 text-[10px]">
          {target.team}
        </Badge>
      )}

      {/* Environment badge — only rendered when env is set */}
      {target.environment && (
        <Badge
          variant="outline"
          className={cn('h-4 flex-shrink-0 px-1.5 py-0 text-[10px]', ENV_BADGE_CLASS[target.environment])}
        >
          {target.environment}
        </Badge>
      )}
    </div>
  )
}
