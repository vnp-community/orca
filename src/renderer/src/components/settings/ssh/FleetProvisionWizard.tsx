// FleetProvisionWizard — 4-step dialog for bulk SSH relay provisioning (CR-003, TASK-003-C)
import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { Zap } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { ProvisionServerSelector } from './ProvisionServerSelector'
import { ProvisionConfirmStep } from './ProvisionConfirmStep'
import { ProvisionProgressPanel } from './ProvisionProgressPanel'
import { ProvisionDoneSummary } from './ProvisionDoneSummary'

type WizardStep = 'select' | 'confirm' | 'provision' | 'done'

export function FleetProvisionWizard({
  open,
  onClose
}: {
  open: boolean
  onClose: () => void
}): React.JSX.Element {
  const [step, setStep] = useState<WizardStep>('select')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const session = useAppStore((s) => s.provisioningSession)
  const startSession = useAppStore((s) => s.startProvisioningSession)
  const cancelSession = useAppStore((s) => s.cancelProvisioningSession)

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      setStep('select')
      setSelectedIds(new Set())
    }
  }, [open])

  // Auto-advance to done when session phase transitions to 'done'
  useEffect(() => {
    if (step === 'provision' && session?.phase === 'done') {
      setStep('done')
    }
  }, [step, session?.phase])

  const handleStartProvision = async (): Promise<void> => {
    const ids = Array.from(selectedIds)
    startSession(ids)
    setStep('provision')
    try {
      await window.api.ssh.provisionFleetServers?.({ serverIds: ids, concurrency: 3 })
    } catch {
      toast.error(translate('fleet.provision.startError', 'Failed to start provisioning'))
      cancelSession()
      setStep('select')
    }
  }

  const handleClose = (): void => {
    // Only allow close when not actively provisioning
    if (step !== 'provision') {
      cancelSession()
      onClose()
    }
  }

  const titles: Record<WizardStep, string> = {
    select: translate('fleet.provision.stepSelect', 'Select Servers'),
    confirm: translate('fleet.provision.stepConfirm', 'Confirm'),
    provision: translate('fleet.provision.stepProvisioning', 'Provisioning...'),
    done: translate('fleet.provision.stepDone', 'Complete')
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) handleClose()
      }}
    >
      <DialogContent className="sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Zap className="size-4" />
            {translate('fleet.provision.title', 'Provision Fleet Servers')} — {titles[step]}
          </DialogTitle>
          <DialogDescription>
            {step === 'select' &&
              translate(
                'fleet.provision.selectHint',
                'Choose which SSH hosts to deploy the Orca relay to.'
              )}
            {step === 'confirm' &&
              translate(
                'fleet.provision.confirmHint',
                'Review your selection before starting the deployment.'
              )}
            {step === 'provision' &&
              translate(
                'fleet.provision.progressHint',
                'Relay is being deployed in parallel. Please wait.'
              )}
            {step === 'done' &&
              translate(
                'fleet.provision.doneHint',
                'Provisioning finished. New hosts are ready for use.'
              )}
          </DialogDescription>
        </DialogHeader>

        {/* ── Step 1: Select servers ── */}
        {step === 'select' && (
          <>
            <ProvisionServerSelector
              selectedIds={selectedIds}
              onSelectionChange={setSelectedIds}
            />
            <div className="flex justify-end gap-2 pt-2">
              <Button
                variant="ghost"
                size="sm"
                onClick={handleClose}
              >
                {translate('common.cancel', 'Cancel')}
              </Button>
              <Button
                size="sm"
                onClick={() => setStep('confirm')}
                disabled={selectedIds.size === 0}
              >
                {translate('common.next', 'Next')} ({selectedIds.size})
              </Button>
            </div>
          </>
        )}

        {/* ── Step 2: Confirm ── */}
        {step === 'confirm' && (
          <ProvisionConfirmStep
            serverIds={Array.from(selectedIds)}
            onBack={() => setStep('select')}
            onConfirm={() => void handleStartProvision()}
          />
        )}

        {/* ── Step 3: Provisioning in progress ── */}
        {step === 'provision' && session && (
          <ProvisionProgressPanel session={session} />
        )}

        {/* ── Step 4: Done summary ── */}
        {step === 'done' && session && (
          <ProvisionDoneSummary session={session} onClose={handleClose} />
        )}
      </DialogContent>
    </Dialog>
  )
}
