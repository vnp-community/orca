import { useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { McpKillSwitchEntry } from '../../../../../shared/mcp-types'

// Constant, deliberately not translated: avoids locale drift in what must be typed.
export const MCP_KILL_SWITCH_CONFIRM_WORD = 'STOP'

export const isKillSwitchConfirmed = (typed: string): boolean =>
  typed.trim() === MCP_KILL_SWITCH_CONFIRM_WORD

function impactText(scope: McpKillSwitchEntry['scope']): string {
  switch (scope) {
    case 'tenant':
      return translate(
        'auto.mcp.killswitch.impact.tenant',
        'All MCP access stops within about a minute. Running tools are cancelled, sessions are closed and OAuth refresh tokens are revoked. Personal access tokens are blocked until you resume.'
      )
    case 'client':
      return translate(
        'auto.mcp.killswitch.impact.client',
        'Everything this app does stops within about a minute and its sessions are closed.'
      )
    case 'grant':
      return translate(
        'auto.mcp.killswitch.impact.grant',
        'This connection stops within about a minute and must be authorized again.'
      )
    default:
      return translate(
        'auto.mcp.killswitch.impact.session',
        'This session is closed within about a minute and its running tools are cancelled.'
      )
  }
}

export function McpKillSwitchConfirmDialog({
  scope,
  onCancel,
  onConfirm
}: {
  scope: McpKillSwitchEntry['scope']
  onCancel: () => void
  onConfirm: () => Promise<void>
}): React.JSX.Element {
  const [typed, setTyped] = useState('')
  const [busy, setBusy] = useState(false)
  const ok = isKillSwitchConfirmed(typed)

  const submit = async (): Promise<void> => {
    if (!ok || busy) {
      return
    }
    setBusy(true)
    try {
      await onConfirm()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => (!o && !busy ? onCancel() : undefined)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {translate('auto.mcp.killswitch.confirmTitle', 'Activate kill switch?')}
          </DialogTitle>
          <DialogDescription>{impactText(scope)}</DialogDescription>
        </DialogHeader>
        <div className="space-y-1">
          <Label htmlFor="mcp-ks-confirm">
            {translate('auto.mcp.killswitch.typeToConfirm', 'Type {{word}} to confirm', {
              word: MCP_KILL_SWITCH_CONFIRM_WORD
            })}
          </Label>
          <Input
            id="mcp-ks-confirm"
            autoComplete="off"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                void submit()
              }
            }}
          />
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel} disabled={busy}>
            {translate('auto.mcp.common.cancel', 'Cancel')}
          </Button>
          <Button
            variant="destructive"
            disabled={!ok || busy}
            aria-busy={busy}
            onClick={() => void submit()}
          >
            {translate('auto.mcp.killswitch.activate', 'Activate')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
