/**
 * RequestBaseRefRow — FE-REQ-TASK-040-04
 *
 * Overview row for the branch every worktree of a Request starts from: read it, change it while no worktree exists, see why it
 * is locked afterwards. Hidden when the gateway cannot store the ref or the Request's flow never creates worktrees.
 *
 * @module components/request/RequestBaseRefRow
 */

import React from 'react'
import { GitBranch, Lock, Pencil } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { supportsBaseRef } from '@/store/slices/request'
import { useRequestFlow } from '../../hooks/useRequestFlow'
import { BaseRefChangeControl } from './BaseRefChangeControl'
import { BaseRefSaveFeedback } from './BaseRefSaveFeedback'
import { baseRefLockReasonText } from './base-ref-write-error'
import { useFreshestRequest, useRequestBaseRefSave } from './use-request-base-ref-save'
import type { OrcaRequest } from '../../../../shared/request-types'

const T = 'auto.components.request.RequestBaseRefRow.'

type Props = {
  request: OrcaRequest
  /** Called after a save, a lock refusal or a conflict reload changed the stored Request, so the pane can refetch it. */
  onChanged?: () => void
}

export function RequestBaseRefRow({ request, onChanged }: Props): React.JSX.Element | null {
  const supported = useAppStore(supportsBaseRef)
  const flow = useRequestFlow(request.type, request.size)
  const current = useFreshestRequest(request)
  const { state, save, applyConflict, dismiss } = useRequestBaseRefSave(current, onChanged)

  // A flow without a plan (question, spike) never makes a worktree; an unclassified Request has no flow yet and keeps the row.
  if (!supported || current.baseRef === undefined || flow?.planKind === 'none') {
    return null
  }

  const locked = current.baseRefLocked === true
  const lockText = baseRefLockReasonText(current.baseRefLockReason)
  const canEdit = !locked && !state.forbidden
  const label = translate(`${T}label`, 'Starting branch')

  return (
    <div className="flex flex-col gap-1" data-testid="request-base-ref-row">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div className="flex flex-wrap items-center gap-2 text-sm text-foreground">
        <GitBranch className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
        {current.baseRef === '' ? (
          <span>
            {translate(`${T}startsFromDefault`, 'Worktrees start from the default branch')}
          </span>
        ) : (
          <span className="min-w-0 break-all">
            {translate(`${T}startsFrom`, 'Worktrees start from {{ref}}', { ref: current.baseRef })}
          </span>
        )}
        {locked && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="inline-flex items-center gap-1 rounded-sm text-xs text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                aria-label={translate(`${T}lockedLabel`, 'Locked: {{reason}}', {
                  reason: lockText
                })}
                data-testid="base-ref-lock"
              >
                <Lock className="size-3.5" aria-hidden="true" />
                {translate(`${T}locked`, 'Locked')}
              </button>
            </TooltipTrigger>
            <TooltipContent>{lockText}</TooltipContent>
          </Tooltip>
        )}
        {canEdit && (
          <BaseRefChangeControl request={current} onSave={(ref) => void save(ref)}>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              disabled={state.saving}
              aria-label={translate(`${T}edit`, 'Change starting branch')}
              data-testid="base-ref-edit"
            >
              <Pencil aria-hidden="true" />
            </Button>
          </BaseRefChangeControl>
        )}
        {state.saving && (
          <span role="status" className="text-xs text-muted-foreground">
            {translate(`${T}saving`, 'Saving...')}
          </span>
        )}
      </div>
      <BaseRefSaveFeedback
        state={state}
        currentBaseRef={current.baseRef}
        unverified={current.baseRefUnverified === true || state.unverifiedRef === current.baseRef}
        onApplyConflict={() => void applyConflict()}
        onDismiss={dismiss}
        onPickSuggestion={(ref) => void save(ref)}
      />
    </div>
  )
}
