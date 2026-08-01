# SOLUTION: Remote Development Domain — Fix tất cả Bugs

**Domain:** remote-development  
**TDD Reference:** TDD-05 (SSH Relay §reconnect), TDD-13 (Dev Server Onboarding)  
**Files cần thay đổi:** `src/main/ssh/ssh-connection-store.ts`, `src/main/dev-server/DevServerManager.ts`  
**Tổng số bugs:** 2 (BE-SSH-001, BE-SSH-002)

---

## BUG-BE-SSH-001 — Fix SSH reconnect không có exponential backoff

**Mức độ:** 🟠 HIGH  
**Root cause:** Khi SSH connection bị mất, reconnect với fixed interval → server bị spam khi unreachable.

### Fix — Implement exponential backoff reconnect

```typescript
// src/main/ssh/ssh-reconnect-manager.ts (NEW hoặc extend ssh-connection-store.ts)

export interface ReconnectConfig {
  initialDelayMs:  number   // default: 1_000 (1s)
  maxDelayMs:      number   // default: 60_000 (60s)
  multiplier:      number   // default: 2
  jitterFactor:    number   // default: 0.1 (±10% randomization)
  maxAttempts?:    number   // undefined = unlimited
}

export class ExponentialBackoffReconnect {
  private attempt    = 0
  private timer?: NodeJS.Timeout
  private aborted    = false

  constructor(
    private readonly config: ReconnectConfig,
    private readonly log: Logger,
  ) {}

  /**
   * Schedule next reconnect attempt với exponential backoff.
   */
  scheduleReconnect(
    devServerId: string,
    reconnectFn: () => Promise<void>,
    onExhausted?: () => void,
  ): void {
    if (this.aborted) return
    if (this.config.maxAttempts && this.attempt >= this.config.maxAttempts) {
      this.log.warn(`[SSH] Max reconnect attempts (${this.config.maxAttempts}) reached for ${devServerId}`)
      onExhausted?.()
      return
    }

    const delay = this.calculateDelay()
    this.log.info(`[SSH] Reconnect attempt ${this.attempt + 1} in ${delay}ms: ${devServerId}`)

    this.timer = setTimeout(async () => {
      if (this.aborted) return
      this.attempt++

      try {
        await reconnectFn()
        this.reset()  // Thành công → reset counter
        this.log.info(`[SSH] Reconnected: ${devServerId} (after ${this.attempt} attempts)`)
      } catch (err) {
        this.log.warn(`[SSH] Reconnect failed (attempt ${this.attempt}): ${devServerId}`, err)
        // Schedule next attempt
        this.scheduleReconnect(devServerId, reconnectFn, onExhausted)
      }
    }, delay)
  }

  /**
   * Calculate delay với exponential backoff + jitter.
   */
  private calculateDelay(): number {
    const { initialDelayMs, maxDelayMs, multiplier, jitterFactor } = this.config
    const base    = Math.min(initialDelayMs * Math.pow(multiplier, this.attempt), maxDelayMs)
    const jitter  = base * jitterFactor * (Math.random() * 2 - 1)  // ±jitter%
    return Math.round(Math.max(0, base + jitter))
  }

  reset(): void {
    this.attempt = 0
    clearTimeout(this.timer)
  }

  abort(): void {
    this.aborted = true
    clearTimeout(this.timer)
  }
}

// Integrate vào DevServerManager:
// src/main/dev-server/DevServerManager.ts

export class DevServerManager {
  private reconnectManagers = new Map<string, ExponentialBackoffReconnect>()

  private onConnectionLost(devServerId: string): void {
    const manager = this.reconnectManagers.get(devServerId)
      ?? new ExponentialBackoffReconnect(
        {
          initialDelayMs: 1_000,
          maxDelayMs:     60_000,
          multiplier:     2,
          jitterFactor:   0.1,
        },
        this.log
      )
    
    this.reconnectManagers.set(devServerId, manager)

    manager.scheduleReconnect(
      devServerId,
      () => this.connectToDevServer(devServerId),
      () => this.markServerOffline(devServerId),
    )
  }

  private onConnectionEstablished(devServerId: string): void {
    this.reconnectManagers.get(devServerId)?.reset()
    this.eventBus.emit('devServer.connected', { devServerId })
  }
}
```

---

## BUG-BE-SSH-002 — Fix port forward không được persist trong DB

**Mức độ:** 🟡 MEDIUM  
**Root cause:** Port forwarding config chỉ lưu in-memory → restart server = mất tất cả port forwards.

### Fix — Persist port forward config trong DB + auto-restore

```typescript
// src/main/ssh/PortForwardService.ts (NEW)

export interface PortForwardConfig {
  id:          string
  devServerId: string
  userId:      string
  localPort:   number
  remoteHost:  string
  remotePort:  number
  active:      boolean
  createdAt:   number
}

export class PortForwardService {
  constructor(
    private readonly repository:     IPortForwardRepository,
    private readonly devServerManager: DevServerManager,
    private readonly log: Logger,
  ) {}

  /**
   * Create và persist port forward.
   */
  async createPortForward(params: Omit<PortForwardConfig, 'id' | 'active' | 'createdAt'>): Promise<PortForwardConfig> {
    const config: PortForwardConfig = {
      id:          generateId(),
      ...params,
      active:      false,
      createdAt:   Date.now(),
    }

    // Persist TRƯỚC khi activate
    await this.repository.create(config)

    // Activate
    await this.activatePortForward(config)

    return config
  }

  /**
   * FIX BE-SSH-002: Restore all active port forwards after server restart.
   * Called in server-bootstrap.ts after DevServerManager init.
   */
  async restorePortForwards(): Promise<void> {
    const configs = await this.repository.listActive()
    this.log.info(`[PortForward] Restoring ${configs.length} port forwards`)

    for (const config of configs) {
      await this.activatePortForward(config).catch(err => {
        this.log.warn(`[PortForward] Failed to restore ${config.id}:`, err)
      })
    }
  }

  private async activatePortForward(config: PortForwardConfig): Promise<void> {
    const bridge = this.devServerManager.getBridge(config.devServerId)
    if (!bridge) throw new Error(`Dev server not connected: ${config.devServerId}`)

    await bridge.call('ssh.portForward', {
      localPort:  config.localPort,
      remoteHost: config.remoteHost,
      remotePort: config.remotePort,
    })

    await this.repository.setActive(config.id, true)
    this.log.info(`[PortForward] Active: localhost:${config.localPort} → ${config.remoteHost}:${config.remotePort}`)
  }

  async deletePortForward(id: string, userId: string): Promise<void> {
    const config = await this.repository.findById(id)
    if (!config || config.userId !== userId) throw new Error('Port forward not found')

    const bridge = this.devServerManager.getBridge(config.devServerId)
    if (bridge) {
      await bridge.call('ssh.stopPortForward', { localPort: config.localPort }).catch(() => {})
    }

    await this.repository.delete(id)
  }
}

// server-bootstrap.ts — thêm restore on startup:
// const portForwardService = new PortForwardService(...)
// await portForwardService.restorePortForwards()
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/ssh/ssh-reconnect-manager.ts` | NEW — ExponentialBackoffReconnect | BE-SSH-001 |
| `src/main/dev-server/DevServerManager.ts` | Use exponential backoff on disconnect | BE-SSH-001 |
| `src/main/ssh/PortForwardService.ts` | NEW — persist + restore port forwards | BE-SSH-002 |
| `src/main/repositories/port-forward-repository.ts` | NEW — repository interface + SQL impl | BE-SSH-002 |
| `src/main/db/migrations/0014_port_forwards.ts` | NEW migration | BE-SSH-002 |
| `src/main/server-bootstrap.ts` | Call portForwardService.restorePortForwards() | BE-SSH-002 |

---

## Verification Plan

```bash
# Test exponential backoff:
# 1. Disconnect SSH → verify attempt 1 at 1s, attempt 2 at 2s, attempt 3 at 4s
# 2. Reconnect succeeds → verify backoff reset to 0
# 3. Max attempts reached → verify server marked offline

# Test port forward persistence:
# 1. Create port forward → restart server → verify auto-restored
# 2. Dev server offline during restore → verify error logged, continue

pnpm vitest run src/main/ssh/__tests__/
pnpm vitest run src/main/ssh/__tests__/port-forward.test.ts
```
