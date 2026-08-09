// FleetImportProgress — progress indicator for fleet config import (CR-001, TASK-001-C)
import { Progress } from '@/components/ui/progress'
import { Badge } from '@/components/ui/badge'
import { translate } from '@/i18n/i18n'
import type { FleetImportStatus } from '@/store/slices/ssh'

type FleetImportProgressProps = {
  status: FleetImportStatus
}

export function FleetImportProgress({ status }: FleetImportProgressProps): React.JSX.Element {
  const progressPercent =
    status.totalServers > 0
      ? Math.round(
          ((status.importedServers + status.skippedServers + status.failedServers) /
            status.totalServers) *
            100
        )
      : 0

  return (
    <div className="space-y-3">
      {/* Phase label */}
      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">
          {status.phase === 'reading' &&
            translate('fleet.import.reading', 'Reading fleet config...')}
          {status.phase === 'validating' &&
            translate('fleet.import.validating', 'Validating servers...')}
          {status.phase === 'importing' &&
            translate('fleet.import.importing', 'Importing servers...')}
          {status.phase === 'done' && translate('fleet.import.done', 'Import complete')}
          {status.phase === 'error' && translate('fleet.import.error', 'Import failed')}
          {status.phase === 'idle' && translate('fleet.import.idle', 'Preparing...')}
        </span>
        <span className="text-xs tabular-nums text-muted-foreground">{progressPercent}%</span>
      </div>

      {/* Progress bar */}
      <Progress value={progressPercent} className="h-2" />

      {/* Stats row */}
      <div className="flex items-center gap-2 text-xs tabular-nums">
        {status.importedServers > 0 && (
          <Badge variant="outline" className="border-green-500/40 text-green-600 dark:text-green-400">
            {status.importedServers} {translate('fleet.import.imported', 'imported')}
          </Badge>
        )}
        {status.skippedServers > 0 && (
          <Badge variant="outline" className="text-muted-foreground">
            {status.skippedServers} {translate('fleet.import.skipped', 'skipped')}
          </Badge>
        )}
        {status.failedServers > 0 && (
          <Badge variant="destructive">
            {status.failedServers} {translate('fleet.import.failed', 'failed')}
          </Badge>
        )}
      </div>

      {/* Error list */}
      {status.errors.length > 0 && (
        <ul className="max-h-24 overflow-y-auto rounded border border-destructive/20 bg-destructive/10 p-2">
          {status.errors.map((err, i) => (
            <li key={i} className="truncate text-xs text-destructive">
              {err}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
