# TASK-FLEET-001: Implement FleetHealthMonitor

**Priority:** 🔴 HIGH — Fleet health monitoring không hoạt động  
**Effort:** ~45 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-FLEET-001, BUG-BE-FLEET-002  
**Solution ref:** [SOLUTION-fleet.md](../solutions/SOLUTION-fleet.md)

## Mục tiêu

Tạo `FleetHealthMonitor` class ping tất cả Dev Servers định kỳ và cập nhật status + metrics.

## File cần tạo / sửa

`src/main/fleet/FleetHealthMonitor.ts` (NEW hoặc extend DevServerManager)

## Pattern

```typescript
export class FleetHealthMonitor {
  private interval: ReturnType<typeof setInterval> | null = null

  constructor(
    private readonly devServerManager: DevServerManager,
    private readonly intervalMs = 30_000  // 30s default
  ) {}

  start(): void {
    this.interval = setInterval(() => void this.sweep(), this.intervalMs)
    if (this.interval.unref) this.interval.unref()
  }

  stop(): void {
    if (this.interval) clearInterval(this.interval)
  }

  private async sweep(): Promise<void> {
    const servers = this.devServerManager.listServers()
    await Promise.allSettled(
      servers.map(server => this.checkServer(server))
    )
  }

  private async checkServer(server: PersistedDevServer): Promise<void> {
    const bridge = this.devServerManager.getBridge(server.id)
    if (!bridge) return

    const startMs = Date.now()
    try {
      const alive = await bridge.isAlive()
      const latencyMs = Date.now() - startMs
      
      await this.devServerManager.updateHealth(server.id, {
        status: alive ? 'healthy' : 'degraded',
        latencyMs,
        checkedAt: Date.now(),
      })
    } catch {
      await this.devServerManager.updateHealth(server.id, {
        status: 'unreachable',
        checkedAt: Date.now(),
      })
    }
  }
}
```

## Verification

```bash
pnpm tsc --noEmit
# Test: FleetHealthMonitor.start() → sau 30s → server health cập nhật
```
