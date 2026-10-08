/**
 * ErdWarningsStrip.tsx — FE-CV-TASK-057-05
 *
 * Persistent inline notice for parser warnings and degraded tables. Messages are free text
 * from the backend: rendered as plain text (never HTML) after masking.
 */

import { TriangleAlert } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { maskSensitiveText } from '../sensitive-text-masking'
import type { ErdModel } from '../../../../../shared/code-intel-architecture-types'

export function ErdWarningsStrip({
  warnings,
  degradedTables
}: {
  warnings: ErdModel['warnings']
  degradedTables: string[]
}): React.JSX.Element | null {
  if (warnings.length === 0 && degradedTables.length === 0) {
    return null
  }
  return (
    <details
      className="rounded-md border bg-muted/30 px-3 py-1.5 text-xs"
      data-testid="erd-warnings"
    >
      <summary className="flex cursor-pointer items-center gap-1.5">
        <TriangleAlert className="size-3.5 shrink-0" aria-hidden="true" />
        {translate(
          'auto.components.reviewMap.ErdWarningsStrip.summary',
          '{{count}} parse warnings, {{tables}} tables may be incomplete',
          {
            count: warnings.length,
            tables: degradedTables.length
          }
        )}
      </summary>
      <ul className="mt-1 space-y-0.5 text-muted-foreground">
        {degradedTables.map((t) => (
          <li key={`d:${t}`}>
            {translate(
              'auto.components.reviewMap.ErdWarningsStrip.degraded',
              'ERD of {{table}} may be incomplete',
              { table: t }
            )}
          </li>
        ))}
        {warnings.map((w, i) => (
          <li key={`${w.file}:${w.line ?? 0}:${i}`}>
            <code>
              {w.file}
              {w.line ? `:${w.line}` : ''}
            </code>{' '}
            {w.code} {maskSensitiveText(w.message).text}
          </li>
        ))}
      </ul>
    </details>
  )
}
