# TASK-004-C — Tạo ServerBootstrapPanel + ServerBootstrapIdleScreen

**Task ID:** TASK-004-C  
**CR:** CR-004 — Dev Server Bootstrap Automation  
**Solution Ref:** SOL-CR-004, Section 4.1, 4.2  
**Dependencies:** TASK-004-A, TASK-004-B  
**Estimated:** 2–3 giờ  
**Status:** ✅ DONE

---

## Mục tiêu

Tạo `ServerBootstrapPanel` (main bootstrap UI) và `ServerBootstrapIdleScreen` (initial screen khi chưa bootstrap). Tích hợp vào SSH target detail panel dưới dạng tab "Bootstrap".

---

## Files cần tạo/sửa

| File | Action |
|------|--------|
| `src/renderer/src/components/settings/ssh/ServerBootstrapPanel.tsx` | CREATE |
| `src/renderer/src/components/settings/ssh/ServerBootstrapIdleScreen.tsx` | CREATE |
| SSH target detail panel | MODIFY — thêm Bootstrap tab |

---

## Bước 1: Tạo ServerBootstrapIdleScreen.tsx

```typescript
// src/renderer/src/components/settings/ssh/ServerBootstrapIdleScreen.tsx
import {
  ServerIcon,
  GitBranchIcon,
  KeyIcon,
  FolderGitIcon,
  PlayIcon,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AlertTriangleIcon } from 'lucide-react'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import type { SshTarget } from 'src/shared/ssh-types'

type BootstrapIdleProps = {
  target: SshTarget
  onStart: () => void
}

function BootstrapStepPreview({
  icon,
  label,
}: {
  icon: React.ReactNode
  label: string
}) {
  return (
    <li className="flex items-center gap-2 text-sm text-muted-foreground">
      <span className="flex-shrink-0">{icon}</span>
      {label}
    </li>
  )
}

export function ServerBootstrapIdleScreen({ target, onStart }: BootstrapIdleProps) {
  const connectionState = useAppStore(
    (s) => s.sshConnectionStates[target.id]
  )
  const isConnected = connectionState?.status === 'connected'

  const repoCount = target.repos?.length ?? 0

  return (
    <div className="space-y-4">
      {/* Preview steps */}
      <div className="rounded-md border bg-muted/30 p-4">
        <h4 className="mb-3 text-sm font-medium">
          {translate('fleet.bootstrap.whatItDoes', 'Bootstrap will:')}
        </h4>
        <ul className="space-y-2">
          <BootstrapStepPreview
            icon={<ServerIcon className="h-4 w-4" />}
            label={translate('fleet.bootstrap.stepNode', 'Install Node.js 22+')}
          />
          <BootstrapStepPreview
            icon={<GitBranchIcon className="h-4 w-4" />}
            label={translate('fleet.bootstrap.stepGit', 'Install Git 2.35+')}
          />
          <BootstrapStepPreview
            icon={<KeyIcon className="h-4 w-4" />}
            label={translate('fleet.bootstrap.stepSshKey', 'Setup SSH key for git')}
          />
          <BootstrapStepPreview
            icon={<FolderGitIcon className="h-4 w-4" />}
            label={
              repoCount > 0
                ? translate(
                    'fleet.bootstrap.stepReposCount',
                    `Clone ${repoCount} repo(s) from fleet config`
                  )
                : translate(
                    'fleet.bootstrap.stepRepos',
                    'Clone project repos'
                  )
            }
          />
          <BootstrapStepPreview
            icon={<PlayIcon className="h-4 w-4" />}
            label={translate(
              'fleet.bootstrap.stepSetup',
              'Run orca.yaml setup scripts'
            )}
          />
        </ul>
      </div>

      {/* Warning if not connected */}
      {!isConnected && (
        <Alert>
          <AlertTriangleIcon className="h-4 w-4" />
          <AlertDescription>
            {translate(
              'fleet.bootstrap.connectFirst',
              'Connect to this server before running bootstrap.'
            )}
          </AlertDescription>
        </Alert>
      )}

      {/* Start button */}
      <Button
        onClick={onStart}
        disabled={!isConnected}
        className="w-full"
        size="default"
      >
        {translate('fleet.bootstrap.start', 'Start Bootstrap')}
      </Button>
    </div>
  )
}
```

## Bước 2: Tạo ServerBootstrapPanel.tsx

```typescript
// src/renderer/src/components/settings/ssh/ServerBootstrapPanel.tsx
import { useState } from 'react'
import { toast } from 'sonner'
import { TerminalIcon, ChevronDownIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import type { SshTarget } from 'src/shared/ssh-types'
import { ServerBootstrapIdleScreen } from './ServerBootstrapIdleScreen'
import { BootstrapStepList } from './BootstrapStepList'
import { BootstrapLogViewer } from './BootstrapLogViewer'

export function ServerBootstrapPanel({ target }: { target: SshTarget }) {
  const bootstrapState = useAppStore(
    (s) => s.bootstrapByServer[target.id]
  )
  const [showLog, setShowLog] = useState(false)

  const handleStartBootstrap = async () => {
    try {
      await window.api.ssh.bootstrapServer({
        serverId: target.id,
        options: { installNode: true, installGit: true, cloneRepos: true },
      })
    } catch (err) {
      toast.error(
        translate('fleet.bootstrap.startError', 'Failed to start bootstrap')
      )
    }
  }

  const handleRetry = () => {
    useAppStore.getState().clearBootstrap(target.id)
    handleStartBootstrap()
  }

  // No state or idle → show idle screen
  if (!bootstrapState || bootstrapState.phase === 'idle') {
    return (
      <ServerBootstrapIdleScreen target={target} onStart={handleStartBootstrap} />
    )
  }

  return (
    <div className="space-y-4">
      {/* Steps tracker */}
      <BootstrapStepList steps={bootstrapState.steps} />

      {/* Phase indicator */}
      {bootstrapState.phase === 'running' && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span className="inline-block h-2 w-2 rounded-full bg-blue-500 animate-pulse" />
          {translate('fleet.bootstrap.running', 'Bootstrap in progress...')}
        </div>
      )}

      {bootstrapState.phase === 'done' && (
        <div className="flex items-center gap-2 text-sm text-green-600">
          <span>✅</span>
          {translate('fleet.bootstrap.complete', 'Bootstrap complete! Server is ready.')}
        </div>
      )}

      {bootstrapState.phase === 'error' && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3 space-y-2">
          <p className="text-sm text-destructive">
            {translate('fleet.bootstrap.failed', 'Bootstrap failed. Check log below.')}
          </p>
          <Button variant="outline" size="sm" onClick={handleRetry}>
            {translate('fleet.bootstrap.retry', 'Retry Bootstrap')}
          </Button>
        </div>
      )}

      {/* Log toggle */}
      <Collapsible open={showLog} onOpenChange={setShowLog}>
        <CollapsibleTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="gap-1.5 text-muted-foreground"
          >
            <TerminalIcon className="h-4 w-4" />
            {translate('fleet.bootstrap.showLog', 'Bootstrap log')}
            <ChevronDownIcon
              className={cn(
                'h-4 w-4 transition-transform',
                showLog && 'rotate-180'
              )}
            />
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <BootstrapLogViewer lines={bootstrapState.logLines} />
        </CollapsibleContent>
      </Collapsible>
    </div>
  )
}
```

## Bước 3: Thêm Bootstrap tab vào SSH target detail

```bash
# Tìm SSH target detail panel:
find src/renderer/src/components -name "*Detail*" -o -name "*detail*" | grep -i "ssh\|target" | head -5
```

Thêm tab "Bootstrap":

```typescript
// Import thêm:
import { ServerBootstrapPanel } from './ServerBootstrapPanel'

// Trong tabs:
<TabsTrigger value="bootstrap">
  {translate('ssh.detail.bootstrap', 'Bootstrap')}
</TabsTrigger>

<TabsContent value="bootstrap">
  <ServerBootstrapPanel target={target} />
</TabsContent>
```

## Bước 4: Verify

```bash
npx tsc --noEmit 2>&1 | grep "Bootstrap\|bootstrap" | head -10
```

---

## Acceptance Criteria

- [x] Idle screen hiển thị 5 bootstrap steps preview
- [x] "Start Bootstrap" disabled khi server chưa connected
- [x] Khi bootstrap chạy: step list hiển thị, log toggle xuất hiện
- [x] `phase === 'done'` → success message
- [x] `phase === 'error'` → error box + Retry button
- [x] Log collapsible, không hiện mặc định
- [x] SSH detail panel có tab "Bootstrap"

---

## Implementation Notes

> **Completed:** 2026-07-23 | `ServerBootstrapPanel.tsx`: orchestrates idle/running/done/error states. `ServerBootstrapIdleScreen.tsx`: 5-step preview list. Start Bootstrap disabled when not connected. Log collapsible (hidden by default). Error shows Retry button. SshPane integration. TypeScript: ✅ 0 errors.
