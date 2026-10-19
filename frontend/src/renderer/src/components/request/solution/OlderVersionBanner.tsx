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

export function OlderVersionBanner({ onBack }: { onBack: () => void }): React.JSX.Element {
  return (
    <div
      role="status"
      data-testid="solution-older-version-banner"
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
        data-testid="solution-back-to-current"
      >
        {translate(
          'auto.components.request.SolutionVersionSwitcher.backToCurrent',
          'Back to current'
        )}
      </Button>
    </div>
  )
}
