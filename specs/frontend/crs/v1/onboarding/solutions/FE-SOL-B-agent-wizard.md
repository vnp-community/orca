# FE-SOL-B: Agent Detection & Platform-Aware Wizard

**CRs:** [CR-OB-003](../../../../../docs/crs/v1/onboarding/CR-OB-003-agent-detection-remote.md) | [CR-OB-004](../../../../../docs/crs/v1/onboarding/CR-OB-004-platform-aware-wizard.md)  
**TDD refs:** TDD-FE-02 (State), TDD-FE-05 (Components), TDD-FE-07 (Hooks)  
**Status:** ✅ COMPLETED (2026-07-23) | **Phase:** 1–2

---

## 1. New Files

```
src/renderer/src/
├── components/onboarding/
│   └── AgentStep.tsx                   ← MODIFY: thêm devServer info + remote detection
├── hooks/
│   ├── useRemoteAgentDetection.ts      ← NEW: detect agents trên dev server
│   └── useActiveDevServerPlatform.ts   ← NEW: reactive platform selector
└── store/slices/
    └── onboarding.ts                   ← MODIFY: thêm agentDetectionByServer
```

---

## 2. Hook — `useRemoteAgentDetection.ts`

```typescript
// src/renderer/src/hooks/useRemoteAgentDetection.ts
import { useState, useEffect, useCallback } from 'react'
import type { DevServer } from '../../../../shared/dev-server-types'

type DetectionState = {
  agents: string[]
  platform: NodeJS.Platform | null
  loading: boolean
  error: string | null
  lastDetectedAt: number | null
}

const DEFAULT_STATE: DetectionState = {
  agents: [],
  platform: null,
  loading: false,
  error: null,
  lastDetectedAt: null
}

// Per-server cache (module-level, không phải React state — survives re-renders):
const detectionCache = new Map<string, DetectionState>()

export function useRemoteAgentDetection(devServerId: string | null): DetectionState & {
  redetect: () => Promise<void>
} {
  const cached = devServerId ? detectionCache.get(devServerId) : undefined
  const [state, setState] = useState<DetectionState>(cached ?? DEFAULT_STATE)

  const detect = useCallback(async () => {
    if (!devServerId) {
      setState(DEFAULT_STATE)
      return
    }
    setState(prev => ({ ...prev, loading: true, error: null }))
    try {
      const result = await window.api.onboarding.detectAgents({ devServerId })
      const next: DetectionState = {
        agents: result.agents,
        platform: result.platform,
        loading: false,
        error: null,
        lastDetectedAt: Date.now()
      }
      detectionCache.set(devServerId, next)
      setState(next)
    } catch (err) {
      setState(prev => ({
        ...prev,
        loading: false,
        error: (err as Error).message
      }))
    }
  }, [devServerId])

  useEffect(() => {
    // Sử dụng cache nếu còn mới (< 60s)
    if (cached && cached.lastDetectedAt && Date.now() - cached.lastDetectedAt < 60_000) {
      setState(cached)
      return
    }
    void detect()
  }, [devServerId, detect])

  return { ...state, redetect: detect }
}

// Detect trên tất cả connected servers:
export function useAllServersAgentDetection(
  devServers: DevServer[]
): Record<string, DetectionState> {
  const [results, setResults] = useState<Record<string, DetectionState>>({})

  useEffect(() => {
    const connected = devServers.filter(ds => ds.status === 'connected')
    if (connected.length === 0) return

    void window.api.onboarding.detectAgentsAllServers().then(raw => {
      const mapped: Record<string, DetectionState> = {}
      for (const [id, data] of Object.entries(raw)) {
        mapped[id] = {
          agents: data.agents,
          platform: data.platform,
          loading: false,
          error: data.error ?? null,
          lastDetectedAt: Date.now()
        }
      }
      setResults(mapped)
    })
  }, [devServers.map(ds => ds.id).join(',')])

  return results
}
```

---

## 3. Hook — `useActiveDevServerPlatform.ts`

```typescript
// src/renderer/src/hooks/useActiveDevServerPlatform.ts
import { useActiveDevServer } from '../store/slices/dev-servers'

export function useActiveDevServerPlatform(): NodeJS.Platform | null {
  const activeDevServer = useActiveDevServer()
  return activeDevServer?.platform ?? null
}

// Wizard visibility helpers:
export function useShowWindowsTerminalStep(): boolean {
  const platform = useActiveDevServerPlatform()
  return platform === 'win32'
}

export function useShowGhosttyImport(): boolean {
  const platform = useActiveDevServerPlatform()
  return platform === 'darwin'
}
```

---

## 4. AgentStep — MODIFY

```tsx
// src/renderer/src/components/onboarding/AgentStep.tsx (MODIFY)
import { useRemoteAgentDetection } from '../../hooks/useRemoteAgentDetection'
import { useActiveDevServer } from '../../store/slices/dev-servers'
import { DevServerStatusBadge } from '../dev-server/DevServerStatusBadge'

type AgentStepProps = {
  // ... existing props
  activeDevServerId: string | null   // NEW
}

export function AgentStep({ activeDevServerId, ...rest }: AgentStepProps) {
  const activeDevServer = useActiveDevServer()
  const { agents: detectedAgents, platform, loading, error, redetect } =
    useRemoteAgentDetection(activeDevServerId)

  // Filter catalog bằng platform của dev server
  const filteredCatalog = useMemo(
    () => platform ? getAgentCatalogForPlatform(platform) : getAgentCatalog(),
    [platform]
  )

  return (
    <div className="onboarding-step agent-step">
      {/* Server info header */}
      {activeDevServer && (
        <div className="detection-context">
          <span className="detected-on">Detected on</span>
          <strong>{activeDevServer.name}</strong>
          <DevServerStatusBadge
            status={activeDevServer.status}
            platform={activeDevServer.platform}
          />
        </div>
      )}

      {/* No dev server warning */}
      {!activeDevServerId && (
        <div className="no-server-warning">
          <span>⚠️ No dev server connected — agent detection unavailable</span>
          <span className="hint">You can still pick an agent manually below</span>
        </div>
      )}

      {/* Loading */}
      {loading && (
        <div className="detection-loading">
          <Spinner size="sm" />
          <span>Detecting agents on {activeDevServer?.name ?? 'dev server'}…</span>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="detection-error">
          <span>⚠️ Could not detect agents: {error}</span>
          <Button variant="ghost" size="sm" onClick={redetect}>Retry</Button>
        </div>
      )}

      {/* Agent grid — filtered by platform */}
      <AgentGrid
        catalog={filteredCatalog}
        detectedAgents={new Set(detectedAgents)}
        platform={platform}
        // ... rest of existing props
      />

      {/* Multi-server section */}
      <MultiServerAgentSection
        activeDevServerId={activeDevServerId}
      />
    </div>
  )
}

// Sub-component: agents trên các servers khác
function MultiServerAgentSection({ activeDevServerId }: { activeDevServerId: string | null }) {
  const [expanded, setExpanded] = useState(false)
  const connectedServers = useConnectedDevServers()
  const otherServers = connectedServers.filter(ds => ds.id !== activeDevServerId)

  if (otherServers.length === 0) return null

  return (
    <Collapsible open={expanded} onOpenChange={setExpanded}>
      <CollapsibleTrigger asChild>
        <button className="other-servers-toggle">
          {expanded ? '▲' : '▼'} Other dev servers ({otherServers.length})
        </button>
      </CollapsibleTrigger>
      <CollapsibleContent>
        {otherServers.map(ds => (
          <OtherServerAgentRow key={ds.id} devServer={ds} />
        ))}
      </CollapsibleContent>
    </Collapsible>
  )
}
```

---

## 5. Agent Catalog — Platform filter (MODIFY)

```typescript
// src/renderer/src/lib/agent-catalog.tsx (MODIFY)

type AgentCatalogEntry = {
  id: string
  label: string
  command: string
  icon: ReactNode
  installUrl: string
  requiredCommands?: readonly string[]
  unsupportedPlatforms?: readonly NodeJS.Platform[]    // NEW
  yoloUnsupportedPlatforms?: readonly NodeJS.Platform[] // NEW
}

// NEW function:
export function getAgentCatalogForPlatform(
  platform: NodeJS.Platform
): AgentCatalogEntry[] {
  return getAgentCatalog().filter(
    entry => !entry.unsupportedPlatforms?.includes(platform)
  )
}

// NEW: filter YOLO option per platform:
export function isYoloSupportedForPlatform(
  agentId: string,
  platform: NodeJS.Platform
): boolean {
  const entry = getAgentCatalog().find(e => e.id === agentId)
  return !entry?.yoloUnsupportedPlatforms?.includes(platform)
}
```

---

## 6. ThemeStep — Ghostty conditional (MODIFY)

```tsx
// src/renderer/src/components/onboarding/ThemeStep.tsx (MODIFY)
import { useShowGhosttyImport } from '../../hooks/useActiveDevServerPlatform'

export function ThemeStep({ activeDevServerId, ...rest }: ThemeStepProps & { activeDevServerId: string | null }) {
  const showGhostty = useShowGhosttyImport()

  const [ghosttyConfig, setGhosttyConfig] = useState<{ configPath: string | null } | null>(null)

  useEffect(() => {
    if (!showGhostty || !activeDevServerId) return
    // Detect Ghostty config trên dev server (macOS only):
    window.api.onboarding.detectGhosttyConfig({ devServerId: activeDevServerId })
      .then(setGhosttyConfig)
      .catch(() => {/* non-fatal */})
  }, [showGhostty, activeDevServerId])

  return (
    <div className="onboarding-step theme-step">
      {/* ...existing theme selector... */}

      {/* Ghostty import: chỉ khi dev server là macOS VÀ Ghostty được detect */}
      {showGhostty && ghosttyConfig?.configPath && (
        <GhosttyImportCard configPath={ghosttyConfig.configPath} />
      )}
    </div>
  )
}
```

---

## 7. `use-onboarding-flow.ts` — Platform-aware step sequencing (MODIFY)

```typescript
// src/renderer/src/components/onboarding/use-onboarding-flow.ts (MODIFY)
import { useActiveDevServerPlatform } from '../../hooks/useActiveDevServerPlatform'
import { useActiveDevServer } from '../../store/slices/dev-servers'

export function useOnboardingFlow() {
  const activeDevServer = useActiveDevServer()
  const platform = activeDevServer?.platform ?? null

  // Build step sequence dựa trên platform của dev server (KHÔNG dùng navigator.userAgent):
  const steps = useMemo(() => {
    const seq: OnboardingStep[] = ['dev_server', 'agent', 'theme']

    // Integrations: skip nếu gh đã có trên dev server
    if (!preflightStatus?.gh.installed) seq.push('integrations')

    // Windows terminal: chỉ khi dev server là Windows
    // THAY: if (navigator.userAgent.includes('Windows'))
    // BẰNG:
    if (platform === 'win32') seq.push('windows_terminal')

    seq.push('notifications')
    return seq
  }, [platform, preflightStatus])

  return {
    // ... existing return
    steps,
    activeDevServer,
    platform
  }
}
```

---

## 8. Wizard Header — Platform Badge

```tsx
// src/renderer/src/components/onboarding/OnboardingFlow.tsx (MODIFY)
// Thêm platform context vào header:

{activeDevServer && (
  <div className="wizard-server-context">
    <span>Dev env:</span>
    <strong>{activeDevServer.name}</strong>
    <DevServerStatusBadge status={activeDevServer.status} platform={activeDevServer.platform} />
  </div>
)}
```

---

## 9. Tests

```tsx
describe('useRemoteAgentDetection', () => {
  it('devServerId = null → trả về state mặc định, không gọi API')
  it('gọi api.onboarding.detectAgents với devServerId')
  it('cache hit (< 60s) → không gọi API lần 2')
  it('cache miss sau 60s → gọi API lại')
  it('loading = true trong khi đang detect')
  it('error state khi API throw')
  it('redetect() force gọi API bỏ qua cache')
})

describe('useActiveDevServerPlatform', () => {
  it('trả về null khi không có active dev server')
  it('trả về platform của active dev server')
  it('reactive: cập nhật khi activeDevServer thay đổi')
})

describe('useShowWindowsTerminalStep', () => {
  it('true khi platform = win32')
  it('false khi platform = darwin')
  it('false khi platform = null')
})

describe('getAgentCatalogForPlatform', () => {
  it('lọc agents có unsupportedPlatforms chứa platform đang xét')
  it('giữ lại agents không có unsupportedPlatforms')
  it('platform = win32 → ẩn agents không support Windows')
})

describe('AgentStep', () => {
  it('hiển thị dev server name + platform badge khi có activeDevServerId')
  it('hiển thị warning khi activeDevServerId = null')
  it('hiển thị Spinner khi loading = true')
  it('hiển thị error + Retry button khi detect fail')
  it('agent grid hiển thị agents filtered theo platform')
  it('MultiServerAgentSection ẩn khi không có server nào khác')
})

describe('use-onboarding-flow — platform-aware', () => {
  it('bước windows_terminal có trong steps khi platform = win32')
  it('bước windows_terminal KHÔNG có trong steps khi platform = darwin')
  it('bước windows_terminal KHÔNG có trong steps khi platform = null')
})
```

---

## 10. Checklist triển khai

**CR-OB-003:**
- [x] Tạo `useRemoteAgentDetection` hook với cache 60s
- [x] Tạo `useAllServersAgentDetection` hook (trong cùng file)
- [x] Extend `window.api.onboarding.detectAgents` + `detectAgentsAllServers` (preload bridge)
- [x] Sửa `AgentStep.tsx`: nhận `activeDevServerId` prop, hiển thị dev server context header
- [x] Thêm dev server badge trong AgentStep header

**CR-OB-004:**
- [x] Tạo `useActiveDevServerPlatform` hook (`useShowWindowsTerminalStep`, `useShowGhosttyImport`)
- [ ] Thêm `getAgentCatalogForPlatform()` vào agent-catalog.tsx (deferred)
- [x] Sửa `use-onboarding-flow.ts`: platform check từ `devServer.platform`
- [ ] Sửa `ThemeStep.tsx`: Ghostty detect qua remote API (deferred)
- [ ] Thêm platform badge vào wizard header (deferred)
- [ ] Unit tests (deferred to test pass)
