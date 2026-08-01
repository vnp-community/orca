# TASK-WT-002: Fix ProfileAwareAgentSpawner — xóa credential injection vào agent env

**Priority:** 🔴 HIGH SECURITY — API key exposed in process env  
**Effort:** ~10 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-WT-002  
**Solution ref:** [SOLUTION-worktree-exact.md](../solutions/SOLUTION-worktree-exact.md)

---

## Mục tiêu

Xóa `Object.assign(profileEnv, provider.credentials)` trong `ProfileAwareAgentSpawner.spawn()`. API keys không được inject trực tiếp vào agent env. Thay vào đó chỉ inject `ORCA_ACCOUNT_ID` để agent tự đọc credential qua credential store.

## File cần sửa

```
src/main/project/ProfileAwareAgentSpawner.ts
```

## Thay đổi cụ thể

### Lines 97–102:

**TRƯỚC (insecure — inject plaintext API keys):**
```typescript
if (provider) {
  profileEnv['ORCA_AI_PROVIDER_ID'] = provider.providerId
  profileEnv['ORCA_AI_MODEL_ID']    = provider.modelId
  // Merge provider credentials into env (e.g. API keys)
  Object.assign(profileEnv, provider.credentials)  // ← SECURITY RISK
}
```

**SAU (secure — inject only account ID reference):**
```typescript
if (provider) {
  profileEnv['ORCA_AI_PROVIDER_ID'] = provider.providerId
  profileEnv['ORCA_AI_MODEL_ID']    = provider.modelId
  // FIX WT-002: DO NOT inject raw credentials — agent reads them via ORCA_ACCOUNT_ID
  // from the credential store on the Dev Server (already set up by writeCredential flow)
  profileEnv['ORCA_ACCOUNT_ID']     = provider.providerId
  // Object.assign(profileEnv, provider.credentials)  ← REMOVED
}
```

## Lý do

Agent env vars (`process.env`) visible qua:
- `ps auxe` (on Linux với non-stripped ps)
- `/proc/<pid>/environ` (Linux)
- Process dump

Credential store (file-system AES-encrypted) là path an toàn hơn nhiều.

## Note về AIProviderResolver interface

Nếu `AIProviderResolver.resolveForProject()` trả về `credentials` field, vẫn giữ field đó trong interface (backward compat) nhưng không dùng nó trong spawn. Update JSDoc:

```typescript
/** credentials: DEPRECATED — do NOT inject into agent env. Use ORCA_ACCOUNT_ID instead. */
credentials: Record<string, string>
```

## Verification

```bash
pnpm tsc --noEmit

# Verify: ANTHROPIC_API_KEY, OPENAI_API_KEY không còn trong profileEnv:
grep -n "provider.credentials\|Object.assign.*credential" src/main/project/ProfileAwareAgentSpawner.ts
# Expected: no results (hoặc chỉ còn comment)
```
