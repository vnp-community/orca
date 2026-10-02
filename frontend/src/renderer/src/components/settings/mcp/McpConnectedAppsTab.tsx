import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { useMcpGrants, sortMcpGrants } from '@/hooks/use-mcp-grants'
import { McpCopyButton } from './McpCopyButton'
import { McpGrantsTable } from './McpGrantsTable'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { useMcpGrantRevokeFlow } from './use-mcp-revoke-flow'

export function McpConnectedAppsTab(): React.JSX.Element {
  const info = useAppStore((s) => s.mcpServerInfo)
  const { grants, status, error, reload, revoke } = useMcpGrants()
  const { busy, requestRevoke } = useMcpGrantRevokeFlow(revoke)

  if (status === 'loading') {
    return <McpListSkeleton />
  }
  if (status === 'unavailable') {
    return (
      <McpMutedNote>
        {translate(
          'auto.mcp.apps.unavailable',
          "Connected apps aren't available on this server yet."
        )}
      </McpMutedNote>
    )
  }
  if (status === 'error' || status === 'forbidden') {
    return <McpInlineAlert message={error ?? ''} onRetry={reload} />
  }
  if (grants.length === 0) {
    const url = info?.resourceUrl ?? ''
    return (
      <div className="space-y-2">
        <McpMutedNote>{translate('auto.mcp.apps.empty', 'No apps connected yet.')}</McpMutedNote>
        {url ? (
          <div className="flex items-center gap-2">
            <code className="rounded-md bg-muted px-2 py-1 font-mono text-xs">{url}</code>
            <McpCopyButton
              text={url}
              ariaLabel={translate('auto.mcp.connect.copyAddress', 'Copy server address')}
            />
          </div>
        ) : null}
      </div>
    )
  }
  return (
    <McpGrantsTable
      grants={sortMcpGrants(grants)}
      info={info}
      busyIds={busy}
      onRevoke={requestRevoke}
    />
  )
}
