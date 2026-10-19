/**
 * StartFromField — FE-REQ-TASK-040-03
 *
 * "Start from" of the create dialog: the branch every worktree of the new Request starts from. It shows the repository
 * default until the user picks something else, and only a picked ref is sent. It is about the project's first repo, the one
 * the Request runs on, and stays hidden when the gateway cannot store the ref (it would be silently dropped).
 *
 * @module components/request/StartFromField
 */

import React, { useEffect, useId, useState } from 'react'
import { ChevronDown, GitBranch } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { supportsBaseRef } from '@/store/slices/request'
import { BaseRefPickerPopover } from './BaseRefPickerPopover'
import { useRepoDefaultBaseRef } from './use-repo-default-base-ref'
import { useRequestProjectRepos } from './use-request-project-repos'
import type { StartFromAction, StartFromState } from './start-from-state'

const T = 'auto.components.request.StartFromField.'

/** A backend refusal of the picked ref, shown under the field with the gateway's near matches. */
export type StartFromFieldError = { message: string; suggestions: string[] }

type Props = {
  projectId: string
  state: StartFromState
  dispatch: React.Dispatch<StartFromAction>
  error: StartFromFieldError | null
  disabled?: boolean
}

export function StartFromField({
  projectId,
  state,
  dispatch,
  error,
  disabled
}: Props): React.JSX.Element | null {
  const supported = useAppStore(supportsBaseRef)
  const repos = useRequestProjectRepos(projectId)
  const repo = repos.status === 'ready' ? repos.repos[0] : undefined
  if (!supported || !repo) {
    return null
  }
  return (
    <StartFromControl
      key={repo.id}
      repoId={repo.id}
      repoName={repo.displayName}
      state={state}
      dispatch={dispatch}
      error={error}
      disabled={disabled}
    />
  )
}

type ControlProps = Omit<Props, 'projectId'> & { repoId: string; repoName: string }

function StartFromControl({
  repoId,
  repoName,
  state,
  dispatch,
  error,
  disabled
}: ControlProps): React.JSX.Element {
  const defaultBranch = useRepoDefaultBaseRef(repoId)
  const [open, setOpen] = useState(false)
  const ids = useId()
  const hintId = `${ids}-hint`
  const errorId = `${ids}-error`
  const defaultName = defaultBranch.status === 'ready' ? defaultBranch.name : null

  // Keep the reducer in step with the lookup, so the dialog can tell the host could not be asked.
  const lookupFailed = defaultBranch.status === 'failed'
  useEffect(() => {
    dispatch({ type: 'hostOffline', offline: lookupFailed })
  }, [lookupFailed, dispatch])

  const defaultLabel = defaultName
    ? translate(`${T}defaultNamed`, 'Default branch ({{name}})', { name: defaultName })
    : translate(`${T}defaultUnnamed`, 'Default branch')
  const label = state.ref ?? defaultLabel

  const pick = (ref: string): void => {
    // Picking the default by name is "no choice": nothing is sent and the repo default keeps applying.
    dispatch(ref === defaultName ? { type: 'useDefault' } : { type: 'select', ref })
    setOpen(false)
  }

  return (
    <div className="flex flex-col gap-1.5" data-testid="start-from-field">
      <Label htmlFor={`${ids}-trigger`}>{translate(`${T}label`, 'Start from')}</Label>
      <div className="flex flex-wrap items-center gap-2">
        <BaseRefPickerPopover
          repoId={repoId}
          value={state.ref ?? undefined}
          offline={state.hostOffline}
          open={open}
          onOpenChange={(next) => {
            setOpen(next)
            if (next && lookupFailed) {
              defaultBranch.retry()
            }
          }}
          onPick={pick}
          onUseDefault={() => {
            dispatch({ type: 'useDefault' })
            setOpen(false)
          }}
        >
          <Button
            id={`${ids}-trigger`}
            type="button"
            variant="outline"
            size="sm"
            disabled={disabled}
            aria-label={translate(`${T}triggerLabel`, 'Starting branch: {{branch}}', {
              branch: label
            })}
            aria-describedby={error ? `${hintId} ${errorId}` : hintId}
            aria-invalid={error ? true : undefined}
            className="max-w-full justify-start"
          >
            <GitBranch className="size-3.5 shrink-0" aria-hidden="true" />
            <span className="min-w-0 truncate" title={label}>
              {label}
            </span>
            <ChevronDown className="size-3.5 shrink-0 opacity-60" aria-hidden="true" />
          </Button>
        </BaseRefPickerPopover>
        {state.ref !== null && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={disabled}
            onClick={() => dispatch({ type: 'useDefault' })}
          >
            {translate(`${T}useDefault`, 'Use default branch')}
          </Button>
        )}
      </div>
      <p id={hintId} className="text-xs text-muted-foreground">
        {translate(`${T}hint`, 'Worktrees for this request start from this branch of {{repo}}.', {
          repo: repoName
        })}
      </p>
      <div id={errorId} role="alert" className="flex flex-col items-start gap-1.5 text-xs">
        {error && (
          <>
            <p className="text-destructive">{error.message}</p>
            {error.suggestions.length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-muted-foreground">
                  {translate(
                    'auto.components.request.RequestBaseRefRow.suggestions',
                    'Did you mean:'
                  )}
                </span>
                {error.suggestions.map((ref) => (
                  <Button
                    key={ref}
                    type="button"
                    variant="outline"
                    size="xs"
                    disabled={disabled}
                    onClick={() => pick(ref)}
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
