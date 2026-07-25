// ProvisionConfirmStep — confirm step before starting bulk provisioning (CR-003)
import { AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'

export function ProvisionConfirmStep({
  serverIds,
  onBack,
  onConfirm
}: {
  serverIds: string[]
  onBack: () => void
  onConfirm: () => void
}): React.JSX.Element {
  const sshTargets = useAppStore((s) => s.sshTargets)
  const selectedTargets = sshTargets.filter((t) => serverIds.includes(t.id))

  return (
    <div className="space-y-4">
      {/* Warning banner — replaces Alert (no alert.tsx in UI kit) */}
      <div className="flex items-start gap-3 rounded-md border border-yellow-500/30 bg-yellow-500/10 px-4 py-3 text-sm">
        <AlertTriangle className="mt-0.5 size-4 flex-shrink-0 text-yellow-600" />
        <p className="text-yellow-800 dark:text-yellow-300">
          {translate(
            'fleet.provision.confirmWarning',
            `This will deploy the Orca relay to ${serverIds.length} server(s). The relay binary will be uploaded via SFTP.`
          )}
        </p>
      </div>

      {/* Selected server list */}
      <div className="space-y-1 rounded-md border p-3">
        {selectedTargets.map((t) => (
          <div key={t.id} className="flex items-center gap-2 text-sm">
            <span className="flex-1 font-medium">{t.label}</span>
            <span className="text-xs text-muted-foreground">{t.host}</span>
          </div>
        ))}
      </div>

      <div className="flex justify-between">
        <Button variant="ghost" onClick={onBack}>
          {translate('common.back', 'Back')}
        </Button>
        <Button onClick={onConfirm}>
          {translate(
            'fleet.provision.startProvision',
            `Provision ${serverIds.length} server(s)`
          )}
        </Button>
      </div>
    </div>
  )
}
