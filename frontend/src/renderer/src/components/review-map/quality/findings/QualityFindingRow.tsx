/**
 * QualityFindingRow.tsx — FE-CV-TASK-087-12
 *
 * One check finding. All tool output (message, ruleId, fixHint) is rendered as plain text; an
 * unknown ruleId is shown verbatim. The message is clamped to two lines with the full text in a
 * tooltip.
 *
 * @module components/review-map/quality/findings/QualityFindingRow
 */

import React, { useState } from 'react'
import { cn } from '@/lib/utils'
import { Button } from '../../../ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '../../../ui/tooltip'
import { SeverityBadge } from '../../../quality-charts/SeverityBadge'
import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'
import { qf } from './quality-findings-copy'
import { formatQualityFindingLocation } from './quality-finding-location'

export type QualityFindingRowProps = {
  finding: QualityFinding
  selected: boolean
  /** Enter or the button: diff for in-scope findings, the file otherwise. */
  onOpen?: (finding: QualityFinding) => void
  /** Waive / revoke control supplied by the dock panel. */
  waiveAction?: React.ReactNode
}

function formatExpiry(iso: string): string {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? iso : date.toLocaleDateString()
}

export function QualityFindingRow({
  finding,
  selected,
  onOpen,
  waiveAction
}: QualityFindingRowProps): React.JSX.Element {
  const [hintOpen, setHintOpen] = useState(false)
  const location = formatQualityFindingLocation(finding)
  return (
    <div
      role="row"
      aria-selected={selected}
      tabIndex={0}
      onKeyDown={(event) => {
        if (event.key === 'Enter' && event.target === event.currentTarget && onOpen) {
          onOpen(finding)
        }
      }}
      data-fingerprint={finding.fingerprint}
      data-selected={selected ? 'true' : undefined}
      className={cn(
        'flex flex-col gap-1 border-b border-border/60 px-3 py-2 text-sm',
        selected && 'bg-accent',
        finding.waiver && 'opacity-70'
      )}
    >
      <div className="flex items-start gap-2">
        <SeverityBadge severity={finding.severity} withLabel={false} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2">
            <span className="font-mono text-xs text-foreground">{finding.ruleId}</span>
            <span className="text-xs text-muted-foreground">
              {qf('rowTool', { tool: finding.tool, version: finding.toolVersion })}
            </span>
          </div>
          <Tooltip>
            <TooltipTrigger asChild>
              <p className="line-clamp-2 break-words text-foreground">{finding.message}</p>
            </TooltipTrigger>
            <TooltipContent className="max-w-md whitespace-pre-wrap break-words">
              {finding.message}
            </TooltipContent>
          </Tooltip>
          <div className="truncate font-mono text-xs text-muted-foreground" title={location}>
            {location}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {onOpen ? (
            <Button type="button" variant="ghost" size="xs" onClick={() => onOpen(finding)}>
              {finding.inScope ? qf('rowViewDiff') : qf('rowOpenFile')}
            </Button>
          ) : null}
          {waiveAction}
        </div>
      </div>
      {finding.fixHint ? (
        <div className="pl-6">
          <button
            type="button"
            className="text-xs text-muted-foreground underline-offset-2 hover:underline"
            aria-expanded={hintOpen}
            onClick={() => setHintOpen((open) => !open)}
          >
            {qf('rowFixHint')}
          </button>
          {hintOpen ? (
            <p className="mt-0.5 whitespace-pre-wrap break-words text-xs text-muted-foreground">
              {finding.fixHint}
            </p>
          ) : null}
        </div>
      ) : null}
      {finding.waiver ? (
        <div className="pl-6 text-xs text-muted-foreground">
          {finding.waiver.by
            ? qf('rowWaived', {
                by: finding.waiver.by,
                date: formatExpiry(finding.waiver.expiresAt)
              })
            : qf('rowWaivedNoBy', { date: formatExpiry(finding.waiver.expiresAt) })}
          {finding.waiver.reason ? (
            <span className="break-words">{` · ${finding.waiver.reason}`}</span>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
