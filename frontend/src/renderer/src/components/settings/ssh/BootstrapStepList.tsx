// BootstrapStepList — visual step tracker with connector lines (CR-004, TASK-004-D)
import { CheckCircle2, XCircle, Loader, Circle, MinusCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { BootstrapStep, BootstrapStepStatus } from '@/store/slices/bootstrap'

function StepIcon({ status }: { status: BootstrapStepStatus }): React.JSX.Element {
  switch (status) {
    case 'running':
      return <Loader className="size-4 animate-spin text-blue-500" />
    case 'done':
      return <CheckCircle2 className="size-4 text-green-500" />
    case 'error':
      return <XCircle className="size-4 text-destructive" />
    case 'skipped':
      return <MinusCircle className="size-4 text-muted-foreground/50" />
    case 'pending':
    default:
      return <Circle className="size-4 text-muted-foreground/30" />
  }
}

export function BootstrapStepList({ steps }: { steps: BootstrapStep[] }): React.JSX.Element {
  return (
    <div className="space-y-0">
      {steps.map((step, index) => {
        const isLast = index === steps.length - 1

        return (
          <div key={step.id} className="flex items-start gap-3">
            {/* Icon + vertical connector line */}
            <div className="flex flex-col items-center">
              <StepIcon status={step.status} />
              {!isLast && (
                <div
                  className={cn(
                    'mt-1 min-h-[20px] w-px flex-1',
                    step.status === 'done' ? 'bg-green-500/30' : 'bg-border'
                  )}
                />
              )}
            </div>

            {/* Step label + detail/error */}
            <div className={cn('flex-1 pb-4', isLast && 'pb-0')}>
              <p
                className={cn('text-sm font-medium', {
                  'text-blue-600 dark:text-blue-400': step.status === 'running',
                  'text-green-600 dark:text-green-400': step.status === 'done',
                  'text-destructive': step.status === 'error',
                  'text-muted-foreground': step.status === 'skipped',
                  'text-foreground': step.status === 'pending'
                })}
              >
                {step.label}
              </p>
              {step.detail && (
                <p className="mt-0.5 text-xs text-muted-foreground">{step.detail}</p>
              )}
              {step.error && (
                <p className="mt-0.5 text-xs text-destructive">{step.error}</p>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
