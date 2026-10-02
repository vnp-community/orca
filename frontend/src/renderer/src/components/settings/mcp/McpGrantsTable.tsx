import type { McpGrant, McpServerInfo } from '../../../../../shared/mcp-types'
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
import { McpScopeBadges } from './McpScopeBadges'

export function hostOf(uri: string | undefined): string {
  if (!uri) {
    return ''
  }
  try {
    return new URL(uri).host
  } catch {
    return ''
  }
}

type Props = {
  grants: readonly McpGrant[]
  info: Pick<McpServerInfo, 'scopesSupported'> | null
  showUser?: boolean
  busyIds?: ReadonlySet<string>
  onRevoke: (grant: McpGrant) => void
}

export function McpGrantsTable({
  grants,
  info,
  showUser = false,
  busyIds,
  onRevoke
}: Props): React.JSX.Element {
  return (
    <Table>
      <caption className="sr-only">
        {translate('auto.mcp.apps.caption', 'Apps with access to your Orca account')}
      </caption>
      <TableHeader>
        <TableRow>
          {showUser ? (
            <TableHead scope="col">{translate('auto.mcp.grants.col.user', 'User')}</TableHead>
          ) : null}
          <TableHead scope="col">{translate('auto.mcp.apps.col.app', 'App')}</TableHead>
          <TableHead scope="col">
            {translate('auto.mcp.apps.col.permissions', 'Permissions')}
          </TableHead>
          <TableHead scope="col">{translate('auto.mcp.apps.col.connected', 'Connected')}</TableHead>
          <TableHead scope="col">{translate('auto.mcp.apps.col.lastUsed', 'Last used')}</TableHead>
          <TableHead scope="col">
            <span className="sr-only">{translate('auto.mcp.sessions.col.actions', 'Actions')}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {grants.map((g) => {
          const revoked = g.status === 'revoked'
          return (
            <TableRow
              key={g.id}
              className={revoked ? 'opacity-60' : undefined}
              data-testid={`mcp-grant-${g.id}`}
            >
              {showUser ? <TableCell>{g.userName ?? g.userId ?? ''}</TableCell> : null}
              <TableCell>
                <div className="font-medium">{g.clientName}</div>
                {g.clientUri ? (
                  <div className="font-mono text-xs text-muted-foreground">
                    {hostOf(g.clientUri)}
                  </div>
                ) : null}
              </TableCell>
              <TableCell>
                <McpScopeBadges scopes={g.scopes} info={info} />
              </TableCell>
              <TableCell>{new Date(g.createdAt).toLocaleString()}</TableCell>
              <TableCell>
                {g.lastUsedAt
                  ? new Date(g.lastUsedAt).toLocaleString()
                  : translate('auto.mcp.apps.never', 'Never')}
              </TableCell>
              <TableCell className="text-right">
                {revoked ? (
                  <Badge variant="outline">{translate('auto.mcp.apps.revoked', 'Revoked')}</Badge>
                ) : (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busyIds?.has(g.id)}
                    aria-label={translate(
                      'auto.mcp.apps.revokeAria',
                      'Revoke access for {{client}}',
                      {
                        client: g.clientName
                      }
                    )}
                    onClick={() => onRevoke(g)}
                  >
                    {translate('auto.mcp.apps.revoke', 'Revoke')}
                  </Button>
                )}
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
