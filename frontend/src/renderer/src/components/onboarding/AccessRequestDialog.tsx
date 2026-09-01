// AccessRequestDialog.tsx — CR-DS-008 skip-onboarding role branch.
// Shown instead of OnboardingSkipConfirmationDialog's plain skip path when a
// non-admin user has no dev servers available yet (devServer.listForUser
// returned empty) — lets them file a DevServerAccessRequest against a
// DevServerGroup rather than silently landing in an empty app.
import { useCallback, useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { translate } from '@/i18n/i18n'
import type { DevServerGroup } from '../../../../shared/dev-server-types'

export function AccessRequestDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called after the request is filed successfully — caller decides what
   *  happens next (this dialog only closes itself). */
  onRequested: () => void
}): React.JSX.Element {
  const { open, onOpenChange, onRequested } = props
  const [groups, setGroups] = useState<DevServerGroup[]>([])
  const [groupsLoading, setGroupsLoading] = useState(false)
  const [selectedGroupId, setSelectedGroupId] = useState('')
  const [message, setMessage] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open || groups.length > 0 || groupsLoading) {
      return
    }
    setGroupsLoading(true)
    window.api.devServerGroup
      .list()
      .then((result) => setGroups(result))
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setGroupsLoading(false))
  }, [open, groups.length, groupsLoading])

  const handleSubmit = useCallback(() => {
    if (!selectedGroupId || submitting) {
      return
    }
    setSubmitting(true)
    setError(null)
    window.api.devServer
      .requestAccess({ devServerGroupId: selectedGroupId, message: message.trim() || undefined })
      .then(() => onRequested())
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setSubmitting(false))
  }, [selectedGroupId, message, submitting, onRequested])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        overlayClassName="z-[120] bg-black/35"
        className="z-[130] sm:max-w-[420px]"
      >
        <DialogHeader>
          <DialogTitle>
            {translate(
              'auto.components.onboarding.AccessRequestDialog.title',
              'Request dev server access'
            )}
          </DialogTitle>
          <DialogDescription>
            {translate(
              'auto.components.onboarding.AccessRequestDialog.description',
              "You don't have access to any dev servers yet. Pick a group and an admin will review your request."
            )}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          <Select value={selectedGroupId} onValueChange={setSelectedGroupId}>
            <SelectTrigger>
              <SelectValue
                placeholder={translate(
                  'auto.components.onboarding.AccessRequestDialog.groupPlaceholder',
                  'Choose a dev server group'
                )}
              />
            </SelectTrigger>
            <SelectContent>
              {groups.map((group) => (
                <SelectItem key={group.id} value={group.id}>
                  {group.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder={translate(
              'auto.components.onboarding.AccessRequestDialog.messagePlaceholder',
              'Optional note for the admin'
            )}
            rows={3}
          />

          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {translate('auto.components.onboarding.AccessRequestDialog.cancel', 'Cancel')}
          </Button>
          <Button type="button" disabled={!selectedGroupId || submitting} onClick={handleSubmit}>
            {submitting
              ? translate('auto.components.onboarding.AccessRequestDialog.sending', 'Sending…')
              : translate('auto.components.onboarding.AccessRequestDialog.submit', 'Send request')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
