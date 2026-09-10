import React, { useCallback, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAppStore } from '@/store'
import { initializeRepoAsGit, initializeRepoAsGitAndRetry } from '@/lib/init-repo-as-git'
import { translate } from '@/i18n/i18n'

/** Opened either from WorktreeCreationPanel's "Initialize as Git repo"
 *  action when a worktree-create attempt fails because the repo's folder
 *  exists but was never `git init`'d (see workspace-create-error-format.ts's
 *  'not-a-git-repo' detection — modalData.creationId, retries the failed
 *  create on success), or from Settings' RepositoryPane when its own
 *  proactive check (repo-git-status-check.ts) finds the same thing with no
 *  pending creation to retry (modalData.repoId, calls the caller's
 *  onSuccess instead). Collects an optional default branch and remote URL,
 *  runs `git init` (+ `git remote add`, same call) against the repo's own
 *  dev-server/path. */
const InitRepoAsGitDialog = React.memo(function InitRepoAsGitDialog() {
  const activeModal = useAppStore((s) => s.activeModal)
  const modalData = useAppStore((s) => s.modalData)
  const closeModal = useAppStore((s) => s.closeModal)
  const [defaultBranch, setDefaultBranch] = useState('')
  const [remoteUrl, setRemoteUrl] = useState('')
  const [remoteName, setRemoteName] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const isOpen = activeModal === 'init-repo-as-git'
  const creationId = typeof modalData.creationId === 'string' ? modalData.creationId : ''
  const repoId = typeof modalData.repoId === 'string' ? modalData.repoId : ''
  const onSuccess = typeof modalData.onSuccess === 'function' ? modalData.onSuccess : undefined
  const folderPath = typeof modalData.folderPath === 'string' ? modalData.folderPath : ''

  const reset = useCallback(() => {
    setDefaultBranch('')
    setRemoteUrl('')
    setRemoteName('')
    setError(null)
    setSubmitting(false)
  }, [])

  const handleOpenChange = useCallback(
    (open: boolean) => {
      if (!open) {
        closeModal()
        reset()
      }
    },
    [closeModal, reset]
  )

  const handleInit = useCallback(async () => {
    if (!creationId && !repoId) {
      return
    }
    setSubmitting(true)
    setError(null)
    const options = {
      defaultBranch: defaultBranch.trim() || undefined,
      remoteUrl: remoteUrl.trim() || undefined,
      remoteName: remoteName.trim() || undefined
    }
    const failure = creationId
      ? await initializeRepoAsGitAndRetry(creationId, options)
      : await initializeRepoAsGit(repoId, options)
    setSubmitting(false)
    if (failure) {
      setError(failure)
      return
    }
    onSuccess?.()
    closeModal()
    reset()
  }, [creationId, repoId, onSuccess, defaultBranch, remoteUrl, remoteName, closeModal, reset])

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-sm sm:max-w-sm">
        <DialogHeader>
          <DialogTitle className="text-sm">
            {translate(
              'auto.components.worktree.creation.InitRepoAsGitDialog.title',
              'Initialize as Git repo'
            )}
          </DialogTitle>
          <DialogDescription className="text-xs">
            {translate(
              'auto.components.worktree.creation.InitRepoAsGitDialog.description',
              'Runs git init on this folder. Optionally attach a remote now — you can always change it later.'
            )}
          </DialogDescription>
        </DialogHeader>

        {folderPath && (
          <div className="rounded-md border border-border/70 bg-muted/35 px-3 py-2 text-xs">
            <div className="break-all text-muted-foreground">{folderPath}</div>
          </div>
        )}

        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="init-repo-default-branch" className="text-xs">
              {translate(
                'auto.components.worktree.creation.InitRepoAsGitDialog.defaultBranchLabel',
                'Default branch (optional)'
              )}
            </Label>
            <Input
              id="init-repo-default-branch"
              placeholder="main"
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
              className="h-8 text-xs"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="init-repo-remote-url" className="text-xs">
              {translate(
                'auto.components.worktree.creation.InitRepoAsGitDialog.remoteUrlLabel',
                'Remote URL (optional)'
              )}
            </Label>
            <Input
              id="init-repo-remote-url"
              placeholder="git@github.com:org/repo.git"
              value={remoteUrl}
              onChange={(e) => setRemoteUrl(e.target.value)}
              className="h-8 text-xs"
            />
          </div>
          {remoteUrl.trim() && (
            <div className="space-y-1.5">
              <Label htmlFor="init-repo-remote-name" className="text-xs">
                {translate(
                  'auto.components.worktree.creation.InitRepoAsGitDialog.remoteNameLabel',
                  'Remote name'
                )}
              </Label>
              <Input
                id="init-repo-remote-name"
                placeholder="origin"
                value={remoteName}
                onChange={(e) => setRemoteName(e.target.value)}
                className="h-8 text-xs"
              />
            </div>
          )}
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)} disabled={submitting}>
            {translate('auto.components.worktree.creation.InitRepoAsGitDialog.cancel', 'Cancel')}
          </Button>
          <Button onClick={() => void handleInit()} disabled={submitting}>
            {submitting
              ? translate(
                  'auto.components.worktree.creation.InitRepoAsGitDialog.initializing',
                  'Initializing…'
                )
              : translate(
                  'auto.components.worktree.creation.InitRepoAsGitDialog.confirm',
                  'Initialize'
                )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
})

export default InitRepoAsGitDialog
