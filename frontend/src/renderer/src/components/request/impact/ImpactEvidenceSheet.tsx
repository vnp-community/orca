/**
 * ImpactEvidenceSheet — FE-REQ-TASK-036-05
 *
 * Raw evidence is shown as plain text in a <pre>; never HTML or Markdown.
 *
 * @module components/request/impact/ImpactEvidenceSheet
 */

import React, { useEffect, useState } from 'react'
import { Copy } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { callRequestRpc } from '../../../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../../../shared/request-rpc-methods'

const T = 'auto.components.request.impact.'

type EvidenceState = { status: 'loading' } | { status: 'ready'; text: string; truncated: boolean } | { status: 'forbidden' | 'error' }

type Props = { findingId: string | null; title?: string; onClose: () => void }

export function ImpactEvidenceSheet({ findingId, title, onClose }: Props): React.JSX.Element {
  const [state, setState] = useState<EvidenceState>({ status: 'loading' })

  useEffect(() => {
    if (!findingId) {return}
    let cancelled = false
    setState({ status: 'loading' })
    void callRequestRpc<{ evidence?: unknown; text?: unknown; truncated?: unknown }>(REQUEST_RPC_METHODS.IMPACT_EVIDENCE, { findingId }).then((res) => {
      if (cancelled) {return}
      if (!res.ok) {
        setState({ status: res.error.kind === 'forbidden' ? 'forbidden' : 'error' })
        return
      }
      const raw = res.value.evidence ?? res.value.text
      setState({
        status: 'ready',
        text: typeof raw === 'string' ? raw : raw === undefined ? '' : JSON.stringify(raw, null, 2),
        truncated: res.value.truncated === true
      })
    })
    return () => { cancelled = true }
  }, [findingId])

  const copy = async (): Promise<void> => {
    if (state.status !== 'ready') {return}
    try {
      await navigator.clipboard.writeText(state.text)
    } catch {
      // Clipboard can be unavailable (permissions, insecure context); copying is a convenience only.
    }
  }

  return (
    <Sheet open={findingId !== null} onOpenChange={(open) => { if (!open) {onClose()} }}>
      <SheetContent side="right" className="gap-3 p-4 sm:max-w-xl">
        <SheetHeader className="p-0">
          <SheetTitle>{title ?? translate(`${T}evidence`, 'Evidence')}</SheetTitle>
          <SheetDescription className="sr-only">{translate(`${T}evidenceDescription`, 'Raw evidence for this finding')}</SheetDescription>
        </SheetHeader>
        {state.status === 'loading' ? <p className="text-xs text-muted-foreground">{translate(`${T}loading`, 'Loading...')}</p> : null}
        {state.status === 'forbidden' ? <p className="text-xs text-muted-foreground">{translate(`${T}forbidden`, 'You do not have permission to view the assessment')}</p> : null}
        {state.status === 'error' ? <p role="alert" className="text-xs text-destructive">{translate(`${T}evidenceError`, 'Could not load the evidence')}</p> : null}
        {state.status === 'ready' ? (
          <>
            <pre className="max-h-[60vh] overflow-auto whitespace-pre-wrap rounded-md border border-border bg-muted p-2 text-xs scrollbar-sleek" data-testid="impact-evidence-text">{state.text}</pre>
            {state.truncated ? <p className="text-xs text-muted-foreground">{translate(`${T}truncated`, 'Truncated')}</p> : null}
            <Button size="sm" variant="outline" className="self-start" onClick={() => void copy()}>
              <Copy className="size-3.5" aria-hidden />
              {translate(`${T}copy`, 'Copy')}
            </Button>
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  )
}
