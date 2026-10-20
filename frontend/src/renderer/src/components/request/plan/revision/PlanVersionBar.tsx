/**
 * PlanVersionBar — CR-REQ-045
 *
 * Everything about which version of the plan is on screen: the list of versions, the banner of an older one with the way back,
 * the notice that a newer one arrived, and what was asked for the version on screen. The tree and the approval above it always
 * belong to the current version; only the documents and the proposal follow the reader's choice.
 *
 * @module components/request/plan/revision/PlanVersionBar
 */

import React from 'react'
import { ChevronDown, Sparkles } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { OlderVersionBanner } from '../../solution/OlderVersionBanner'
import type { PlanDisplayView } from '../plan-view-state'
import { PlanVersionSwitcher } from './PlanVersionSwitcher'
import type { PlanVersionsApi } from './usePlanVersions'

type Props = {
  versions: PlanVersionsApi
  /** What the tab shows below this bar. */
  view: PlanDisplayView
  /** Choosing a version also brings the documents or the proposal on screen, the only places an older version can be read. */
  onSelect: (revision: number) => void
}

function Disclosure({
  label,
  testId,
  children
}: {
  label: string
  testId: string
  children: React.ReactNode
}): React.JSX.Element {
  return (
    <Collapsible data-testid={testId}>
      <CollapsibleTrigger className="group flex items-center gap-1 text-xs font-medium outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
        <ChevronDown
          className="size-3.5 transition-transform group-data-[state=closed]:-rotate-90"
          aria-hidden
        />
        {label}
      </CollapsibleTrigger>
      <CollapsibleContent className="pt-1">{children}</CollapsibleContent>
    </Collapsible>
  )
}

export function PlanVersionBar({ versions, view, onSelect }: Props): React.JSX.Element | null {
  const { list, shown, viewingOlder, newerAvailable, backToCurrent } = versions
  if (!list.supported || list.revisions.length < 2) {
    return null
  }
  const onScreen = shown ?? list.current
  const asked = onScreen?.revisionFeedback ?? ''
  return (
    <div
      className="flex flex-col gap-2 border-b border-border px-4 py-3"
      data-testid="plan-version-bar"
    >
      {viewingOlder && (
        <OlderVersionBanner
          onBack={backToCurrent}
          testId="plan-older-version-banner"
          backTestId="plan-back-to-current"
        />
      )}
      {viewingOlder && view === 'documents' && (
        <p className="text-xs text-muted-foreground" data-testid="plan-tree-current-note">
          {translate(
            'auto.components.request.plan.revision.treeIsCurrent',
            'The tree shows the current version.'
          )}
        </p>
      )}
      {newerAvailable && (
        <div
          role="status"
          data-testid="plan-newer-version-notice"
          className="flex flex-wrap items-center gap-2 rounded-md border border-border px-3 py-2 text-xs text-muted-foreground"
        >
          <Sparkles className="size-3.5 shrink-0" aria-hidden />
          <span>
            {translate(
              'auto.components.request.plan.revision.newerAvailable',
              'A newer version is available.'
            )}
          </span>
          <Button size="xs" variant="outline" onClick={backToCurrent} data-testid="plan-show-newer">
            {translate('auto.components.request.plan.revision.showNewer', 'Show it')}
          </Button>
        </div>
      )}
      <Disclosure
        testId="plan-versions-disclosure"
        label={translate(
          'auto.components.request.plan.revision.versionsCount',
          'Versions ({{count}})',
          {
            count: list.revisions.length
          }
        )}
      >
        <PlanVersionSwitcher
          revisions={list.revisions}
          shownRevision={onScreen?.revision ?? 0}
          hasMore={list.hasMore}
          loadingMore={list.isLoadingMore}
          onLoadMore={list.loadMore}
          onSelect={onSelect}
        />
      </Disclosure>
      {asked !== '' && (
        <Disclosure
          testId="plan-version-asked"
          label={translate(
            'auto.components.request.plan.revision.askedFor',
            'What was asked for this version'
          )}
        >
          <p className="rounded-md border border-border bg-muted/40 p-2 text-xs whitespace-pre-wrap break-words">
            {asked}
          </p>
        </Disclosure>
      )}
    </div>
  )
}
