/**
 * StorageInferenceNotice.tsx — FE-CV-TASK-058-04
 *
 * Persistent inline notice: the map is partly inferred, plus how many values the backend
 * redacted. Backend warnings are free text and shown as plain text.
 */

import { Info } from 'lucide-react'
import { translate } from '@/i18n/i18n'

export function StorageInferenceNotice({
  inferredCount,
  redactedCount,
  warnings
}: {
  inferredCount: number
  redactedCount: number
  warnings: readonly { text: string }[]
}): React.JSX.Element | null {
  if (inferredCount === 0 && redactedCount === 0 && warnings.length === 0) {
    return null
  }
  return (
    <div
      className="space-y-0.5 rounded-md border bg-muted/30 px-3 py-1.5 text-xs text-muted-foreground"
      data-testid="storage-notice"
    >
      {inferredCount > 0 ? (
        <p className="flex items-center gap-1.5">
          <Info className="size-3.5 shrink-0" aria-hidden="true" />
          {translate(
            'auto.components.reviewMap.StorageInferenceNotice.inferred',
            '{{count}} items are inferred from configuration, not declared.',
            { count: inferredCount }
          )}
        </p>
      ) : null}
      {redactedCount > 0 ? (
        <p>
          {translate(
            'auto.components.reviewMap.StorageInferenceNotice.redacted',
            '{{count}} values were redacted by the backend.',
            { count: redactedCount }
          )}
        </p>
      ) : null}
      {warnings.map((w, i) => (
        <p key={i}>{w.text}</p>
      ))}
    </div>
  )
}
