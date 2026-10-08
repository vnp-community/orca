/**
 * UnlinkedChangesList.tsx — FE-CV-TASK-092-05
 *
 * Changed files that no requirement points at. Informational; each file opens its diff.
 *
 * @module components/review-map/requirements/UnlinkedChangesList
 */

import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import type { RequirementTrace } from './requirement-trace-view-model'

const BASE = 'auto.components.reviewMap.requirements.trace'

export function UnlinkedChangesList({
  changes,
  onOpenFile,
  translate = translateCatalogKey
}: {
  changes: RequirementTrace['unlinkedChanges']
  onOpenFile: (file: string) => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}): React.JSX.Element | null {
  if (changes.length === 0) {
    return null
  }
  return (
    <section aria-label={translate(`${BASE}.group.unlinked`)} className="mt-3">
      <h3 className="px-1 pb-1 text-[11px] font-medium text-muted-foreground">
        {translate(`${BASE}.group.unlinked`)} ({changes.length})
      </h3>
      <ul className="space-y-0.5">
        {changes.map((change) => (
          <li key={change.file} className="text-[11px]">
            <button
              type="button"
              onClick={() => onOpenFile(change.file)}
              className="max-w-full truncate text-left text-foreground underline-offset-2 hover:underline"
              title={change.file}
            >
              {change.file}
            </button>
            {change.symbols.length > 0 ? (
              <span className="ml-1 text-muted-foreground">{change.symbols.slice(0, 3).join(', ')}</span>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  )
}
