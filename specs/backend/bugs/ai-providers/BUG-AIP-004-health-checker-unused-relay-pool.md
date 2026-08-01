# BUG-AIP-004: `ProviderHealthChecker.runCheck` nhận `_relayPool` (underscore) — tham số không dùng

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-AIP-004  
**Note:** ProviderHealthChecker.ts: removed unused _relayPool param from start()/runCheck()  

## Mức độ: 🟢 LOW

## Tóm tắt

`src/main/ai-providers/ProviderHealthChecker.ts:52`:
```typescript
private async runCheck(service: AIProviderService, _relayPool: RelayConnectionPool): Promise<void> {
```

Tham số `_relayPool` (tiền tố `_` = intentionally unused) được pass vào nhưng không dùng trong hàm.

Trong khi `service.testConnection(accountId)` đã internally gọi `relay.call()` qua `relayPool` trong `AIProviderService`.

Đây không phải lỗi nghiêm trọng, nhưng cho thấy `_relayPool` được pass nhằm "phòng trường hợp cần" nhưng không được integrate vào health check logic thực sự (chỉ delegate cho `service`).

## Vấn đề thực sự

`service.testConnection()` trong `AIProviderService.ts:246-266` gọi:
```typescript
const relay = await this.relayPool.getOrConnect(account.devServerId, server)
await relay.call('ai.provider.testConnection', { accountId })
```

Relay RPC method là `ai.provider.testConnection` nhưng `ProviderHealthChecker.start()` gọi:
```typescript
start(service: AIProviderService, relayPool: RelayConnectionPool): void {
```

`relayPool` được pass nhưng không dùng trực tiếp → **pattern không nhất quán**.

## Fix đề xuất

Xóa `_relayPool` khỏi signature nếu không dùng:
```typescript
start(service: AIProviderService): void {
  void this.runCheck(service)
  this.timer = setInterval(() => void this.runCheck(service), HEALTH_CHECK_INTERVAL_MS)
}

private async runCheck(service: AIProviderService): Promise<void> { ... }
```

## Files liên quan

- `src/main/ai-providers/ProviderHealthChecker.ts:52`
