import { useState } from 'react'
import { toast } from 'sonner'
import type { McpToken } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { selectMcpKillSwitchActive } from '@/store/slices/mcp-slice'
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
import { McpCreateTokenDialog } from './McpCreateTokenDialog'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { McpScopeBadges } from './McpScopeBadges'
import { McpTokenCliSnippet } from './McpTokenCliSnippet'
import { useMcpTokens } from './use-mcp-tokens'

const SOON_MS = 7 * 24 * 60 * 60 * 1000

function StatusBadge({ token }: { token: McpToken }): React.JSX.Element {
  if (token.status === 'active') {
    return <Badge variant="secondary">{translate('auto.mcp.tokens.status.active', 'Active')}</Badge>
  }
  return (
    <Badge variant="outline">
      {token.status === 'revoked'
        ? translate('auto.mcp.tokens.status.revoked', 'Revoked')
        : translate('auto.mcp.tokens.status.expired', 'Expired')}
    </Badge>
  )
}

export function McpAccessTokensTab(): React.JSX.Element {
  const info = useAppStore((s) => s.mcpServerInfo)
  const role = useAppStore((s) => s.currentUser?.role)
  const refreshInfo = useAppStore((s) => s.refreshMcpServerInfo)
  const killSwitch = useAppStore(selectMcpKillSwitchActive)
  const { tokens, status, error, reload, addCreated, revoke } = useMcpTokens()
  const confirm = useConfirmationDialog()
  const [dialogOpen, setDialogOpen] = useState(false)
  const createBlocked = killSwitch || info?.enabled === false

  const onRevoke = async (t: McpToken): Promise<void> => {
    const ok = await confirm({
      title: translate('auto.mcp.tokens.revokeTitle', 'Revoke "{{name}}"?', {
        name: t.name
      }),
      description: translate(
        'auto.mcp.tokens.revokeBody',
        "Anything using this token stops working within about a minute. This can't be undone."
      ),
      confirmLabel: translate('auto.mcp.tokens.revokeConfirm', 'Revoke token'),
      confirmVariant: 'destructive'
    })
    if (!ok) {
      return
    }
    try {
      await revoke(t.id)
      toast.success(translate('auto.mcp.tokens.revokedToast', 'Token revoked'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const createButton = (
    <Button disabled={createBlocked || !info} onClick={() => setDialogOpen(true)}>
      {translate('auto.mcp.tokens.create', 'Create token')}
    </Button>
  )

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          {translate(
            'auto.mcp.tokens.intro',
            'Access tokens let scripts and CI agents use Orca without a browser.'
          )}
        </p>
        {createButton}
      </div>
      {killSwitch ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.tokens.killSwitchNote',
            'MCP access is paused, so new tokens cannot be created. You can still revoke existing ones.'
          )}
        </McpMutedNote>
      ) : null}
      {status === 'loading' ? <McpListSkeleton /> : null}
      {status === 'unavailable' ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.tokens.unavailable',
            "Access tokens aren't available on this server yet."
          )}
        </McpMutedNote>
      ) : null}
      {status === 'error' || status === 'forbidden' ? (
        <McpInlineAlert message={error ?? ''} onRetry={reload} />
      ) : null}
      {status === 'ready' && tokens.length === 0 ? (
        <McpMutedNote>{translate('auto.mcp.tokens.empty', 'No access tokens yet.')}</McpMutedNote>
      ) : null}
      {status === 'ready' && tokens.length > 0 ? (
        <Table>
          <caption className="sr-only">
            {translate('auto.mcp.tokens.caption', 'Your access tokens')}
          </caption>
          <TableHeader>
            <TableRow>
              <TableHead scope="col">{translate('auto.mcp.tokens.name', 'Name')}</TableHead>
              <TableHead scope="col">
                {translate('auto.mcp.tokens.permissions', 'Permissions')}
              </TableHead>
              <TableHead scope="col">
                {translate('auto.mcp.tokens.col.created', 'Created')}
              </TableHead>
              <TableHead scope="col">
                {translate('auto.mcp.tokens.col.expires', 'Expires')}
              </TableHead>
              <TableHead scope="col">
                {translate('auto.mcp.apps.col.lastUsed', 'Last used')}
              </TableHead>
              <TableHead scope="col">
                {translate('auto.mcp.clients.col.status', 'Status')}
              </TableHead>
              <TableHead scope="col">
                <span className="sr-only">
                  {translate('auto.mcp.sessions.col.actions', 'Actions')}
                </span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tokens.map((t) => {
              const inactive = t.status !== 'active'
              const soon = !inactive && Date.parse(t.expiresAt) - Date.now() < SOON_MS
              return (
                <TableRow
                  key={t.id}
                  className={inactive ? 'opacity-60' : undefined}
                  data-testid={`mcp-token-${t.id}`}
                >
                  <TableCell className="font-medium">{t.name}</TableCell>
                  <TableCell>
                    <McpScopeBadges scopes={t.scopes} info={info} />
                  </TableCell>
                  <TableCell>{new Date(t.createdAt).toLocaleDateString()}</TableCell>
                  <TableCell>
                    {new Date(t.expiresAt).toLocaleDateString()}{' '}
                    {soon ? (
                      <Badge variant="outline">
                        {translate('auto.mcp.tokens.expiresSoon', 'Expires soon')}
                      </Badge>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    {t.lastUsedAt
                      ? new Date(t.lastUsedAt).toLocaleString()
                      : translate('auto.mcp.tokens.neverUsed', 'Never used')}
                  </TableCell>
                  <TableCell>
                    <StatusBadge token={t} />
                  </TableCell>
                  <TableCell className="text-right">
                    {t.status === 'active' ? (
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={translate(
                          'auto.mcp.tokens.revokeAria',
                          'Revoke token {{name}}',
                          {
                            name: t.name
                          }
                        )}
                        onClick={() => void onRevoke(t)}
                      >
                        {translate('auto.mcp.apps.revoke', 'Revoke')}
                      </Button>
                    ) : null}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      ) : null}
      {info?.resourceUrl ? (
        <section className="space-y-2">
          <h3 className="text-sm font-medium">
            {translate('auto.mcp.tokens.cliTitle', 'Use a token from the command line')}
          </h3>
          <McpTokenCliSnippet url={info.resourceUrl} />
        </section>
      ) : null}
      <McpCreateTokenDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onCreated={addCreated}
        maxTokenDays={info?.maxTokenDays}
        scopes={info?.scopesSupported ?? []}
        role={role}
        resourceUrl={info?.resourceUrl ?? ''}
        createBlocked={createBlocked}
        onRefreshServerInfo={() => void refreshInfo()}
      />
    </div>
  )
}
