import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { useMcpSessions, type McpSessionsScope } from '@/hooks/useMcpSessions'
import { formatMcpRelativeTime } from '@/lib/mcp-relative-time'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'

function Time({ iso }: { iso: string }): React.JSX.Element {
  const date = new Date(iso)
  return (
    <time dateTime={iso} title={Number.isNaN(date.getTime()) ? undefined : date.toLocaleString()}>
      {formatMcpRelativeTime(iso)}
    </time>
  )
}

export function McpSessionsTable(): React.JSX.Element {
  const currentUser = useAppStore((s) => s.currentUser)
  const isAdmin = currentUser?.role === 'admin'
  const [showAll, setShowAll] = useState(false)
  const scope: McpSessionsScope = isAdmin && showAll ? 'all' : 'mine'
  const { sessions, status, error, reload, closeSession } = useMcpSessions(scope)
  const confirm = useConfirmationDialog()
  const [closing, setClosing] = useState<Set<string>>(new Set())

  // Why: a forbidden answer means the role changed; drop back to our own sessions.
  const toggleVisible = isAdmin && status !== 'forbidden'
  useEffect(() => {
    if (status === 'forbidden' && showAll) {
      setShowAll(false)
    }
  }, [status, showAll])

  const onClose = async (id: string): Promise<void> => {
    const ok = await confirm({
      title: translate('auto.mcp.sessions.closeConfirmTitle', 'Close this session?'),
      description: translate(
        'auto.mcp.sessions.closeConfirmBody',
        'The agent will be disconnected and any tool call it is running will be cancelled. It can reconnect by signing in again.'
      ),
      confirmLabel: translate('auto.mcp.sessions.close', 'Close')
    })
    if (!ok) {
      return
    }
    setClosing((s) => new Set(s).add(id))
    try {
      await closeSession(id)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setClosing((s) => {
        const next = new Set(s)
        next.delete(id)
        return next
      })
    }
  }

  return (
    <section className="space-y-3" aria-labelledby="mcp-sessions-title">
      <div className="flex items-center justify-between gap-3">
        <h3 id="mcp-sessions-title" className="text-sm font-medium">
          {translate('auto.mcp.sessions.title', 'Active sessions')}
        </h3>
        {toggleVisible ? (
          <div className="flex items-center gap-2">
            <Checkbox
              id="mcp-sessions-show-all"
              checked={showAll}
              onCheckedChange={(v) => setShowAll(v === true)}
            />
            <Label htmlFor="mcp-sessions-show-all" className="text-sm font-normal">
              {translate('auto.mcp.sessions.showAll', 'Show all users')}
            </Label>
          </div>
        ) : null}
      </div>
      {status === 'loading' ? <McpListSkeleton /> : null}
      {status === 'unavailable' ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.sessions.unavailable',
            "Session list isn't available on this server yet."
          )}
        </McpMutedNote>
      ) : null}
      {status === 'error' ? <McpInlineAlert message={error ?? ''} onRetry={reload} /> : null}
      {status === 'ready' && sessions.length === 0 ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.sessions.empty',
            'No agents are connected. Add Orca to your agent using the instructions above.'
          )}
        </McpMutedNote>
      ) : null}
      {status === 'ready' && sessions.length > 0 ? (
        <TooltipProvider>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">
                  {translate('auto.mcp.sessions.col.client', 'Client')}
                </TableHead>
                {scope === 'all' ? (
                  <TableHead scope="col">
                    {translate('auto.mcp.sessions.col.user', 'User')}
                  </TableHead>
                ) : null}
                <TableHead scope="col">
                  {translate('auto.mcp.sessions.col.connected', 'Connected')}
                </TableHead>
                <TableHead scope="col">
                  {translate('auto.mcp.sessions.col.lastActive', 'Last active')}
                </TableHead>
                <TableHead scope="col">
                  {translate('auto.mcp.sessions.col.streams', 'Streams')}
                </TableHead>
                <TableHead scope="col">
                  {translate('auto.mcp.sessions.col.toolCalls', 'Tool calls')}
                </TableHead>
                <TableHead scope="col">
                  {translate('auto.mcp.sessions.col.protocol', 'Protocol')}
                </TableHead>
                <TableHead scope="col">
                  <span className="sr-only">
                    {translate('auto.mcp.sessions.col.actions', 'Actions')}
                  </span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sessions.map((s) => {
                const foreign = scope === 'all' && !!s.userId && s.userId !== currentUser?.id
                const busy = closing.has(s.id)
                const button = (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={foreign || busy}
                    aria-label={translate(
                      'auto.mcp.sessions.closeAria',
                      'Close session for {{client}}',
                      {
                        client:
                          s.clientName ||
                          translate('auto.mcp.sessions.unknownClient', 'Unknown client')
                      }
                    )}
                    onClick={() => void onClose(s.id)}
                  >
                    {busy
                      ? translate('auto.mcp.sessions.closing', 'Closing…')
                      : translate('auto.mcp.sessions.close', 'Close')}
                  </Button>
                )
                return (
                  <TableRow key={s.id} data-testid={`mcp-session-${s.id}`}>
                    <TableCell>
                      {s.clientName ||
                        translate('auto.mcp.sessions.unknownClient', 'Unknown client')}
                    </TableCell>
                    {scope === 'all' ? (
                      <TableCell>{s.userName ?? (s.userId ?? '').slice(0, 8)}</TableCell>
                    ) : null}
                    <TableCell>
                      <Time iso={s.createdAt} />
                    </TableCell>
                    <TableCell>
                      <Time iso={s.lastSeenAt} />
                    </TableCell>
                    <TableCell>{s.activeStreams}</TableCell>
                    <TableCell>{s.toolCalls}</TableCell>
                    <TableCell className="font-mono text-xs">{s.protocolVersion}</TableCell>
                    <TableCell className="text-right">
                      {foreign ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span tabIndex={0}>{button}</span>
                          </TooltipTrigger>
                          <TooltipContent>
                            {translate(
                              'auto.mcp.sessions.ownerOnly',
                              'Only the session owner can close it. To block an agent, use the kill switch.'
                            )}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        button
                      )}
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </TooltipProvider>
      ) : null}
    </section>
  )
}
