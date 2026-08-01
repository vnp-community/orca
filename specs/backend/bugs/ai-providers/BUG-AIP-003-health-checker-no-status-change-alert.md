# BUG-AIP-003: `ProviderHealthChecker` không phát alert khi status thay đổi — webhook thiếu

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-AIP-003  
**Note:** ProviderHealthChecker.ts: onStatusChanged callback + status transition detection  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD BL-AIP-03 mô tả:
```
IF status changed:
    → [SRV→WEB] WS push: provider status changed
    → [SRV] POST webhookUrl (Slack/PagerDuty nếu cấu hình)
```

Thực tế `src/main/ai-providers/ProviderHealthChecker.ts:60-81`:
```typescript
private async runCheck(service: AIProviderService, _relayPool: RelayConnectionPool): Promise<void> {
  const accounts = await service.getAllAccounts()
  for (const account of accounts) {
    const result = await service.testConnection(account.id)
    let newStatus: 'active' | 'quota_exceeded' | 'invalid'
    // ...
    await service.updateAccount(account.id, { status: newStatus, lastHealthCheck: new Date() })
    // ← KHÔNG có: emit WS event, KHÔNG có: webhook call
  }
}
```

**Không có status-changed detection, WS push, hay webhook call.**

## Ảnh hưởng

1. Admin không nhận được alert khi provider đột ngột fail
2. Không có Slack/PagerDuty notification
3. UI provider dashboard không update real-time khi status đổi

## Fix đề xuất

```typescript
private async runCheck(service: AIProviderService): Promise<void> {
  const accounts = await service.getAllAccounts()
  for (const account of accounts) {
    const oldStatus = account.status
    const result = await service.testConnection(account.id)
    const newStatus = result.ok ? 'active' : result.error?.includes('quota') ? 'quota_exceeded' : 'invalid'
    
    await service.updateAccount(account.id, { status: newStatus as AIProviderStatus, lastHealthCheck: new Date() })
    
    // ← CẦN THÊM:
    if (oldStatus !== newStatus) {
      this.emit('statusChanged', { accountId: account.id, oldStatus, newStatus })
      // Trigger WS push → Browser
      // Trigger webhook (nếu cấu hình)
    }
  }
}
```

## Files liên quan

- `src/main/ai-providers/ProviderHealthChecker.ts:60-81`: thiếu status-change detection
