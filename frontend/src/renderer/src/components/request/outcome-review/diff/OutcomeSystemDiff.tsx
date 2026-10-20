/**
 * OutcomeSystemDiff — FE-OR-T201, T202, T213, T214
 *
 * The sections of the review as tabs: Architecture and Flow first, then Behavior, Data, Contracts, Requirements and Config. A
 * drawn section shows its drawing or drawings for the chosen mode, and under them always the same facts as rows, so a person who
 * cannot see the drawing loses nothing. The drawings are read only for the section that is open.
 *
 * @module components/request/outcome-review/diff/OutcomeSystemDiff
 */

import React, { useEffect, useMemo, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { sectionText } from '../outcome-review-copy'
import { phaseOf } from '../outcome-review-view-model'
import { ReviewArtifactFrame, type RegeneratePreset } from '../artifact/ReviewArtifactFrame'
import {
  diagramLabelText,
  emptyText,
  laterText,
  modeReasonText,
  showUnchangedText,
  sideText,
  tabBadgeText
} from './outcome-diff-copy'
import { OutcomeContainerDrilldown } from './OutcomeContainerDrilldown'
import { OutcomeDiffRow } from './OutcomeDiffRow'
import { OutcomeModeSwitch } from './OutcomeModeSwitch'
import { OutcomeRequirementsTable } from './OutcomeRequirementsTable'
import { useSeenRows } from './outcome-review-seen'
import {
  containersOf,
  diagramsOf,
  isDrawnSection,
  modeAvailability,
  rowsOf,
  sectionItems,
  sectionOrDefault,
  subTabsOf,
  type DiffMode
} from './outcome-diff-view-model'
import type {
  DiagramRef,
  OutcomeReview,
  ReviewItem,
  ReviewSection
} from '../../../../../../shared/outcome-review-types'
import type { RequestStatus } from '../../../../../../shared/request-types'

type Props = {
  requestId: string
  review: OutcomeReview
  requestStatus: RequestStatus
  section: ReviewSection
  onSectionChange: (section: ReviewSection) => void
  /** The viewer may start a regeneration (the server decides again). */
  canRegenerate: boolean
  onRegenerate?: (preset: RegeneratePreset) => void
  /** A drawing changed or is gone: read the review again. */
  onChanged?: () => void
  onOpenFile?: (path: string) => void
  /** The way into the code review beside a row and beside a container (Phase 3). */
  renderRowAction?: (item: ReviewItem) => React.ReactNode
  renderContainerAction?: (containerId: string) => React.ReactNode
}

function DiagramColumn({
  title,
  diagrams,
  props
}: {
  title?: string
  diagrams: DiagramRef[]
  props: Omit<React.ComponentProps<typeof ReviewArtifactFrame>, 'diagram' | 'label'> & {
    section: ReviewSection
  }
}): React.JSX.Element {
  const { section, ...frame } = props
  return (
    <div className="flex min-w-0 flex-col gap-2" data-testid="outcome-diagram-column">
      {title && <h5 className="text-xs font-medium text-muted-foreground">{title}</h5>}
      {diagrams.length === 0 ? (
        <p
          className="rounded-md border border-dashed border-border p-3 text-sm text-muted-foreground"
          data-testid="outcome-no-diagram"
        >
          {emptyText()}
        </p>
      ) : (
        diagrams.map((d) => (
          <ReviewArtifactFrame
            key={`${d.variant}:${d.name ?? ''}:${d.inputDigest}`}
            {...frame}
            diagram={d}
            label={diagramLabelText(sectionText(section), d.variant, d.name)}
          />
        ))
      )}
    </div>
  )
}

export function OutcomeSystemDiff(props: Props): React.JSX.Element {
  const {
    requestId,
    review,
    requestStatus,
    section,
    onSectionChange,
    canRegenerate,
    onRegenerate,
    onChanged,
    onOpenFile,
    renderRowAction,
    renderContainerAction
  } = props
  const availability = useMemo(
    () => modeAvailability(review, requestStatus),
    [review, requestStatus]
  )
  const [mode, setMode] = useState<DiffMode>(availability.initial)
  const [showUnchanged, setShowUnchanged] = useState(false)
  const { seen, setSeen } = useSeenRows(review.reviewId)
  const phase = phaseOf(review)

  // Why: when the after-code review is made the screen moves to it, but only if the person had not chosen another mode.
  useEffect(() => {
    setMode((current) => (availability[current].enabled ? current : availability.initial))
  }, [availability])

  const tabs = subTabsOf(review, seen)
  const active = sectionOrDefault(section)
  const drawn = isDrawnSection(active)
  const rows = rowsOf(review, active, mode, showUnchanged)
  const frameProps = { requestId, canRegenerate, onRegenerate, onChanged, section: active }
  const designColumn = (
    <DiagramColumn
      title={sideText('design')}
      diagrams={diagramsOf(review, active, 'design')}
      props={frameProps}
    />
  )
  const deltaColumn = (
    <DiagramColumn
      title={sideText('after_code')}
      diagrams={diagramsOf(review, active, 'delta')}
      props={frameProps}
    />
  )
  const needsCode = (active === 'contracts' || active === 'config') && phase !== 'delivered'

  return (
    <section className="flex flex-col gap-2" data-testid="outcome-system-diff">
      <Tabs value={active} onValueChange={(v) => onSectionChange(sectionOrDefault(v))}>
        <TabsList className="flex-wrap">
          {tabs.map((t) => (
            <TabsTrigger
              key={t.section}
              value={t.section}
              data-testid={`outcome-subtab-${t.section}`}
            >
              {sectionText(t.section)}
              {t.unplanned !== null && (t.section === 'architecture' || t.section === 'flow') ? (
                <Badge
                  variant="secondary"
                  className="ml-1.5"
                  data-testid={`outcome-subtab-badge-${t.section}`}
                >
                  {tabBadgeText('unplanned', t.unplanned)}
                </Badge>
              ) : (
                t.unseen > 0 && (
                  <Badge
                    variant="outline"
                    className="ml-1.5"
                    data-testid={`outcome-subtab-badge-${t.section}`}
                  >
                    {tabBadgeText('unseen', t.unseen)}
                  </Badge>
                )
              )}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value={active} className="mt-2 flex flex-col gap-3">
          {drawn && (
            <>
              <OutcomeModeSwitch mode={mode} availability={availability} onChange={setMode} />
              {mode === 'design' && !availability.design.enabled && availability.design.reason && (
                <p className="text-xs text-muted-foreground">
                  {modeReasonText(availability.design.reason)}
                </p>
              )}
              {mode === 'design' && designColumn}
              {mode === 'after_code' && deltaColumn}
              {mode === 'side_by_side' && (
                <div
                  className="grid grid-cols-1 gap-3 min-[900px]:grid-cols-2"
                  data-testid="outcome-side-by-side"
                >
                  {designColumn}
                  {deltaColumn}
                </div>
              )}
            </>
          )}
          {needsCode ? (
            <p className="text-sm text-muted-foreground" data-testid="outcome-appears-later">
              {laterText()}
            </p>
          ) : active === 'requirements' ? (
            <OutcomeRequirementsTable items={sectionItems(review, 'requirements')} />
          ) : (
            <>
              {rows.length === 0 ? (
                <p className="text-sm text-muted-foreground">{emptyText()}</p>
              ) : (
                <ul className="flex flex-col gap-2" data-testid="outcome-diff-rows">
                  {rows.map((row) => (
                    <OutcomeDiffRow
                      key={row.item.key}
                      row={row}
                      showDots={phase === 'delivered' && mode !== 'design'}
                      seen={seen.has(row.item.key)}
                      onSeenChange={(value) => setSeen(row.item.key, value)}
                      onOpenFile={onOpenFile}
                      actions={renderRowAction?.(row.item)}
                    />
                  ))}
                </ul>
              )}
              {mode !== 'design' &&
                sectionItems(review, active).some((i) => i.change === 'unchanged') && (
                  <Button
                    size="xs"
                    variant="ghost"
                    className="self-start"
                    onClick={() => setShowUnchanged((v) => !v)}
                    aria-pressed={showUnchanged}
                  >
                    {showUnchangedText()}
                  </Button>
                )}
              {active === 'architecture' && (
                <OutcomeContainerDrilldown
                  containers={containersOf(review)}
                  renderAction={renderContainerAction}
                />
              )}
            </>
          )}
        </TabsContent>
      </Tabs>
    </section>
  )
}
