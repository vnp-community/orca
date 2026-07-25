# TASK-FE-009: Tạo useRemoteAgentDetection hook

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/hooks/useRemoteAgentDetection.ts` [NEW]
> - `src/renderer/src/hooks/__tests__/useRemoteAgentDetection.test.ts` [NEW]

**Phase:** 2 | **Solution:** [FE-SOL-B](../solutions/FE-SOL-B-agent-wizard.md) | **CR:** CR-OB-003  
**Depends on:** TASK-FE-001, TASK-FE-002, TASK-FE-020  
**Estimated effort:** ~45 phút

---

## Context

Đọc trước:
- [`src/renderer/src/lib/agent-catalog.tsx`](../../../../../src/renderer/src/lib/agent-catalog.tsx) — xem `AgentCatalogEntry` type hiện tại
- [`../solutions/FE-SOL-B-agent-wizard.md`](../solutions/FE-SOL-B-agent-wizard.md) — Section 2

---

## Goal

Tạo `useRemoteAgentDetection` hook detect agents trên dev server từ xa, với client-side cache TTL 60s. Tạo thêm `useAllServersAgentDetection` cho nhiều servers.

---

## Steps

1. **Tạo** `src/renderer/src/hooks/useRemoteAgentDetection.ts`:

```typescript
import { useState, useEffect, useCallback } from 'react'
import type { DevServer } from '../../../shared/dev-server-types'

export type AgentDetectionState = {
  agents: string[]
  platform: NodeJS.Platform | null
  loading: boolean
  error: string | null
  lastDetectedAt: number | null
}

const DEFAULT_STATE: AgentDetectionState = {
  agents: [],
  platform: null,
  loading: false,
  error: null,
  lastDetectedAt: null,
}

// Module-level cache: survives React re-renders
const detectionCache = new Map<string, AgentDetectionState>()
const CACHE_TTL_MS = 60_000

export function useRemoteAgentDetection(devServerId: string | null): AgentDetectionState & {
  redetect: () => Promise<void>
} {
  const cached = devServerId ? detectionCache.get(devServerId) : undefined
  const isCacheValid = !!cached?.lastDetectedAt && Date.now() - cached.lastDetectedAt < CACHE_TTL_MS

  const [state, setState] = useState<AgentDetectionState>(
    isCacheValid ? cached! : DEFAULT_STATE
  )

  const detect = useCallback(async () => {
    if (!devServerId) {
      setState(DEFAULT_STATE)
      return
    }
    setState((prev) => ({ ...prev, loading: true, error: null }))
    try {
      const result = await window.api.onboarding.detectAgents({ devServerId })
      const next: AgentDetectionState = {
        agents: result.agents,
        platform: result.platform,
        loading: false,
        error: null,
        lastDetectedAt: Date.now(),
      }
      detectionCache.set(devServerId, next)
      setState(next)
    } catch (err) {
      setState((prev) => ({
        ...prev,
        loading: false,
        error: (err as Error).message,
      }))
    }
  }, [devServerId])

  useEffect(() => {
    if (!devServerId) {
      setState(DEFAULT_STATE)
      return
    }
    if (isCacheValid) {
      setState(cached!)
      return
    }
    void detect()
  }, [devServerId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { ...state, redetect: detect }
}

export function useAllServersAgentDetection(
  devServers: DevServer[]
): Record<string, AgentDetectionState> {
  const [results, setResults] = useState<Record<string, AgentDetectionState>>({})

  const serverIds = devServers
    .filter((ds) => ds.status === 'connected')
    .map((ds) => ds.id)
    .join(',')

  useEffect(() => {
    if (!serverIds) return
    void window.api.onboarding.detectAgentsAllServers().then((raw) => {
      const mapped: Record<string, AgentDetectionState> = {}
      for (const [id, data] of Object.entries(raw)) {
        mapped[id] = {
          agents: data.agents,
          platform: data.platform,
          loading: false,
          error: data.error ?? null,
          lastDetectedAt: Date.now(),
        }
      }
      setResults(mapped)
    })
  }, [serverIds])

  return results
}
```

2. **Viết test** `src/renderer/src/hooks/__tests__/useRemoteAgentDetection.test.ts`:

```typescript
// @vitest-environment happy-dom
describe('useRemoteAgentDetection', () => {
  beforeEach(() => {
    vi.stubGlobal('window', {
      api: {
        onboarding: {
          detectAgents: vi.fn().mockResolvedValue({ agents: ['claude'], platform: 'darwin' }),
        },
      },
    })
    // Clear module-level cache between tests
    detectionCache.clear()
  })

  it('devServerId = null → DEFAULT_STATE, không gọi API')
  it('gọi window.api.onboarding.detectAgents với devServerId khi mount')
  it('state.loading = true trong khi đang detect')
  it('state.agents + state.platform được set sau khi detect xong')
  it('cache hit (< 60s) → không gọi API lần 2')
  it('cache miss sau 60s → gọi API lại')
  it('error state khi API throw')
  it('redetect() bỏ qua cache và gọi API')
  it('devServerId thay đổi → reset state và detect lại')
})
```

---

## Acceptance Criteria

- [ ] Hook tạo đúng `AgentDetectionState` type
- [ ] Cache module-level với TTL 60s hoạt động đúng
- [ ] `redetect()` force bypass cache
- [ ] `useAllServersAgentDetection` gọi `detectAgentsAllServers`
- [ ] Tests pass

## Output Files

- **[NEW]** `src/renderer/src/hooks/useRemoteAgentDetection.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useRemoteAgentDetection.test.ts`
