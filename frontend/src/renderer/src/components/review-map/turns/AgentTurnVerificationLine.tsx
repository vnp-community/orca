/**
 * AgentTurnVerificationLine.tsx — FE-CV-TASK-089-06
 *
 * "Agent ran: ..." line plus one row per re-run comparison. Icon and text carry the
 * state (never color alone); `unverified` and `unknown` never show a pass icon.
 *
 * @module components/review-map/turns/AgentTurnVerificationLine
 */

import { CircleCheck, CircleHelp, TriangleAlert, Terminal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import type {
  AgentTurnVerificationViewModel,
  VerificationCheckViewModel
} from './agent-turn-verification-view-model'

export type AgentTurnVerificationLineProps = {
  viewModel: AgentTurnVerificationViewModel
  /** Whether a quality re-run can be started from here. */
  canRun?: boolean
  onRunChecks?: () => void
  onViewRun?: (runId: string) => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}

const BASE = 'auto.components.reviewMap.turns.verification'

function CheckIcon({ agreement }: { agreement: VerificationCheckViewModel['agreement'] }) {
  const cls = 'size-3.5 shrink-0'
  if (agreement === 'contradicted') {
    return <TriangleAlert className={`${cls} text-destructive`} aria-hidden />
  }
  if (agreement === 'consistent') {
    return <CircleCheck className={`${cls} text-muted-foreground`} aria-hidden />
  }
  return <CircleHelp className={`${cls} text-muted-foreground`} aria-hidden />
}

export function AgentTurnVerificationLine({
  viewModel,
  canRun = false,
  onRunChecks,
  onViewRun,
  translate = translateCatalogKey
}: AgentTurnVerificationLineProps): React.JSX.Element | null {
  const { ranLine, checks } = viewModel
  if (!ranLine && checks.length === 0) {
    return null
  }

  const ranText = ranLine
    ? translate(`${BASE}.ran`, {
        commands:
          ranLine.parts
            .map((p) =>
              p.count > 1
                ? translate(`${BASE}.ranItemMany`, { name: translate(p.categoryKey), count: p.count })
                : translate(p.categoryKey)
            )
            .join(', ') + (ranLine.truncated ? ` ${translate(`${BASE}.ranMore`)}` : '')
      })
    : null

  return (
    <div className="space-y-1 text-[11px]">
      {ranText ? (
        <p
          className="flex items-center gap-1.5 text-muted-foreground"
          title={translate(`${BASE}.recordedNote`)}
        >
          <Terminal className="size-3.5 shrink-0" aria-hidden />
          <span className="break-words">{ranText}</span>
        </p>
      ) : null}
      {checks.length > 0 ? (
        <ul className="space-y-1">
          {checks.map((check) => (
            <li key={check.id} className="flex items-start gap-1.5">
              <CheckIcon agreement={check.agreement} />
              <div className="min-w-0 flex-1">
                <span className={check.agreement === 'contradicted' ? 'text-destructive' : 'text-foreground'}>
                  {translate(check.labelKey, { kind: translate(check.kindKey) })}
                </span>
                {check.basisKey ? (
                  <span className="ml-1 text-muted-foreground">{translate(check.basisKey)}</span>
                ) : null}
                {check.reasonKey ? (
                  <span className="ml-1 text-muted-foreground">{translate(check.reasonKey)}</span>
                ) : null}
                <div className="mt-0.5 flex flex-wrap gap-1">
                  {check.verifyingRunId && onViewRun ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="xs"
                      onClick={() => onViewRun(check.verifyingRunId as string)}
                    >
                      {translate(`${BASE}.viewRun`)}
                    </Button>
                  ) : null}
                  {check.agreement === 'unverified' && canRun && onRunChecks ? (
                    <Button type="button" variant="outline" size="xs" onClick={onRunChecks}>
                      {translate(`${BASE}.runChecks`)}
                    </Button>
                  ) : null}
                </div>
              </div>
            </li>
          ))}
        </ul>
      ) : null}
      <p className="text-muted-foreground">{translate(`${BASE}.recordedNote`)}</p>
    </div>
  )
}
