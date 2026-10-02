import { useEffect, useMemo, useRef, useState } from 'react'
import { OctagonXIcon } from 'lucide-react'
import type { McpToolView } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { selectMcpKillSwitchActive } from '@/store/slices/mcp-slice'
import { useMcpQuery } from '@/hooks/useMcpQuery'
import { Button } from '@/components/ui/button'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { McpToolCatalogTable } from './McpToolCatalogTable'
import { McpToolsToolbar } from './McpToolsToolbar'
import { MCP_TABS } from './mcp-tab-registry'
import {
  DEFAULT_TOOL_FILTERS,
  filterTools,
  groupTools,
  listToolNamespaces,
  summarizeTools,
  type ToolFilters,
  type ToolGroupBy
} from './mcp-tool-catalog-grouping'

const NO_TOOLS: McpToolView[] = []
const POLICY_TAB = 'policy'

export function McpToolsTab(): React.JSX.Element {
  const killSwitch = useAppStore(selectMcpKillSwitchActive)
  const resync = useAppStore((s) => s.mcpResyncCounter)
  const openMcpTab = useAppStore((s) => s.openMcpTab)
  const query = useMcpQuery('mcp.admin.tool.list', {}, NO_TOOLS, { refetchOnFocus: true })
  const [filters, setFilters] = useState<ToolFilters>(DEFAULT_TOOL_FILTERS)
  const [groupBy, setGroupBy] = useState<ToolGroupBy>('namespace')

  // The effective decision depends on the kill switch and on reconnects, so reload on both.
  const reloadRef = useRef(query.reload)
  reloadRef.current = query.reload
  const first = useRef(true)
  useEffect(() => {
    if (first.current) {
      first.current = false
      return
    }
    reloadRef.current()
  }, [killSwitch, resync])

  const tools = query.data
  const filtered = useMemo(() => filterTools(tools, filters), [tools, filters])
  const groups = useMemo(() => groupTools(filtered, groupBy), [filtered, groupBy])
  const namespaces = useMemo(() => listToolNamespaces(tools), [tools])
  const summary = useMemo(() => summarizeTools(filtered), [filtered])

  if (query.status === 'loading') {
    return <McpListSkeleton rows={8} />
  }
  if (query.status === 'forbidden') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.tools.forbidden', 'You need admin rights to view tools.')}
      </McpMutedNote>
    )
  }
  if (query.status === 'unavailable') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.tools.unavailable', 'MCP is turned off for this organization.')}
      </McpMutedNote>
    )
  }
  if (query.status === 'error') {
    return <McpInlineAlert message={query.error ?? ''} onRetry={query.reload} />
  }

  // Why: the policy tab is built separately; link only when it is registered (graceful degrade).
  const policyAvailable = MCP_TABS.some((t) => t.id === POLICY_TAB)
  const showKillSwitchBanner = killSwitch || tools.some((t) => t.effectiveSource === 'kill_switch')

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">
        {policyAvailable
          ? translate(
              'auto.mcp.tools.description',
              'Tools that AI agents can call through MCP. This view is read-only; edit policies in the Policy tab.'
            )
          : translate(
              'auto.mcp.tools.descriptionNoPolicy',
              'Tools that AI agents can call through MCP. This view is read-only.'
            )}
      </p>
      {showKillSwitchBanner ? (
        <div
          role="status"
          className="flex items-center gap-2 rounded-md border border-destructive/40 px-3 py-2 text-sm text-destructive"
        >
          <OctagonXIcon className="size-4 shrink-0" aria-hidden />
          {translate(
            'auto.mcp.tools.killSwitch',
            'The MCP kill switch is on: agents cannot call any tool.'
          )}
        </div>
      ) : null}
      <McpToolsToolbar
        filters={filters}
        onFiltersChange={setFilters}
        groupBy={groupBy}
        onGroupByChange={setGroupBy}
        namespaces={namespaces}
        disabled={false}
        onRefresh={query.reload}
      />
      {tools.length === 0 ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.tools.empty',
            'No tools are exposed yet. Tool packs are enabled on the server.'
          )}
        </McpMutedNote>
      ) : filtered.length === 0 ? (
        <div className="flex items-center gap-2">
          <McpMutedNote>
            {translate('auto.mcp.tools.noMatch', 'No tools match these filters.')}
          </McpMutedNote>
          <Button variant="ghost" size="sm" onClick={() => setFilters(DEFAULT_TOOL_FILTERS)}>
            {translate('auto.mcp.tools.clearFilters', 'Clear filters')}
          </Button>
        </div>
      ) : (
        <>
          <p className="text-sm" aria-live="polite">
            {translate(
              'auto.mcp.tools.summary',
              '{{total}} tools · {{allowed}} allowed · {{approval}} need approval · {{blocked}} blocked · {{hard}} always blocked',
              {
                total: summary.total,
                allowed: summary.byDecision.allow,
                approval: summary.byDecision.require_approval,
                blocked: summary.byDecision.deny,
                hard: summary.hardDenied
              }
            )}
          </p>
          <McpToolCatalogTable
            groups={groups}
            groupBy={groupBy}
            onEditPolicy={policyAvailable ? (name) => openMcpTab(POLICY_TAB, name) : undefined}
          />
        </>
      )}
    </div>
  )
}
