import React, { useCallback, useState } from 'react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { useAppStore } from '@/store'
import { useRepoHostRebind } from '@/hooks/use-repo-host-rebind'
import { retryBackgroundWorktreeCreation } from '@/lib/worktree-creation-flow'
import { translate } from '@/i18n/i18n'

/**
 * "Choose host" for a repo that has no dev server (or whose host must change). Opened from the
 * sidebar "+" on an unbound repo, from the worktree-creation error panel (modalData.creationId, the
 * failed create is retried on success) and from Settings → Repository ("Change host").
 */
const RepoHostPickerDialog = React.memo(function RepoHostPickerDialog() {
  const activeModal = useAppStore((s) => s.activeModal)
  const modalData = useAppStore((s) => s.modalData)
  const closeModal = useAppStore((s) => s.closeModal)
  const openSettingsPage = useAppStore((s) => s.openSettingsPage)
  const openSettingsTarget = useAppStore((s) => s.openSettingsTarget)
  const repos = useAppStore((s) => s.repos)
  const { hostChoices, rebindingRepoId, rebind } = useRepoHostRebind()
  const [selectedHostId, setSelectedHostId] = useState('')
  const [error, setError] = useState<string | null>(null)

  const isOpen = activeModal === 'repo-host-picker'
  const repoId = typeof modalData.repoId === 'string' ? modalData.repoId : ''
  const creationId = typeof modalData.creationId === 'string' ? modalData.creationId : ''
  const repo = repos.find((candidate) => candidate.id === repoId)
  const hasSelectableHost = hostChoices.some((host) => host.selectable)
  const submitting = rebindingRepoId !== null

  const reset = useCallback(() => {
    setSelectedHostId('')
    setError(null)
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

  const handleConfirm = useCallback(async () => {
    if (!repoId || !selectedHostId) {
      return
    }
    setError(null)
    const result = await rebind(repoId, selectedHostId)
    if (!result.ok) {
      setError(result.message)
      return
    }
    toast.success(
      translate('auto.components.sidebar.RepoHostPickerDialog.updated', 'Dev server updated')
    )
    closeModal()
    reset()
    // Why retry here: the create that failed for lack of a host can now succeed on the new one.
    if (creationId) {
      retryBackgroundWorktreeCreation(creationId)
    }
  }, [closeModal, creationId, rebind, repoId, reset, selectedHostId])

  const handleOpenDevServers = useCallback(() => {
    closeModal()
    reset()
    openSettingsTarget({ pane: 'dev-servers', repoId: null })
    openSettingsPage()
  }, [closeModal, openSettingsPage, openSettingsTarget, reset])

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-sm sm:max-w-sm" data-testid="repo-host-picker">
        <DialogHeader>
          <DialogTitle className="text-sm">
            {translate('auto.components.sidebar.RepoHostPickerDialog.title', 'Choose a dev server')}
          </DialogTitle>
          <DialogDescription className="text-xs">
            {translate(
              'auto.components.sidebar.RepoHostPickerDialog.description',
              'Orca runs git for {{project}} on its dev server. Pick the one this project lives on.',
              { project: repo?.displayName || repo?.path || '' }
            )}
          </DialogDescription>
        </DialogHeader>

        {hasSelectableHost ? (
          <div className="space-y-1.5">
            <Label htmlFor="repo-host-picker-select" className="text-xs">
              {translate('auto.components.sidebar.RepoHostPickerDialog.hostLabel', 'Dev server')}
            </Label>
            <Select value={selectedHostId} onValueChange={setSelectedHostId}>
              <SelectTrigger
                id="repo-host-picker-select"
                className="w-full"
                data-testid="repo-host-picker-select"
              >
                <SelectValue
                  placeholder={translate(
                    'auto.components.sidebar.RepoHostPickerDialog.placeholder',
                    'Choose a dev server'
                  )}
                />
              </SelectTrigger>
              <SelectContent>
                {hostChoices.map((host) => (
                  <SelectItem key={host.id} value={host.id} disabled={!host.selectable}>
                    {host.selectable
                      ? host.name
                      : translate(
                          'auto.components.sidebar.RepoHostPickerDialog.unavailableHost',
                          '{{name}} ({{status}})',
                          { name: host.name, status: host.statusLabel }
                        )}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        ) : (
          <div
            role="alert"
            className="space-y-2 rounded-md border border-border bg-muted px-3 py-2 text-xs text-muted-foreground"
          >
            <p>
              {translate(
                'auto.components.sidebar.RepoHostPickerDialog.noConnectedHost',
                'No dev server is connected. Connect one, then choose it here.'
              )}
            </p>
            <Button type="button" size="xs" variant="outline" onClick={handleOpenDevServers}>
              {translate(
                'auto.components.sidebar.RepoHostPickerDialog.openDevServers',
                'Open Dev Servers'
              )}
            </Button>
          </div>
        )}

        {error ? (
          <p role="alert" className="text-xs text-destructive" data-testid="repo-host-picker-error">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          <Button variant="outline" onClick={() => handleOpenChange(false)}>
            {translate('auto.components.sidebar.RepoHostPickerDialog.cancel', 'Cancel')}
          </Button>
          <Button
            onClick={() => void handleConfirm()}
            disabled={!selectedHostId || submitting}
            data-testid="repo-host-picker-confirm"
          >
            {submitting
              ? translate('auto.components.sidebar.RepoHostPickerDialog.saving', 'Saving…')
              : translate(
                  'auto.components.sidebar.RepoHostPickerDialog.confirm',
                  'Use this dev server'
                )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
})

export default RepoHostPickerDialog
