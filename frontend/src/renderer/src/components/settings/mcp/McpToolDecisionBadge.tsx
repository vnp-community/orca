import { BanIcon, CheckIcon, LockIcon, PowerIcon, UserCheckIcon } from 'lucide-react'
import type { McpDecision, McpToolView } from '../../../../../shared/mcp-types'
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'

export function decisionLabel(d: McpDecision): string {
  switch (d) {
    case 'allow':
      return translate('auto.mcp.tools.decision.allow', 'Allowed')
    case 'require_approval':
      return translate('auto.mcp.tools.decision.require_approval', 'Needs approval')
    default:
      return translate('auto.mcp.tools.decision.deny', 'Blocked')
  }
}

export function sourceLabel(source: McpToolView['effectiveSource']): string {
  switch (source) {
    case 'tenant_policy':
      return translate('auto.mcp.tools.source.tenant_policy', 'Tenant policy')
    case 'kill_switch':
      return translate('auto.mcp.tools.source.kill_switch', 'Kill switch')
    case 'hard_deny':
      return translate('auto.mcp.tools.source.hard_deny', 'Always blocked')
    default:
      return translate('auto.mcp.tools.source.default', 'Default')
  }
}

export function McpToolDecisionBadge({
  decision,
  source
}: {
  decision: McpDecision
  source: McpToolView['effectiveSource']
}): React.JSX.Element {
  const Icon = decision === 'allow' ? CheckIcon : decision === 'deny' ? BanIcon : UserCheckIcon
  const SourceIcon = source === 'hard_deny' ? LockIcon : source === 'kill_switch' ? PowerIcon : null
  return (
    <div className="flex flex-col items-start gap-0.5">
      <Badge
        variant={
          decision === 'allow' ? 'secondary' : decision === 'deny' ? 'destructive' : 'outline'
        }
      >
        <Icon aria-hidden />
        {decisionLabel(decision)}
      </Badge>
      <span className="flex items-center gap-1 text-xs text-muted-foreground">
        {SourceIcon ? <SourceIcon className="size-3" aria-hidden /> : null}
        {sourceLabel(source)}
      </span>
    </div>
  )
}
