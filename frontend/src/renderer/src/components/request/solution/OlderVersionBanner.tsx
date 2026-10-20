/**
 * OlderVersionBanner — CR-REQ-041
 *
 * Shown while a version other than the current one is on screen: that version is read-only, and the way back is one click.
 *
 * @module components/request/solution/OlderVersionBanner
 */

import React from 'react'
import { History } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'

type Props = {
  onBack: () => void
  /** Test ids of the banner and of its button; the defaults are the analysis ones (the Plan tab passes its own). */
  testId?: string
  backTestId?: string
}

export function OlderVersionBanner({
  onBack,
  testId = 'solution-older-version-banner',
  backTestId = 'solution-back-to-current'
}: Props): React.JSX.Element {
  return (
    <div
      role="status"
      data-testid={testId}
      className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-muted/50 px-3 py-2 text-sm text-muted-foreground"
    >
      <History className="size-4 shrink-0" aria-hidden />
      <span className="font-medium">
        {translate(
          'auto.components.request.SolutionVersionSwitcher.viewingOlder',
          "You're viewing an older version."
        )}
      </span>
      <Button
        size="xs"
        variant="outline"
        className="ml-auto"
        onClick={onBack}
        data-testid={backTestId}
      >
        {translate(
          'auto.components.request.SolutionVersionSwitcher.backToCurrent',
          'Back to current'
        )}
      </Button>
    </div>
  )
}
