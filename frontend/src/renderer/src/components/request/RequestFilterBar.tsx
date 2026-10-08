/**
 * RequestFilterBar — CR-REQ-019-02
 *
 * Status / type / source filters plus two quick chips. Filters live in
 * `requestPage.listFilters` so they survive switching tabs.
 *
 * @module components/request/RequestFilterBar
 */

import React from 'react'
import { ChevronDown, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import {
  FILTERABLE_SOURCES,
  FILTERABLE_STATUSES,
  FILTERABLE_TYPES,
  applyQuickFilter,
  clearListFilters,
  hasActiveListFilters,
  isQuickFilterActive,
  toggleFilterValue,
  type QuickFilterId
} from './request-list-filters'
import type { RequestListFilters } from '../../store/slices/request'

const ANY_SOURCE = '__any__'
const T = 'auto.components.request.RequestFilterBar.'

type Props = {
  filters: RequestListFilters
  onChange: (next: RequestListFilters) => void
}

function MultiFilter<V extends string>({
  label,
  values,
  options,
  optionLabel,
  onToggle
}: {
  label: string
  values: V[] | undefined
  options: V[]
  optionLabel: (v: V) => string
  onToggle: (v: V) => void
}): React.JSX.Element {
  const count = values?.length ?? 0
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="xs" aria-label={label}>
          {label}
          {count > 0 && (
            <span className="rounded bg-primary/15 px-1 text-[10px] text-primary">{count}</span>
          )}
          <ChevronDown className="size-3" aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="max-h-72 overflow-y-auto">
        {options.map((opt) => (
          <DropdownMenuCheckboxItem
            key={opt}
            checked={values?.includes(opt) ?? false}
            onCheckedChange={() => onToggle(opt)}
            onSelect={(e) => e.preventDefault()}
          >
            {optionLabel(opt)}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function RequestFilterBar({ filters, onChange }: Props): React.JSX.Element {
  const chip = (id: QuickFilterId, label: string): React.JSX.Element => {
    const active = isQuickFilterActive(filters, id)
    return (
      <Button
        variant={active ? 'secondary' : 'outline'}
        size="xs"
        aria-pressed={active}
        onClick={() => onChange(applyQuickFilter(filters, id))}
        className={cn(active && 'border-primary/50 text-primary')}
      >
        {label}
      </Button>
    )
  }

  return (
    <div
      className="flex flex-wrap items-center gap-1.5 border-b border-border px-3 py-2"
      data-testid="request-filter-bar"
    >
      {chip(
        'needsMyConfirmation',
        translate(`${T}chip.needsMyConfirmation`, 'Needs my confirmation')
      )}
      {chip('running', translate(`${T}chip.running`, 'Running'))}
      {chip('awaitingInfo', translate(`${T}chip.awaitingInfo`, 'Awaiting information'))}
      <MultiFilter
        label={translate(`${T}status`, 'Status')}
        values={filters.status}
        options={FILTERABLE_STATUSES}
        optionLabel={(s) => translate(`auto.components.request.RequestStatus.${s}`, s)}
        onToggle={(s) => onChange({ ...filters, status: toggleFilterValue(filters.status, s) })}
      />
      <MultiFilter
        label={translate(`${T}type`, 'Type')}
        values={filters.type}
        options={FILTERABLE_TYPES}
        optionLabel={(t) => translate(`auto.components.request.RequestType.${t}.label`, t)}
        onToggle={(t) => onChange({ ...filters, type: toggleFilterValue(filters.type, t) })}
      />
      <Select
        value={filters.sourceProvider ?? ANY_SOURCE}
        onValueChange={(v) =>
          onChange({ ...filters, sourceProvider: v === ANY_SOURCE ? undefined : v })
        }
      >
        <SelectTrigger
          size="sm"
          className="h-6 w-auto gap-1 px-2 text-xs"
          aria-label={translate(`${T}source`, 'Source')}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ANY_SOURCE}>{translate(`${T}anySource`, 'Any source')}</SelectItem>
          {FILTERABLE_SOURCES.map((p) => (
            <SelectItem key={p} value={p}>
              {translate(`auto.components.request.RequestSource.${p}`, p)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {hasActiveListFilters(filters) && (
        <Button variant="ghost" size="xs" onClick={() => onChange(clearListFilters(filters))}>
          <X className="size-3" aria-hidden />
          {translate(`${T}clear`, 'Clear filters')}
        </Button>
      )}
    </div>
  )
}
