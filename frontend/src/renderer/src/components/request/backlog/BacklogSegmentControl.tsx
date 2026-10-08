/**
 * BacklogSegmentControl — CR-REQ-023-04
 *
 * Request / Task / Execute switcher with loaded-count hints and 1/2/3 keys.
 * The keys use no modifier, so there is no Mac/Windows branch.
 *
 * @module components/request/backlog/BacklogSegmentControl
 */

import React from 'react'
import { ShortcutKeyCombo } from '@/components/ShortcutKeyCombo'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { translate } from '@/i18n/i18n'
import { isTypingTarget } from '@/hooks/useRowListKeyboardNavigation'
import type { BacklogView } from '../../../../../shared/request-types'

export const BACKLOG_VIEWS: BacklogView[] = ['requests', 'tasks', 'execute']
const VIEW_FOR_KEY: Record<string, BacklogView> = { '1': 'requests', '2': 'tasks', '3': 'execute' }
const LABELS: Record<BacklogView, string> = { requests: 'Requests', tasks: 'Tasks', execute: 'Execute' }
// Label keys follow the CR naming (request/task/execute).
const LABEL_KEY: Record<BacklogView, string> = { requests: 'request', tasks: 'task', execute: 'execute' }

export type BacklogCount = { count: number; plus: boolean } | null

/** Returns the view a key press selects, or null when the press must be ignored. */
export function backlogViewForKeyEvent(event: {
  key: string
  target: EventTarget | null
  ctrlKey?: boolean
  metaKey?: boolean
  altKey?: boolean
  shiftKey?: boolean
  isComposing?: boolean
  nativeEvent?: { isComposing?: boolean }
}): BacklogView | null {
  if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) {return null}
  if (event.isComposing || event.nativeEvent?.isComposing) {return null}
  if (isTypingTarget(event.target)) {return null}
  return VIEW_FOR_KEY[event.key] ?? null
}

export function BacklogSegmentControl({
  value,
  onChange,
  counts
}: {
  value: BacklogView
  onChange: (view: BacklogView) => void
  counts: Record<BacklogView, BacklogCount>
}): React.JSX.Element {
  return (
    <ToggleGroup
      type="single"
      variant="outline"
      size="sm"
      value={value}
      // Radix reports '' when the active item is clicked: keep the current view.
      onValueChange={(v) => { if (v) {onChange(v as BacklogView)} }}
      aria-label={translate('auto.components.request.RequestPage.tab.backlog', 'Backlog')}
    >
      {BACKLOG_VIEWS.map((view, index) => {
        const c = counts[view]
        return (
          <Tooltip key={view}>
            <TooltipTrigger asChild>
              <ToggleGroupItem value={view}>
                {translate(`auto.components.request.backlog.BacklogSegmentControl.${LABEL_KEY[view]}`, LABELS[view])}
                {c && (
                  <span className="ml-1.5 text-xs tabular-nums text-muted-foreground" data-testid={`backlog-count-${view}`}>
                    {c.count}{c.plus ? '+' : ''}
                  </span>
                )}
              </ToggleGroupItem>
            </TooltipTrigger>
            <TooltipContent>
              <ShortcutKeyCombo keys={[String(index + 1)]} />
            </TooltipContent>
          </Tooltip>
        )
      })}
    </ToggleGroup>
  )
}
