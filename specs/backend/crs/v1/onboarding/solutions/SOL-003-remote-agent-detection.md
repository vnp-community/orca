# SOL-003: Remote Agent Detection — Backend Solution

**CR:** [CR-OB-003](../../../../../docs/crs/v1/onboarding/CR-OB-003-agent-detection-remote.md)  
**TDD refs:** TDD-05 (SSH Relay), TDD-09 (IPC Handlers)  
**Status:** ✅ Implemented | **Phase:** 1 (Foundation)  
**Depends on:** SOL-002

---

## 1. Tổng quan giải pháp

`preflight.detectAgents` đã chạy đúng trên relay (relay process chạy trên dev server).  
Thay đổi duy nhất: **route call đến relay của đúng dev server** thay vì relay local.

```
TRƯỚC:
  Onboarding → preflightStatus (store) ← local relay preflight

SAU:
  Onboarding → api.onboarding.detectAgents({ devServerId })
             → DevServerManager.getRelay(devServerId)
             → relay.detectAgents(commands)        ← trên dev server
             → return { agents[], platform }
```

---

## 2. Relay — `src/relay/preflight-handler.ts` (MODIFY)

```typescript
// Thêm platform vào detectAgents response:
private async detectAgents(params: Record<string, unknown>): Promise<{
  agents: string[]
  platform: NodeJS.Platform   // NEW
}> {
  // ... existing detection logic ...
  return {
    agents: [...foundAgentIds],
    platform: process.platform   // NEW — dev server platform
  }
}
```

---

## 3. IPC Handler — `src/main/ipc/onboarding-ipc.ts` (NEW hoặc MODIFY)

```typescript
import type { DevServerManager } from '../dev-server/dev-server-manager'
import { TUI_AGENT_CONFIG } from '../../shared/tui-agent-config'

export function registerOnboardingIpcHandlers(
  ipc: IpcMain | WebIpcBridge,
  devServerManager: DevServerManager
): void {

  // Detect agents trên một dev server cụ thể
  ipc.handle('onboarding.detectAgents', async (_, params: {
    devServerId: string | null
  }): Promise<{
    agents: string[]
    platform: NodeJS.Platform | null
    devServerId: string | null
  }> => {
    const { devServerId } = params

    if (!devServerId) {
      // Không có dev server: trả về rỗng
      return { agents: [], platform: null, devServerId: null }
    }

    const relay = devServerManager.getRelay(devServerId)
    if (!relay) {
      throw new Error(`Dev server ${devServerId} not connected`)
    }

    // Gửi catalog commands đến relay
    const commands = getAgentDetectionCommands()   // từ agent catalog
    const result = await relay.detectAgents(commands)

    const devServer = devServerManager.get(devServerId)
    return {
      agents: result.agents,
      platform: devServer?.platform ?? null,
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
        const commands = getAgentDetectionCommands()
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
        out[serverId] = { agents: [], platform: servers[i].platform, error: r.reason?.message }
      }
    })
    return out
  })
}

// Helper: lấy agent detection commands từ catalog
// (Client gửi catalog; relay không import catalog — theo thiết kế hiện tại)
function getAgentDetectionCommands(): AgentDetectionCommand[] {
  return Object.entries(TUI_AGENT_CONFIG).map(([id, cfg]) => ({
    id,
    cmd: cfg.command,
    requiredCommands: cfg.requiredCommands,
    unsupportedRuntimes: cfg.unsupportedRuntimes
  }))
}
```

---

## 4. DevServerRelayBridge — `detectAgents` method (SOL-002 extend)

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
async detectAgents(commands: AgentDetectionCommand[]): Promise<{
  agents: string[]
  platform: NodeJS.Platform
}> {
  if (!this.session) throw new Error('Not connected')

  // Timeout: agent detection tối đa 15 giây
  const result = await this.callWithTimeout(
    'preflight.detectAgents',
    { commands },
    15_000
  )
  return {
    agents: result.agents as string[],
    platform: result.platform as NodeJS.Platform
  }
}

private async callWithTimeout<T>(
  method: string,
  params: unknown,
  timeoutMs: number
): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error(`Relay call ${method} timed out after ${timeoutMs}ms`)),
      timeoutMs
    )
    this.session!.call(method, params)
      .then(result => { clearTimeout(timer); resolve(result as T) })
      .catch(err => { clearTimeout(timer); reject(err) })
  })
}
```

---

## 5. Agent Detection Commands — `src/shared/agent-detection-commands.ts` (NEW)

```typescript
// Tách riêng để dùng cả server-side lẫn client-side
// (relay không import, server import để gửi xuống relay)

export type AgentDetectionCommand = {
  id: string
  cmd: string
  requiredCommands?: readonly string[]
  unsupportedRuntimes?: readonly ('darwin' | 'win32' | 'linux' | 'wsl')[]
}

// Build commands từ agent catalog (server-side only):
export function buildAgentDetectionCommands(): AgentDetectionCommand[] {
  // Map từ src/renderer/src/lib/agent-catalog.tsx (shared types)
  // Chỉ include { id, cmd, requiredCommands, unsupportedRuntimes }
  // — không include UI-only fields (label, description, icon, installUrl)
  return AGENT_CATALOG_RAW.map(entry => ({
    id: entry.id,
    cmd: entry.command,
    requiredCommands: entry.requiredCommands,
    unsupportedRuntimes: entry.unsupportedRuntimes
  }))
}
```

---

## 6. Preload Bridge — `src/preload/` hoặc `WebIpcBridge`

```typescript
// Thêm vào window.api (web mode hoặc electron preload):
window.api.onboarding = {
  detectAgents: (params: { devServerId: string | null }) =>
    ipcRenderer.invoke('onboarding.detectAgents', params),

  detectAgentsAllServers: () =>
    ipcRenderer.invoke('onboarding.detectAgentsAllServers')
}
```

---

## 7. Caching — per-server, TTL 60s

```typescript
// src/main/ipc/onboarding-ipc.ts — thêm cache:
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
```

---

## 8. Tests

```typescript
// src/main/ipc/__tests__/onboarding-ipc.test.ts
describe('onboarding.detectAgents', () => {
  it('devServerId = null → trả về { agents: [], platform: null }')
  it('devServerId không tồn tại → throw Error')
  it('dev server connected → forward detectAgents đến relay')
  it('relay trả về agents và platform')
  it('cache hit trong 60s → không gọi relay lần 2')
  it('cache miss sau 60s → gọi relay lại')
  it('relay timeout 15s → throw timeout error')
  it('relay error → throw, không cache')
})

describe('onboarding.detectAgentsAllServers', () => {
  it('0 connected servers → {} rỗng')
  it('2 connected servers → map { dsId: { agents, platform } }')
  it('1 thành công, 1 lỗi → { ds1: { agents }, ds2: { error } }')
  it('chạy song song (Promise.allSettled)')
})
```

---

## 9. Checklist triển khai

- [x] Thêm `platform` vào `preflight.detectAgents` response trong relay
- [x] Tạo `src/shared/agent-detection-commands.ts`
- [x] Implement `DevServerRelayBridge.detectAgents()` với timeout
- [x] Tạo `onboarding-ipc.ts` với `onboarding.detectAgents` handler
- [x] Implement `onboarding.detectAgentsAllServers`
- [x] Thêm per-server cache TTL 60s
- [x] Đăng ký handlers trong `server-bootstrap.ts`
- [x] Expose `window.api.onboarding.detectAgents` trong preload/bridge
- [x] Unit tests
