import { useState } from 'react'
import {
  ChevronDownIcon,
  ChevronRightIcon,
  FlameIcon,
  GlobeIcon,
  LockIcon,
  RepeatIcon,
  EyeIcon,
  type LucideIcon
} from 'lucide-react'
import type { McpToolView } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { mcpRiskLabel, mcpScopeLabel } from '@/lib/mcp-labels'
import { Button } from '@/components/ui/button'
import { Table, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { McpToolDecisionBadge } from './McpToolDecisionBadge'
import { McpToolRiskBadge } from './McpToolRiskBadge'
import { AUTO_OPEN_GROUP_MAX, type ToolGroup, type ToolGroupBy } from './mcp-tool-catalog-grouping'

export function groupLabel(key: string, by: ToolGroupBy): string {
  if (by === 'pack') {
    const packs: Record<string, string> = {
      '1': translate('auto.mcp.tools.pack.1', 'Pack 1 — Read'),
      '2': translate('auto.mcp.tools.pack.2', 'Pack 2 — Write'),
      '3': translate('auto.mcp.tools.pack.3', 'Pack 3 — Execute'),
      '4': translate('auto.mcp.tools.pack.4', 'Pack 4 — Admin')
    }
    return packs[key] ?? translate('auto.mcp.tools.pack.n', 'Pack {{n}}', { n: key })
  }
  return by === 'risk' ? mcpRiskLabel(key as McpToolView['risk']) : key
}

const HINTS: { key: keyof McpToolView['annotations']; Icon: LucideIcon; label: () => string }[] = [
  {
    key: 'readOnly',
    Icon: EyeIcon,
    label: () => translate('auto.mcp.tools.hint.readOnly', 'Read-only')
  },
  {
    key: 'destructive',
    Icon: FlameIcon,
    label: () => translate('auto.mcp.tools.hint.destructive', 'Destructive')
  },
  {
    key: 'idempotent',
    Icon: RepeatIcon,
    label: () => translate('auto.mcp.tools.hint.idempotent', 'Idempotent')
  },
  {
    key: 'openWorld',
    Icon: GlobeIcon,
    label: () => translate('auto.mcp.tools.hint.openWorld', 'Open-world')
  }
]

function ToolCell({ tool }: { tool: McpToolView }): React.JSX.Element {
  const blocked = translate('auto.mcp.tools.alwaysBlocked', 'Always blocked')
  return (
    <TableCell className="max-w-72">
      <div className="flex items-center gap-1.5 font-medium">
        {tool.hardDenied ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span tabIndex={0} className="inline-flex" aria-label={blocked}>
                <LockIcon className="size-3.5 text-muted-foreground" aria-hidden />
              </span>
            </TooltipTrigger>
            <TooltipContent>
              {translate(
                'auto.mcp.tools.hardDenyTooltip',
                'Blocked by a built-in rule. No policy can enable this tool.'
              )}
            </TooltipContent>
          </Tooltip>
        ) : null}
        <span className="truncate">{tool.title || tool.name}</span>
      </div>
      <div className="truncate font-mono text-xs text-muted-foreground">{tool.name}</div>
      {/* Server text is untrusted: rendered as a plain text node only. */}
      <div className="truncate text-xs text-muted-foreground" title={tool.description}>
        {tool.description}
      </div>
    </TableCell>
  )
}

function ToolRow({
  tool,
  onEditPolicy
}: {
  tool: McpToolView
  onEditPolicy?: (name: string) => void
}): React.JSX.Element {
  return (
    <TableRow>
      <ToolCell tool={tool} />
      <TableCell>{tool.namespace}</TableCell>
      <TableCell>{translate('auto.mcp.tools.packN', 'Pack {{n}}', { n: tool.pack })}</TableCell>
      <TableCell>
        <McpToolRiskBadge risk={tool.risk} />
      </TableCell>
      <TableCell className="hidden md:table-cell text-xs">
        {mcpScopeLabel(tool.requiredScope)}
      </TableCell>
      <TableCell>
        <McpToolDecisionBadge decision={tool.effective} source={tool.effectiveSource} />
      </TableCell>
      <TableCell className="hidden md:table-cell">
        <div className="flex gap-1.5">
          {HINTS.filter((h) => tool.annotations?.[h.key]).map(({ key, Icon, label }) => (
            <Icon key={key} className="size-3.5 text-muted-foreground" aria-label={label()}>
              <title>{label()}</title>
            </Icon>
          ))}
        </div>
      </TableCell>
      <TableCell className="text-right">
        {onEditPolicy && !tool.hardDenied ? (
          <Button
            variant="ghost"
            size="xs"
            aria-label={translate('auto.mcp.tools.editPolicyAria', 'Edit policy for {{name}}', {
              name: tool.name
            })}
            onClick={() => onEditPolicy(tool.name)}
          >
            {translate('auto.mcp.tools.editPolicy', 'Edit policy')}
          </Button>
        ) : null}
      </TableCell>
    </TableRow>
  )
}

export function McpToolCatalogTable({
  groups,
  groupBy,
  onEditPolicy
}: {
  groups: ToolGroup[]
  groupBy: ToolGroupBy
  /** Omitted when the policy tab is not available; the column then stays empty. */
  onEditPolicy?: (toolName: string) => void
}): React.JSX.Element {
  const [overrides, setOverrides] = useState<Record<string, boolean>>({})
  return (
    <TooltipProvider>
      <Table>
        <TableHeader>
          <tr>
            <TableHead>{translate('auto.mcp.tools.col.tool', 'Tool')}</TableHead>
            <TableHead>{translate('auto.mcp.tools.col.namespace', 'Namespace')}</TableHead>
            <TableHead>{translate('auto.mcp.tools.col.pack', 'Pack')}</TableHead>
            <TableHead>{translate('auto.mcp.tools.col.risk', 'Risk')}</TableHead>
            <TableHead className="hidden md:table-cell">
              {translate('auto.mcp.tools.col.scope', 'Scope')}
            </TableHead>
            <TableHead>{translate('auto.mcp.tools.col.policy', 'Policy')}</TableHead>
            <TableHead className="hidden md:table-cell">
              {translate('auto.mcp.tools.col.hints', 'Hints')}
            </TableHead>
            <TableHead className="text-right">
              <span className="sr-only">{translate('auto.mcp.tools.col.actions', 'Actions')}</span>
            </TableHead>
          </tr>
        </TableHeader>
        {groups.map((g) => {
          const open = overrides[g.key] ?? g.tools.length <= AUTO_OPEN_GROUP_MAX
          const id = `mcp-tool-group-${groupBy}-${g.key.replace(/[^A-Za-z0-9_-]/g, '_')}`
          const Chevron = open ? ChevronDownIcon : ChevronRightIcon
          return (
            <tbody key={g.key} id={id}>
              <tr className="border-b bg-muted/40">
                <td colSpan={8} className="px-3 py-1.5">
                  <button
                    type="button"
                    aria-expanded={open}
                    aria-controls={id}
                    className="flex items-center gap-1.5 rounded-sm text-sm font-medium focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
                    onClick={() => setOverrides((o) => ({ ...o, [g.key]: !open }))}
                  >
                    <Chevron className="size-4" aria-hidden />
                    {groupLabel(g.key, groupBy)}
                    <span className="font-normal text-muted-foreground">({g.tools.length})</span>
                  </button>
                </td>
              </tr>
              {open
                ? g.tools.map((t) => <ToolRow key={t.name} tool={t} onEditPolicy={onEditPolicy} />)
                : null}
            </tbody>
          )
        })}
      </Table>
    </TooltipProvider>
  )
}
