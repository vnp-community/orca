# TASK-003-C — Tạo FleetProvisionWizard + ProvisionServerSelector

**Task ID:** TASK-003-C  
**CR:** CR-003 — Bulk Server Provisioning  
**Solution Ref:** SOL-CR-003, Section 4.1, 4.2  
**Dependencies:** TASK-003-A, TASK-003-B, TASK-002-E  
**Estimated:** 3–4 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `FleetProvisionWizard` — multi-step dialog (Select servers → Confirm → Provisioning → Done) và `ProvisionServerSelector` — server selection step.

---

## Files cần tạo

| File | Mô tả |
|------|-------|
| `src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx` | Wizard shell (4 steps) |
| `src/renderer/src/components/settings/ssh/ProvisionServerSelector.tsx` | Server checkbox list |
| `src/renderer/src/components/settings/ssh/ProvisionConfirmStep.tsx` | Confirm before start |
| `src/renderer/src/components/settings/ssh/ProvisionDoneSummary.tsx` | Done summary |

---

## Bước 1: Tạo ProvisionServerSelector.tsx

```typescript
// src/renderer/src/components/settings/ssh/ProvisionServerSelector.tsx
import { useMemo } from 'react'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { SshConnectionStatusDot } from './SshConnectionStatusDot'
import type { SshTarget } from 'src/shared/ssh-types'

type ProvisionServerSelectorProps = {
  selectedIds: Set<string>
  onSelectionChange: (ids: Set<string>) => void
}

export function ProvisionServerSelector({
  selectedIds,
  onSelectionChange,
}: ProvisionServerSelectorProps) {
  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  const connectionStates = useAppStore((s) => s.sshConnectionStates)

  const grouped = useMemo(() => {
    return sshTargets.reduce<Record<string, SshTarget[]>>((acc, t) => {
      const key = t.project ?? '__unassigned__'
      if (!acc[key]) acc[key] = []
      acc[key].push(t)
      return acc
    }, {})
  }, [sshTargets])

  const allSelected = selectedIds.size === sshTargets.length && sshTargets.length > 0

  const handleToggleAll = () => {
    if (allSelected) {
      onSelectionChange(new Set())
    } else {
      onSelectionChange(new Set(sshTargets.map((t) => t.id)))
    }
  }

  const handleToggle = (id: string, checked: boolean) => {
    const next = new Set(selectedIds)
    checked ? next.add(id) : next.delete(id)
    onSelectionChange(next)
  }

  return (
    <div className="space-y-3">
      {/* Select all */}
      <div className="flex items-center gap-2 pb-2 border-b">
        <Checkbox
          id="select-all-servers"
          checked={allSelected}
          onCheckedChange={(checked) => handleToggleAll()}
        />
        <label htmlFor="select-all-servers" className="text-sm cursor-pointer flex-1">
          {translate('fleet.provision.selectAll', 'Select all servers')}
        </label>
        <span className="text-xs text-muted-foreground tabular-nums">
          {selectedIds.size}/{sshTargets.length}
        </span>
      </div>

      {/* Server list by project */}
      <ScrollArea className="max-h-[320px] pr-2">
        <div className="space-y-3">
          {Object.entries(grouped).map(([project, targets]) => (
            <div key={project}>
              <p className="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {project === '__unassigned__'
                  ? translate('fleet.group.unassigned', 'Unassigned')
                  : project}
              </p>
              <div className="space-y-0.5">
                {targets.map((target) => {
                  const isConnected =
                    connectionStates[target.id]?.status === 'connected'
                  return (
                    <label
                      key={target.id}
                      className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-muted/50"
                    >
                      <Checkbox
                        checked={selectedIds.has(target.id)}
                        onCheckedChange={(checked) =>
                          handleToggle(target.id, !!checked)
                        }
                      />
                      <SshConnectionStatusDot
                        status={
                          connectionStates[target.id]?.status ?? 'disconnected'
                        }
                      />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate">
                          {target.label}
                        </p>
                        <p className="text-xs text-muted-foreground">{target.host}</p>
                      </div>
                      {isConnected && (
                        <Badge variant="outline" className="text-xs text-green-600">
                          {translate('fleet.provision.relayActive', 'relay active')}
                        </Badge>
                      )}
                    </label>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}
```

## Bước 2: Tạo ProvisionConfirmStep.tsx

```typescript
// src/renderer/src/components/settings/ssh/ProvisionConfirmStep.tsx
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AlertTriangleIcon } from 'lucide-react'

export function ProvisionConfirmStep({
  serverIds,
  onBack,
  onConfirm,
}: {
  serverIds: string[]
  onBack: () => void
  onConfirm: () => void
}) {
  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  const selectedTargets = sshTargets.filter((t) => serverIds.includes(t.id))

  return (
    <div className="space-y-4">
      <Alert>
        <AlertTriangleIcon className="h-4 w-4" />
        <AlertDescription>
          {translate(
            'fleet.provision.confirmWarning',
            `This will deploy the Orca relay to ${serverIds.length} server(s). The relay binary will be uploaded via SFTP.`
          )}
        </AlertDescription>
      </Alert>

      <div className="rounded-md border p-3 space-y-1">
        {selectedTargets.map((t) => (
          <div key={t.id} className="flex items-center gap-2 text-sm">
            <span className="flex-1">{t.label}</span>
            <span className="text-xs text-muted-foreground">{t.host}</span>
          </div>
        ))}
      </div>

      <div className="flex justify-between">
        <Button variant="ghost" onClick={onBack}>
          {translate('common.back', 'Back')}
        </Button>
        <Button onClick={onConfirm}>
          {translate('fleet.provision.startProvision', `Provision ${serverIds.length} server(s)`)}
        </Button>
      </div>
    </div>
  )
}
```

## Bước 3: Tạo FleetProvisionWizard.tsx (shell)

```typescript
// src/renderer/src/components/settings/ssh/FleetProvisionWizard.tsx
import { useState } from 'react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { ProvisionServerSelector } from './ProvisionServerSelector'
import { ProvisionConfirmStep } from './ProvisionConfirmStep'
import { ProvisionProgressPanel } from './ProvisionProgressPanel'
import { ProvisionDoneSummary } from './ProvisionDoneSummary'

type WizardStep = 'select' | 'confirm' | 'provision' | 'done'

export function FleetProvisionWizard({
  open,
  onClose,
}: {
  open: boolean
  onClose: () => void
}) {
  const [step, setStep] = useState<WizardStep>('select')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const session = useAppStore((s) => s.provisioningSession)
  const startSession = useAppStore((s) => s.startProvisioningSession)
  const cancelSession = useAppStore((s) => s.cancelProvisioningSession)

  const handleStartProvision = async () => {
    const ids = Array.from(selectedIds)
    startSession(ids)
    setStep('provision')
    try {
      await window.api.ssh.provisionFleetServers({ serverIds: ids, concurrency: 3 })
    } catch (err) {
      toast.error(translate('fleet.provision.startError', 'Failed to start provisioning'))
      cancelSession()
      setStep('select')
    }
  }

  const handleClose = () => {
    setStep('select')
    setSelectedIds(new Set())
    cancelSession()
    onClose()
  }

  // Auto-advance to done when session finishes
  if (step === 'provision' && session?.phase === 'done') {
    setStep('done')
  }

  const titles: Record<WizardStep, string> = {
    select: translate('fleet.provision.stepSelect', 'Select Servers'),
    confirm: translate('fleet.provision.stepConfirm', 'Confirm'),
    provision: translate('fleet.provision.stepProvisioning', 'Provisioning...'),
    done: translate('fleet.provision.stepDone', 'Complete'),
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle>
            {translate('fleet.provision.title', 'Provision Fleet Servers')} — {titles[step]}
          </DialogTitle>
        </DialogHeader>

        {step === 'select' && (
          <>
            <ProvisionServerSelector
              selectedIds={selectedIds}
              onSelectionChange={setSelectedIds}
            />
            <div className="flex justify-end gap-2">
              <button onClick={handleClose} className="text-sm text-muted-foreground hover:text-foreground">
                {translate('common.cancel', 'Cancel')}
              </button>
              <button
                onClick={() => setStep('confirm')}
                disabled={selectedIds.size === 0}
                className="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground disabled:opacity-50"
              >
                {translate('common.next', 'Next')} ({selectedIds.size})
              </button>
            </div>
          </>
        )}

        {step === 'confirm' && (
          <ProvisionConfirmStep
            serverIds={Array.from(selectedIds)}
            onBack={() => setStep('select')}
            onConfirm={handleStartProvision}
          />
        )}

        {step === 'provision' && session && (
          <ProvisionProgressPanel session={session} />
        )}

        {step === 'done' && session && (
          <ProvisionDoneSummary session={session} onClose={handleClose} />
        )}
      </DialogContent>
    </Dialog>
  )
}
```

## Bước 4: Tạo ProvisionDoneSummary.tsx (placeholder)

```typescript
// src/renderer/src/components/settings/ssh/ProvisionDoneSummary.tsx
import { CheckCircle2Icon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { translate } from '@/i18n/i18n'
import type { ProvisioningSession } from '@/store/slices/provisioning'

export function ProvisionDoneSummary({
  session,
  onClose,
}: {
  session: ProvisioningSession
  onClose: () => void
}) {
  const done = session.servers.filter((s) => s.status === 'done').length
  const failed = session.servers.filter((s) => s.status === 'error').length

  return (
    <div className="space-y-4 text-center">
      <CheckCircle2Icon className="mx-auto h-12 w-12 text-green-500" />
      <div>
        <p className="text-lg font-semibold">
          {translate('fleet.provision.doneTitle', 'Provisioning Complete')}
        </p>
        <p className="text-sm text-muted-foreground">
          {done} servers ready
          {failed > 0 && `, ${failed} failed`}
        </p>
      </div>
      <Button onClick={onClose} className="w-full">
        {translate('common.close', 'Close')}
      </Button>
    </div>
  )
}
```

## Bước 5: Thêm button "Provision Fleet" vào SshSettingsPanel

```typescript
// Thêm state và button vào SshSettingsPanel.tsx:
const [provisionWizardOpen, setProvisionWizardOpen] = useState(false)

// Button:
<Button variant="outline" size="sm" onClick={() => setProvisionWizardOpen(true)}>
  {translate('fleet.provision.button', 'Provision Fleet')}
</Button>

// Dialog:
<FleetProvisionWizard
  open={provisionWizardOpen}
  onClose={() => setProvisionWizardOpen(false)}
/>
```

---

## Acceptance Criteria

- [x] Wizard mở từ "Provision Fleet" button
- [x] Step 1: Server list có checkboxes, select-all, grouped by project
- [x] Step 2: Confirm với warning alert + server list
- [x] "Provision" button → gọi `provisionFleetServers()` → chuyển step 3
- [x] Step 3: ProvisionProgressPanel (xem TASK-003-D)
- [x] Step 4: Done summary với count
- [x] Cancel ở mọi bước → cleanup session

---

## Implementation Notes

> **Completed:** 2026-07-23 | `FleetProvisionWizard.tsx`: multi-step modal (select→confirm→progress→done). Step 1 checkboxes+select-all+grouped by project. Step 2 confirm alert+list. Step 3 ProvisionProgressPanel. Step 4 DoneSummary. Cancel cleans session at all steps. TypeScript: ✅ 0 errors.
