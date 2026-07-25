// ProvisionDoneSummary — final step of FleetProvisionWizard showing outcome (CR-003)
import { CheckCircle2, XCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import type { ProvisioningSession } from '@/store/slices/provisioning'

export function ProvisionDoneSummary({
  session,
  onClose
}: {
  session: ProvisioningSession
  onClose: () => void
}): React.JSX.Element {
  const done = session.servers.filter((s) => s.status === 'done').length
  const failed = session.servers.filter((s) => s.status === 'error').length
  const skipped = session.servers.filter((s) => s.status === 'skipped').length
  const hasErrors = failed > 0

  return (
    <div className="space-y-4 py-2 text-center">
      {hasErrors ? (
        <XCircle className="mx-auto size-12 text-destructive" />
      ) : (
        <CheckCircle2 className="mx-auto size-12 text-green-500" />
      )}

      <div>
        <p className="text-lg font-semibold">
          {translate('fleet.provision.doneTitle', 'Provisioning Complete')}
        </p>
        <p className="text-sm text-muted-foreground">
          {done} {done === 1 ? 'server' : 'servers'} ready
          {failed > 0 && `, ${failed} failed`}
          {skipped > 0 && `, ${skipped} skipped`}
        </p>
      </div>

      {/* Per-server outcome summary */}
      <div className="space-y-1 text-left">
        {session.servers.map((entry) => (
          <div key={entry.serverId} className="flex items-center gap-2 text-sm">
            <span
              className={cn('size-1.5 flex-shrink-0 rounded-full', {
                'bg-green-500': entry.status === 'done',
                'bg-destructive': entry.status === 'error',
                'bg-muted-foreground/40': entry.status === 'skipped'
              })}
            />
            <span className="flex-1 truncate">{entry.label}</span>
            {entry.status === 'done' && entry.relayVersion && (
              <Badge variant="secondary" className="font-mono text-xs">
                v{entry.relayVersion}
              </Badge>
            )}
            {entry.status === 'error' && entry.error && (
              <span className="max-w-[160px] truncate text-xs text-destructive">
                {entry.error}
              </span>
            )}
          </div>
        ))}
      </div>

      <Button onClick={onClose} className="w-full">
        {translate('common.close', 'Close')}
      </Button>
    </div>
  )
}
