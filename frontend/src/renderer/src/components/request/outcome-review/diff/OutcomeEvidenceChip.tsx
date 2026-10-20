/**
 * OutcomeEvidenceChip — FE-OR-T203
 *
 * One piece of evidence behind a row. A knowledge-graph id is shown and can be copied but is not a link: Orca does not read the
 * graph, so there is nowhere to go. A SAD reference shows the document and section. A file opens the diff of that file when the
 * screen can; the others are names.
 *
 * @module components/request/outcome-review/diff/OutcomeEvidenceChip
 */

import React from 'react'
import { Copy } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { copiedText, copyText, evidenceText, openFileText } from './outcome-diff-copy'
import type { ReviewEvidence } from '../../../../../../shared/outcome-review-types'

type Props = {
  evidence: ReviewEvidence
  /** Opens the diff of a changed file; absent leaves the path as text. */
  onOpenFile?: (path: string) => void
}

async function copy(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    toast.success(copiedText())
  } catch {
    // Clipboard access can be refused; the id stays on screen to be selected by hand.
  }
}

export function OutcomeEvidenceChip({ evidence, onOpenFile }: Props): React.JSX.Element {
  const label = evidenceText(evidence)
  if (evidence.kind === 'kg') {
    return (
      <Badge variant="outline" className="gap-1" data-testid="evidence-chip" data-kind="kg">
        <span>{label}</span>
        <button
          type="button"
          className="rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={copyText(evidence.ref)}
          onClick={() => void copy(evidence.ref)}
        >
          <Copy className="size-3" aria-hidden />
        </button>
      </Badge>
    )
  }
  if (evidence.kind === 'file' && onOpenFile) {
    return (
      <Badge variant="outline" asChild>
        <button
          type="button"
          onClick={() => onOpenFile(evidence.ref)}
          aria-label={openFileText(evidence.ref)}
          data-testid="evidence-chip"
          data-kind="file"
        >
          {label}
        </button>
      </Badge>
    )
  }
  return (
    <Badge variant="outline" data-testid="evidence-chip" data-kind={evidence.kind}>
      {label}
    </Badge>
  )
}
