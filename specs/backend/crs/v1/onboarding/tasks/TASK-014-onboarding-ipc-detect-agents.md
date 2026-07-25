# TASK-014: Tạo/Sửa `src/main/ipc/onboarding-ipc.ts` — `detectAgents` + `detectAgentsAllServers` + Cache

**Phase:** 1 — Remote Agent Detection  
**Solution:** [SOL-003](../solutions/SOL-003-remote-agent-detection.md) §3, §7  
**Depends on:** TASK-004, TASK-011, TASK-013  
**Blocks:** TASK-015, TASK-016

---

## Mục tiêu

Tạo (hoặc sửa nếu đã tồn tại) file `onboarding-ipc.ts` với:
1. IPC handler `onboarding.detectAgents` — detect agents trên 1 dev server cụ thể, có cache TTL 60s
2. IPC handler `onboarding.detectAgentsAllServers` — detect song song trên tất cả connected servers

---

## File cần tạo/sửa

**Path:** `src/main/ipc/onboarding-ipc.ts`

---

## Nội dung cần implement

```typescript
import type { DevServerManager } from '../dev-server/dev-server-manager'
import { buildAgentDetectionCommands } from '../../shared/agent-detection-commands'

// Cache per-server, TTL 60s
const agentDetectionCache = new Map<string, {
  result: { agents: string[]; platform: NodeJS.Platform | null }
  cachedAt: number
}>()
const AGENT_DETECTION_CACHE_TTL_MS = 60_000

function getCachedDetection(devServerId: string) {
  const entry = agentDetectionCache.get(devServerId)
  if (entry && Date.now() - entry.cachedAt < AGENT_DETECTION_CACHE_TTL_MS) {
    return entry.result
  }
  return null
}

export function registerOnboardingIpcHandlers(
  ipc: IpcBridge,
  devServerManager: DevServerManager
): void {

  // Detect agents trên 1 dev server
  ipc.handle('onboarding.detectAgents', async (_, params: {
    devServerId: string | null
  }): Promise<{
    agents: string[]
    platform: NodeJS.Platform | null
    devServerId: string | null
  }> => {
    const { devServerId } = params

    if (!devServerId) {
      return { agents: [], platform: null, devServerId: null }
    }

    // Cache check
    const cached = getCachedDetection(devServerId)
    if (cached) {
      return { ...cached, devServerId }
    }

    const relay = devServerManager.getRelay(devServerId)
    if (!relay) {
      throw new Error(`Dev server ${devServerId} not connected`)
    }

    const commands = buildAgentDetectionCommands()
    const result = await relay.detectAgents(commands)

    // Save to cache
    agentDetectionCache.set(devServerId, {
      result: { agents: result.agents, platform: result.platform },
      cachedAt: Date.now()
    })

    return {
      agents: result.agents,
      platform: result.platform,
      devServerId
    }
  })

  // Detect agents trên TẤT CẢ connected dev servers (song song)
  ipc.handle('onboarding.detectAgentsAllServers', async (): Promise<Record<string, {
    agents: string[]
    platform: NodeJS.Platform | null
    error?: string
  }>> => {
    const servers = devServerManager.list().filter(ds => ds.status === 'connected')

    const results = await Promise.allSettled(
      servers.map(async ds => {
        const relay = devServerManager.getRelay(ds.id)!
        const commands = buildAgentDetectionCommands()
        const result = await relay.detectAgents(commands)
        return { id: ds.id, agents: result.agents, platform: ds.platform }
      })
    )

    const out: Record<string, { agents: string[]; platform: NodeJS.Platform | null; error?: string }> = {}
    results.forEach((r, i) => {
      const serverId = servers[i].id
      if (r.status === 'fulfilled') {
        out[serverId] = { agents: r.value.agents, platform: r.value.platform }
      } else {
        out[serverId] = {
          agents: [],
          platform: servers[i].platform,
          error: r.reason?.message ?? 'Unknown error'
        }
      }
    })
    return out
  })
}
```

---

## Acceptance Criteria

- [x] File tồn tại tại `src/main/ipc/onboarding-ipc.ts`
- [x] `registerOnboardingIpcHandlers()` được export
- [x] `devServerId = null` → return `{ agents: [], platform: null, devServerId: null }`
- [x] Dev server không tồn tại hoặc không connected → throw Error
- [x] Cache hit trong 60s → không gọi relay lần 2
- [x] Cache miss (sau 60s hoặc chưa có) → gọi relay và lưu cache
- [x] Error từ relay → không cache, throw error
- [x] `detectAgentsAllServers` chỉ query servers có `status === 'connected'`
- [x] `detectAgentsAllServers` dùng `Promise.allSettled` (1 server lỗi không ảnh hưởng servers khác)
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

Nếu `onboarding-ipc.ts` đã tồn tại, hãy thêm các handlers này vào hàm register hiện có thay vì tạo file mới.
