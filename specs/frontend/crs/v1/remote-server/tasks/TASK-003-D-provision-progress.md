# TASK-003-D — Tạo ProvisionProgressPanel

**Task ID:** TASK-003-D  
**CR:** CR-003 — Bulk Server Provisioning  
**Solution Ref:** SOL-CR-003, Section 4.3  
**Dependencies:** TASK-003-A  
**Estimated:** 2 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `ProvisionProgressPanel` — real-time progress view hiển thị per-server status với status icons và overall progress bar.

---

## File cần tạo

`src/renderer/src/components/settings/ssh/ProvisionProgressPanel.tsx`

---

## Bước thực thi

### Bước 1: Tạo ProvisionProgressPanel.tsx

```typescript
// src/renderer/src/components/settings/ssh/ProvisionProgressPanel.tsx
import {
  CheckCircleIcon,
  XCircleIcon,
  LoaderIcon,
  ClockIcon,
  UploadCloudIcon,
  MinusCircleIcon,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Progress } from '@/components/ui/progress'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { translate } from '@/i18n/i18n'
import type {
  ProvisioningSession,
  ProvisioningServerEntry,
  ProvisioningServerStatus,
} from '@/store/slices/provisioning'

export function ProvisionProgressPanel({
  session,
}: {
  session: ProvisioningSession
}) {
  const finishedCount = session.servers.filter((s) =>
    ['done', 'error', 'skipped'].includes(s.status)
  ).length
  const doneCount = session.servers.filter((s) => s.status === 'done').length
  const failCount = session.servers.filter((s) => s.status === 'error').length
  const total = session.servers.length
  const progress = total > 0 ? Math.round((finishedCount / total) * 100) : 0

  return (
    <div className="space-y-4">
      {/* Overall progress */}
      <div className="space-y-2">
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {translate('fleet.provision.inProgress', 'Provisioning in progress...')}
          </span>
          <span className="font-medium tabular-nums">{progress}%</span>
        </div>
        <Progress value={progress} className="h-2" />
        <div className="flex gap-4 text-xs text-muted-foreground">
          <span className="text-green-600">✓ {doneCount} done</span>
          {failCount > 0 && (
            <span className="text-destructive">✗ {failCount} failed</span>
          )}
          <span>{total - finishedCount} remaining</span>
        </div>
      </div>

      {/* Per-server status list */}
      <ScrollArea className="max-h-[280px] pr-2">
        <div className="space-y-1">
          {session.servers.map((entry) => (
            <ProvisionServerStatusRow key={entry.serverId} entry={entry} />
          ))}
        </div>
      </ScrollArea>
    </div>
  )
}

// ─── Per-server row ────────────────────────────────────────────────────────────

type StatusConfig = {
  icon: React.ReactNode
  label: string
  textClass: string
}

function getStatusConfig(entry: ProvisioningServerEntry): StatusConfig {
  switch (entry.status) {
    case 'pending':
      return {
        icon: <ClockIcon className="h-4 w-4" />,
        label: translate('fleet.provision.status.pending', 'Waiting...'),
        textClass: 'text-muted-foreground',
      }
    case 'connecting':
      return {
        icon: <LoaderIcon className="h-4 w-4 animate-spin" />,
        label: translate('fleet.provision.status.connecting', 'Connecting...'),
        textClass: 'text-blue-500',
      }
    case 'deploying-relay':
      return {
        icon: <UploadCloudIcon className="h-4 w-4" />,
        label: translate('fleet.provision.status.deploying', 'Deploying relay...'),
        textClass: 'text-yellow-500',
      }
    case 'done':
      return {
        icon: <CheckCircleIcon className="h-4 w-4" />,
        label: translate('fleet.provision.status.done', 'Ready'),
        textClass: 'text-green-500',
      }
    case 'error':
      return {
        icon: <XCircleIcon className="h-4 w-4" />,
        label: entry.error ?? translate('fleet.provision.status.error', 'Error'),
        textClass: 'text-destructive',
      }
    case 'skipped':
      return {
        icon: <MinusCircleIcon className="h-4 w-4" />,
        label: translate('fleet.provision.status.skipped', 'Skipped'),
        textClass: 'text-muted-foreground',
      }
  }
}

function ProvisionServerStatusRow({ entry }: { entry: ProvisioningServerEntry }) {
  const config = getStatusConfig(entry)

  return (
    <div className="flex items-center gap-2.5 rounded px-1.5 py-1.5">
      {/* Status icon */}
      <span className={cn('flex-shrink-0', config.textClass)}>
        {config.icon}
      </span>

      {/* Server info */}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate">{entry.label}</p>
        <p className={cn('text-xs truncate', config.textClass)}>
          {config.label}
        </p>
      </div>

      {/* Relay version badge */}
      {entry.status === 'done' && entry.relayVersion && (
        <Badge variant="secondary" className="flex-shrink-0 text-xs font-mono">
          v{entry.relayVersion}
        </Badge>
      )}
    </div>
  )
}
```

### Bước 2: Verify

```bash
npx tsc --noEmit 2>&1 | grep "ProvisionProgress" | head -10
```

---

## Acceptance Criteria

- [x] Progress bar tính % dựa trên finished/total
- [x] Per-server rows: pending(clock) → connecting(spinner) → deploying(upload) → done(check)/error(x)
- [x] Error status hiển thị error message (truncated)
- [x] Done status hiển thị relay version badge
- [x] Stats line: "X done, Y failed, Z remaining"
- [x] Scroll khi list dài hơn max-height
- [x] TypeScript compile clean

---

## Notes cho AI

- `animate-spin` class cho LoaderIcon (Tailwind CSS)
- `tabular-nums` để count không nhảy khi số thay đổi
- `truncate` trên text để không overflow layout
- `flex-shrink-0` cho icons để không bị co lại

---

## Implementation Notes

> **Completed:** 2026-07-23 | `ProvisionProgressPanel.tsx`: progress bar (%), per-server rows (clock→spinner→upload→check/x icons), error message truncated, relay version badge on done, stats line (X done, Y failed, Z remaining), scroll on overflow. TypeScript: ✅ 0 errors.
