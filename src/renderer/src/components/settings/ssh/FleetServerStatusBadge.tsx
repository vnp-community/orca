// FleetServerStatusBadge — connection status badge for fleet health table (CR-005, TASK-005-D)
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'
import type { SshConnectionStatus } from '../../../../../shared/ssh-types'

type StatusConfig = {
  label: string
  variant: 'outline' | 'secondary' | 'destructive' | 'default'
  className: string
}

const STATUS_CONFIG: Record<SshConnectionStatus, StatusConfig> = {
  connected: {
    label: translate('ssh.status.connected', 'Connected'),
    variant: 'outline',
    className: 'border-green-500/40 text-green-600 dark:text-green-400'
  },
  disconnected: {
    label: translate('ssh.status.disconnected', 'Offline'),
    variant: 'outline',
    className: 'text-muted-foreground'
  },
  connecting: {
    label: translate('ssh.status.connecting', 'Connecting'),
    variant: 'outline',
    className: 'border-blue-500/40 text-blue-500'
  },
  reconnecting: {
    label: translate('ssh.status.reconnecting', 'Reconnecting'),
    variant: 'outline',
    className: 'border-blue-500/40 text-blue-400 animate-pulse'
  },
  'deploying-relay': {
    label: translate('ssh.status.deployingRelay', 'Deploying'),
    variant: 'outline',
    className: 'border-yellow-500/40 text-yellow-600'
  },
  error: {
    label: translate('ssh.status.error', 'Error'),
    variant: 'destructive',
    className: ''
  },
  'auth-failed': {
    label: translate('ssh.status.authFailed', 'Auth Failed'),
    variant: 'destructive',
    className: ''
  },
  'reconnection-failed': {
    label: translate('ssh.status.reconnFailed', 'Failed'),
    variant: 'destructive',
    className: ''
  }
}

export function FleetServerStatusBadge({
  status
}: {
  status: SshConnectionStatus
}): React.JSX.Element {
  const config = STATUS_CONFIG[status] ?? STATUS_CONFIG['disconnected']
  return (
    <Badge variant={config.variant} className={config.className}>
      {config.label}
    </Badge>
  )
}
