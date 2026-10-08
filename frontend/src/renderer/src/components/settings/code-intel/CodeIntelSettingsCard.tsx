import { useCallback, useEffect, useState } from 'react'
import { translate } from '@/i18n/i18n'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import type { CodeIntelSettings } from '../../../../../shared/code-intel-types'

type TenantFlag = 'codeIntelEnabled' | 'qualityGateEnabled'

export type CodeIntelSettingsApi = {
  get: () => Promise<CodeIntelSettings>
  // Why: contract settings.set requires at least one field, so callers send only the changed one.
  set: (patch: Partial<Record<TenantFlag, boolean>>) => Promise<CodeIntelSettings>
}

type Props = { isAdmin: boolean; api: CodeIntelSettingsApi }

function errorText(e: unknown): string {
  const msg = e instanceof Error ? e.message : String(e)
  return msg.startsWith('CODEINTEL_NOT_AUTHORIZED')
    ? translate('auto.settings.codeIntel.notAuthorized', 'Only administrators can change these switches.')
    : msg
}

export function CodeIntelSettingsCard({ isAdmin, api }: Props): React.JSX.Element {
  const [settings, setSettings] = useState<CodeIntelSettings | null>(null)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let live = true
    api
      .get()
      .then((s) => live && setSettings(s))
      .catch((e) => live && setError(errorText(e)))
    return () => {
      live = false
    }
  }, [api])

  const toggle = useCallback(
    async (flag: TenantFlag, value: boolean) => {
      setPending(true)
      setError(null)
      try {
        await api.set({ [flag]: value })
        setSettings(await api.get())
      } catch (e) {
        setError(errorText(e))
      } finally {
        setPending(false)
      }
    },
    [api]
  )

  if (!settings) {
    return <p className="text-sm text-muted-foreground">{error ?? translate('auto.settings.codeIntel.loading', 'Loading code intelligence settings')}</p>
  }
  const { tenant, effective } = settings
  const rows: { flag: TenantFlag; label: string; disabledByMaster: boolean }[] = [
    { flag: 'codeIntelEnabled', label: translate('auto.settings.codeIntel.codeIntelLabel', 'Code intelligence'), disabledByMaster: false },
    { flag: 'qualityGateEnabled', label: translate('auto.settings.codeIntel.qualityLabel', 'Quality gate'), disabledByMaster: !tenant.codeIntelEnabled }
  ]
  return (
    <div className="space-y-3">
      {rows.map((r) => {
        const on = tenant[r.flag]
        const off = !effective[r.flag]
        const id = `code-intel-${r.flag}`
        return (
          <div key={r.flag} className="space-y-1">
            <div className="flex items-center gap-2">
              <Checkbox
                id={id}
                checked={on}
                disabled={!isAdmin || pending || r.disabledByMaster}
                onCheckedChange={(v) => void toggle(r.flag, v === true)}
              />
              <Label htmlFor={id}>{r.label}</Label>
            </div>
            {r.disabledByMaster ? (
              <p className="text-xs text-muted-foreground">
                {translate('auto.settings.codeIntel.needsMaster', 'Turn on code intelligence first.')}
              </p>
            ) : null}
            {on && off ? (
              <p className="text-xs text-muted-foreground">
                {translate('auto.settings.codeIntel.serverOff', 'Your organization turned this on, but the server switch is currently off.')}
              </p>
            ) : null}
          </div>
        )
      })}
      {!isAdmin ? (
        <p className="text-xs text-muted-foreground">
          {translate('auto.settings.codeIntel.readOnly', 'Ask an administrator to change these switches.')}
        </p>
      ) : null}
      {error ? <p role="alert" className="text-xs text-destructive">{error}</p> : null}
    </div>
  )
}
