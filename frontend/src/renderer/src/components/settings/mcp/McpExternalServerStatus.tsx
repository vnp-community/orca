import { BanIcon, ClockIcon, ShieldCheckIcon, TriangleAlertIcon } from 'lucide-react'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { formatMcpRelativeTime } from '@/lib/mcp-relative-time'
import { Badge } from '@/components/ui/badge'

export function scopeLabel(scope: McpExternalServer['scope']): string {
  if (scope === 'tenant') {
    return translate('auto.mcp.external.scope.tenant', 'Organization')
  }
  return scope === 'team'
    ? translate('auto.mcp.external.scope.team', 'Team')
    : translate('auto.mcp.external.scope.user', 'Just me')
}

export function McpExternalServerStatusBadge({
  server
}: {
  server: McpExternalServer
}): React.JSX.Element {
  return (
    <div className="flex flex-col items-start gap-1">
      {server.status === 'approved' ? (
        <Badge variant="secondary">
          <ShieldCheckIcon aria-hidden />
          {translate('auto.mcp.external.status.approved', 'Approved')}
        </Badge>
      ) : server.status === 'pending_review' ? (
        <Badge variant="outline">
          <ClockIcon aria-hidden />
          {translate('auto.mcp.external.status.pending', 'Pending review')}
        </Badge>
      ) : (
        <Badge variant="dot">
          <BanIcon aria-hidden />
          {translate('auto.mcp.external.status.disabled', 'Disabled')}
        </Badge>
      )}
      {server.toolsChanged ? (
        <Badge
          variant="destructive"
          title={translate(
            'auto.mcp.external.toolsChangedHint',
            "This server's tool descriptions differ from the version an admin approved. It is not given to agents until re-approved."
          )}
        >
          <TriangleAlertIcon aria-hidden />
          {translate('auto.mcp.external.toolsChanged', 'Tools changed — re-review required')}
        </Badge>
      ) : null}
    </div>
  )
}

export function McpExternalServerHealth({
  server
}: {
  server: McpExternalServer
}): React.JSX.Element {
  const h = server.health
  if (!h) {
    return (
      <span className="text-muted-foreground">
        {translate('auto.mcp.external.health.none', 'Not checked yet')}
      </span>
    )
  }
  if (h.ok) {
    return (
      <span>
        {translate('auto.mcp.external.health.ok', 'Healthy · checked {{when}}', {
          when: formatMcpRelativeTime(h.checkedAt)
        })}
      </span>
    )
  }
  return (
    <span className="text-destructive">
      {translate('auto.mcp.external.health.bad', 'Unreachable')}
      {h.error ? ` — ${h.error}` : ''}
    </span>
  )
}
