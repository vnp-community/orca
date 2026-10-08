/**
 * ContractMigrationGroup.tsx — FE-CV-TASK-059-04
 *
 * Migration block of one service: SQL statements, touched tables ("Open in ERD") and the
 * structural findings raised by the `sql.*` rules.
 *
 * @module components/review-map/contract/ContractMigrationGroup
 */

import type { ContractDiff } from '../../../../../shared/code-intel-types'
import { Button } from '@/components/ui/button'
import { normalizeCompatibility } from './contract-grouping'
import { tc } from './contract-i18n'
import { ContractCompatibilityBadge } from './ContractCompatibilityBadge'

type Migration = ContractDiff['migrations'][number]

export function ContractMigrationGroup({
  migration,
  onOpenErd,
  renderFinding
}: {
  migration: Migration
  onOpenErd?: (table: string, service: string) => void
  /** Reuses the Findings row so a migration finding looks like any other. */
  renderFinding?: (finding: Migration['findings'][number]) => React.ReactNode
}): React.JSX.Element {
  return (
    <section
      aria-label={tc('migration.label', 'Migration {{service}}', { service: migration.service })}
      className="space-y-2 p-2 text-xs"
    >
      <div className="text-[11px] text-muted-foreground">
        {migration.dialects.join(', ')} · {migration.files.length}{' '}
        {tc('migration.files', 'files')}
      </div>
      {migration.statements.length > 0 ? (
        <ul className="space-y-0.5">
          {migration.statements.map((s, i) => (
            <li key={`${s.table}-${s.op}-${s.column ?? ''}-${i}`} className="flex flex-wrap items-center gap-2">
              <code className="font-mono text-[11px]">
                {s.op} {s.table}
                {s.column ? `.${s.column}` : ''}
              </code>
              <ContractCompatibilityBadge compatibility={normalizeCompatibility(s.compatibility)} ruleId={s.ruleId} />
              {s.dialectOnly ? (
                <span className="text-muted-foreground">
                  {tc('migration.dialectOnly', 'only {{dialect}}', { dialect: s.dialectOnly })}
                </span>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {migration.tables.length > 0 ? (
        <ul className="space-y-0.5">
          {migration.tables.map((t) => (
            <li key={t.table} className="flex items-center gap-2">
              <span className="font-medium">{t.table}</span>
              <span className="text-muted-foreground">{t.change}</span>
              {onOpenErd ? (
                <Button type="button" variant="ghost" size="xs" onClick={() => onOpenErd(t.table, migration.service)}>
                  {tc('detail.openErd', 'Open in ERD')}
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {renderFinding && migration.findings.length > 0 ? (
        <ul className="space-y-1">{migration.findings.map((f) => <li key={f.findingKey}>{renderFinding(f)}</li>)}</ul>
      ) : null}
    </section>
  )
}
