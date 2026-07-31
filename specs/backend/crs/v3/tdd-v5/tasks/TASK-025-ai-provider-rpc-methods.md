# TASK-025: AI Provider RPC Methods

**Phase:** 4 — AI Provider Management  
**Prerequisite:** TASK-021, TASK-022  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/ai-providers/ai-provider-rpc-handler.ts`

**Methods:**
- `aiProvider.list` → `service.listAccounts(params.devServerId, params.scope?)`
- `aiProvider.create` → `service.createAccount({ ..., createdBy: session.userId })`
- `aiProvider.get` → `service.getAccount(params.accountId)`
- `aiProvider.update` → `service.updateAccount` (owner/admin)
- `aiProvider.delete` → `service.deleteAccount` (admin)
- `aiProvider.writeCredential` → `service.writeCredentialToDevServer(params.accountId, params.encryptedBlob, params.iv)`
- `aiProvider.testConnection` → `service.testConnection(params.accountId)`
- `aiProvider.getUsageToday` → `service.getUsageToday(params.accountId)`
- `aiProvider.resolve` → `resolver.resolve(params)` — returns account without credential

**Access control:**
- `writeCredential`: only account owner or admin
- `delete`: admin only
- `list`: any authenticated user (filtered by devServer access)

## Acceptance Criteria

- [x] 9 RPC methods registered
- [x] `writeCredential` does not expose plaintext
- [x] Access control enforced
- [x] Không TypeScript errors
