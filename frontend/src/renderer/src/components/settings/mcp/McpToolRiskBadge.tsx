import {
  EyeIcon,
  PencilIcon,
  ShieldAlertIcon,
  SquareTerminalIcon,
  Trash2Icon,
  type LucideIcon
} from 'lucide-react'
import type { McpRisk } from '../../../../../shared/mcp-types'
import { Badge } from '@/components/ui/badge'
import { mcpRiskLabel } from '@/lib/mcp-labels'

const ICONS: Record<McpRisk, LucideIcon> = {
  read: EyeIcon,
  write_reversible: PencilIcon,
  exec: SquareTerminalIcon,
  destructive: Trash2Icon,
  admin: ShieldAlertIcon
}

/** Icon + text so risk never relies on colour alone. */
export function McpToolRiskBadge({ risk }: { risk: McpRisk }): React.JSX.Element {
  const Icon = ICONS[risk] ?? EyeIcon
  const variant =
    risk === 'read'
      ? 'secondary'
      : risk === 'destructive' || risk === 'admin'
        ? 'destructive'
        : 'outline'
  return (
    <Badge variant={variant}>
      <Icon aria-hidden />
      {mcpRiskLabel(risk)}
    </Badge>
  )
}
