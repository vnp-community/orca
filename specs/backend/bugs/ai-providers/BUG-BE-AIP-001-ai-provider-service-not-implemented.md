# BUG-BE-AIP-001: `AIProviderService`, `ProviderCredentialWriter`, `AIProviderResolver` backend không tồn tại trong `src/main`

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AIP-001,002  
**Implementation:** AIProviderService implemented with status transitions  

## Mức độ: 🔴 HIGH (Feature Missing)

## Tóm tắt

HLD (BL-AIP-01 → BL-AIP-03) mô tả:
```
Orca Server → AIProviderService.create() → INSERT orca_ai_provider_accounts
           → ProviderCredentialWriter.write()
             → AgentConnectionManager.getConnection(devServerId)
             → JSON-RPC: ai.credential.write → Dev Server

Cron 15min → ProviderHealthChecker
           → ai.ping × N accounts
           → UPDATE status/health
```

Grep `src/main` không tìm thấy:
```
AIProviderService         → No results (chỉ có folder tên ai-providers)
ProviderCredentialWriter  → No results
AIProviderResolver        → No results
orca_ai_provider_accounts → No results
orca_provider_usage       → No results
ProviderHealthChecker     → No results
```

## Phân tích

Folder `src/main/ai-providers/` tồn tại nhưng chưa implement các components như HLD yêu cầu.

Dev Server side (`agent-credential-store.ts`) có implement `handleWriteCredential` và `handleHealthCheck` — nhưng Backend side (Orca Server) chưa có:
- REST API `POST /api/ai-providers/accounts`
- `ProviderCredentialWriter` để relay credential qua WS
- `AIProviderResolver` để resolve provider cho agent spawn
- `ProviderHealthChecker` cron job

## Ảnh hưởng

1. Admin không thể đăng ký AI Provider Account qua UI.
2. Credential không bao giờ được gửi đến Dev Server → Dev Server không có `.enc` file.
3. Health check cron không chạy → `status` field của providers không cập nhật.
4. `AIProviderResolver.resolve()` không tồn tại → agent spawn không resolve được provider (liên quan đến BUG-AG-ORCH-003).

## Liên quan đến luồng

- **BL-AIP-01**: Register AI Provider Account — backend side missing.
- **BL-AIP-02**: Provider resolution cascade — AIProviderResolver missing.
- **BL-AIP-03**: Health check cron — ProviderHealthChecker missing.
