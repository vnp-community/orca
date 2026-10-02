import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import type { McpAdminSettings } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useConfirmationDialog } from '@/components/confirmation-dialog'
import { useMcpQuery } from '@/hooks/useMcpQuery'
import { useAppStore } from '@/store'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { parseMcpError } from '@/runtime/runtime-mcp-error'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import {
  APPROVAL_TTL_RANGE,
  MAX_TOKEN_DAYS_RANGE,
  diffSettings,
  pickSettingsValues,
  validateSettings,
  type McpSettingsValues
} from './mcp-settings-form'

export function McpTenantSettingsForm(): React.JSX.Element {
  const q = useMcpQuery('mcp.admin.settings.get', undefined, null as McpAdminSettings | null)
  const confirm = useConfirmationDialog()
  const [base, setBase] = useState<McpSettingsValues | null>(null)
  const [values, setValues] = useState<McpSettingsValues | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  useEffect(() => {
    if (q.data) {
      const v = pickSettingsValues(q.data)
      setBase(v)
      setValues(v)
    }
  }, [q.data])

  if (q.status === 'loading' || (q.status === 'ready' && !values)) {
    return <McpListSkeleton rows={2} />
  }
  if (q.status === 'forbidden') {
    return (
      <McpMutedNote>
        {translate('auto.mcp.admin.required', 'Administrator access required.')}
      </McpMutedNote>
    )
  }
  if (q.status === 'unavailable') {
    return (
      <McpMutedNote>
        {translate(
          'auto.mcp.settings.unavailable',
          "Settings aren't available on this server yet."
        )}
      </McpMutedNote>
    )
  }
  if (q.status === 'error' || !values || !base) {
    return <McpInlineAlert message={q.error ?? ''} onRetry={q.reload} />
  }

  const errors = validateSettings(values)
  const patch = diffSettings(base, values)
  const dirty = Object.keys(patch).length > 0
  const set = (p: Partial<McpSettingsValues>): void => setValues({ ...values, ...p })
  const num = (raw: string): number => (raw.trim() === '' ? Number.NaN : Number(raw))

  const save = async (): Promise<void> => {
    if (patch.enabled === false) {
      const ok = await confirm({
        title: translate('auto.mcp.settings.disableTitle', 'Turn off MCP?'),
        description: translate(
          'auto.mcp.settings.disableBody',
          'AI clients will be rejected until re-enabled; existing sessions are not closed. Use the kill switch for that.'
        ),
        confirmLabel: translate('auto.mcp.settings.disableConfirm', 'Turn off'),
        confirmVariant: 'default'
      })
      if (!ok) {
        return
      }
    }
    setSaving(true)
    setSaveError(null)
    try {
      const next = await mcpClient.call('mcp.admin.settings.set', patch)
      const v = pickSettingsValues(next)
      setBase(v)
      setValues(v)
      toast.success(translate('auto.mcp.settings.saved', 'Settings saved'))
      void useAppStore.getState().refreshMcpServerInfo()
    } catch (e) {
      setSaveError(parseMcpError(e).detail)
    } finally {
      setSaving(false)
    }
  }

  const rangeText = (r: { min: number; max: number }): string =>
    translate('auto.mcp.settings.range', 'Enter a whole number from {{min}} to {{max}}.', r)

  return (
    <section aria-labelledby="mcp-settings-title" className="space-y-3">
      <h3 id="mcp-settings-title" className="text-sm font-medium">
        {translate('auto.mcp.settings.title', 'Organization settings')}
      </h3>
      {!values.enabled ? (
        <McpMutedNote>
          {translate('auto.mcp.pane.disabledTitle', 'MCP is turned off for your organization')}
        </McpMutedNote>
      ) : null}
      <div className="flex items-center gap-2">
        <Checkbox
          id="mcp-set-enabled"
          checked={values.enabled}
          onCheckedChange={(c) => set({ enabled: c === true })}
        />
        <Label htmlFor="mcp-set-enabled">
          {translate('auto.mcp.settings.enabled', 'Enable MCP for this organization')}
        </Label>
      </div>
      <div className="space-y-1">
        <div className="flex items-center gap-2">
          <Checkbox
            id="mcp-set-dcr"
            checked={values.dcrEnabled}
            aria-describedby="mcp-set-dcr-help"
            onCheckedChange={(c) => set({ dcrEnabled: c === true })}
          />
          <Label htmlFor="mcp-set-dcr">
            {translate('auto.mcp.settings.dcr', 'Allow dynamic client registration')}
          </Label>
        </div>
        <p id="mcp-set-dcr-help" className="pl-6 text-xs text-muted-foreground">
          {translate(
            'auto.mcp.settings.dcrHelp',
            'Any MCP client can register itself and ask users for consent.'
          )}
        </p>
      </div>
      <div className="grid max-w-md gap-3 sm:grid-cols-2">
        <div className="space-y-1">
          <Label htmlFor="mcp-set-days">
            {translate('auto.mcp.settings.maxDays', 'Max token lifetime (days)')}
          </Label>
          <Input
            id="mcp-set-days"
            type="number"
            min={MAX_TOKEN_DAYS_RANGE.min}
            max={MAX_TOKEN_DAYS_RANGE.max}
            value={Number.isNaN(values.maxTokenDays) ? '' : values.maxTokenDays}
            aria-invalid={errors.maxTokenDays ? true : undefined}
            aria-describedby="mcp-set-days-msg"
            onChange={(e) => set({ maxTokenDays: num(e.target.value) })}
          />
          <p id="mcp-set-days-msg" className="text-xs text-muted-foreground">
            {errors.maxTokenDays
              ? rangeText(MAX_TOKEN_DAYS_RANGE)
              : translate('auto.mcp.settings.maxDaysHelp', 'Applies to personal access tokens.')}
          </p>
        </div>
        <div className="space-y-1">
          <Label htmlFor="mcp-set-ttl">
            {translate('auto.mcp.settings.ttl', 'Approval expiry (seconds)')}
          </Label>
          <Input
            id="mcp-set-ttl"
            type="number"
            min={APPROVAL_TTL_RANGE.min}
            max={APPROVAL_TTL_RANGE.max}
            value={Number.isNaN(values.approvalTtlSeconds) ? '' : values.approvalTtlSeconds}
            aria-invalid={errors.approvalTtlSeconds ? true : undefined}
            aria-describedby="mcp-set-ttl-msg"
            onChange={(e) => set({ approvalTtlSeconds: num(e.target.value) })}
          />
          <p id="mcp-set-ttl-msg" className="text-xs text-muted-foreground">
            {errors.approvalTtlSeconds ? rangeText(APPROVAL_TTL_RANGE) : ' '}
          </p>
        </div>
      </div>
      {saveError ? <McpInlineAlert message={saveError} /> : null}
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          disabled={!dirty || saving || Object.keys(errors).length > 0}
          aria-busy={saving}
          onClick={() => void save()}
        >
          {translate('auto.mcp.settings.save', 'Save settings')}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={!dirty || saving}
          onClick={() => setValues(base)}
        >
          {translate('auto.mcp.settings.reset', 'Reset')}
        </Button>
        <span className="text-xs text-muted-foreground">
          {translate('auto.mcp.settings.lastWins', 'Last save wins.')}
        </span>
      </div>
    </section>
  )
}
