/**
 * QualityCoverageUncoveredList.tsx — FE-CV-TASK-087-16
 *
 * "Lines not covered": top files by uncovered lines; each range opens the diff at its first line.
 *
 * @module components/review-map/quality/QualityCoverageUncoveredList
 */

import type { CoverageFile } from '../../../../../shared/code-intel-quality-visualization-types'
import { listUncoveredFiles } from './quality-coverage-uncovered-files'
import { qv } from './quality-visualization-copy'

function rangeText(from: number, to: number): string {
  return from === to
    ? qv('coverage.uncoveredLine', { line: from })
    : qv('coverage.uncoveredRange', { from, to })
}

export function QualityCoverageUncoveredList({
  files,
  onOpenDiff
}: {
  files: readonly CoverageFile[]
  onOpenDiff: (path: string, line?: number) => void
}): React.JSX.Element {
  const { shown, total } = listUncoveredFiles(files)
  return (
    <section
      aria-label={qv('coverage.uncoveredTitle')}
      className="flex flex-col gap-2"
      data-uncovered-list
    >
      <h4 className="text-sm font-medium">{qv('coverage.uncoveredTitle')}</h4>
      {shown.length === 0 ? (
        <p className="text-xs text-muted-foreground">{qv('coverage.uncoveredNone')}</p>
      ) : (
        <ul className="flex flex-col gap-2 text-xs">
          {shown.map((file) => (
            <li key={file.path} className="flex flex-col gap-1">
              <span className="truncate font-mono text-foreground" title={file.path}>
                {qv('coverage.uncoveredFile', { path: file.path, count: file.lines })}
              </span>
              <span className="flex flex-wrap gap-1">
                {file.ranges.slice(0, 20).map(([from, to]) => (
                  <button
                    key={`${from}-${to}`}
                    type="button"
                    data-open-diff={`${file.path}:${from}`}
                    aria-label={qv('coverage.openDiff', { path: file.path, line: from })}
                    className="rounded border border-border px-1.5 py-0.5 text-foreground hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    onClick={() => onOpenDiff(file.path, from)}
                  >
                    {rangeText(from, to)}
                  </button>
                ))}
              </span>
            </li>
          ))}
        </ul>
      )}
      {total > shown.length ? (
        <p className="text-xs text-muted-foreground">
          {qv('coverage.uncoveredMore', { shown: shown.length, total })}
        </p>
      ) : null}
    </section>
  )
}
