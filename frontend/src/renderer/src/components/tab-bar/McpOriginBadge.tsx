import { BotIcon } from 'lucide-react'
import type { McpOrigin } from '../../../../shared/mcp-types'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { displayMcpClientName } from '@/lib/mcp-terminal-origin'
import { translate } from '@/i18n/i18n'

/** Icon + text, never colour alone. clientName is untrusted, so it only renders as plain text. */
export function McpOriginBadge({
  origin,
  compact = false
}: {
  origin: McpOrigin
  compact?: boolean
}): React.JSX.Element {
  const name = displayMcpClientName(origin.clientName)
  const label = translate('auto.mcp.origin.createdByAgent', 'Created by agent {{name}}', { name })
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge
            variant="outline"
            tabIndex={0}
            aria-label={label}
            title={origin.clientName}
            data-testid="mcp-origin-badge"
            className="mr-1 px-1.5"
          >
            <BotIcon aria-hidden />
            {compact ? null : label}
          </Badge>
        </TooltipTrigger>
        <TooltipContent>
          {translate(
            'auto.mcp.origin.tooltip',
            'Started by MCP client "{{name}}". You can stop it from the tab menu.',
            { name }
          )}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
