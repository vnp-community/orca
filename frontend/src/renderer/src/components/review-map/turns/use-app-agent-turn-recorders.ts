/**
 * use-app-agent-turn-recorders.ts — FE-CV-TASK-060-07 / 089-04
 *
 * App-level mount of both agent-turn recorders so a turn that ends while the Review tab is
 * closed is still recorded. The Review workspace only reads the result through the bus; having
 * it mount the recorders too would record every turn twice.
 *
 * @module components/review-map/turns/use-app-agent-turn-recorders
 */

import { useEffect } from 'react'
import {
  getRegisteredTurnSymbolKeys,
  publishTurnSaved,
  setTurnSaveFailure
} from './review-turn-recorder-bus'
import { useAgentTurnBackendRecorder } from './use-agent-turn-backend-recorder'
import { useReviewTurnRecorder } from './useReviewTurnRecorder'

export function useAppAgentTurnRecorders(): void {
  const { failedWorktreeId, retry } = useReviewTurnRecorder({
    getSymbolKeys: getRegisteredTurnSymbolKeys,
    onSaved: publishTurnSaved
  })
  useAgentTurnBackendRecorder()

  useEffect(() => {
    setTurnSaveFailure(failedWorktreeId ? { worktreeId: failedWorktreeId, retry } : null)
  }, [failedWorktreeId, retry])
}
