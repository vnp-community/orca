import { useState } from 'react'
import { toast } from 'sonner'
import type { McpKillSwitchEntry } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useMcpQuery } from '@/hooks/useMcpQuery'
import { useAppStore } from '@/store'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { formatMcpRelativeTime } from '@/lib/mcp-relative-time'
import { trackMcpKillSwitchToggled } from '@/lib/mcp-telemetry'
import { McpInlineAlert } from './McpListStates'
import { McpKillSwitchConfirmDialog } from './McpKillSwitchConfirmDialog'
import { McpKillSwitchTargetSelect } from './McpKillSwitchTargetSelect'

type Scope = McpKillSwitchEntry['scope']
const REASON_MIN = 3
const REASON_MAX = 500

function scopeLabel(s: Scope): string {
  switch (s) {
    case 'tenant':
      return translate('auto.mcp.killswitch.scope.tenant', 'Entire organization')
    case 'client':
      return translate('auto.mcp.killswitch.scope.client', 'OAuth client')
    case 'grant':
      return translate('auto.mcp.killswitch.scope.grant', 'Grant')
    default:
      return translate('auto.mcp.killswitch.scope.session', 'Session')
  }
}

export function McpKillSwitchPanel(): React.JSX.Element {
  const list = useMcpQuery('mcp.admin.killswitch.list', undefined, [] as McpKillSwitchEntry[])
  const tenantActive = useAppStore((s) => s.mcpServerInfo?.killSwitch?.active === true)
  const [scope, setScope] = useState<Scope>('tenant')
  const [targetId, setTargetId] = useState('')
  const [reason, setReason] = useState('')
  const [confirming, setConfirming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busyKey, setBusyKey] = useState<string | null>(null)

  const reasonValid = reason.trim().length >= REASON_MIN
  const valid = reasonValid && (scope === 'tenant' || targetId !== '')

  const send = async (
    s: Scope,
    target: string | undefined,
    active: boolean,
    why: string
  ): Promise<boolean> => {
    setError(null)
    try {
      await mcpClient.call('mcp.admin.killswitch.set', {
        scope: s,
        ...(target ? { targetId: target } : {}),
        active,
        reason: why
      })
      trackMcpKillSwitchToggled({ scope: s, active })
      void useAppStore.getState().refreshMcpServerInfo()
      list.reload()
      return true
    } catch (e) {
      setError(parseMcpError(e).detail)
      return false
    }
  }

  const activate = async (): Promise<void> => {
    if (await send(scope, scope === 'tenant' ? undefined : targetId, true, reason.trim())) {
      toast.success(translate('auto.mcp.killswitch.activated', 'Kill switch activated'))
      setConfirming(false)
      setReason('')
    } else {
      setConfirming(false)
    }
  }

  const resume = async (e: McpKillSwitchEntry): Promise<void> => {
    const key = `${e.scope}:${e.targetId ?? ''}`
    setBusyKey(key)
    const why = reasonValid
      ? reason.trim()
      : translate('auto.mcp.killswitch.resumeReason', 'Resumed by administrator')
    if (await send(e.scope, e.targetId, false, why)) {
      toast.success(translate('auto.mcp.killswitch.resumed', 'Access resumed'))
    }
    setBusyKey(null)
  }

  const rows: McpKillSwitchEntry[] = [...list.data]
  if (tenantActive && !rows.some((r) => r.scope === 'tenant')) {
    rows.unshift({ scope: 'tenant', reason: '', at: '', by: '' })
  }

  return (
    <section aria-labelledby="mcp-ks-title" className="space-y-3">
      <h3 id="mcp-ks-title" className="text-sm font-medium">
        {translate('auto.mcp.killswitch.title', 'Kill switch')}
      </h3>
      <p className="text-sm text-muted-foreground">
        {translate(
          'auto.mcp.killswitch.desc',
          'Immediately stop AI access. Running tools are cancelled and sessions are closed.'
        )}
      </p>
      <div className="grid max-w-xl gap-3 sm:grid-cols-2">
        <div className="space-y-1">
          <Label htmlFor="mcp-ks-scope">
            {translate('auto.mcp.killswitch.scopeLabel', 'Scope')}
          </Label>
          <Select
            value={scope}
            onValueChange={(v) => {
              setScope(v as Scope)
              setTargetId('')
            }}
          >
            <SelectTrigger id="mcp-ks-scope" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(['tenant', 'client', 'grant', 'session'] as Scope[]).map((s) => (
                <SelectItem key={s} value={s}>
                  {scopeLabel(s)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        {scope !== 'tenant' ? (
          <McpKillSwitchTargetSelect scope={scope} value={targetId} onChange={setTargetId} />
        ) : null}
      </div>
      <div className="max-w-xl space-y-1">
        <Label htmlFor="mcp-ks-reason">{translate('auto.mcp.killswitch.reason', 'Reason')}</Label>
        <Textarea
          id="mcp-ks-reason"
          value={reason}
          maxLength={REASON_MAX}
          onChange={(e) => setReason(e.target.value)}
          aria-describedby="mcp-ks-reason-help"
        />
        <p id="mcp-ks-reason-help" className="text-xs text-muted-foreground">
          {translate('auto.mcp.killswitch.reasonHelp', 'Required, at least {{min}} characters.', {
            min: REASON_MIN
          })}
        </p>
      </div>
      {error ? <McpInlineAlert message={error} /> : null}
      <Button variant="destructive" size="sm" disabled={!valid} onClick={() => setConfirming(true)}>
        {translate('auto.mcp.killswitch.activateAction', 'Activate kill switch…')}
      </Button>
      {rows.length > 0 ? (
        <ul
          className="max-w-xl space-y-2"
          aria-label={translate('auto.mcp.killswitch.activeList', 'Active kill switches')}
        >
          {rows.map((r) => {
            const key = `${r.scope}:${r.targetId ?? ''}`
            return (
              <li
                key={key}
                className="flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2 text-sm"
              >
                <span className="min-w-0">
                  <span className="font-medium">{scopeLabel(r.scope)}</span>
                  {r.targetId ? <span className="ml-1 font-mono text-xs">{r.targetId}</span> : null}
                  {r.reason ? (
                    <span className="block truncate text-xs text-muted-foreground">{r.reason}</span>
                  ) : null}
                  {r.at ? (
                    <span className="block text-xs text-muted-foreground">
                      {formatMcpRelativeTime(r.at)}
                    </span>
                  ) : null}
                </span>
                <Button size="sm" disabled={busyKey === key} onClick={() => void resume(r)}>
                  {translate('auto.mcp.killswitch.resume', 'Resume access')}
                </Button>
              </li>
            )
          })}
        </ul>
      ) : null}
      {confirming ? (
        <McpKillSwitchConfirmDialog
          scope={scope}
          onCancel={() => setConfirming(false)}
          onConfirm={activate}
        />
      ) : null}
    </section>
  )
}
