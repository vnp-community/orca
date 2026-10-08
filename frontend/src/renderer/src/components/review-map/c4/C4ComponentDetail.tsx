/**
 * C4ComponentDetail.tsx — FE-CV-TASK-055-04
 *
 * Detail panel for a selected component, external or relation. Backend strings (names,
 * descriptions, labels) are rendered as plain text only.
 */

import { useState } from 'react'
import { Copy, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type {
  C4Component,
  C4External,
  C4Relation
} from '../../../../../shared/code-intel-architecture-types'
import { C4OriginBadge } from './C4InferredNotice'
import { c4RelationKindLabel } from './C4EdgeLegend'
import { relationKey } from './C4RelationsTable'
import type { C4OverlayFlags } from './c4-overlay-model'

export const C4_EVIDENCE_PAGE = 20

export type C4Selection =
  | { kind: 'component'; component: C4Component; flags: C4OverlayFlags }
  | { kind: 'external'; external: C4External }
  | { kind: 'relation'; relation: C4Relation }

function copyText(text: string): void {
  const api = (window as unknown as { api?: { ui?: { writeClipboardText?: (t: string) => unknown } } }).api
  if (api?.ui?.writeClipboardText) {
    void api.ui.writeClipboardText(text)
  } else {
    void navigator.clipboard?.writeText(text)
  }
}

function EvidenceList({
  evidence,
  onOpenDiff
}: {
  evidence: C4Relation['evidence']
  onOpenDiff: (path: string, line?: number) => void
}): React.JSX.Element | null {
  const [shown, setShown] = useState(C4_EVIDENCE_PAGE)
  if (evidence.length === 0) {
    return null
  }
  return (
    <section>
      <h4 className="text-xs font-medium">{translate('auto.components.reviewMap.c4.evidence', 'Evidence')}</h4>
      <ul className="mt-1 space-y-1 text-xs">
        {evidence.slice(0, shown).map((e) => (
          <li key={e.key} className="flex items-center gap-2">
            <span className="min-w-0 flex-1 truncate" title={e.filePath}>
              {e.name} <span className="text-muted-foreground">{e.filePath}</span>
            </span>
            <Button variant="ghost" size="xs" onClick={() => onOpenDiff(e.filePath, e.startLine)}>
              {translate('auto.components.reviewMap.c4.viewDiff', 'View diff')}
            </Button>
          </li>
        ))}
      </ul>
      {evidence.length > shown ? (
        <Button variant="link" size="xs" onClick={() => setShown((n) => n + C4_EVIDENCE_PAGE)}>
          {translate('auto.components.reviewMap.c4.showMore', 'Show more ({{count}})', { count: evidence.length - shown })}
        </Button>
      ) : null}
    </section>
  )
}

export function C4ComponentDetail({
  selection,
  names,
  relations,
  onSelectRelation,
  onOpenDiff,
  onAddDescription,
  onClose
}: {
  selection: C4Selection
  names: ReadonlyMap<string, string>
  /** All relations of the view, to list incoming/outgoing ones. */
  relations: readonly C4Relation[]
  onSelectRelation: (relation: C4Relation) => void
  onOpenDiff: (path: string, line?: number) => void
  onAddDescription?: () => void
  onClose: () => void
}): React.JSX.Element {
  const nameOf = (id: string): string => names.get(id) ?? id
  return (
    <aside
      aria-label={translate('auto.components.reviewMap.c4.detail', 'Details')}
      className="flex w-80 shrink-0 flex-col gap-3 overflow-y-auto border-l p-3 text-sm"
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="min-w-0 break-words font-medium">
          {selection.kind === 'component'
            ? selection.component.name
            : selection.kind === 'external'
              ? selection.external.name
              : `${nameOf(selection.relation.from)} → ${nameOf(selection.relation.to)}`}
        </h3>
        <Button variant="ghost" size="icon-xs" aria-label={translate('auto.components.reviewMap.c4.close', 'Close')} onClick={onClose}>
          <X className="size-3.5" />
        </Button>
      </div>

      {selection.kind === 'component' ? (
        <>
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <C4OriginBadge origin={selection.component.origin} />
            <span className="text-muted-foreground">{selection.component.kind}</span>
          </div>
          <div className="flex items-center gap-1 text-xs">
            <code className="min-w-0 flex-1 truncate">{selection.component.path}</code>
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={translate('auto.components.reviewMap.c4.copyPath', 'Copy path')}
              onClick={() => copyText(selection.component.path)}
            >
              <Copy className="size-3" />
            </Button>
          </div>
          {selection.component.description ? (
            <p className="text-xs">
              {selection.component.description}
              <span className="ml-1 text-muted-foreground">
                ({translate(`auto.components.reviewMap.c4.descSource.${selection.component.descriptionSource.replace(/\./g, "_")}`, selection.component.descriptionSource)})
              </span>
            </p>
          ) : (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              {translate('auto.components.reviewMap.c4.noDescription', 'No description yet')}
              {onAddDescription ? (
                <Button variant="link" size="xs" onClick={onAddDescription}>
                  {translate('auto.components.reviewMap.c4.addDescription', 'Add description')}
                </Button>
              ) : null}
            </div>
          )}
          <ul className="flex flex-wrap gap-2 text-[11px] text-muted-foreground">
            <li>{translate('auto.components.reviewMap.c4.symbols', '{{count}} symbols', { count: selection.component.symbolCount })}</li>
            {selection.component.techHint ? <li>{selection.component.techHint}</li> : null}
            {selection.flags.changed ? <li>{translate('auto.components.reviewMap.c4.flag.changed', 'changed')}</li> : null}
            {selection.flags.untested ? <li>{translate('auto.components.reviewMap.c4.flag.untested', 'untested')}</li> : null}
            {selection.flags.violation ? <li>{translate('auto.components.reviewMap.c4.flag.violation', 'layering')}</li> : null}
          </ul>
          {(['out', 'in'] as const).map((dir) => {
            const list = relations.filter((r) => (dir === 'out' ? r.from : r.to) === selection.component.id)
            if (list.length === 0) {
              return null
            }
            return (
              <section key={dir}>
                <h4 className="text-xs font-medium">
                  {dir === 'out'
                    ? translate('auto.components.reviewMap.c4.outgoing', 'Outgoing')
                    : translate('auto.components.reviewMap.c4.incoming', 'Incoming')}
                </h4>
                <ul className="mt-1 space-y-0.5 text-xs">
                  {list.map((r) => (
                    <li key={relationKey(r)}>
                      <button type="button" className="text-left underline-offset-2 hover:underline" onClick={() => onSelectRelation(r)}>
                        {c4RelationKindLabel(r.kind)} {nameOf(dir === 'out' ? r.to : r.from)}
                      </button>
                    </li>
                  ))}
                </ul>
              </section>
            )
          })}
        </>
      ) : null}

      {selection.kind === 'external' ? (
        <>
          <div className="flex items-center gap-2 text-xs">
            <C4OriginBadge origin={selection.external.origin} />
            <span className="text-muted-foreground">{selection.external.kind}</span>
          </div>
          {selection.external.description ? <p className="text-xs">{selection.external.description}</p> : null}
        </>
      ) : null}

      {selection.kind === 'relation' ? (
        <>
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <C4OriginBadge origin={selection.relation.origin} />
            <span>{c4RelationKindLabel(selection.relation.kind)}</span>
            <span className="text-muted-foreground">
              ×{selection.relation.count} · {Math.round(selection.relation.confidence * 100)}%
            </span>
          </div>
          {selection.relation.label ? <p className="text-xs">{selection.relation.label}</p> : null}
          {selection.relation.violatesLayering ? (
            <p role="status" className="text-xs text-destructive">
              {translate('auto.components.reviewMap.c4.violationNote', 'This dependency crosses a hexagonal layer boundary the wrong way.')}
            </p>
          ) : null}
          <EvidenceList evidence={selection.relation.evidence} onOpenDiff={onOpenDiff} />
        </>
      ) : null}
    </aside>
  )
}
