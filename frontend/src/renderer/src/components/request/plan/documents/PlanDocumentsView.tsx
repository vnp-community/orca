/**
 * PlanDocumentsView — CR-REQ-045
 *
 * The "Documents" view of an OpenSpec Plan: the checks and the author's knowledge report above, a tab per document of the change
 * below. Read only; the way to change a document is to ask for it (PlanRevisionPanel). The version on screen is the current one
 * unless the reader chose an older one.
 *
 * @module components/request/plan/documents/PlanDocumentsView
 */

import React, { useMemo, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent } from '@/components/ui/tabs'
import { usePlanProposalOfRun } from '../../../../hooks/usePlanProposalOfRun'
import { requestErrorMessage } from '../../request-error-message'
import { PlanChecksPanel } from './PlanChecksPanel'
import { PlanDocumentBody, type DocumentMode } from './PlanDocumentBody'
import { PlanDocumentTabs } from './PlanDocumentTabs'
import { orderDocuments } from './plan-document-model'
import type { PlanRevision } from '../../../../../../shared/request-plan-revision'
import type { OrcaRequest } from '../../../../../../shared/request-types'

type Props = {
  request: Pick<OrcaRequest, 'id' | 'version' | 'planTaskId'>
  /** The version the reader chose; null shows the current one. */
  shown: PlanRevision | null
  /** The current version when the gateway lists versions; null on an older gateway. */
  current: PlanRevision | null
}

export function PlanDocumentsView({ request, shown, current }: Props): React.JSX.Element {
  const onScreen = shown ?? current
  const viewingOlder = shown !== null && shown.revision !== current?.revision
  const proposal = usePlanProposalOfRun(request.id, {
    revision: onScreen?.revision,
    refreshKey: `${request.version ?? ''}|${request.planTaskId ?? ''}|${current?.commit ?? ''}`
  })
  const [selectedPath, setSelectedPath] = useState('')
  const [mode, setMode] = useState<DocumentMode>('rendered')
  const view = proposal.view
  const tabs = useMemo(() => orderDocuments(view?.bundle?.files ?? []), [view])
  // Why: the chosen document stays selected across versions when it still exists there.
  const selected = tabs.find((t) => t.path === selectedPath) ?? tabs[0]
  const textKept = onScreen?.textKept ?? true

  if (view === null && proposal.error === null) {
    return (
      <div
        className="flex flex-col gap-2 p-4"
        aria-busy="true"
        data-testid="plan-documents-loading"
      >
        <Skeleton className="h-8 w-2/3" />
        <Skeleton className="h-24 w-full" />
      </div>
    )
  }
  if (view === null) {
    return (
      <div
        role="alert"
        data-testid="plan-documents-error"
        className="flex flex-wrap items-center gap-2 p-4 text-sm text-destructive"
      >
        <AlertTriangle className="size-4 shrink-0" aria-hidden />
        <span className="min-w-0 flex-1">{requestErrorMessage(proposal.error)}</span>
        <Button size="sm" variant="outline" onClick={proposal.refetch}>
          {translate('auto.components.request.plan.documents.tryAgain', 'Try again')}
        </Button>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3 px-4 py-3" data-testid="plan-documents-view">
      <PlanChecksPanel checks={view.checks} knowledge={view.knowledge} isCurrent={!viewingOlder} />
      {selected === undefined ? (
        <p className="text-sm text-muted-foreground" data-testid="plan-documents-empty">
          {textKept
            ? translate(
                'auto.components.request.plan.documents.empty',
                'This change has no documents yet.'
              )
            : translate(
                'auto.components.request.plan.documents.pruned',
                'The text of this version is no longer kept.'
              )}
        </p>
      ) : (
        <Tabs value={selected.path} onValueChange={setSelectedPath} className="gap-3">
          <PlanDocumentTabs tabs={tabs} showChanges={(onScreen?.revision ?? 2) > 1} />
          <TabsContent value={selected.path}>
            <PlanDocumentBody
              requestId={request.id}
              tab={selected}
              revision={viewingOlder ? shown?.revision : undefined}
              currentKey={
                view.bundle?.tasksMdDigest || view.bundle?.commit || current?.commit || ''
              }
              textKept={textKept}
              branch={view.bundle?.branch ?? ''}
              mode={mode}
              onModeChange={setMode}
            />
          </TabsContent>
        </Tabs>
      )}
    </div>
  )
}
