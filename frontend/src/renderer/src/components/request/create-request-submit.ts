/**
 * create-request-submit.ts — CR-REQ-019-06, FE-REQ-TASK-040-03
 *
 * The `request.create` call of the create dialog and what follows a created Request: store, navigation and the notices about
 * the starting branch. `baseRef` is in the call only when the user picked one, and the answer is checked for it afterwards
 * because a gateway older than CR-REQ-039 ignores the field and would look like success.
 *
 * @module components/request/create-request-submit
 */

import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { callRequestRpc } from '../../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../../shared/request-rpc-methods'
import {
  parseRequestCreateResult,
  verifyBaseRefEcho,
  type RequestCreateResult
} from '../../../../shared/request-base-ref'
import type { Result } from '../../runtime/request-rpc-client'
import type { OrcaRequest, RequestUrgency } from '../../../../shared/request-types'
import type { CreateRequestParams } from './create-request-from-issue'

const T = 'auto.components.request.CreateRequestDialog.'
// The same sentences the Overview row uses when it saves a branch, so one translation serves both.
const ROW = 'auto.components.request.RequestBaseRefRow.'

export type CreateRequestInput = {
  projectId: string
  title: string
  body: string
  source: CreateRequestParams['source']
  urgency: RequestUrgency
  /** `undefined` lets the AI classify. */
  typeHint: string | undefined
  clientRequestId: string
  /** Only a branch the user picked; undefined sends no `baseRef` at all. */
  baseRef: string | undefined
}

export async function createRequest(
  input: CreateRequestInput
): Promise<Result<RequestCreateResult>> {
  const result = await callRequestRpc<unknown>(
    REQUEST_RPC_METHODS.CREATE,
    {
      projectId: input.projectId,
      title: input.title.trim(),
      ...(input.body.trim() ? { body: input.body.trim() } : {}),
      ...(input.source ? { source: input.source } : {}),
      hints: {
        priority: input.urgency === 'urgent' ? 'high' : undefined,
        ...(input.typeHint ? { issueType: input.typeHint } : {})
      },
      clientRequestId: input.clientRequestId,
      ...(input.baseRef ? { baseRef: input.baseRef } : {})
    },
    // Why: the same clientRequestId makes a repeat answer `created: false`, so a throttled create may be retried once.
    { idempotent: true }
  )
  return result.ok ? { ok: true, value: parseRequestCreateResult(result.value) } : result
}

function openRequest(id: string): void {
  const s = useAppStore.getState()
  s.setRequestPageData({ section: 'requests', requestId: id })
  s.setActiveView('requests')
}

function openAction(request: OrcaRequest): { label: string; onClick: () => void } {
  return { label: translate(`${T}open`, 'Open'), onClick: () => openRequest(request.id) }
}

/** Warns when the stored ref is not the one asked for, or that it could not be checked yet. */
function announceBaseRef(
  request: OrcaRequest,
  sent: string | undefined,
  unverified: boolean
): void {
  if (verifyBaseRefEcho(sent, request) === 'mismatch') {
    toast.warning(translate(`${ROW}notApplied`, "This server didn't apply the starting branch."), {
      description: translate(
        `${T}baseRefNotAppliedDetail`,
        'The request was created, but its worktrees will not start from {{ref}}.',
        { ref: sent ?? '' }
      ),
      action: openAction(request)
    })
    return
  }
  if (unverified) {
    toast.info(
      translate(
        `${ROW}unverified`,
        "We couldn't reach the host to check this branch. It will be checked when the worktree is created."
      ),
      { action: openAction(request) }
    )
  }
}

/** Stores the created Request, closes the dialog and opens it (or says the issue already had one). */
export function finishCreate(
  result: RequestCreateResult,
  sentBaseRef: string | undefined,
  closeDialog: () => void
): void {
  const { request } = result
  useAppStore.getState().upsertRequests([request])
  closeDialog()
  if (result.created === false) {
    // Why no branch check: this is the Request the issue already had, made earlier with its own starting branch.
    toast(translate(`${T}duplicate`, 'This issue already has a request'), {
      action: openAction(request)
    })
    return
  }
  openRequest(request.id)
  announceBaseRef(request, sentBaseRef, result.baseRefUnverified)
}
