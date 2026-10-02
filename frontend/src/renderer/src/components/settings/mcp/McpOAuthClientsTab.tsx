import { toast } from 'sonner'
import type { McpOAuthClient } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { sortMcpClients, useMcpOAuthClients } from '@/hooks/use-mcp-oauth-clients'
import { formatMcpRelativeTime } from '@/lib/mcp-relative-time'
import { McpCopyButton } from './McpCopyButton'
import { hostOf } from './McpGrantsTable'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'

function statusLabel(s: McpOAuthClient['status']): string {
  switch (s) {
    case 'allowed':
      return translate('auto.mcp.clients.status.allowed', 'Allowed')
    case 'blocked':
      return translate('auto.mcp.clients.status.blocked', 'Blocked')
    default:
      return translate('auto.mcp.clients.status.pending', 'Pending')
  }
}

export function McpOAuthClientsTab(): React.JSX.Element {
  const { clients, status, error, reload, busy, setStatus } = useMcpOAuthClients()
  const confirm = useConfirmationDialog()

  const change = async (c: McpOAuthClient, next: 'allowed' | 'blocked'): Promise<void> => {
    if (next === 'blocked') {
      const ok = await confirm({
        title: translate('auto.mcp.clients.blockTitle', 'Block {{name}}?', {
          name: c.name
        }),
        description: translate(
          'auto.mcp.clients.blockBody',
          'Blocking {{name}} signs out its {{count}} connections. You can allow it again later.',
          { name: c.name, count: c.activeGrants }
        ),
        confirmLabel: translate('auto.mcp.clients.block', 'Block'),
        confirmVariant: 'destructive'
      })
      if (!ok) {
        return
      }
    }
    try {
      await setStatus(c.clientId, next)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

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
        {translate(
          'auto.mcp.clients.unavailable',
          "OAuth clients aren't available on this server yet."
        )}
      </McpMutedNote>
    )
  }
  if (status === 'error') {
    return <McpInlineAlert message={error ?? ''} onRetry={reload} />
  }
  if (clients.length === 0) {
    return (
      <McpMutedNote>{translate('auto.mcp.clients.empty', 'No OAuth clients yet.')}</McpMutedNote>
    )
  }
  const pending = clients.filter((c) => c.status === 'pending').length
  return (
    <div className="space-y-3">
      {pending > 0 ? (
        <p role="status" className="text-sm font-medium">
          {translate('auto.mcp.clients.pendingBanner', '{{count}} apps waiting for approval', {
            count: pending
          })}
        </p>
      ) : null}
      <Table>
        <caption className="sr-only">
          {translate('auto.mcp.clients.caption', 'Registered OAuth clients')}
        </caption>
        <TableHeader>
          <TableRow>
            <TableHead scope="col">{translate('auto.mcp.clients.col.name', 'Name')}</TableHead>
            <TableHead scope="col">
              {translate('auto.mcp.clients.col.clientId', 'Client ID')}
            </TableHead>
            <TableHead scope="col">
              {translate('auto.mcp.clients.col.redirects', 'Redirect hosts')}
            </TableHead>
            <TableHead scope="col">
              {translate('auto.mcp.clients.col.via', 'Registered via')}
            </TableHead>
            <TableHead scope="col">{translate('auto.mcp.clients.col.status', 'Status')}</TableHead>
            <TableHead scope="col">
              {translate('auto.mcp.clients.col.grants', 'Active grants')}
            </TableHead>
            <TableHead scope="col">
              {translate('auto.mcp.apps.col.lastUsed', 'Last used')}
            </TableHead>
            <TableHead scope="col">
              <span className="sr-only">
                {translate('auto.mcp.sessions.col.actions', 'Actions')}
              </span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sortMcpClients(clients).map((c) => (
            <TableRow key={c.clientId} data-testid={`mcp-client-${c.clientId}`}>
              <TableCell className="font-medium">{c.name}</TableCell>
              <TableCell>
                <span className="inline-flex items-center gap-1 font-mono text-xs">
                  {c.clientId.length > 12 ? `${c.clientId.slice(0, 12)}…` : c.clientId}
                  <McpCopyButton
                    text={c.clientId}
                    ariaLabel={translate('auto.mcp.clients.copyId', 'Copy client ID of {{name}}', {
                      name: c.name
                    })}
                  />
                </span>
              </TableCell>
              <TableCell className="font-mono text-xs" title={c.redirectUris.join('\n')}>
                {[...new Set(c.redirectUris.map(hostOf))].filter(Boolean).join(', ')}
              </TableCell>
              <TableCell>
                <Badge variant="outline">{c.registeredVia === 'dcr' ? 'DCR' : 'Admin'}</Badge>
              </TableCell>
              <TableCell>
                <Badge variant={c.status === 'blocked' ? 'destructive' : 'secondary'}>
                  {statusLabel(c.status)}
                </Badge>
              </TableCell>
              <TableCell>{c.activeGrants}</TableCell>
              <TableCell>
                {c.lastUsedAt
                  ? formatMcpRelativeTime(c.lastUsedAt)
                  : translate('auto.mcp.apps.never', 'Never')}
              </TableCell>
              <TableCell className="text-right">
                {c.status === 'allowed' ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy.has(c.clientId)}
                    onClick={() => void change(c, 'blocked')}
                  >
                    {translate('auto.mcp.clients.block', 'Block')}
                  </Button>
                ) : (
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={busy.has(c.clientId)}
                    onClick={() => void change(c, 'allowed')}
                  >
                    {translate('auto.mcp.clients.allow', 'Allow')}
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
