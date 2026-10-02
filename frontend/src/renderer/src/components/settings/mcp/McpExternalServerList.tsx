import { PencilIcon, ScanSearchIcon, Trash2Icon } from 'lucide-react'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { visualizeControlChars } from '@/components/mcp/mcp-approval-display'
import {
  McpExternalServerHealth,
  McpExternalServerStatusBadge,
  scopeLabel
} from './McpExternalServerStatus'

export type McpExternalServerListProps = {
  servers: McpExternalServer[]
  isAdmin: boolean
  busyIds: ReadonlySet<string>
  /** Kill switch: write actions are disabled while MCP is paused. */
  paused: boolean
  onEdit: (s: McpExternalServer) => void
  onReview: (s: McpExternalServer) => void
  onDelete: (s: McpExternalServer) => void
}

function targetText(s: McpExternalServer): string {
  const raw = s.transport === 'http' ? (s.url ?? '') : [s.command, ...(s.args ?? [])].join(' ')
  return visualizeControlChars(raw)
}

export function McpExternalServerList({
  servers,
  isAdmin,
  busyIds,
  paused,
  onEdit,
  onReview,
  onDelete
}: McpExternalServerListProps): React.JSX.Element {
  return (
    <Table>
      <TableHeader>
        <tr>
          <TableHead>{translate('auto.mcp.external.col.name', 'Name')}</TableHead>
          <TableHead>{translate('auto.mcp.external.col.scope', 'Scope')}</TableHead>
          <TableHead>{translate('auto.mcp.external.col.transport', 'Transport')}</TableHead>
          <TableHead>{translate('auto.mcp.external.col.target', 'Target')}</TableHead>
          <TableHead>{translate('auto.mcp.external.col.status', 'Status')}</TableHead>
          <TableHead>{translate('auto.mcp.external.col.health', 'Health')}</TableHead>
          <TableHead className="text-right">
            <span className="sr-only">{translate('auto.mcp.external.col.actions', 'Actions')}</span>
          </TableHead>
        </tr>
      </TableHeader>
      <TableBody>
        {servers.map((s) => {
          const busy = busyIds.has(s.id)
          const canManage = isAdmin || s.scope === 'user'
          const needsReview = s.status === 'pending_review' || s.toolsChanged
          const target = targetText(s)
          return (
            <TableRow key={s.id}>
              <TableCell className="font-mono text-xs">{visualizeControlChars(s.name)}</TableCell>
              <TableCell>{scopeLabel(s.scope)}</TableCell>
              <TableCell className="uppercase">{s.transport}</TableCell>
              <TableCell className="max-w-64">
                <span className="block truncate font-mono text-xs" title={target}>
                  {target}
                </span>
              </TableCell>
              <TableCell>
                <McpExternalServerStatusBadge server={s} />
                {!isAdmin && s.status === 'pending_review' ? (
                  <p className="mt-1 text-xs text-muted-foreground">
                    {translate('auto.mcp.external.waitingAdmin', 'Waiting for an admin to review')}
                  </p>
                ) : null}
              </TableCell>
              <TableCell className="text-xs">
                {s.transport === 'stdio' || s.status === 'pending_review' ? (
                  <span className="text-muted-foreground">—</span>
                ) : (
                  <McpExternalServerHealth server={s} />
                )}
              </TableCell>
              <TableCell>
                <div className="flex justify-end gap-1">
                  {isAdmin ? (
                    <Button
                      variant="ghost"
                      size="xs"
                      disabled={busy || paused}
                      aria-label={
                        needsReview
                          ? translate('auto.mcp.external.reviewAria', 'Review server {{name}}', {
                              name: s.name
                            })
                          : translate('auto.mcp.external.probeAria', 'Probe server {{name}}', {
                              name: s.name
                            })
                      }
                      onClick={() => onReview(s)}
                    >
                      <ScanSearchIcon aria-hidden />
                      {needsReview
                        ? translate('auto.mcp.external.review', 'Review')
                        : translate('auto.mcp.external.probe', 'Probe')}
                    </Button>
                  ) : null}
                  {canManage ? (
                    <>
                      <Button
                        variant="ghost"
                        size="xs"
                        disabled={busy || paused}
                        aria-label={translate(
                          'auto.mcp.external.editAria',
                          'Edit server {{name}}',
                          {
                            name: s.name
                          }
                        )}
                        onClick={() => onEdit(s)}
                      >
                        <PencilIcon aria-hidden />
                        {translate('auto.mcp.external.edit', 'Edit')}
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        disabled={busy}
                        aria-label={translate(
                          'auto.mcp.external.deleteAria',
                          'Delete server {{name}}',
                          { name: s.name }
                        )}
                        onClick={() => onDelete(s)}
                      >
                        <Trash2Icon aria-hidden />
                        {translate('auto.mcp.external.delete', 'Delete')}
                      </Button>
                    </>
                  ) : null}
                </div>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
