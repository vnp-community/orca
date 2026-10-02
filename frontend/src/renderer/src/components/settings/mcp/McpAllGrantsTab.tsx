import { useMemo, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { Input } from '@/components/ui/input'
import { sortMcpGrants, useMcpAllGrants } from '@/hooks/use-mcp-grants'
import { McpGrantsTable } from './McpGrantsTable'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { useMcpGrantRevokeFlow } from './use-mcp-revoke-flow'

const ownerOf = (g: { userName?: string; userId?: string }): string => g.userName ?? g.userId ?? ''

export function McpAllGrantsTab({ userId }: { userId?: string }): React.JSX.Element {
  const info = useAppStore((s) => s.mcpServerInfo)
  const { grants, status, error, reload, revoke } = useMcpAllGrants(userId)
  const { busy, requestRevoke } = useMcpGrantRevokeFlow(revoke, ownerOf)
  const [filter, setFilter] = useState('')
  const rows = useMemo(() => {
    const q = filter.trim().toLowerCase()
    const sorted = sortMcpGrants(grants)
    return q
      ? sorted.filter((g) => `${ownerOf(g)} ${g.clientName}`.toLowerCase().includes(q))
      : sorted
  }, [grants, filter])

  if (status === 'loading') {
    return <McpListSkeleton />
  }
  if (status === 'forbidden') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.admin.required', 'Administrator access required.')}
      </McpMutedNote>
    )
  }
  if (status === 'unavailable') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.grants.unavailable', "Grants aren't available on this server yet.")}
      </McpMutedNote>
    )
  }
  if (status === 'error') {
    return <McpInlineAlert message={error ?? ''} onRetry={reload} />
  }
  return (
    <div className="space-y-3">
      <Input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        aria-label={translate('auto.mcp.grants.filter', 'Filter by user or app')}
        placeholder={translate('auto.mcp.grants.filter', 'Filter by user or app')}
      />
      {rows.length === 0 ? (
        <McpMutedNote>{translate('auto.mcp.grants.empty', 'No grants found.')}</McpMutedNote>
      ) : (
        <McpGrantsTable
          grants={rows}
          info={info}
          showUser
          busyIds={busy}
          onRevoke={requestRevoke}
        />
      )}
    </div>
  )
}
