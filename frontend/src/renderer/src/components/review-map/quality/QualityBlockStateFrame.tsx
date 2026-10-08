/**
 * QualityBlockStateFrame.tsx — FE-CV-TASK-087-16
 *
 * ChartFrame for the states without a drawing (loading, empty with a reason, error with retry),
 * so every lens block reports its state the same way.
 *
 * @module components/review-map/quality/QualityBlockStateFrame
 */

import { ChartFrame } from '../../quality-charts/ChartFrame'

const NO_TABLE = { caption: '', columns: [], rows: [] }

export function QualityBlockStateFrame(props: {
  id: string
  title: string
  description?: string
  status: 'loading' | 'empty' | 'error'
  emptyReason?: React.ReactNode
  errorMessage?: string
  onRetry?: () => void
  minHeight?: number
}): React.JSX.Element {
  return (
    <ChartFrame
      id={props.id}
      title={props.title}
      description={props.description}
      summary={props.title}
      status={props.status}
      emptyReason={props.emptyReason}
      error={
        props.errorMessage ? { message: props.errorMessage, onRetry: props.onRetry } : undefined
      }
      table={NO_TABLE}
      minHeight={props.minHeight ?? 72}
    >
      {() => null}
    </ChartFrame>
  )
}
