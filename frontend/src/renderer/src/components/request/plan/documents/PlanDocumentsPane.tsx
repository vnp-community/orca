/**
 * PlanDocumentsPane — CR-REQ-045
 *
 * The third view of the Plan tab: the OpenSpec change's documents, or the native plan's proposal. The engine decides which;
 * the version on screen comes from the version list.
 *
 * @module components/request/plan/documents/PlanDocumentsPane
 */

import React from 'react'
import type { PlanVersionsApi } from '../revision/usePlanVersions'
import type { OrcaRequest } from '../../../../../../shared/request-types'
import { PlanDocumentsView } from './PlanDocumentsView'
import { PlanProposalView } from './PlanProposalView'

type Props = {
  request: Pick<OrcaRequest, 'id' | 'engine' | 'status' | 'version' | 'planTaskId'>
  versions: PlanVersionsApi
}

export function PlanDocumentsPane({ request, versions }: Props): React.JSX.Element {
  const View = request.engine === 'openspec' ? PlanDocumentsView : PlanProposalView
  return <View request={request} shown={versions.shown} current={versions.list.current} />
}
