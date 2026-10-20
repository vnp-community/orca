/**
 * OutcomeCodeReviewButton — FE-OR-T322
 *
 * "Open in code review": opens the code review (v7) at the lens that goes with the row. When it cannot be opened it stays on the
 * screen, dimmed, and says why (the code review is off, the review has no worktree, this part has no view), never hidden and
 * never a button that does nothing. With more than one worktree the person chooses which.
 *
 * @module components/request/outcome-review/decision/OutcomeCodeReviewButton
 */

import React from 'react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  codeReviewAvailability,
  type CodeReviewLens,
  type Worktree
} from './outcome-code-review-link'
import { codeReviewText } from './outcome-decision-copy'

export type CodeReviewLink = {
  /** The code review is on for this runtime. */
  enabled: boolean
  worktrees: Worktree[]
  open: (worktreeId: string, lens: CodeReviewLens) => void
}

type Props = { lens: CodeReviewLens | null; link: CodeReviewLink }

export function OutcomeCodeReviewButton({ lens, link }: Props): React.JSX.Element {
  const availability = codeReviewAvailability({
    codeReviewEnabled: link.enabled,
    lens,
    worktrees: link.worktrees
  })
  if (availability.state !== 'ready' || lens === null) {
    const why = codeReviewText(availability.state === 'ready' ? 'no_lens' : availability.state)
    return (
      <Button
        size="xs"
        variant="ghost"
        aria-disabled="true"
        title={why}
        className="cursor-not-allowed opacity-60"
        data-testid="outcome-code-review"
        data-state={availability.state}
        onClick={(e) => e.preventDefault()}
      >
        {codeReviewText('open')}
        <span className="sr-only"> — {why}</span>
      </Button>
    )
  }
  const worktrees = availability.worktrees
  if (worktrees.length === 1) {
    return (
      <Button
        size="xs"
        variant="ghost"
        data-testid="outcome-code-review"
        data-state="ready"
        onClick={() => link.open(worktrees[0].worktreeId, lens)}
      >
        {codeReviewText('open')}
      </Button>
    )
  }
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button size="xs" variant="ghost" data-testid="outcome-code-review" data-state="ready">
          {codeReviewText('open')}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-64" align="end">
        <p className="mb-2 text-xs text-muted-foreground">{codeReviewText('choose')}</p>
        <ul className="flex flex-col gap-1">
          {worktrees.map((w) => (
            <li key={w.worktreeId}>
              <Button
                size="xs"
                variant="outline"
                className="w-full justify-start"
                onClick={() => link.open(w.worktreeId, lens)}
                data-testid="outcome-code-review-worktree"
              >
                {w.branch || w.worktreeId}
              </Button>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  )
}
