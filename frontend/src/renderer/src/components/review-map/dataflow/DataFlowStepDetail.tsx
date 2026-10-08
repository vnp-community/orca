/** DataFlowStepDetail.tsx — FE-CV-TASK-056-06: detail for a step that has no symbol to open. */

import { translate } from '@/i18n/i18n'
import type { DataFlowStep } from '../../../../../shared/code-intel-architecture-types'
import type { DataFlowStore } from './data-flow-overlay'

export function DataFlowStepDetail({ step, stores }: { step: DataFlowStep; stores: readonly DataFlowStore[] }): React.JSX.Element {
  return (
    <dl aria-label={translate('auto.components.reviewMap.dataflow.stepDetail', 'Step details')} className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 rounded-md border p-3 text-xs">
      <dt className="text-muted-foreground">{translate('auto.components.reviewMap.dataflow.col.step', 'Step')}</dt>
      <dd>{step.n}</dd>
      <dt className="text-muted-foreground">{translate('auto.components.reviewMap.dataflow.col.route', 'From → To')}</dt>
      <dd>{step.from.name} → {step.to.name}</dd>
      {step.requestType ? (<><dt className="text-muted-foreground">{translate('auto.components.reviewMap.dataflow.request', 'Request')}</dt><dd>{step.requestType}</dd></>) : null}
      {step.responseType ? (<><dt className="text-muted-foreground">{translate('auto.components.reviewMap.dataflow.response', 'Response')}</dt><dd>{step.responseType}</dd></>) : null}
      <dt className="text-muted-foreground">{translate('auto.components.reviewMap.dataflow.col.confidence', 'Confidence')}</dt>
      <dd>{Math.round(step.confidence * 100)}% · {step.origin}</dd>
      {stores.length > 0 ? (
        <>
          <dt className="text-muted-foreground">{translate('auto.components.reviewMap.dataflow.col.stores', 'Stores')}</dt>
          <dd>{stores.map((s) => `${s.op} ${s.store.name}${s.table ? `.${s.table}` : ''}`).join(', ')}</dd>
        </>
      ) : null}
    </dl>
  )
}
