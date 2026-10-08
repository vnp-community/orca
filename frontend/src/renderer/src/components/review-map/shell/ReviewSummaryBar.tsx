import { translate } from '@/i18n/i18n'
import { REVIEW_CHIP_IDS, reviewChipCount, type ReviewChipId } from '../review-chip-filter'
import type { ChangeOverlayView } from '../review-wire-types'

const CHIP_LABEL: Record<ReviewChipId, [string, string]> = {
  files: ['auto.components.reviewMap.shell.chip.files', '{{count}} files'],
  symbols: ['auto.components.reviewMap.shell.chip.symbols', '{{count}} symbols'],
  flows: ['auto.components.reviewMap.shell.chip.flows', '{{count}} flows'],
  tables: ['auto.components.reviewMap.shell.chip.tables', '{{count}} tables'],
  contracts: ['auto.components.reviewMap.shell.chip.contracts', '{{count}} contracts'],
  untested: ['auto.components.reviewMap.shell.chip.untested', '{{count}} untested'],
  violations: ['auto.components.reviewMap.shell.chip.violations', '{{count}} violations']
}

type Props = {
  overlay: ChangeOverlayView
  active: ReviewChipId | null
  onChipClick: (chip: ReviewChipId) => void
}

/** Seven chips always render (zero ones dimmed) so the bar does not reflow. */
export function ReviewSummaryBar({ overlay, active, onChipClick }: Props): React.JSX.Element {
  return (
    <div
      role="toolbar"
      aria-label={translate('auto.components.reviewMap.shell.chip.aria', 'Change summary')}
      className="flex flex-wrap gap-1.5"
    >
      {REVIEW_CHIP_IDS.map((chip) => {
        const count = reviewChipCount(overlay, chip)
        const [key, fallback] = CHIP_LABEL[chip]
        return (
          <button
            key={chip}
            type="button"
            data-chip={chip}
            aria-pressed={active === chip}
            disabled={count === 0}
            onClick={() => onChipClick(chip)}
            className={`h-6 rounded-full border px-2 text-xs ${
              active === chip ? 'bg-accent text-accent-foreground' : 'text-muted-foreground'
            } ${count === 0 ? 'opacity-50' : 'hover:bg-accent/50'}`}
          >
            {translate(key, fallback, { count })}
          </button>
        )
      })}
    </div>
  )
}
