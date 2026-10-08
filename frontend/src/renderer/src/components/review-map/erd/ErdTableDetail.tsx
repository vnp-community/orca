/**
 * ErdTableDetail.tsx — FE-CV-TASK-057-06
 *
 * Detail of one table: columns, indexes, checks, RLS, relations and the code that reads or
 * writes it. The accessor list comes from a table-name scan, so every label says "hint".
 * It never claims anything is safe, and "no accessors" is not "unused".
 */

import { useState } from 'react'
import { X } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { translate } from '@/i18n/i18n'
import type { ErdChange } from '../../../../../shared/code-intel-architecture-types'
import { ReviewNoteButton } from '../notes/ReviewNoteButton'
import { erdRelationSourceLabel } from './ErdRelationEdge'
import { groupAccessors, type ErdAccessor } from './erd-repository-links'
import type { ErdRelationView, ErdTableView } from './erd-view-model'

export type ErdDetailOverlay = {
  changedFiles: readonly { path: string }[]
  changedSymbols: readonly { symbol: { key: string } }[]
}

export type ErdTableDetailProps = {
  /** Enables the review note action; absent when the detail is shown outside a worktree. */
  worktreeId?: string
  table: ErdTableView
  relations: readonly ErdRelationView[]
  changes: readonly ErdChange[]
  service: string
  overlay: ErdDetailOverlay | null
  onOpenSymbol: (symbolKey: string) => void
  onOpenDiff: (path: string, line?: number) => void
  onOpenService: (service: string) => void
  onClose: () => void
}

type AccessFilter = 'all' | 'write' | 'read'
const t = translate

function endpointLabel(e: { service?: string; table: string; columns: string[] }): string {
  return `${e.service ? `${e.service}.` : ''}${e.table}(${e.columns.join(', ')})`
}

function AccessorRow({
  a,
  files,
  onOpenSymbol,
  onOpenDiff
}: {
  a: ErdAccessor
  files: ReadonlySet<string>
  onOpenSymbol: (key: string) => void
  onOpenDiff: (path: string, line?: number) => void
}): React.JSX.Element {
  const diffable = files.has(a.symbol.filePath)
  return (
    <li className="flex items-center gap-2 py-0.5">
      <span className="truncate font-medium">{a.symbol.name}</span>
      <Badge variant="outline" className="text-[10px]">
        {a.op}
      </Badge>
      {a.changed ? (
        <Badge variant="secondary" className="text-[10px]">
          {t('auto.components.reviewMap.ErdTableDetail.changedSymbol', 'changed')}
        </Badge>
      ) : null}
      <span className="ml-auto flex shrink-0 gap-1">
        <Button type="button" variant="ghost" size="xs" onClick={() => onOpenSymbol(a.symbol.key)}>
          {t('auto.components.reviewMap.ErdTableDetail.openSymbol', 'Open')}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="xs"
          disabled={!diffable}
          onClick={() => onOpenDiff(a.symbol.filePath, a.symbol.startLine)}
        >
          {t('auto.components.reviewMap.ErdTableDetail.viewDiff', 'View diff')}
        </Button>
      </span>
    </li>
  )
}

export function ErdTableDetail(p: ErdTableDetailProps): React.JSX.Element {
  const { table } = p
  const [filter, setFilter] = useState<AccessFilter>('all')
  const files = new Set((p.overlay?.changedFiles ?? []).map((f) => f.path))
  const groups = groupAccessors(table, p.overlay)
  const all = [...groups.write, ...groups.readwrite, ...groups.read, ...groups.other]
  const shown = all.filter(
    (a) =>
      filter === 'all' ||
      (filter === 'write'
        ? a.op === 'write' || a.op === 'readwrite'
        : a.op === 'read' || a.op === 'readwrite')
  )
  const rels = p.relations.filter((r) => r.from.table === table.name || r.to.table === table.name)
  const tableChanges = p.changes.filter((c) => c.table === table.name || c.table === table.key)
  const title = `${table.schema}.${table.name}`
  return (
    <aside
      className="flex min-h-0 w-96 shrink-0 flex-col gap-3 overflow-y-auto border-l p-3 text-xs"
      aria-label={t('auto.components.reviewMap.ErdTableDetail.label', 'Table detail: {{name}}', {
        name: title
      })}
    >
      <div className="flex items-start gap-2">
        <h3 className="truncate text-sm font-semibold">{title}</h3>
        {p.worktreeId ? (
          <ReviewNoteButton
            worktreeId={p.worktreeId}
            anchor={{
              kind: 'graph-node',
              lens: 'erd',
              nodeKey: table.key,
              // Why: a note must live in a file; the latest migration is the table's source.
              filePath: table.lastMigration || table.firstMigration || '',
              label: title
            }}
          />
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          className="ml-auto"
          aria-label={t('auto.components.reviewMap.ErdTableDetail.close', 'Close detail')}
          onClick={p.onClose}
        >
          <X className="size-3.5" aria-hidden="true" />
        </Button>
      </div>
      {table.degraded ? (
        <p className="text-muted-foreground">
          {t(
            'auto.components.reviewMap.ErdTableDetail.degraded',
            'The ERD of this table may be incomplete.'
          )}
        </p>
      ) : null}
      {table.comment ? <p className="text-muted-foreground">{table.comment}</p> : null}
      <p className="text-muted-foreground">
        {t('auto.components.reviewMap.ErdTableDetail.tenantScoped', 'Tenant-scoped: {{value}}', {
          value: table.tenantScoped
            ? t('auto.components.reviewMap.ErdTableDetail.yes', 'yes')
            : t('auto.components.reviewMap.ErdTableDetail.no', 'no')
        })}
        {' · '}
        {t('auto.components.reviewMap.ErdTableDetail.rlsState', 'Row-level security: {{state}}', {
          state: table.rlsState
        })}
      </p>

      <section>
        <h4 className="mb-1 font-medium">
          {t('auto.components.reviewMap.ErdTableDetail.columns', 'Columns')}
        </h4>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('auto.components.reviewMap.ErdTableDetail.colName', 'Name')}</TableHead>
              <TableHead>{t('auto.components.reviewMap.ErdTableDetail.colType', 'Type')}</TableHead>
              <TableHead>
                {t('auto.components.reviewMap.ErdTableDetail.colDefault', 'Default')}
              </TableHead>
              <TableHead>{t('auto.components.reviewMap.ErdTableDetail.colKey', 'Key')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {table.columns.map((c) => (
              <TableRow key={`${c.name}:${c.ghost ? 'g' : ''}`}>
                <TableCell>
                  <span className="font-mono" aria-hidden="true">
                    {c.changeSymbol ?? ''}
                  </span>{' '}
                  {c.name}
                  {c.nullable ? '?' : ''}
                </TableCell>
                <TableCell>
                  {c.change === 'modified' && c.before
                    ? `${c.before.type} → ${c.after?.type ?? c.type}`
                    : c.canonicalType || c.type}
                </TableCell>
                <TableCell>{c.defaultExpr ?? ''}</TableCell>
                <TableCell>
                  {[c.isPk ? 'PK' : '', c.isFk ? 'FK' : ''].filter(Boolean).join(' ')}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </section>

      {table.indexes.length > 0 ? (
        <section>
          <h4 className="mb-1 font-medium">
            {t('auto.components.reviewMap.ErdTableDetail.indexes', 'Indexes')}
          </h4>
          <ul>
            {table.indexes.map((i) => (
              <li key={i.name}>
                {i.name} ({i.columns.join(', ')})
                {i.unique
                  ? ` · ${t('auto.components.reviewMap.ErdTableDetail.unique', 'unique')}`
                  : ''}
                {i.emulatesPartialUnique
                  ? ` · ${t('auto.components.reviewMap.ErdTableDetail.partialUnique', 'emulates a partial unique index')}`
                  : ''}
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {table.checks.length > 0 || table.rls.length > 0 ? (
        <section>
          <h4 className="mb-1 font-medium">
            {t('auto.components.reviewMap.ErdTableDetail.rules', 'Checks and row-level security')}
          </h4>
          <ul>
            {table.checks.map((c) => (
              <li key={`c:${c.name}`}>
                {c.name}: <code>{c.expr}</code>
              </li>
            ))}
            {table.rls.map((r) => (
              <li key={`r:${r.name}`}>
                {r.name} ({r.command})
                {r.usingExpr ? (
                  <>
                    {' '}
                    · <code>{r.usingExpr}</code>
                  </>
                ) : null}
                {r.withCheckExpr ? (
                  <>
                    {' '}
                    · <code>{r.withCheckExpr}</code>
                  </>
                ) : null}
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {rels.length > 0 ? (
        <section>
          <h4 className="mb-1 font-medium">
            {t('auto.components.reviewMap.ErdTableDetail.relations', 'Relations')}
          </h4>
          <ul className="space-y-0.5">
            {rels.map((r) => {
              const other = r.from.table === table.name ? r.to : r.from
              return (
                <li key={r.id}>
                  {endpointLabel(r.from)} → {endpointLabel(r.to)} · {r.kind} ·{' '}
                  {erdRelationSourceLabel(r)}
                  {r.anomaly
                    ? ` · ${t('auto.components.reviewMap.ErdTableDetail.anomaly', 'unexpected data')}`
                    : ''}
                  {other.service && other.service !== p.service ? (
                    <Button
                      type="button"
                      variant="link"
                      size="xs"
                      onClick={() => p.onOpenService(other.service as string)}
                    >
                      {t(
                        'auto.components.reviewMap.ErdTableDetail.openService',
                        'Open {{service}}',
                        { service: other.service }
                      )}
                    </Button>
                  ) : null}
                </li>
              )
            })}
          </ul>
        </section>
      ) : null}

      <section>
        <div className="mb-1 flex items-center gap-2">
          <h4 className="font-medium">
            {t(
              'auto.components.reviewMap.ErdTableDetail.accessors',
              'Code that reads or writes this table ({{count}})',
              { count: all.length }
            )}
          </h4>
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            value={filter}
            aria-label={t('auto.components.reviewMap.ErdTableDetail.accessFilter', 'Access filter')}
            onValueChange={(v) => v && setFilter(v as AccessFilter)}
          >
            <ToggleGroupItem value="all">
              {t('auto.components.reviewMap.ErdTableDetail.filterAll', 'All')}
            </ToggleGroupItem>
            <ToggleGroupItem value="write">
              {t('auto.components.reviewMap.ErdTableDetail.filterWrite', 'Write')}
            </ToggleGroupItem>
            <ToggleGroupItem value="read">
              {t('auto.components.reviewMap.ErdTableDetail.filterRead', 'Read')}
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
        <p className="mb-1 text-muted-foreground">
          {t(
            'auto.components.reviewMap.ErdTableDetail.accessHint',
            'Hint from a table-name scan; dynamic queries can be missed.'
          )}
        </p>
        {groups.staleAccessWarning ? (
          <p className="mb-1 rounded border px-2 py-1" role="status">
            {t(
              'auto.components.reviewMap.ErdTableDetail.staleAccessWarning',
              'Hint, may be wrong: this change alters columns that code outside the change set still touches.'
            )}
          </p>
        ) : null}
        {shown.length === 0 ? (
          <p className="text-muted-foreground">
            {t(
              'auto.components.reviewMap.ErdTableDetail.noAccessors',
              'No links to code were found. The index may be missing or the queries are built dynamically.'
            )}
          </p>
        ) : (
          <ul>
            {shown.map((a) => (
              <AccessorRow
                key={`${a.symbol.key}:${a.op}`}
                a={a}
                files={files}
                onOpenSymbol={p.onOpenSymbol}
                onOpenDiff={p.onOpenDiff}
              />
            ))}
          </ul>
        )}
      </section>

      {tableChanges.length > 0 ? (
        <section>
          <h4 className="mb-1 font-medium">
            {t('auto.components.reviewMap.ErdTableDetail.migrations', 'Migrations in this change')}
          </h4>
          <ul>
            {tableChanges.map((c, i) => (
              <li
                key={`${c.migrationFile}:${c.column ?? ''}:${i}`}
                className="flex items-center gap-2"
              >
                <span className="truncate">
                  {c.column ? `${c.column} · ` : ''}
                  {c.kind} · {c.migrationFile}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  className="ml-auto"
                  onClick={() => p.onOpenDiff(c.migrationFile, c.line)}
                >
                  {t(
                    'auto.components.reviewMap.ErdTableDetail.viewMigration',
                    'View migration diff'
                  )}
                </Button>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </aside>
  )
}
