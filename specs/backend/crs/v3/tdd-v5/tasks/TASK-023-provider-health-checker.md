# TASK-023: ProviderHealthChecker Cron

**Phase:** 4 — AI Provider Management  
**Solution ref:** [SOL-V5-003](../solutions/SOL-V5-003-ai-provider.md) §6  
**Prerequisite:** TASK-021  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/ai-providers/ProviderHealthChecker.ts`

15-minute background health check cron cho tất cả AI provider accounts.

```typescript
export class ProviderHealthChecker {
  private timer: ReturnType<typeof setInterval> | null = null
  
  start(service: AIProviderService, relayPool: RelayConnectionPool): void {
    // Run immediately then every 15 min
    this.runCheck(service, relayPool)
    this.timer = setInterval(() => this.runCheck(service, relayPool), 15 * 60 * 1000)
  }
  
  stop(): void {
    if (this.timer) { clearInterval(this.timer); this.timer = null }
  }
  
  private async runCheck(service: AIProviderService, relayPool: RelayConnectionPool): Promise<void> {
    const accounts = await service.getAllAccounts()
    for (const account of accounts) {
      const result = await service.testConnection(account.id)
      const newStatus = result.ok ? 'active' : (result.error?.includes('quota') ? 'quota_exceeded' : 'invalid')
      await service.updateAccount(account.id, { 
        status: newStatus,
        lastHealthCheck: new Date()
      })
    }
  }
}
```

## Acceptance Criteria

- [x] `start()` runs check immediately + sets interval
- [x] `stop()` clears interval
- [x] Updates account status based on testConnection result
- [x] Non-fatal — errors logged, not thrown
