/**
 * Maps node `status` strings to a label key + icon. Unknown statuses return
 * nulls so the UI renders the raw string (status is free-form per lens).
 *
 * @module components/graph/graph-status-presentation
 */

import {
  Ban, Circle, CircleCheck, CircleSlash, Eye, Loader, Minus, Pencil, Plus, TriangleAlert, Undo2
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type Entry = { labelKey: string; fallback: string; Icon: LucideIcon }
const P = 'auto.components.graph.Status.'

const KNOWN: Record<string, Entry> = {
  modified: { labelKey: `${P}modified`, fallback: 'Modified', Icon: Pencil },
  breaking: { labelKey: `${P}breaking`, fallback: 'Breaking change', Icon: TriangleAlert },
  irreversible: { labelKey: `${P}irreversible`, fallback: 'Irreversible', Icon: Undo2 },
  added: { labelKey: `${P}added`, fallback: 'Added', Icon: Plus },
  removed: { labelKey: `${P}removed`, fallback: 'Removed', Icon: Minus },
  open: { labelKey: `${P}open`, fallback: 'Open', Icon: Circle },
  todo: { labelKey: `${P}open`, fallback: 'Open', Icon: Circle },
  in_progress: { labelKey: `${P}in_progress`, fallback: 'In progress', Icon: Loader },
  review: { labelKey: `${P}review`, fallback: 'In review', Icon: Eye },
  done: { labelKey: `${P}done`, fallback: 'Done', Icon: CircleCheck },
  blocked: { labelKey: `${P}blocked`, fallback: 'Blocked', Icon: Ban },
  cancelled: { labelKey: `${P}cancelled`, fallback: 'Cancelled', Icon: CircleSlash }
}

export type GraphStatusPresentation =
  | { labelKey: string; fallback: string; Icon: LucideIcon }
  | { labelKey: null; fallback: null; Icon: null }

export function graphStatusPresentation(status: string): GraphStatusPresentation {
  if (Object.prototype.hasOwnProperty.call(KNOWN, status)) {return KNOWN[status]}
  return { labelKey: null, fallback: null, Icon: null }
}
