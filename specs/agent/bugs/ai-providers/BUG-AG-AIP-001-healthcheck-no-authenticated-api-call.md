# BUG-AG-AWS-001: `ai.provider.healthCheck` chỉ check reachability URL — không gọi API thực với credential

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AIP-03) mô tả health check:
```
Dev Server: đọc .enc file → decrypt apiKey → gọi test API (e.g. GET /v1/models) → trả { latencyMs, ok }
```

Nhưng `handleHealthCheck` trong `agent-credential-store.ts` chỉ:
1. Verify credential file có thể đọc được (structural check)
2. Gọi `HEAD` request đến provider URL — **không dùng apiKey**

```typescript
// Lines 213-214
const note = await checkProviderReachability(provider)
// checkProviderReachability → HEAD https://api.anthropic.com (không có Auth header)
```

## File liên quan

- [`src/relay/agent-credential-store.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-credential-store.ts) — Lines 195-254

## Code sai

```typescript
// handleHealthCheck (Lines 206-228):
const credResult = await handleReadCredential(...)  // ✅ đọc credential
// ... nhưng không dùng credResult.encryptedBlob để thực sự call API

const note = await checkProviderReachability(provider)  // chỉ HEAD request
```

```typescript
// checkProviderReachability (Lines 239-254):
const resp = await fetch(url, { method: 'HEAD', signal: ctrl.signal })
// HEAD /anthropic.com → trả 200 → "reachable"
// Nhưng key có thể invalid (401) → health check vẫn báo "ok"
```

## Ảnh hưởng

1. **False positive health**: API key hết hạn hoặc revoked → `ai.provider.healthCheck` vẫn trả `ok: true`.
2. BL-AIP-03 quota exceeded detection không hoạt động: health checker không gọi API thực → không detect `quota_exceeded`.
3. Admin dashboard sẽ hiển thị provider "healthy" dù key không dùng được.

## Cách fix đề xuất

Trong `handleHealthCheck`, cần decode credential và dùng apiKey thực để call test endpoint:
```typescript
// Sau khi đọc credential, decode và dùng:
const apiKey = decodeApiKey(credResult.result.encryptedBlob)  // decrypt browser layer
// Gọi authenticated API:
// Anthropic: GET /v1/models với Authorization: x-api-key <apiKey>
// OpenAI: GET /v1/models với Authorization: Bearer <apiKey>
```

## Liên quan đến luồng

- **BL-AIP-03**: Provider Health Check — health check không authenticated.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** handleHealthCheck returns {credentialFound, statusCode, ok, note, latencyMs}. JSON-RPC error when credential missing. checkProviderReachabilityDetailed with HTTP statusCode.
