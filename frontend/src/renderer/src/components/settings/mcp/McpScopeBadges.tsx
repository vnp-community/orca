import { ShieldAlertIcon } from 'lucide-react'
import type { McpRisk, McpScopeId, McpServerInfo } from '../../../../../shared/mcp-types'
import { Badge } from '@/components/ui/badge'
import { isHighRisk } from '@/lib/mcp-risk'
import { mcpScopeLabel } from '@/lib/mcp-labels'
import { translate } from '@/i18n/i18n'

const FALLBACK_RISK: Record<string, McpRisk> = {
  'orca:exec': 'exec',
  'orca:admin': 'admin'
}

export function scopeRisk(
  id: string,
  info: Pick<McpServerInfo, 'scopesSupported'> | null
): McpRisk {
  return info?.scopesSupported.find((s) => s.id === id)?.risk ?? FALLBACK_RISK[id] ?? 'read'
}

/** Risk is conveyed by icon + text, never colour alone. */
export function McpScopeBadges({
  scopes,
  info
}: {
  scopes: readonly (McpScopeId | string)[]
  info: Pick<McpServerInfo, 'scopesSupported'> | null
}): React.JSX.Element {
  return (
    <div className="flex flex-wrap gap-1">
      {scopes.map((id) => {
        const high = isHighRisk(scopeRisk(id, info))
        return (
          <Badge key={id} variant="outline">
            {high ? (
              <ShieldAlertIcon
                aria-label={translate('auto.mcp.risk.highRisk', 'High risk')}
                className="text-destructive"
              />
            ) : null}
            {mcpScopeLabel(id)}
          </Badge>
        )
      })}
    </div>
  )
}
