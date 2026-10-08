/**
 * ReviewNodeNoteBadge.tsx — FE-CV-TASK-060-03
 *
 * Small count of unsent notes on a graph node. Hidden at zero.
 *
 * @module components/review-map/notes/ReviewNodeNoteBadge
 */

import { MessageSquare } from 'lucide-react'
import { tn } from './notes-i18n'

export function ReviewNodeNoteBadge({ count }: { count: number }): React.JSX.Element | null {
  if (count <= 0) {
    return null
  }
  return (
    <span
      className="inline-flex items-center gap-0.5 rounded-full bg-secondary px-1.5 text-[10px] text-secondary-foreground"
      aria-label={tn('ReviewNote.badge', '{{count}} unsent notes', { count })}
    >
      <MessageSquare className="size-2.5" aria-hidden />
      {count}
    </span>
  )
}
