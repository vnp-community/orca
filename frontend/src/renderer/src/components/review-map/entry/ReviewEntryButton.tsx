import React from 'react'
import { ScanSearch } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { translate } from '@/i18n/i18n'
import { cn } from '@/lib/utils'

type Props = {
  onReview: () => void
  tooltip?: string
  className?: string
}

export function ReviewEntryButton({ onReview, tooltip, className }: Props): React.JSX.Element {
  const label = tooltip ?? translate('auto.components.reviewMap.EntryButton.label', 'Review changes')
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={label}
          className={cn(
            'inline-flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring',
            className
          )}
          onClick={(event) => {
            event.stopPropagation()
            onReview()
          }}
          onMouseDown={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <ScanSearch className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" sideOffset={4}>
        {label}
      </TooltipContent>
    </Tooltip>
  )
}
