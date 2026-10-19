// Banner khi task bị reject bởi quality gate (CR-ORCA-003)
//
// Backend error format từ CompletionHook.OnTaskComplete() (SOL-ORCA-003 §2.5):
//   reason: "convention check failed: %d files missing '// implements: CRS-XXX', %d invalid"
//   action_required: "Add '// implements: CRS-045' comment to each changed file"
//
// parseConventionError() parse exact format backend — không dùng generic regex

import type { FC } from 'react'
import type { TaskRejection } from '../../types/task-types'

interface TaskShape {
  id: string
  status: string
  rejection?: TaskRejection
}

interface Props {
  task: TaskShape
}

// Parse backend convention error format (SOL-ORCA-003 §2.5 NewRejectedResponse)
// Format: "convention check failed: %d files missing '// implements: CRS-XXX', %d invalid"
interface ParsedConventionError {
  isConventionFail: boolean
  missingCount?: number
  crsId?: string
}

export function parseConventionError(reason: string): ParsedConventionError {
  const match = reason.match(
    /convention check failed: (\d+) files missing '\/\/ implements: ([^']+)'/
  )
  if (match == null) return { isConventionFail: false }
  return {
    isConventionFail: true,
    missingCount: parseInt(match[1], 10),
    crsId: match[2],
  }
}

export const TaskRejectionBanner: FC<Props> = ({ task }) => {
  if (task.status !== 'rejected' || task.rejection == null) return null

  const { reason, action_required } = task.rejection
  const parsed = parseConventionError(reason)

  return (
    <div
      className="task-rejection-banner"
      role="alert"
      aria-label="Task rejected by quality gate"
      id={`task-rejection-${task.id}`}
    >
      <div className="task-rejection-banner__header">
        <span className="task-rejection-banner__icon" aria-hidden="true">
          ⛔
        </span>
        <strong className="task-rejection-banner__title">Quality Gate Failed</strong>
      </div>

      {parsed.isConventionFail ? (
        // Structured display khi parse thành công
        <div className="task-rejection-banner__convention">
          <p>
            {parsed.missingCount} file{parsed.missingCount !== 1 ? 's' : ''} missing{' '}
            <code>// implements: {parsed.crsId}</code> comment.
          </p>
        </div>
      ) : (
        // Fallback: raw reason text
        <p className="task-rejection-banner__reason">{reason}</p>
      )}

      {action_required.length > 0 && (
        <div className="task-rejection-banner__action">
          <strong>Action Required:</strong>
          <code className="task-rejection-banner__action-code">{action_required}</code>
        </div>
      )}

      <p className="task-rejection-banner__hint">Fix the convention issue and re-submit the task.</p>
    </div>
  )
}
