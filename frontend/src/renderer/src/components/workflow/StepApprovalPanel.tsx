import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '../ui/button'
import { Textarea } from '../ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '../ui/dialog'
import { useStepApprovals, type StepApproval } from '../../hooks/useStepApprovals'

function gateLabel(a: StepApproval): string {
  const gate = a.subject?.gate ? `Gate ${a.subject.gate}` : a.stepId
  return a.subject?.role ? `${gate} · ${a.subject.role}` : gate
}

/** Decisions this execution's steps wait for; the reviewer sees the exact decision before approving it. */
export function StepApprovalPanel({
  executionId,
  active
}: {
  executionId: string
  active: boolean
}) {
  const { approvals, load, resolve } = useStepApprovals(executionId, active)
  const [open, setOpen] = useState<StepApproval | null>(null)
  const [rationale, setRationale] = useState('')
  const [busy, setBusy] = useState(false)

  if (approvals.length === 0) {
    return null
  }

  const review = async (a: StepApproval) => {
    try {
      setRationale('')
      setOpen(await load(a.id))
    } catch (err) {
      toast.error(
        `Could not load the decision: ${err instanceof Error ? err.message : String(err)}`
      )
    }
  }

  const decide = async (decision: 'approved' | 'rejected') => {
    if (!open) {
      return
    }
    setBusy(true)
    try {
      await resolve(open, decision, rationale.trim())
      toast.success(decision === 'approved' ? 'Approved' : 'Rejected')
      setOpen(null)
    } catch (err) {
      // STEP_APPROVAL_STALE: the decision changed while it was open; the reviewer must read the new one.
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-2" data-testid="step-approvals">
      <div className="text-xs font-medium text-muted-foreground">Waiting for approval</div>
      {approvals.map((a) => (
        <div
          key={a.id}
          className="flex items-center justify-between rounded border p-2"
          data-testid={`step-approval-${a.id}`}
        >
          <div className="text-sm">
            <span className="font-medium">{gateLabel(a)}</span>
            <span className="ml-2 text-xs text-muted-foreground">step {a.stepId}</span>
          </div>
          <Button
            size="sm"
            variant="outline"
            onClick={() => void review(a)}
            data-testid="step-approval-review"
          >
            Review
          </Button>
        </div>
      ))}

      <Dialog open={open !== null} onOpenChange={(v) => !v && setOpen(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{open ? gateLabel(open) : ''}</DialogTitle>
            <DialogDescription>
              Approving applies to exactly this decision. If it changes, you will be asked to read
              it again.
            </DialogDescription>
          </DialogHeader>
          <pre
            className="max-h-80 overflow-auto rounded bg-muted/50 p-2 text-xs font-mono"
            data-testid="step-approval-decision"
          >
            {JSON.stringify(open?.decision ?? null, null, 2)}
          </pre>
          <Textarea
            placeholder="Rationale (required)"
            value={rationale}
            onChange={(e) => setRationale(e.target.value)}
            data-testid="step-approval-rationale"
          />
          {open && !open.canDecide && (
            <div className="text-xs text-muted-foreground" data-testid="step-approval-readonly">
              Your project role cannot decide this approval.
            </div>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              disabled={busy || !open?.canDecide || !rationale.trim()}
              onClick={() => void decide('rejected')}
              data-testid="step-approval-reject"
            >
              Reject
            </Button>
            <Button
              disabled={busy || !open?.canDecide || !rationale.trim()}
              onClick={() => void decide('approved')}
              data-testid="step-approval-approve"
            >
              Approve
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
