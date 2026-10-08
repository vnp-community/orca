/**
 * C4InferredNotice.tsx — FE-CV-TASK-055-04
 *
 * The diagram is always heuristic; this notice and the per-element origin badge keep it from
 * ever reading as verified.
 */

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'

export type C4OriginValue = string

export function c4OriginLabel(origin: C4OriginValue): string {
  switch (origin) {
    case 'merged':
      return translate('auto.components.reviewMap.c4.origin.merged', 'Partly edited')
    case 'declared':
      return translate('auto.components.reviewMap.c4.origin.declared', 'Declared')
    default:
      // 'derived' and any future wire value read as inferred, never as verified.
      return translate('auto.components.reviewMap.c4.origin.derived', 'Inferred')
  }
}

export function C4OriginBadge({ origin }: { origin: C4OriginValue }): React.JSX.Element {
  return (
    <Badge variant={origin === 'declared' ? 'secondary' : 'outline'} data-origin={origin}>
      {c4OriginLabel(origin)}
    </Badge>
  )
}

export function C4InferredNotice({
  hasOverrides,
  updatedBy,
  updatedAt,
  onEdit
}: {
  hasOverrides: boolean
  updatedBy?: string | null
  updatedAt?: string | null
  onEdit?: () => void
}): React.JSX.Element {
  const text =
    hasOverrides && updatedBy
      ? translate(
          'auto.components.reviewMap.c4.notice.withOverrides',
          'Inferred, with overrides by {{by}}{{at}}',
          { by: updatedBy, at: updatedAt ? ` · ${updatedAt}` : '' }
        )
      : hasOverrides
        ? translate('auto.components.reviewMap.c4.notice.withOverridesAnon', 'Inferred, with manual overrides')
        : translate(
            'auto.components.reviewMap.c4.notice.inferred',
            'This diagram is inferred from the folder structure and hexagonal rules; it may not match the intended design.'
          )
  return (
    <div role="note" className="flex items-center gap-2 border-b bg-muted/40 px-3 py-1.5 text-xs text-muted-foreground">
      <span className="min-w-0 flex-1">{text}</span>
      {onEdit ? (
        <Button variant="ghost" size="xs" onClick={onEdit}>
          {translate('auto.components.reviewMap.c4.editYaml', 'Edit c4.yaml')}
        </Button>
      ) : null}
    </div>
  )
}
