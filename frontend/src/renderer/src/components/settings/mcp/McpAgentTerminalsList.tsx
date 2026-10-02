import { useEffect, useRef, useState } from 'react'
import { BotIcon, Loader2Icon } from 'lucide-react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { selectMcpEnabled } from '@/store/slices/mcp-slice'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { useMcpTerminalOrigins } from '@/hooks/useMcpTerminalOrigins'
import { STOP_GRACE_MS, useStopMcpTerminal } from '@/hooks/useStopMcpTerminal'
import { displayMcpClientName } from '@/lib/mcp-terminal-origin'
import { McpMutedNote } from './McpListStates'

const shortHandle = (h: string): string => (h.length > 12 ? `${h.slice(0, 8)}…${h.slice(-3)}` : h)

/** Stop path that works even when the agent PTY has no tab in this client. */
export function McpAgentTerminalsList(): React.JSX.Element | null {
  useMcpTerminalOrigins()
  const enabled = useAppStore(selectMcpEnabled)
  const origins = useAppStore((s) => s.mcpOriginByHandle)
  const refreshedAt = useAppStore((s) => s.mcpOriginsRefreshedAt)
  const stop = useStopMcpTerminal()
  const [stopping, setStopping] = useState<Set<string>>(new Set())
  const [forceable, setForceable] = useState<Set<string>>(new Set())
  const timers = useRef(new Set<ReturnType<typeof setTimeout>>())
  useEffect(() => {
    const live = timers.current
    return () => live.forEach(clearTimeout)
  }, [])

  if (!enabled) {
    return null
  }
  const entries = Object.entries(origins ?? {})

  const mark = (set: Set<string>, h: string, on: boolean): Set<string> => {
    const next = new Set(set)
    if (on) {
      next.add(h)
    } else {
      next.delete(h)
    }
    return next
  }
  const onStop = async (handle: string, force: boolean): Promise<void> => {
    setStopping((s) => mark(s, handle, true))
    try {
      await stop(handle, { force })
      toast.success(translate('auto.mcp.origin.stopped', 'Process stopped'))
      if (!force) {
        // Offer force-close only if the entry survives the grace period.
        const t = setTimeout(() => {
          timers.current.delete(t)
          if (useAppStore.getState().mcpOriginByHandle?.[handle]) {
            setForceable((s) => mark(s, handle, true))
          }
        }, STOP_GRACE_MS)
        timers.current.add(t)
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    } finally {
      setStopping((s) => mark(s, handle, false))
    }
  }

  return (
    <section className="space-y-2" aria-labelledby="mcp-agent-terminals-heading">
      <h3
        id="mcp-agent-terminals-heading"
        className="flex items-center gap-1.5 text-sm font-medium"
      >
        <BotIcon className="size-4" aria-hidden />
        {translate('auto.mcp.origin.listTitle', 'Agent-created terminals')}
      </h3>
      {entries.length === 0 ? (
        <McpMutedNote>
          {refreshedAt === null
            ? translate('auto.mcp.common.loading', 'Loading…')
            : translate('auto.mcp.origin.empty', 'No terminals started by agents.')}
        </McpMutedNote>
      ) : (
        <Table>
          <TableHeader>
            <tr>
              <TableHead>{translate('auto.mcp.origin.colHandle', 'Terminal')}</TableHead>
              <TableHead>{translate('auto.mcp.origin.colClient', 'Client')}</TableHead>
              <TableHead>{translate('auto.mcp.origin.colSession', 'Session')}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">
                  {translate('auto.mcp.origin.colActions', 'Actions')}
                </span>
              </TableHead>
            </tr>
          </TableHeader>
          <TableBody>
            {entries.map(([handle, origin]) => {
              const name = displayMcpClientName(origin.clientName)
              const busy = stopping.has(handle)
              const force = forceable.has(handle)
              return (
                <TableRow key={handle}>
                  <TableCell className="font-mono text-xs" title={handle}>
                    {shortHandle(handle)}
                  </TableCell>
                  <TableCell title={origin.clientName}>{name}</TableCell>
                  <TableCell className="font-mono text-xs" title={origin.mcpSessionId}>
                    {shortHandle(origin.mcpSessionId)}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={busy}
                      aria-label={
                        force
                          ? translate(
                              'auto.mcp.origin.forceCloseAria',
                              'Force close terminal started by {{name}}',
                              { name }
                            )
                          : translate(
                              'auto.mcp.origin.stopAria',
                              'Stop terminal started by {{name}}',
                              { name }
                            )
                      }
                      onClick={() => void onStop(handle, force)}
                    >
                      {busy ? <Loader2Icon className="animate-spin" aria-hidden /> : null}
                      {force
                        ? translate('auto.mcp.origin.forceClose', 'Force close')
                        : translate('auto.mcp.origin.stop', 'Stop')}
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
    </section>
  )
}
