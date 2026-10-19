import React from 'react'
import { Info } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type { BaseRefSaveState } from './use-request-base-ref-save'

const T = 'auto.components.request.RequestBaseRefRow.'

type Props = {
  state: BaseRefSaveState
  /** Branch the Request starts from now (`''` = default), for the stale-edit question. */
  currentBaseRef: string
  /** A ref the host could not confirm is stored, as sent by the gateway or kept from the last save. */
  unverified: boolean
  onApplyConflict: () => void
  onDismiss: () => void
  onPickSuggestion: (ref: string) => void
}

function describeRef(ref: string): string {
  return ref === '' ? translate(`${T}defaultBranch`, 'the default branch') : ref
}

/** What a save left to say: the "could not check" notice, the stale-edit question and a refusal with near matches. */
export function BaseRefSaveFeedback({
  state,
  currentBaseRef,
  unverified,
  onApplyConflict,
  onDismiss,
  onPickSuggestion
}: Props): React.JSX.Element {
  const { failure, conflict, saving } = state
  return (
    <div className="flex flex-col gap-1.5">
      {unverified && (
        <p
          className="flex items-start gap-1.5 text-xs text-muted-foreground"
          data-testid="base-ref-unverified"
        >
          <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {translate(
            `${T}unverified`,
            "We couldn't reach the host to check this branch. It will be checked when the worktree is created."
          )}
        </p>
      )}
      <div role="alert" className="flex flex-col items-start gap-1.5 text-xs">
        {conflict && (
          <div className="flex flex-col items-start gap-1.5" data-testid="base-ref-conflict">
            <p>
              {translate(
                `${T}conflict`,
                'Someone else changed this request first. It now starts from {{current}}. Your change was to {{mine}}.',
                { current: describeRef(currentBaseRef), mine: describeRef(conflict.mine) }
              )}
            </p>
            <div className="flex flex-wrap gap-1.5">
              <Button type="button" size="xs" disabled={saving} onClick={onApplyConflict}>
                {translate(`${T}applyMine`, 'Apply my change')}
              </Button>
              <Button
                type="button"
                size="xs"
                variant="outline"
                disabled={saving}
                onClick={onDismiss}
              >
                {translate(`${T}keepCurrent`, 'Keep the current one')}
              </Button>
            </div>
          </div>
        )}
        {failure && (
          <>
            <p className="text-destructive">{failure.message}</p>
            {failure.kind === 'notFound' && failure.suggestions.length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-muted-foreground">
                  {translate(`${T}suggestions`, 'Did you mean:')}
                </span>
                {failure.suggestions.map((ref) => (
                  <Button
                    key={ref}
                    type="button"
                    size="xs"
                    variant="outline"
                    disabled={saving}
                    onClick={() => onPickSuggestion(ref)}
                  >
                    {ref}
                  </Button>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
