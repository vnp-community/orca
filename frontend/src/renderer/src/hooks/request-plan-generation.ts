/**
 * Two-phase request.generatePlan (propose, then commit) per CONTRACT-request-ui-api.
 *
 * @module hooks/request-plan-generation
 */

import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import type { Result } from '../runtime/request-rpc-client'

type ProposeValue = { proposal?: unknown; rawAiResponse?: string; alreadyExists?: boolean }

// Why: `propose` writes nothing and `commit` requires the proposal, so a one-click
// "generate plan" must run both; an existing plan or missing proposal stops after propose.
export async function generatePlanProposeCommit(id: string): Promise<Result<unknown>> {
  const proposed = await callRequestRpc<ProposeValue>(REQUEST_RPC_METHODS.GENERATE_PLAN, {
    id,
    mode: 'propose'
  })
  if (!proposed.ok || proposed.value?.alreadyExists || !proposed.value?.proposal) {
    return proposed
  }
  return callRequestRpc(REQUEST_RPC_METHODS.GENERATE_PLAN, {
    id,
    mode: 'commit',
    proposal: proposed.value.proposal,
    ...(proposed.value.rawAiResponse ? { rawAiResponse: proposed.value.rawAiResponse } : {})
  })
}
