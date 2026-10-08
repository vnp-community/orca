/**
 * RequestAnalysisTab — CR-REQ-019-03 mounts it; CR-REQ-020-02 fills it.
 *
 * @module components/request/solution/RequestAnalysisTab
 */

import React from 'react'
import { REQUEST_FLOW_REGISTRY } from '../../../../../shared/request-flow-registry'
import type { OrcaRequest } from '../../../../../shared/request-types'
import { SolutionPanel } from './SolutionPanel'

export type RequestAnalysisTabProps = {
  request: OrcaRequest
  /** Call after any mutation so the detail pane refetches the request. */
  onChanged: () => void
}

export function RequestAnalysisTab({ request, onChanged }: RequestAnalysisTabProps): React.JSX.Element | null {
  // Types without an analysis step (task, docs, ...) have no tab content.
  const flow = request.type === 'unknown' ? undefined : REQUEST_FLOW_REGISTRY[request.type]
  if (!flow || flow.analysisKind === null) {return null}
  return <SolutionPanel request={request} onChanged={onChanged} />
}
