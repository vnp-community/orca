/**
 * GraphToolbar — FE-REQ-TASK-032-06
 *
 * Lens chips, graph/list toggle, before/after toggle, search button, legend.
 *
 * @module components/graph/GraphToolbar
 */

import React from 'react'
import { Search } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { ShortcutKeyCombo } from '@/components/ShortcutKeyCombo'
import { GRAPH_LENSES } from './graph-lens-registry'
import { riskPresentation } from './risk-presentation'
import type { GraphChangeView } from './graph-before-after'
import type { GraphPanelView, LensDisabledReason } from './graph-panel-state'
import type { GraphLens, GraphRisk } from '../../../../shared/graph-types'

const T = 'auto.components.graph.'
const REASON_TEXT: Record<LensDisabledReason, [string, string]> = {
  noAssessment: ['LensDisabled.noAssessment', 'No impact assessment yet'],
  noPlan: ['LensDisabled.noPlan', 'No plan yet'],
  notExecuting: ['LensDisabled.notExecuting', 'Not executing yet'],
  unsupported: ['LensDisabled.unsupported', 'Not supported by this runtime']
}

type Props = {
  lens: GraphLens
  view: GraphPanelView
  changeView: GraphChangeView
  showChangeToggle: boolean
  disabledReasons: Partial<Record<GraphLens, LensDisabledReason>>
  disabled?: boolean
  onLens: (lens: GraphLens) => void
  onView: (view: GraphPanelView) => void
  onChangeView: (view: GraphChangeView) => void
  onSearch: () => void
}

const LEGEND_RISKS: GraphRisk[] = ['low', 'medium', 'high', 'critical', 'unknown']

function Legend(): React.JSX.Element {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button size="xs" variant="ghost">{translate(`${T}Legend.title`, 'Legend')}</Button>
      </PopoverTrigger>
      <PopoverContent className="w-64 text-xs">
        <ul className="space-y-1">
          {LEGEND_RISKS.map((r) => {
            const p = riskPresentation(r)
            return (
              <li key={r} className="flex items-center gap-2">
                <p.Icon className={`size-3.5 ${p.textClass}`} aria-hidden />
                <span>{translate(p.labelKey, p.labelFallback)}</span>
              </li>
            )
          })}
          <li><span className="font-mono">+</span> {translate(`${T}Legend.added`, 'Added link (solid)')}</li>
          <li><span className="font-mono">−</span> {translate(`${T}Legend.removed`, 'Removed link (dashed)')}</li>
        </ul>
      </PopoverContent>
    </Popover>
  )
}

export function GraphToolbar(p: Props): React.JSX.Element {
  return (
    <TooltipProvider>
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-3 py-2">
        <ToggleGroup
          type="single"
          size="sm"
          variant="outline"
          value={p.lens}
          onValueChange={(v) => { if (v) {p.onLens(v as GraphLens)} }}
          aria-label={translate(`${T}Toolbar.lens`, 'Lens')}
        >
          {GRAPH_LENSES.map((l) => {
            const reason = p.disabledReasons[l.id]
            const item = (
              <ToggleGroupItem key={l.id} value={l.id} disabled={Boolean(reason) || p.disabled} aria-label={translate(l.labelKey, l.labelFallback)}>
                <l.Icon className="size-3.5" aria-hidden />
                <span className="text-xs">{translate(l.labelKey, l.labelFallback)}</span>
              </ToggleGroupItem>
            )
            if (!reason) {return item}
            const [key, fallback] = REASON_TEXT[reason]
            // Why: disabled buttons swallow pointer events, so the tooltip hangs off a wrapper.
            return (
              <Tooltip key={l.id}>
                <TooltipTrigger asChild><span tabIndex={0}>{item}</span></TooltipTrigger>
                <TooltipContent>{translate(`${T}${key}`, fallback)}</TooltipContent>
              </Tooltip>
            )
          })}
        </ToggleGroup>
        <ToggleGroup
          type="single"
          size="sm"
          variant="outline"
          value={p.view}
          onValueChange={(v) => { if (v) {p.onView(v as GraphPanelView)} }}
          aria-label={translate(`${T}Toolbar.view`, 'View')}
        >
          <ToggleGroupItem value="graph" disabled={p.disabled}>{translate(`${T}Toolbar.graph`, 'Graph')}</ToggleGroupItem>
          <ToggleGroupItem value="list" disabled={p.disabled}>{translate(`${T}Toolbar.list`, 'List')}</ToggleGroupItem>
        </ToggleGroup>
        {p.showChangeToggle ? (
          <ToggleGroup
            type="single"
            size="sm"
            variant="outline"
            value={p.changeView}
            onValueChange={(v) => { if (v) {p.onChangeView(v as GraphChangeView)} }}
            aria-label={translate(`${T}Toolbar.changeView`, 'Before or after')}
          >
            <ToggleGroupItem value="before">{translate(`${T}Toolbar.before`, 'Before')}</ToggleGroupItem>
            <ToggleGroupItem value="after">{translate(`${T}Toolbar.after`, 'After')}</ToggleGroupItem>
          </ToggleGroup>
        ) : null}
        <Button size="xs" variant="outline" onClick={p.onSearch} disabled={p.disabled}>
          <Search aria-hidden />
          {translate(`${T}Toolbar.search`, 'Search')}
          <ShortcutKeyCombo keys={['/']} />
        </Button>
        <Legend />
      </div>
    </TooltipProvider>
  )
}
