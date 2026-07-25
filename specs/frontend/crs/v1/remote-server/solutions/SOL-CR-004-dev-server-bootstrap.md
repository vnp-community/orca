# SOL-CR-004 — Frontend Solution: Dev Server Bootstrap Automation

**CR:** CR-004 — Dev Server Bootstrap Automation  
**Priority:** 🟠 High  
**TDD References:** TDD-FE-02 (State Management), TDD-FE-05 (UI Components), TDD-FE-07 (Hooks & IPC)  
**Depends on:** SOL-CR-001 (FleetConfig), SOL-CR-003 (Provisioning patterns)  
**Estimated effort:** 2–3 ngày frontend  
**Implementation Status:** ✅ IMPLEMENTED — 2026-07-23  
**Tasks:** TASK-004-A (BootstrapSlice), TASK-004-B (IPC/preload), TASK-004-C (ServerBootstrapPanel), TASK-004-D (BootstrapStepsLog)

---

## 1. Tổng quan giải pháp

CR-004 yêu cầu **Bootstrap Automation** cho dev servers — cài Node.js, Git, clone repos. Frontend cần:

1. Thêm **BootstrapSlice** để track bootstrap state per-server
2. Tạo **ServerBootstrapPanel** — step-by-step bootstrap UI
3. Tích hợp vào **SSH target detail panel**
4. Handle **streaming bootstrap log** từ backend

---

## 2. Zustand Slice — `bootstrapSlice`

```typescript
// src/renderer/src/store/slices/bootstrap.ts
// [NEW FILE]

export type BootstrapStepStatus = 'pending' | 'running' | 'done' | 'error' | 'skipped'

export type BootstrapStep = {
  id: string
  label: string
  status: BootstrapStepStatus
  detail: string | null   // e.g. "Node.js v22.3.0 already installed"
  error: string | null
}

export type ServerBootstrapState = {
  serverId: string
  phase: 'idle' | 'running' | 'done' | 'error'
  steps: BootstrapStep[]
  logLines: string[]      // streaming log output
  startedAt: number | null
  completedAt: number | null
}

export type BootstrapSlice = {
  bootstrapByServer: Record<string, ServerBootstrapState>
  initBootstrap: (serverId: string) => void
  updateBootstrapStep: (serverId: string, stepId: string, update: Partial<BootstrapStep>) => void
  appendBootstrapLog: (serverId: string, line: string) => void
  finishBootstrap: (serverId: string, success: boolean) => void
  clearBootstrap: (serverId: string) => void
}

export const createBootstrapSlice: StateCreator<AppState, [], [], BootstrapSlice> = (set) => ({
  bootstrapByServer: {},

  initBootstrap: (serverId) =>
    set(s => {
      s.bootstrapByServer[serverId] = {
        serverId,
        phase: 'running',
        startedAt: Date.now(),
        completedAt: null,
        logLines: [],
        steps: [
          { id: 'node', label: 'Node.js 22+', status: 'pending', detail: null, error: null },
          { id: 'git', label: 'Git 2.35+', status: 'pending', detail: null, error: null },
          { id: 'ssh-key', label: 'SSH key setup', status: 'pending', detail: null, error: null },
          { id: 'repos', label: 'Clone/update repos', status: 'pending', detail: null, error: null },
          { id: 'setup-script', label: 'Run setup scripts', status: 'pending', detail: null, error: null },
        ],
      }
    }),

  updateBootstrapStep: (serverId, stepId, update) =>
    set(s => {
      const state = s.bootstrapByServer[serverId]
      if (!state) return
      const step = state.steps.find(st => st.id === stepId)
      if (step) Object.assign(step, update)
    }),

  appendBootstrapLog: (serverId, line) =>
    set(s => {
      const state = s.bootstrapByServer[serverId]
      if (!state) return
      // Cap log at 500 lines
      if (state.logLines.length >= 500) state.logLines.shift()
      state.logLines.push(line)
    }),

  finishBootstrap: (serverId, success) =>
    set(s => {
      const state = s.bootstrapByServer[serverId]
      if (!state) return
      state.phase = success ? 'done' : 'error'
      state.completedAt = Date.now()
    }),

  clearBootstrap: (serverId) =>
    set(s => { delete s.bootstrapByServer[serverId] }),
})
```

---

## 3. IPC Events

```typescript
// src/renderer/src/hooks/useIpcEvents.ts

// [NEW] Bootstrap events
window.api.ssh.onBootstrapProgress?.((event) => {
  const store = useAppStore.getState()

  switch (event.type) {
    case 'bootstrap.started':
      store.initBootstrap(event.serverId)
      break

    case 'bootstrap.step.started':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'running',
      })
      break

    case 'bootstrap.step.done':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'done',
        detail: event.detail,
      })
      break

    case 'bootstrap.step.error':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'error',
        error: event.error,
      })
      break

    case 'bootstrap.step.skipped':
      store.updateBootstrapStep(event.serverId, event.stepId, {
        status: 'skipped',
        detail: event.reason,
      })
      break

    case 'bootstrap.log':
      store.appendBootstrapLog(event.serverId, event.line)
      break

    case 'bootstrap.done':
      store.finishBootstrap(event.serverId, true)
      toast.success(
        translate('fleet.bootstrap.done', `Bootstrap complete: ${event.serverLabel}`)
      )
      scheduleRuntimeGraphSync()
      break

    case 'bootstrap.error':
      store.finishBootstrap(event.serverId, false)
      toast.error(
        translate('fleet.bootstrap.error', `Bootstrap failed: ${event.serverLabel}`)
      )
      break
  }
})
```

---

## 4. UI Components

### 4.1 `ServerBootstrapPanel` — Main bootstrap UI

```typescript
// src/renderer/src/components/settings/ssh/ServerBootstrapPanel.tsx

export function ServerBootstrapPanel({
  target,
}: {
  target: SshTarget
}) {
  const bootstrapState = useAppStore(
    s => s.bootstrapByServer[target.id]
  )
  const [showLog, setShowLog] = useState(false)

  const handleStartBootstrap = async () => {
    try {
      await window.api.ssh.bootstrapServer({
        serverId: target.id,
        options: { installNode: true, installGit: true, cloneRepos: true },
      })
    } catch (err) {
      toast.error(translate('fleet.bootstrap.startError', 'Failed to start bootstrap'))
    }
  }

  // Không có state → idle screen
  if (!bootstrapState || bootstrapState.phase === 'idle') {
    return (
      <ServerBootstrapIdleScreen
        target={target}
        onStart={handleStartBootstrap}
      />
    )
  }

  return (
    <div className="space-y-4">
      {/* Steps */}
      <BootstrapStepList steps={bootstrapState.steps} />

      {/* Phase indicator */}
      {bootstrapState.phase === 'running' && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2Icon className="h-4 w-4 animate-spin" />
          {translate('fleet.bootstrap.running', 'Bootstrap in progress...')}
        </div>
      )}

      {bootstrapState.phase === 'done' && (
        <div className="flex items-center gap-2 text-sm text-green-600">
          <CheckCircle2Icon className="h-4 w-4" />
          {translate('fleet.bootstrap.complete', 'Bootstrap complete! Server is ready.')}
        </div>
      )}

      {bootstrapState.phase === 'error' && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">
            {translate('fleet.bootstrap.failed', 'Bootstrap failed. Check log below.')}
          </p>
          <Button
            variant="outline"
            size="sm"
            className="mt-2"
            onClick={handleStartBootstrap}
          >
            {translate('fleet.bootstrap.retry', 'Retry')}
          </Button>
        </div>
      )}

      {/* Log toggle */}
      <Collapsible open={showLog} onOpenChange={setShowLog}>
        <CollapsibleTrigger asChild>
          <Button variant="ghost" size="sm" className="gap-1">
            <TerminalIcon className="h-4 w-4" />
            {translate('fleet.bootstrap.showLog', 'Bootstrap log')}
            <ChevronDownIcon
              className={cn('h-4 w-4 transition-transform', showLog && 'rotate-180')}
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

### 4.2 `ServerBootstrapIdleScreen`

```typescript
// src/renderer/src/components/settings/ssh/ServerBootstrapIdleScreen.tsx

export function ServerBootstrapIdleScreen({
  target,
  onStart,
}: {
  target: SshTarget
  onStart: () => void
}) {
  const connectionState = useAppStore(s => s.sshConnectionStates[target.id])
  const isConnected = connectionState?.status === 'connected'

  return (
    <div className="space-y-4">
      <div className="rounded-md border bg-muted/30 p-4">
        <h4 className="text-sm font-medium mb-2">
          {translate('fleet.bootstrap.whatItDoes', 'Bootstrap will:')}
        </h4>
        <ul className="space-y-1.5 text-sm text-muted-foreground">
          <BootstrapStepPreview icon={<NodeIcon />} label="Install Node.js 22+" />
          <BootstrapStepPreview icon={<GitIcon />} label="Install Git 2.35+" />
          <BootstrapStepPreview icon={<KeyIcon />} label="Setup SSH key for git operations" />
          <BootstrapStepPreview icon={<GitCloneIcon />}
            label={
              target.repos && target.repos.length > 0
                ? `Clone ${target.repos.length} repo(s)`
                : 'Clone project repos (from fleet config)'
            }
          />
          <BootstrapStepPreview icon={<PlayIcon />} label="Run orca.yaml setup scripts" />
        </ul>
      </div>

      {!isConnected && (
        <Alert>
          <AlertTriangleIcon className="h-4 w-4" />
          <AlertDescription>
            {translate(
              'fleet.bootstrap.connectFirst',
              'Server must be connected before bootstrapping.'
            )}
          </AlertDescription>
        </Alert>
      )}

      <Button
        onClick={onStart}
        disabled={!isConnected}
        className="w-full"
      >
        {translate('fleet.bootstrap.start', 'Start Bootstrap')}
      </Button>
    </div>
  )
}
```

### 4.3 `BootstrapStepList` — Visual step tracker

```typescript
// src/renderer/src/components/settings/ssh/BootstrapStepList.tsx

export function BootstrapStepList({ steps }: { steps: BootstrapStep[] }) {
  return (
    <div className="space-y-2">
      {steps.map((step, index) => (
        <div key={step.id} className="flex items-start gap-3">
          {/* Step connector line */}
          <div className="flex flex-col items-center">
            <BootstrapStepIcon status={step.status} />
            {index < steps.length - 1 && (
              <div className={cn(
                'mt-1 w-px flex-1 min-h-[16px]',
                step.status === 'done' ? 'bg-green-500/50' : 'bg-border'
              )} />
            )}
          </div>

          {/* Step content */}
          <div className="flex-1 pb-3">
            <div className="flex items-center gap-2">
              <span className={cn(
                'text-sm font-medium',
                step.status === 'running' && 'text-blue-600',
                step.status === 'done' && 'text-green-600',
                step.status === 'error' && 'text-destructive',
                step.status === 'skipped' && 'text-muted-foreground',
              )}>
                {step.label}
              </span>
              {step.status === 'running' && (
                <Loader2Icon className="h-3 w-3 animate-spin text-blue-500" />
              )}
            </div>
            {step.detail && (
              <p className="text-xs text-muted-foreground mt-0.5">{step.detail}</p>
            )}
            {step.error && (
              <p className="text-xs text-destructive mt-0.5">{step.error}</p>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
```

### 4.4 `BootstrapLogViewer` — Streaming log

```typescript
// src/renderer/src/components/settings/ssh/BootstrapLogViewer.tsx
// Tương tự terminal log, nhưng readonly

export function BootstrapLogViewer({ lines }: { lines: string[] }) {
  const bottomRef = useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [lines.length])

  return (
    <div className="mt-2 max-h-[200px] overflow-y-auto rounded-md bg-background border font-mono text-xs p-2">
      {lines.map((line, i) => (
        <div key={i} className="text-muted-foreground whitespace-pre-wrap leading-5">
          {line}
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  )
}
```

### 4.5 Tích hợp vào SSH target detail

```typescript
// src/renderer/src/components/settings/ssh/SshTargetDetailPanel.tsx
// Thêm Bootstrap tab

export function SshTargetDetailPanel({ target }: { target: SshTarget }) {
  return (
    <Tabs defaultValue="info">
      <TabsList>
        <TabsTrigger value="info">
          {translate('ssh.detail.info', 'Info')}
        </TabsTrigger>
        <TabsTrigger value="port-forwards">
          {translate('ssh.detail.portForwards', 'Port Forwards')}
        </TabsTrigger>
        {/* [NEW] Bootstrap tab */}
        <TabsTrigger value="bootstrap">
          {translate('ssh.detail.bootstrap', 'Bootstrap')}
        </TabsTrigger>
      </TabsList>

      <TabsContent value="info">
        <SshTargetInfoPanel target={target} />
      </TabsContent>

      <TabsContent value="port-forwards">
        <SshPortForwardList targetId={target.id} />
      </TabsContent>

      {/* [NEW] */}
      <TabsContent value="bootstrap">
        <ServerBootstrapPanel target={target} />
      </TabsContent>
    </Tabs>
  )
}
```

---

## 5. File mới cần tạo

| File | Loại | Mô tả |
|------|------|-------|
| `src/renderer/src/store/slices/bootstrap.ts` | [NEW] | Bootstrap state per-server |
| `src/renderer/src/components/settings/ssh/ServerBootstrapPanel.tsx` | [NEW] | Main bootstrap UI |
| `src/renderer/src/components/settings/ssh/ServerBootstrapIdleScreen.tsx` | [NEW] | Idle/start screen |
| `src/renderer/src/components/settings/ssh/BootstrapStepList.tsx` | [NEW] | Step-by-step tracker |
| `src/renderer/src/components/settings/ssh/BootstrapLogViewer.tsx` | [NEW] | Streaming log |

## 6. File cần chỉnh sửa

| File | Thay đổi |
|------|---------|
| `src/renderer/src/store/types.ts` | Thêm `BootstrapSlice` |
| `src/renderer/src/store/index.ts` | Register `createBootstrapSlice` |
| `src/renderer/src/hooks/useIpcEvents.ts` | Thêm `onBootstrapProgress` handlers |
| `src/renderer/src/components/settings/ssh/SshTargetDetailPanel.tsx` | Thêm "Bootstrap" tab |
| `src/preload/index.ts` | Expose `ssh.bootstrapServer`, `ssh.cancelBootstrap` |

---

## 7. Acceptance Criteria (Frontend)

- [x] SSH target detail panel có tab "Bootstrap" (expandable section trong `SshPane`)
- [x] Idle screen mô tả rõ các bước sẽ thực hiện
- [x] "Start Bootstrap" disabled nếu server chưa connected
- [x] Khi chạy: step list cập nhật real-time (pending → running → done/error/skipped)
- [x] Log viewer hiển thị output raw từ bootstrap script, auto-scroll
- [x] Retry button nếu bootstrap thất bại
- [x] Toast notification khi bootstrap hoàn tất
- [x] Bootstrap state persist trong session (không mất khi navigate)

## 8. Implementation Notes

> **Implemented 2026-07-23**
>
> - `src/renderer/src/store/slices/bootstrap.ts`: [NEW] `BootstrapSlice` — per-server `BootstrapSession` with step list, log lines, phase tracking.
> - `src/renderer/src/store/slices/bootstrap-events.ts`: [NEW] `BootstrapProgressEvent` discriminated union (6 event types + `default` fallback).
> - `src/renderer/src/store/types.ts`: Registered `BootstrapSlice`.
> - `src/renderer/src/store/index.ts`: Registered `createBootstrapSlice`.
> - `src/preload/api-types.ts`: Added `bootstrapServer`, `cancelBootstrap`, `onBootstrapProgress` to `ssh` namespace.
> - `src/preload/index.ts`: IPC bridges with `ipcRenderer.invoke` and listener pattern.
> - `src/renderer/src/web/web-preload-api.ts`: No-op stubs for web mode.
> - `src/renderer/src/hooks/useIpcEvents.ts`: `onBootstrapProgress` handler with all 6 event types + `default: break`.
> - `src/renderer/src/components/settings/ssh/BootstrapStepList.tsx`: [NEW] Step list with status icons.
> - `src/renderer/src/components/settings/ssh/BootstrapLogViewer.tsx`: [NEW] Virtualized log viewer with auto-scroll.
> - `src/renderer/src/components/settings/ssh/ServerBootstrapIdleScreen.tsx`: [NEW] Start screen with step descriptions.
> - `src/renderer/src/components/settings/ssh/ServerBootstrapPanel.tsx`: [NEW] Main panel orchestrating idle/running/done/error states.
> - `src/renderer/src/components/settings/SshPane.tsx`: Bootstrap expandable section per SSH target.
> - **TypeScript:** ✅ 0 new errors.
