import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import type { McpGrant } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { useConfirmationDialog } from '@/components/confirmation-dialog'

/** Confirm -> revoke -> toast, with per-grant busy tracking; shared by user and admin grant tabs. */
export function useMcpGrantRevokeFlow(
  revoke: (grantId: string) => Promise<void>,
  describeOwner?: (g: McpGrant) => string
): { busy: ReadonlySet<string>; requestRevoke: (g: McpGrant) => void } {
  const confirm = useConfirmationDialog()
  const [busy, setBusy] = useState<ReadonlySet<string>>(new Set())
  const requestRevoke = useCallback(
    (g: McpGrant): void => {
      void (async () => {
        const owner = describeOwner?.(g)
        const ok = await confirm({
          title: owner
            ? translate('auto.mcp.grants.revokeTitle', 'Revoke {{client}} for {{user}}?', {
                client: g.clientName,
                user: owner
              })
            : translate('auto.mcp.apps.revokeTitle', 'Revoke access for {{client}}?', {
                client: g.clientName
              }),
          description: translate(
            'auto.mcp.apps.revokeBody',
            'The app will be signed out within about a minute. You can reconnect it later.'
          ),
          confirmLabel: translate('auto.mcp.apps.revoke', 'Revoke'),
          confirmVariant: 'destructive'
        })
        if (!ok) {
          return
        }
        setBusy((s) => new Set(s).add(g.id))
        try {
          await revoke(g.id)
          toast.success(translate('auto.mcp.apps.revokedToast', 'Access revoked'))
        } catch (e) {
          toast.error(e instanceof Error ? e.message : String(e))
        } finally {
          setBusy((s) => {
            const next = new Set(s)
            next.delete(g.id)
            return next
          })
        }
      })()
    },
    [confirm, revoke, describeOwner]
  )
  return { busy, requestRevoke }
}
