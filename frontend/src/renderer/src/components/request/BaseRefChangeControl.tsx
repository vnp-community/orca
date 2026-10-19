import React, { useState } from 'react'
import { useRepoDefaultBaseRef } from './use-repo-default-base-ref'
import { useRequestProjectRepos } from './use-request-project-repos'
import { BaseRefPickerPopover } from './BaseRefPickerPopover'
import type { OrcaRequest } from '../../../../shared/request-types'

type Props = {
  request: Pick<OrcaRequest, 'projectId' | 'baseRef'>
  /** Stores the chosen ref; `''` means the repository default. */
  onSave: (baseRef: string) => void
  /** The trigger button (the pen on the Overview row, the action of the failure card). */
  children: React.ReactNode
}

/**
 * Lets the user choose another starting branch for an existing Request. It works on the project's first repo, the one the
 * Request runs on, and renders nothing until that repo is known: a trigger that cannot open anything would be a dead control.
 */
export function BaseRefChangeControl({
  request,
  onSave,
  children
}: Props): React.JSX.Element | null {
  const repos = useRequestProjectRepos(request.projectId)
  const repo = repos.status === 'ready' ? repos.repos[0] : undefined
  if (!repo) {
    return null
  }
  return (
    <ChangeForRepo repoId={repo.id} current={request.baseRef ?? ''} onSave={onSave}>
      {children}
    </ChangeForRepo>
  )
}

function ChangeForRepo({
  repoId,
  current,
  onSave,
  children
}: {
  repoId: string
  current: string
  onSave: (baseRef: string) => void
  children: React.ReactNode
}): React.JSX.Element {
  const [open, setOpen] = useState(false)
  const defaultBranch = useRepoDefaultBaseRef(repoId)
  const defaultName = defaultBranch.status === 'ready' ? defaultBranch.name : null

  const choose = (ref: string): void => {
    setOpen(false)
    // Picking the default by name means "follow the default", stored as the empty ref; an unchanged choice saves nothing.
    const next = ref === defaultName ? '' : ref
    if (next !== current) {
      onSave(next)
    }
  }

  return (
    <BaseRefPickerPopover
      repoId={repoId}
      value={current}
      offline={defaultBranch.status === 'failed'}
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (next && defaultBranch.status === 'failed') {
          defaultBranch.retry()
        }
      }}
      onPick={choose}
      onUseDefault={() => choose('')}
    >
      {children}
    </BaseRefPickerPopover>
  )
}
