/**
 * RequirementRow.tsx — FE-CV-TASK-092-05
 *
 * One requirement with its state. The state icon differs by shape (not color alone) and
 * no wording claims a requirement is met: it only says what evidence was found.
 *
 * @module components/review-map/requirements/RequirementRow
 */

import { Circle, CircleDashed, CircleDot, CircleHelp, UserCheck } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import { RequirementEvidenceList } from './RequirementEvidenceList'
import type { EvidenceRowViewModel, RequirementRowViewModel } from './requirement-trace-view-model'

const BASE = 'auto.components.reviewMap.requirements.trace'

export type RequirementRowProps = {
  row: RequirementRowViewModel
  readOnly: boolean
  busy: boolean
  onOpenEvidence: (evidence: EvidenceRowViewModel) => void
  onConfirm: (requirementKey: string, evidence: { kind: string; ref: string }) => void
  onReject: (requirementKey: string, evidence: { kind: string; ref: string }) => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}

function StateIcon({ icon }: { icon: RequirementRowViewModel['stateIcon'] }) {
  const cls = 'mt-0.5 size-3.5 shrink-0 text-muted-foreground'
  if (icon === 'trace') {return <CircleDot className={cls} aria-hidden />}
  if (icon === 'partial') {return <CircleDashed className={cls} aria-hidden />}
  if (icon === 'manual') {return <UserCheck className={cls} aria-hidden />}
  if (icon === 'unknown') {return <CircleHelp className={cls} aria-hidden />}
  return <Circle className={cls} aria-hidden />
}

export function RequirementRow({
  row,
  readOnly,
  busy,
  onOpenEvidence,
  onConfirm,
  onReject,
  translate = translateCatalogKey
}: RequirementRowProps): React.JSX.Element {
  return (
    <li className="flex items-start gap-2 border-b border-border py-2 last:border-0">
      <StateIcon icon={row.stateIcon} />
      <div className="min-w-0 flex-1">
        {/* Requirement text is free text from a task: rendered as plain text only. */}
        <p className="break-words text-xs text-foreground">{row.text}</p>
        <p className="mt-0.5 text-[11px] text-muted-foreground">{translate(row.stateLabelKey)}</p>
        <RequirementEvidenceList
          evidence={row.evidence}
          suggestions={row.suggestions}
          readOnly={readOnly}
          busy={busy}
          onOpen={onOpenEvidence}
          onConfirm={(e) => onConfirm(row.key, { kind: e.kind, ref: e.ref })}
          onReject={(e) => onReject(row.key, { kind: e.kind, ref: e.ref })}
          translate={translate}
        />
        {row.state === 'manual_pending' && !readOnly ? (
          <Button
            type="button"
            variant="outline"
            size="xs"
            className="mt-1"
            disabled={busy}
            onClick={() => onConfirm(row.key, { kind: 'manual_confirmation', ref: row.key })}
          >
            <UserCheck aria-hidden />
            {translate(`${BASE}.manualConfirm`)}
          </Button>
        ) : null}
      </div>
    </li>
  )
}
