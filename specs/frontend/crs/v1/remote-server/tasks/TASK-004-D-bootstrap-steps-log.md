# TASK-004-D — Tạo BootstrapStepList + BootstrapLogViewer

**Task ID:** TASK-004-D  
**CR:** CR-004 — Dev Server Bootstrap Automation  
**Solution Ref:** SOL-CR-004, Section 4.3, 4.4  
**Dependencies:** TASK-004-A  
**Estimated:** 1–2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo 2 sub-components:
1. `BootstrapStepList` — visual step tracker với connector lines
2. `BootstrapLogViewer` — scrollable log viewer với auto-scroll

---

## Files cần tạo

| File |
|------|
| `src/renderer/src/components/settings/ssh/BootstrapStepList.tsx` |
| `src/renderer/src/components/settings/ssh/BootstrapLogViewer.tsx` |

---

## Bước 1: Tạo BootstrapStepList.tsx

```typescript
// src/renderer/src/components/settings/ssh/BootstrapStepList.tsx
import {
  CheckCircle2Icon,
  XCircleIcon,
  LoaderIcon,
  CircleIcon,
  MinusCircleIcon,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import type { BootstrapStep, BootstrapStepStatus } from '@/store/slices/bootstrap'

type StepIconProps = { status: BootstrapStepStatus }

function StepIcon({ status }: StepIconProps) {
  switch (status) {
    case 'running':
      return <LoaderIcon className="h-4 w-4 animate-spin text-blue-500" />
    case 'done':
      return <CheckCircle2Icon className="h-4 w-4 text-green-500" />
    case 'error':
      return <XCircleIcon className="h-4 w-4 text-destructive" />
    case 'skipped':
      return <MinusCircleIcon className="h-4 w-4 text-muted-foreground/50" />
    case 'pending':
    default:
      return <CircleIcon className="h-4 w-4 text-muted-foreground/30" />
  }
}

export function BootstrapStepList({ steps }: { steps: BootstrapStep[] }) {
  return (
    <div className="space-y-0">
      {steps.map((step, index) => {
        const isLast = index === steps.length - 1

        return (
          <div key={step.id} className="flex items-start gap-3">
            {/* Icon + connector line */}
            <div className="flex flex-col items-center">
              <StepIcon status={step.status} />
              {!isLast && (
                <div
                  className={cn(
                    'mt-1 w-px flex-1 min-h-[20px]',
                    step.status === 'done'
                      ? 'bg-green-500/30'
                      : 'bg-border'
                  )}
                />
              )}
            </div>

            {/* Step content */}
            <div className={cn('flex-1 pb-4', isLast && 'pb-0')}>
              <p
                className={cn('text-sm font-medium', {
                  'text-blue-600 dark:text-blue-400': step.status === 'running',
                  'text-green-600 dark:text-green-400': step.status === 'done',
                  'text-destructive': step.status === 'error',
                  'text-muted-foreground': step.status === 'skipped',
                  'text-foreground': step.status === 'pending',
                })}
              >
                {step.label}
              </p>
              {step.detail && (
                <p className="text-xs text-muted-foreground mt-0.5">
                  {step.detail}
                </p>
              )}
              {step.error && (
                <p className="text-xs text-destructive mt-0.5">
                  {step.error}
                </p>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
```

## Bước 2: Tạo BootstrapLogViewer.tsx

```typescript
// src/renderer/src/components/settings/ssh/BootstrapLogViewer.tsx
import { useEffect, useRef } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'
import { translate } from '@/i18n/i18n'

export function BootstrapLogViewer({ lines }: { lines: string[] }) {
  const bottomRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom when new lines arrive
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines.length])

  return (
    <div className="mt-2 rounded-md border bg-[hsl(var(--background))] font-mono text-xs">
      <div className="flex items-center justify-between border-b px-3 py-1.5">
        <span className="text-muted-foreground text-xs">
          {translate('fleet.bootstrap.logTitle', 'Bootstrap output')}
        </span>
        <span className="text-muted-foreground/50 text-xs tabular-nums">
          {lines.length} lines
        </span>
      </div>
      <ScrollArea className="h-[200px]">
        <div className="p-3 space-y-0.5">
          {lines.length === 0 ? (
            <p className="text-muted-foreground/50 italic">
              {translate('fleet.bootstrap.logEmpty', 'Waiting for output...')}
            </p>
          ) : (
            lines.map((line, i) => (
              <div
                key={i}
                className="text-muted-foreground whitespace-pre-wrap leading-5 break-all"
              >
                {line}
              </div>
            ))
          )}
          <div ref={bottomRef} />
        </div>
      </ScrollArea>
    </div>
  )
}
```

## Bước 3: Verify

```bash
npx tsc --noEmit 2>&1 | grep "BootstrapStep\|BootstrapLog" | head -10
```

---

## Acceptance Criteria

**BootstrapStepList:**
- [x] 5 steps hiển thị theo thứ tự với icons đúng per status
- [x] Connector lines giữa steps (không có sau step cuối)
- [x] Colors: running=blue, done=green, error=red, skipped=gray, pending=faded
- [x] `detail` hiển thị dưới label (nhỏ hơn, muted)
- [x] `error` hiển thị màu đỏ dưới label

**BootstrapLogViewer:**
- [x] Scrollable area với max height
- [x] Auto-scroll to bottom khi có lines mới
- [x] Line count hiển thị ở header
- [x] Empty state khi chưa có lines
- [x] Monospace font
- [x] `whitespace-pre-wrap` để preserve indentation

---

## Implementation Notes

> **Completed:** 2026-07-23 | `BootstrapStepList.tsx`: 5 steps, connector lines between (not after last), blue=running/green=done/red=error/gray=skipped/faded=pending, detail+error under label. `BootstrapLogViewer.tsx`: scrollable max-h, auto-scroll useEffect, line count in header, empty state, monospace, whitespace-pre-wrap. TypeScript: ✅ 0 errors.
