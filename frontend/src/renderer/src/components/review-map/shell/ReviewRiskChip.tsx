import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { AlertTriangle, CircleHelp, ShieldAlert, ShieldCheck, Shield } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import type { RiskAssessmentView, RiskReasonView } from '../review-wire-types'

const LEVEL_LABEL: Record<RiskAssessmentView['level'], [string, string]> = {
  LOW: ['auto.components.reviewMap.shell.risk.low', 'Low risk by index'],
  MEDIUM: ['auto.components.reviewMap.shell.risk.medium', 'Medium risk'],
  HIGH: ['auto.components.reviewMap.shell.risk.high', 'High risk'],
  CRITICAL: ['auto.components.reviewMap.shell.risk.critical', 'Critical risk']
}

const LEVEL_ICON = {
  LOW: ShieldCheck,
  MEDIUM: Shield,
  HIGH: ShieldAlert,
  CRITICAL: AlertTriangle
} as const

const LEVEL_TONE: Record<RiskAssessmentView['level'], string> = {
  LOW: 'text-foreground',
  MEDIUM: 'text-quality-warning',
  HIGH: 'text-destructive',
  CRITICAL: 'text-destructive'
}

/** Unknown reason keys render verbatim: we never invent an explanation. */
function reasonText(r: RiskReasonView): string {
  return translate(r.messageKey, r.messageKey, r.params)
}

export function ReviewRiskChip({
  risk
}: {
  risk: RiskAssessmentView | null
}): React.JSX.Element | null {
  if (!risk) {
    return null
  }
  const incomplete = risk.incomplete
  const [key, fallback] = LEVEL_LABEL[risk.level] ?? LEVEL_LABEL.MEDIUM
  const Icon = incomplete ? CircleHelp : (LEVEL_ICON[risk.level] ?? Shield)
  const label = incomplete
    ? translate('auto.components.reviewMap.shell.risk.incomplete', 'Not enough data')
    : translate(key, fallback)
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          data-risk={incomplete ? 'INCOMPLETE' : risk.level}
          className={`inline-flex h-7 items-center gap-1.5 rounded-md border px-2 text-xs ${
            incomplete ? 'text-muted-foreground' : LEVEL_TONE[risk.level]
          }`}
        >
          <Icon className="size-3.5" aria-hidden />
          {label}
        </button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 space-y-2 text-xs">
        {incomplete ? (
          <p className="text-muted-foreground">
            {translate(
              'auto.components.reviewMap.shell.risk.incompleteNote',
              'The index is missing data, so no conclusion is drawn.'
            )}
          </p>
        ) : null}
        {risk.reasons.length === 0 ? (
          <p className="text-muted-foreground">
            {translate('auto.components.reviewMap.shell.risk.noReasons', 'No reasons reported.')}
          </p>
        ) : (
          <ul className="list-disc space-y-1 pl-4">
            {risk.reasons.map((r, i) => (
              <li key={`${r.code}-${i}`}>{reasonText(r)}</li>
            ))}
          </ul>
        )}
      </PopoverContent>
    </Popover>
  )
}
