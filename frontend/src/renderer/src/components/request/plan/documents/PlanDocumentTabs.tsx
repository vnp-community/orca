/**
 * PlanDocumentTabs — CR-REQ-045
 *
 * The tab strip of an OpenSpec change: one tab per document, in reading order, each with what the version did to it and a
 * mark for a file too large to display. Must sit inside the `Tabs` root of PlanDocumentsView, whose arrow-key handling and
 * `aria-selected` it relies on. A change with many specs scrolls inside the strip, never the page.
 *
 * @module components/request/plan/documents/PlanDocumentTabs
 */

import React from 'react'
import { FileWarning } from 'lucide-react'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { Badge } from '@/components/ui/badge'
import { TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { PlanFileChange } from '../../../../../../shared/request-plan-revision'
import type { DocumentTab } from './plan-document-model'

const CHANGE_STYLE: Record<Exclude<PlanFileChange, 'unchanged'>, string> = {
  added: 'border-status-success-border bg-status-success-background text-status-success',
  changed: 'border-quality-warning-border bg-quality-warning-background text-quality-warning',
  removed: 'border-destructive/30 bg-destructive/5 text-destructive'
}

function changeText(change: Exclude<PlanFileChange, 'unchanged'>): string {
  switch (change) {
    case 'added':
      return translate('auto.components.request.plan.documents.change.added', 'New')
    case 'changed':
      return translate('auto.components.request.plan.documents.change.changed', 'Changed')
    case 'removed':
      return translate('auto.components.request.plan.documents.change.removed', 'Removed')
  }
}

type Props = {
  tabs: readonly DocumentTab[]
  /** False for the first version, where every file would read "New". */
  showChanges: boolean
}

export function PlanDocumentTabs({ tabs, showChanges }: Props): React.JSX.Element {
  return (
    <div className="max-w-full overflow-x-auto scrollbar-sleek" data-testid="plan-document-tabs">
      <TabsList
        variant="line"
        className="w-max"
        aria-label={translate('auto.components.request.plan.documents.tabsLabel', 'Documents')}
      >
        {tabs.map((tab) => (
          <TabsTrigger
            key={tab.path}
            value={tab.path}
            title={tab.path}
            data-testid={`plan-document-tab-${tab.path}`}
            className={cn('flex-none', tab.change === 'removed' && 'line-through')}
          >
            {tab.label}
            {showChanges && tab.change !== 'unchanged' && (
              <Badge
                variant="outline"
                data-change={tab.change}
                className={cn('font-normal', CHANGE_STYLE[tab.change])}
              >
                {changeText(tab.change)}
              </Badge>
            )}
            {tab.omitted && (
              <span
                title={translate(
                  'auto.components.request.plan.documents.tooLargeTitle',
                  'Too large to display'
                )}
              >
                <FileWarning className="size-3.5 text-muted-foreground" aria-hidden />
                <span className="sr-only">
                  {translate(
                    'auto.components.request.plan.documents.tooLargeTitle',
                    'Too large to display'
                  )}
                </span>
              </span>
            )}
          </TabsTrigger>
        ))}
      </TabsList>
    </div>
  )
}
