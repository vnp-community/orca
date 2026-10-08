/**
 * ContractChangeDetail.tsx — FE-CV-TASK-059-04
 *
 * Detail of one change: rule, details, files, evidence, consumers. A breaking change with no
 * consumers says none were *found* (index coverage is limited to this repo), never "unused".
 *
 * @module components/review-map/contract/ContractChangeDetail
 */

import type { ContractChange } from '../../../../../shared/code-intel-types'
import { Button } from '@/components/ui/button'
import { maskSensitiveText } from '../sensitive-text-masking'
import { ReviewNoteButton } from '../notes/ReviewNoteButton'
import { contractDetailRows } from './contract-detail-rows'
import { contractKindGroup, normalizeCompatibility } from './contract-grouping'
import { tc } from './contract-i18n'
import { ContractCompatibilityBadge } from './ContractCompatibilityBadge'

export function ContractChangeDetail({
  change,
  worktreeId,
  changedFiles,
  onOpenDiff,
  onSelectSymbol,
  onOpenErd
}: {
  change: ContractChange
  /** Enables the Note action; omitted in read-only contexts. */
  worktreeId?: string
  changedFiles: ReadonlySet<string>
  onOpenDiff: (path: string, line?: number) => void
  onSelectSymbol: (symbolKey: string) => void
  /** Present only when an ERD lens can take the table. */
  onOpenErd?: (table: string, service: string | undefined) => void
}): React.JSX.Element {
  const { rows } = contractDetailRows(change)
  const compat = normalizeCompatibility(change.compatibility)
  const isSql = contractKindGroup(change.kind) === 'migration'
  const erdTable = change.kind === 'sql-table' ? change.name : change.name.split('.')[0]
  return (
    <section aria-label={tc('detail.label', 'Contract change detail')} className="space-y-3 p-3 text-xs">
      <header className="space-y-1">
        <div className="break-all font-medium">{change.name}</div>
        <div className="flex flex-wrap items-center gap-2">
          <ContractCompatibilityBadge compatibility={change.compatibility} ruleId={change.ruleId} />
          <span className="text-muted-foreground">{change.ruleId}</span>
        </div>
      </header>

      {rows.length > 0 ? (
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-2 gap-y-0.5">
          {rows.map((r) => (
            <div key={r.key} className="contents">
              <dt className="text-muted-foreground">{r.key}</dt>
              <dd className="break-all">{r.value}</dd>
            </div>
          ))}
        </dl>
      ) : null}

      <div>
        <h4 className="mb-1 font-medium">{tc('detail.files', 'Files')}</h4>
        {change.files.length === 0 ? (
          <p className="text-muted-foreground">{tc('detail.noFiles', 'No files reported')}</p>
        ) : (
          <ul className="space-y-0.5">
            {change.files.map((file) => {
              const line = change.evidence.find((e) => e.path === file)?.line
              return (
                <li key={file} className="flex items-center gap-2">
                  <span className="min-w-0 truncate" title={file}>
                    {file}
                    {line ? `:${line}` : ''}
                  </span>
                  {changedFiles.has(file) ? (
                    <Button type="button" variant="ghost" size="xs" onClick={() => onOpenDiff(file, line)}>
                      {tc('detail.viewDiff', 'View diff')}
                    </Button>
                  ) : (
                    <span className="text-muted-foreground">{tc('detail.notInChange', 'not in this change')}</span>
                  )}
                </li>
              )
            })}
          </ul>
        )}
      </div>

      <div>
        <h4 className="mb-1 font-medium">{tc('detail.consumers', 'Consumers')}</h4>
        {change.consumers.length === 0 ? (
          <p className="text-muted-foreground">
            {compat === 'breaking'
              ? tc(
                  'detail.noConsumersBreaking',
                  'No consumers found (they may be outside this repo or not indexed)'
                )
              : tc('detail.noConsumers', 'No consumers found')}
          </p>
        ) : (
          <ul className="space-y-0.5">
            {change.consumers.map((c, i) => (
              <li key={`${c.kind}-${c.symbol?.key ?? c.path ?? i}`} className="flex items-center gap-2">
                <span className="text-muted-foreground">{c.kind}</span>
                {c.symbol ? (
                  <button
                    type="button"
                    className="truncate underline-offset-2 hover:underline"
                    onClick={() => onSelectSymbol(c.symbol!.key)}
                  >
                    {c.symbol.name}
                  </button>
                ) : (
                  <span className="truncate">{maskSensitiveText(c.path ?? c.service ?? '').text}</span>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      {worktreeId ? (
        <ReviewNoteButton
          worktreeId={worktreeId}
          anchor={{
            kind: 'graph-node',
            lens: 'contract',
            nodeKey: change.id,
            filePath: change.files[0] ?? '',
            ...(change.evidence[0]?.line ? { startLine: change.evidence[0].line } : {}),
            label: change.name
          }}
        />
      ) : null}

      {isSql && onOpenErd ? (
        <Button type="button" variant="outline" size="xs" onClick={() => onOpenErd(erdTable, change.service)}>
          {tc('detail.openErd', 'Open in ERD')}
        </Button>
      ) : null}
    </section>
  )
}
