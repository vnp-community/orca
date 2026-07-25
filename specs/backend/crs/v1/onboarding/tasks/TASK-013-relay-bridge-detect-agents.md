# TASK-013: Sửa `src/main/dev-server/dev-server-relay-bridge.ts` — Thêm `detectAgents` + `callWithTimeout`

**Phase:** 1 — Remote Agent Detection  
**Solution:** [SOL-003](../solutions/SOL-003-remote-agent-detection.md) §4  
**Depends on:** TASK-005, TASK-011, TASK-012  
**Blocks:** TASK-014

---

## Mục tiêu

Thêm method `detectAgents()` và helper `callWithTimeout()` vào `DevServerRelayBridge`. Method này forward agent detection request đến relay với timeout 15 giây.

---

## File cần sửa

**Path:** `src/main/dev-server/dev-server-relay-bridge.ts`

---

## Thay đổi cần thực hiện

Thêm 2 methods vào class `DevServerRelayBridge`:

```typescript
import type { AgentDetectionCommand } from '../../shared/agent-detection-commands'

// Trong class DevServerRelayBridge:

async detectAgents(commands: AgentDetectionCommand[]): Promise<{
  agents: string[]
  platform: NodeJS.Platform
}> {
  if (!this.session) throw new Error('Not connected')

  // Timeout: agent detection tối đa 15 giây
  const result = await this.callWithTimeout<{
    agents: string[]
    platform: NodeJS.Platform
  }>(
    'preflight.detectAgents',
    { commands },
    15_000
  )
  return {
    agents: result.agents,
    platform: result.platform
  }
}

private async callWithTimeout<T>(
  method: string,
  params: unknown,
  timeoutMs: number
): Promise<T> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error(`Relay call '${method}' timed out after ${timeoutMs}ms`)),
      timeoutMs
    )
    this.session!.call(method, params)
      .then(result => { clearTimeout(timer); resolve(result as T) })
      .catch(err => { clearTimeout(timer); reject(err) })
  })
}
```

---

## Acceptance Criteria

- [x] `detectAgents()` được thêm vào class `DevServerRelayBridge`
- [x] Throw `Error('Not connected')` khi `session` là null
- [x] Sử dụng `callWithTimeout()` với 15_000ms
- [x] Return có đủ `agents` và `platform`
- [x] `callWithTimeout()` là `private` method
- [x] Timeout error message có tên method và thời gian
- [x] TypeScript compile thành công
