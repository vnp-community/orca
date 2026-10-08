/**
 * ErdToolbar.tsx — FE-CV-TASK-057-05
 *
 * Service/dialect/schema pickers, search (press "/"), focus-vs-all, graph-vs-list.
 * Controls are disabled while loading so a half-loaded view cannot be driven.
 */

import { forwardRef } from 'react'
import { Search } from 'lucide-react'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { translate } from '@/i18n/i18n'
import type { ErdServiceInfo } from '../../../../../shared/code-intel-architecture-types'
import type { ErdFilterMode } from './erd-table-filter'

export type ErdViewMode = 'graph' | 'list'

export type ErdToolbarProps = {
  services: readonly ErdServiceInfo[]
  service: string | null
  dialect: 'postgres' | 'mysql' | null
  dialects: readonly ('postgres' | 'mysql')[]
  schemas: readonly string[]
  selectedSchemas: ReadonlySet<string>
  query: string
  mode: ErdFilterMode
  view: ErdViewMode
  asOfMigration: string | null
  disabled: boolean
  onService: (service: string) => void
  onDialect: (dialect: 'postgres' | 'mysql') => void
  onSchemas: (schemas: string[]) => void
  onQuery: (query: string) => void
  onMode: (mode: ErdFilterMode) => void
  onView: (view: ErdViewMode) => void
}

export const ErdToolbar = forwardRef<HTMLInputElement, ErdToolbarProps>(
  function ErdToolbar(p, searchRef) {
    return (
      <div
        className="flex flex-wrap items-center gap-2 border-b px-3 py-2 text-xs"
        role="toolbar"
        aria-label={translate('auto.components.reviewMap.ErdToolbar.label', 'ERD controls')}
      >
        <Select value={p.service ?? undefined} onValueChange={p.onService} disabled={p.disabled}>
          <SelectTrigger
            size="sm"
            aria-label={translate('auto.components.reviewMap.ErdToolbar.service', 'Service')}
            className="w-44"
          >
            <SelectValue
              placeholder={translate('auto.components.reviewMap.ErdToolbar.service', 'Service')}
            />
          </SelectTrigger>
          <SelectContent>
            {p.services.map((s) => (
              <SelectItem key={s.name} value={s.name}>
                {s.name} ({s.tableCount})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {p.dialects.length > 1 ? (
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            value={p.dialect ?? ''}
            disabled={p.disabled}
            aria-label={translate('auto.components.reviewMap.ErdToolbar.dialect', 'SQL dialect')}
            onValueChange={(v) => v && p.onDialect(v as 'postgres' | 'mysql')}
          >
            {p.dialects.map((d) => (
              <ToggleGroupItem key={d} value={d}>
                {d}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        ) : null}
        {p.schemas.length > 1 ? (
          <ToggleGroup
            type="multiple"
            variant="outline"
            size="sm"
            value={[...p.selectedSchemas]}
            disabled={p.disabled}
            aria-label={translate('auto.components.reviewMap.ErdToolbar.schemas', 'Schemas')}
            onValueChange={p.onSchemas}
          >
            {p.schemas.map((s) => (
              <ToggleGroupItem key={s} value={s}>
                {s}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        ) : null}
        <div className="relative">
          <Search
            className="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            ref={searchRef}
            value={p.query}
            disabled={p.disabled}
            onChange={(e) => p.onQuery(e.target.value)}
            placeholder={translate(
              'auto.components.reviewMap.ErdToolbar.search',
              'Search tables, columns, code (/)'
            )}
            aria-label={translate(
              'auto.components.reviewMap.ErdToolbar.searchLabel',
              'Search the ERD'
            )}
            className="h-8 w-56 pl-7 text-xs"
          />
        </div>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={p.mode}
          disabled={p.disabled}
          aria-label={translate('auto.components.reviewMap.ErdToolbar.scope', 'Tables shown')}
          onValueChange={(v) => v && p.onMode(v as ErdFilterMode)}
        >
          <ToggleGroupItem value="focus">
            {translate('auto.components.reviewMap.ErdToolbar.focus', 'Changed + related')}
          </ToggleGroupItem>
          <ToggleGroupItem value="all">
            {translate('auto.components.reviewMap.ErdToolbar.all', 'All')}
          </ToggleGroupItem>
        </ToggleGroup>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={p.view}
          disabled={p.disabled}
          aria-label={translate('auto.components.reviewMap.ErdToolbar.view', 'View')}
          onValueChange={(v) => v && p.onView(v as ErdViewMode)}
        >
          <ToggleGroupItem value="graph">
            {translate('auto.components.reviewMap.ErdToolbar.graph', 'Graph')}
          </ToggleGroupItem>
          <ToggleGroupItem value="list">
            {translate('auto.components.reviewMap.ErdToolbar.list', 'List')}
          </ToggleGroupItem>
        </ToggleGroup>
        {p.asOfMigration ? (
          <span className="ml-auto text-muted-foreground">
            {translate(
              'auto.components.reviewMap.ErdToolbar.asOf',
              'As of migration {{migration}}',
              { migration: p.asOfMigration }
            )}
          </span>
        ) : null}
      </div>
    )
  }
)
