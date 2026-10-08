/**
 * RequirementEvidenceList.tsx — FE-CV-TASK-092-05
 *
 * Evidence for one requirement. Confirmed/derived evidence is a plain list;
 * inferred evidence sits under "Suggestions" with confirm / dismiss actions.
 * Labels come from task or code content, so they render as text only.
 *
 * @module components/review-map/requirements/RequirementEvidenceList
 */

import { Check, FileCode2, FlaskConical, Lightbulb, PlayCircle, UserCheck, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import type { EvidenceRowViewModel } from './requirement-trace-view-model'

const BASE = 'auto.components.reviewMap.requirements.trace'

export type RequirementEvidenceListProps = {
  evidence: EvidenceRowViewModel[]
  suggestions: EvidenceRowViewModel[]
  readOnly: boolean
  /** Locks the action buttons while a write is in flight. */
  busy: boolean
  onOpen: (evidence: EvidenceRowViewModel) => void
  onConfirm: (evidence: EvidenceRowViewModel) => void
  onReject: (evidence: EvidenceRowViewModel) => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}

function KindIcon({ kind }: { kind: EvidenceRowViewModel['kind'] }) {
  const cls = 'size-3.5 shrink-0 text-muted-foreground'
  if (kind === 'test') {return <FlaskConical className={cls} aria-hidden />}
  if (kind === 'check_run') {return <PlayCircle className={cls} aria-hidden />}
  if (kind === 'manual_confirmation') {return <UserCheck className={cls} aria-hidden />}
  return <FileCode2 className={cls} aria-hidden />
}

export function RequirementEvidenceList({
  evidence,
  suggestions,
  readOnly,
  busy,
  onOpen,
  onConfirm,
  onReject,
  translate = translateCatalogKey
}: RequirementEvidenceListProps): React.JSX.Element | null {
  if (evidence.length === 0 && suggestions.length === 0) {
    return null
  }
  return (
    <div className="mt-1 space-y-1 text-[11px]">
      {evidence.length > 0 ? (
        <ul className="space-y-0.5">
          {evidence.map((e) => (
            <li key={`${e.kind}:${e.ref}`} className="flex items-center gap-1.5">
              <KindIcon kind={e.kind} />
              <button
                type="button"
                onClick={() => onOpen(e)}
                className="min-w-0 truncate text-left text-foreground underline-offset-2 hover:underline"
                title={e.ref}
              >
                {e.label || e.ref}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      {suggestions.length > 0 ? (
        <div>
          <p className="flex items-center gap-1 text-muted-foreground">
            <Lightbulb className="size-3.5 shrink-0" aria-hidden />
            {translate(`${BASE}.suggestions`)}
          </p>
          <ul className="mt-0.5 space-y-0.5">
            {suggestions.map((e) => (
              <li key={`${e.kind}:${e.ref}`} className="flex items-center gap-1.5">
                <KindIcon kind={e.kind} />
                <span className="min-w-0 truncate text-muted-foreground" title={e.ref}>
                  {e.label || e.ref}
                </span>
                <span className="shrink-0 italic text-muted-foreground">{translate(`${BASE}.inferred`)}</span>
                {!readOnly ? (
                  <span className="ml-auto flex shrink-0 gap-1">
                    <Button type="button" variant="outline" size="xs" disabled={busy} onClick={() => onConfirm(e)}>
                      <Check aria-hidden />
                      {translate(`${BASE}.confirm`)}
                    </Button>
                    <Button type="button" variant="ghost" size="xs" disabled={busy} onClick={() => onReject(e)}>
                      <X aria-hidden />
                      {translate(`${BASE}.dismiss`)}
                    </Button>
                  </span>
                ) : null}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  )
}
