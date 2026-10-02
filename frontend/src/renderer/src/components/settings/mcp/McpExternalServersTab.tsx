import { useMemo, useState } from 'react'
import { PlusIcon } from 'lucide-react'
import { toast } from 'sonner'
import type { McpExternalServer } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { useAppStore } from '@/store'
import { selectMcpKillSwitchActive } from '@/store/slices/mcp-slice'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { McpExternalServerDialog } from './McpExternalServerDialog'
import { McpExternalServerList } from './McpExternalServerList'
import { McpExternalServerReviewDialog } from './McpExternalServerReviewDialog'
import { scopeLabel } from './McpExternalServerStatus'
import { externalServerErrorText } from './mcp-external-server-validation'
import { useExternalServers } from './use-external-servers'

type Dialogs =
  | { kind: 'edit'; server: McpExternalServer | null }
  | { kind: 'review'; server: McpExternalServer }
type ScopeFilter = 'all' | McpExternalServer['scope']

export function McpExternalServersTab(): React.JSX.Element {
  const api = useExternalServers()
  const confirm = useConfirmationDialog()
  const isAdmin = useAppStore((s) => s.currentUser?.role === 'admin')
  const paused = useAppStore(selectMcpKillSwitchActive)
  const [filter, setFilter] = useState('')
  const [scope, setScope] = useState<ScopeFilter>('all')
  const [dialog, setDialog] = useState<Dialogs | null>(null)
  const [busy, setBusy] = useState<ReadonlySet<string>>(new Set())

  const rows = useMemo(() => {
    const q = filter.trim().toLowerCase()
    return [...api.data]
      .filter(
        (s) => (scope === 'all' || s.scope === scope) && (!q || s.name.toLowerCase().includes(q))
      )
      .sort((a, b) => a.name.localeCompare(b.name))
  }, [api.data, filter, scope])

  const onDelete = async (s: McpExternalServer): Promise<void> => {
    const ok = await confirm({
      title: translate('auto.mcp.external.deleteTitle', 'Delete server {{name}}?', {
        name: s.name
      }),
      description: translate(
        'auto.mcp.external.deleteBody',
        'Agents will stop receiving this server on their next start. Stored secrets are deleted.'
      ),
      confirmLabel: translate('auto.mcp.external.delete', 'Delete'),
      confirmVariant: 'destructive'
    })
    if (!ok) {
      return
    }
    setBusy((b) => new Set(b).add(s.id))
    try {
      await api.remove(s.id)
      toast.success(translate('auto.mcp.external.deleted', 'Server deleted'))
    } catch (e) {
      const err = parseMcpError(e)
      toast.error(externalServerErrorText(err.code, err.detail))
    } finally {
      setBusy((b) => {
        const next = new Set(b)
        next.delete(s.id)
        return next
      })
    }
  }

  if (api.status === 'loading') {
    return <McpListSkeleton rows={3} />
  }
  if (api.status === 'forbidden') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.external.forbidden', 'You do not have access to external servers.')}
      </McpMutedNote>
    )
  }
  if (api.status === 'unavailable') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.external.unavailable', 'MCP is turned off for this organization.')}
      </McpMutedNote>
    )
  }
  if (api.status === 'error') {
    return <McpInlineAlert message={api.error ?? ''} onRetry={api.reload} />
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.mcp.external.description',
          "External servers are managed by your organization and given to agents that Orca starts. They are separate from the MCP config files in a repository's settings (.mcp.json, Cursor, Claude), which only describe files on your machine."
        )}
      </p>
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.mcp.external.profileNote',
          'Servers a profile lists under mcp.servers are matched by name against this registry. Only approved servers are given to agents.'
        )}
      </p>
      {paused ? (
        <p role="status" className="rounded-md border border-border px-3 py-2 text-sm">
          {translate('auto.mcp.external.paused', 'MCP is paused by an admin.')}
        </p>
      ) : null}
      <div className="flex flex-wrap items-center gap-2">
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label={translate('auto.mcp.external.filter', 'Filter servers')}
          placeholder={translate('auto.mcp.external.filter', 'Filter servers')}
          className="h-8 max-w-xs"
        />
        {isAdmin ? (
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            value={scope}
            onValueChange={(v) => setScope((v || 'all') as ScopeFilter)}
            aria-label={translate('auto.mcp.external.scopeFilter', 'Filter by scope')}
          >
            <ToggleGroupItem value="all">
              {translate('auto.mcp.external.scope.all', 'All')}
            </ToggleGroupItem>
            {(['tenant', 'team', 'user'] as const).map((s) => (
              <ToggleGroupItem key={s} value={s}>
                {scopeLabel(s)}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        ) : null}
        <Button
          size="sm"
          disabled={paused}
          onClick={() => setDialog({ kind: 'edit', server: null })}
        >
          <PlusIcon aria-hidden />
          {isAdmin
            ? translate('auto.mcp.external.add', 'Add server')
            : translate('auto.mcp.external.addMine', 'Add my server')}
        </Button>
      </div>
      {api.data.length === 0 ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.external.empty',
            'No external MCP servers yet. Add a server so agents can use its tools once an admin approves it.'
          )}
        </McpMutedNote>
      ) : rows.length === 0 ? (
        <McpMutedNote>{translate('auto.mcp.external.noMatch', 'No servers match.')}</McpMutedNote>
      ) : (
        <McpExternalServerList
          servers={rows}
          isAdmin={isAdmin}
          busyIds={busy}
          paused={paused}
          onEdit={(s) => setDialog({ kind: 'edit', server: s })}
          onReview={(s) => setDialog({ kind: 'review', server: s })}
          onDelete={(s) => void onDelete(s)}
        />
      )}
      {dialog?.kind === 'edit' ? (
        <McpExternalServerDialog
          server={dialog.server}
          isAdmin={isAdmin}
          api={api}
          onClose={() => {
            setDialog(null)
            api.reload()
          }}
        />
      ) : null}
      {dialog?.kind === 'review' ? (
        <McpExternalServerReviewDialog
          server={dialog.server}
          api={api}
          paused={paused}
          onClose={() => {
            setDialog(null)
            api.reload()
          }}
        />
      ) : null}
    </div>
  )
}
