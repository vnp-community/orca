import { useState } from 'react'
import { PencilIcon, SearchIcon, Trash2Icon } from 'lucide-react'
import { toast } from 'sonner'
import type { McpOAuthClient, McpToolPolicy, McpToolView } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { mcpRiskLabel } from '@/lib/mcp-labels'
import { formatMcpRelativeTime } from '@/lib/mcp-relative-time'
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
import { useMcpQuery } from '@/hooks/useMcpQuery'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { McpKillSwitchPanel } from './McpKillSwitchPanel'
import { McpPolicyEditorDialog, mcpDecisionLabel } from './McpPolicyEditorDialog'
import { McpPolicyExplainDialog } from './McpPolicyExplainDialog'
import { McpTenantSettingsForm } from './McpTenantSettingsForm'

type Dialogs =
  | { kind: 'edit'; policy: McpToolPolicy | null }
  | { kind: 'explain'; tool?: string }
  | null

function matchChips(p: McpToolPolicy, clients: McpOAuthClient[]): string[] {
  const m = p.match
  return [
    m.tool && `${translate('auto.mcp.policies.tool', 'Tool')}: ${m.tool}`,
    m.namespace && `${translate('auto.mcp.policies.namespace', 'Namespace')}: ${m.namespace}`,
    m.risk && `${translate('auto.mcp.policies.risk', 'Risk')}: ${mcpRiskLabel(m.risk)}`,
    m.clientId &&
      `${translate('auto.mcp.policies.client', 'Client')}: ${clients.find((c) => c.clientId === m.clientId)?.name ?? m.clientId}`,
    m.roles?.length && `${translate('auto.mcp.policies.roles', 'Roles')}: ${m.roles.join(', ')}`
  ].filter((x): x is string => typeof x === 'string' && x !== '')
}

function PolicyTable({
  policies,
  clients,
  onEdit,
  onExplain,
  onDelete
}: {
  policies: McpToolPolicy[]
  clients: McpOAuthClient[]
  onEdit: (p: McpToolPolicy) => void
  onExplain: (p: McpToolPolicy) => void
  onDelete: (p: McpToolPolicy) => void
}): React.JSX.Element {
  return (
    <Table>
      <caption className="sr-only">
        {translate('auto.mcp.policies.caption', 'Tool policies')}
      </caption>
      <TableHeader>
        <TableRow>
          <TableHead scope="col">{translate('auto.mcp.policies.col.match', 'Match')}</TableHead>
          <TableHead scope="col">
            {translate('auto.mcp.policies.decisionLabel', 'Decision')}
          </TableHead>
          <TableHead scope="col">{translate('auto.mcp.policies.col.updated', 'Updated')}</TableHead>
          <TableHead scope="col">{translate('auto.mcp.policies.col.version', 'Version')}</TableHead>
          <TableHead scope="col">
            <span className="sr-only">{translate('auto.mcp.sessions.col.actions', 'Actions')}</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {policies.map((p) => {
          const label = matchChips(p, clients).join(' · ')
          return (
            <TableRow key={p.id} id={`mcp-policy-${p.id}`}>
              <TableCell>
                <div className="flex flex-wrap gap-1">
                  {matchChips(p, clients).map((c) => (
                    <Badge key={c} variant="outline" className="font-mono text-xs">
                      {c}
                    </Badge>
                  ))}
                </div>
                {p.note ? <p className="mt-1 text-xs text-muted-foreground">{p.note}</p> : null}
              </TableCell>
              <TableCell>
                <Badge
                  variant={
                    p.decision === 'deny'
                      ? 'destructive'
                      : p.decision === 'allow'
                        ? 'secondary'
                        : 'outline'
                  }
                >
                  {mcpDecisionLabel(p.decision)}
                </Badge>
              </TableCell>
              <TableCell className="text-xs">
                {formatMcpRelativeTime(p.updatedAt)} · {p.updatedBy}
              </TableCell>
              <TableCell className="font-mono text-xs">v{p.version}</TableCell>
              <TableCell className="text-right">
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={translate('auto.mcp.policies.editRule', 'Edit rule: {{label}}', {
                    label
                  })}
                  onClick={() => onEdit(p)}
                >
                  <PencilIcon aria-hidden />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={translate(
                    'auto.mcp.policies.explainRule',
                    'Explain rule: {{label}}',
                    { label }
                  )}
                  onClick={() => onExplain(p)}
                >
                  <SearchIcon aria-hidden />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={translate('auto.mcp.policies.deleteRule', 'Delete rule: {{label}}', {
                    label
                  })}
                  onClick={() => onDelete(p)}
                >
                  <Trash2Icon aria-hidden />
                </Button>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}

export function McpPoliciesTab(): React.JSX.Element {
  const policies = useMcpQuery('mcp.admin.policy.list', undefined, [] as McpToolPolicy[])
  const tools = useMcpQuery('mcp.admin.tool.list', {}, [] as McpToolView[])
  const clients = useMcpQuery('mcp.admin.client.list', undefined, [] as McpOAuthClient[])
  const confirm = useConfirmationDialog()
  const [dialog, setDialog] = useState<Dialogs>(null)

  const remove = async (p: McpToolPolicy): Promise<void> => {
    const ok = await confirm({
      title: translate('auto.mcp.policies.deleteTitle', 'Delete this rule?'),
      description: translate(
        'auto.mcp.policies.deleteBody',
        'This rule stops applying immediately.'
      ),
      confirmLabel: translate('auto.mcp.policies.delete', 'Delete'),
      confirmVariant: 'destructive'
    })
    if (!ok) {
      return
    }
    try {
      await mcpClient.call('mcp.admin.policy.delete', { policyId: p.id })
    } catch (e) {
      if (parseMcpError(e).code !== 'MCP_NOT_FOUND') {
        toast.error(parseMcpError(e).detail)
        return
      }
    }
    policies.reload()
  }

  let body: React.ReactNode
  if (policies.status === 'loading') {
    body = <McpListSkeleton />
  } else if (policies.status === 'forbidden') {
    body = (
      <McpMutedNote>
        {translate('auto.mcp.admin.required', 'Administrator access required.')}
      </McpMutedNote>
    )
  } else if (policies.status === 'unavailable') {
    body = (
      <McpMutedNote>
        {translate(
          'auto.mcp.policies.unavailable',
          "Policies aren't available on this server yet."
        )}
      </McpMutedNote>
    )
  } else if (policies.status === 'error') {
    body = <McpInlineAlert message={policies.error ?? ''} onRetry={policies.reload} />
  } else if (policies.data.length === 0) {
    body = (
      <McpMutedNote>
        {translate(
          'auto.mcp.policies.empty',
          "No custom policies. Tools follow Orca's defaults: read/write allowed, exec/destructive need approval, admin denied."
        )}
      </McpMutedNote>
    )
  } else {
    body = (
      <PolicyTable
        policies={policies.data}
        clients={clients.data}
        onEdit={(p) => setDialog({ kind: 'edit', policy: p })}
        onExplain={(p) => setDialog({ kind: 'explain', tool: p.match.tool })}
        onDelete={(p) => void remove(p)}
      />
    )
  }

  return (
    <div className="space-y-6">
      <McpTenantSettingsForm />
      <McpKillSwitchPanel />
      <section aria-labelledby="mcp-pol-title" className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <h3 id="mcp-pol-title" className="text-sm font-medium">
            {translate('auto.mcp.policies.title', 'Tool policies')}
          </h3>
          {policies.status === 'ready' || policies.data.length > 0 ? (
            <div className="flex gap-2">
              <Button size="sm" variant="outline" onClick={() => setDialog({ kind: 'explain' })}>
                {translate('auto.mcp.policies.explainAction', 'Explain a tool call')}
              </Button>
              <Button size="sm" onClick={() => setDialog({ kind: 'edit', policy: null })}>
                {translate('auto.mcp.policies.add', 'Add policy')}
              </Button>
            </div>
          ) : null}
        </div>
        {body}
      </section>
      {dialog?.kind === 'edit' ? (
        <McpPolicyEditorDialog
          policy={dialog.policy}
          tools={tools.data}
          clients={clients.data}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null)
            policies.reload()
          }}
          onGone={() => {
            setDialog(null)
            policies.reload()
          }}
        />
      ) : null}
      {dialog?.kind === 'explain' ? (
        <McpPolicyExplainDialog
          tools={tools.data}
          clients={clients.data}
          initialTool={dialog.tool}
          onClose={() => setDialog(null)}
        />
      ) : null}
    </div>
  )
}
