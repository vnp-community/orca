/**
 * QualityFindingsToolbar.tsx — FE-CV-TASK-087-12
 *
 * Filters of the check-findings list. Controlled: the dock keeps the values in the quality ui
 * state (severity / category / onlyInScope / showWaived) and the search text locally.
 *
 * @module components/review-map/quality/findings/QualityFindingsToolbar
 */

import React from 'react'
import { Checkbox } from '../../../ui/checkbox'
import { Button } from '../../../ui/button'
import { Input } from '../../../ui/input'
import { ToggleGroup, ToggleGroupItem } from '../../../ui/toggle-group'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger
} from '../../../ui/dropdown-menu'
import { SeverityBadge } from '../../../quality-charts/SeverityBadge'
import type {
  QualityCategory,
  QualitySeverity
} from '../../../../../../shared/code-intel-quality-types'
import { qf } from './quality-findings-copy'
import type { QualityFindingsCopyKey } from './quality-findings-copy'

const SEVERITIES = ['error', 'warning', 'info'] as const
const CATEGORY_KEYS = {
  lint: 'categoryLint',
  typecheck: 'categoryTypecheck',
  test: 'categoryTest',
  coverage: 'categoryCoverage',
  complexity: 'categoryComplexity',
  security: 'categorySecurity',
  dependency: 'categoryDependency',
  convention: 'categoryConvention',
  architecture: 'categoryArchitecture',
  ai: 'categoryAi'
} as const satisfies Record<string, QualityFindingsCopyKey>

export type QualityFindingsToolbarProps = {
  severity: QualitySeverity[]
  category: QualityCategory[]
  onlyInScope: boolean
  showWaived: boolean
  waivedCount: number
  search: string
  onSeverityChange: (next: QualitySeverity[]) => void
  onCategoryChange: (next: QualityCategory[]) => void
  onOnlyInScopeChange: (next: boolean) => void
  onShowWaivedChange: (next: boolean) => void
  onSearchChange: (next: string) => void
}

export function QualityFindingsToolbar(props: QualityFindingsToolbarProps): React.JSX.Element {
  const { category } = props
  return (
    <div className="flex flex-wrap items-center gap-2 border-b border-border px-3 py-2">
      <ToggleGroup
        type="multiple"
        variant="outline"
        size="sm"
        aria-label={qf('toolbarSeverity')}
        value={props.severity}
        onValueChange={(next) => props.onSeverityChange(next as QualitySeverity[])}
      >
        {SEVERITIES.map((sev) => (
          <ToggleGroupItem key={sev} value={sev}>
            <SeverityBadge severity={sev} className="border-transparent bg-transparent" />
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="outline" size="xs">
            {category.length > 0
              ? qf('toolbarCategoryCount', { count: category.length })
              : qf('toolbarCategory')}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {(Object.keys(CATEGORY_KEYS) as (keyof typeof CATEGORY_KEYS)[]).map((cat) => (
            <DropdownMenuCheckboxItem
              key={cat}
              checked={category.includes(cat)}
              onSelect={(event) => event.preventDefault()}
              onCheckedChange={(checked) =>
                props.onCategoryChange(
                  checked ? [...category, cat] : category.filter((c) => c !== cat)
                )
              }
            >
              {qf(CATEGORY_KEYS[cat])}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <label className="flex items-center gap-1.5 text-xs">
        <Checkbox
          checked={props.onlyInScope}
          onCheckedChange={(v) => props.onOnlyInScopeChange(v === true)}
        />
        {qf('toolbarOnlyInScope')}
      </label>
      <label className="flex items-center gap-1.5 text-xs">
        <Checkbox
          checked={props.showWaived}
          onCheckedChange={(v) => props.onShowWaivedChange(v === true)}
        />
        {qf('toolbarShowWaived', { count: props.waivedCount })}
      </label>
      <Input
        value={props.search}
        onChange={(event) => props.onSearchChange(event.target.value)}
        placeholder={qf('toolbarSearchPlaceholder')}
        aria-label={qf('toolbarSearchPlaceholder')}
        className="h-7 min-w-40 flex-1 text-xs"
      />
    </div>
  )
}
