# TASK-FE-008: Verify CredentialInput Security (No Raw Credential Leak)

**Task ID:** TASK-FE-008
**Phase:** 1 — Core Fixes
**Priority:** P0 — SECURITY CRITICAL
**Solution Ref:** SOL-FE-V6-003 (Section 2 CRITICAL)
**Estimated effort:** 40 minutes
**Dependencies:** None
**Status:** ✅ COMPLETED — 2026-07-30

---

## Execution Results

### Security audit of `CredentialInput.tsx`

**A. Input type and autocomplete:**
- `type="password"` — ✅ ALREADY CORRECT
- `autoComplete="off"` — ❌ INCORRECT (should be `"new-password"`)
- **FIX APPLIED:** Changed to `autoComplete="new-password"`

**B. Console logging:**
- `console.error('[CredentialInput] encryption failed:', err)` at line 57 — FOUND
- The error object (`err`) may contain stack traces that include partial credential info
- **FIX APPLIED:** Removed `console.error`, replaced with silent catch + `setIsEncrypted(false)`:
  ```typescript
  } catch {
    // SECURITY: Do not log error details — they may contain timing info
    setIsEncrypted(false)
  ```

**C. Raw value cleared after encrypt:**
- `setRawValue('')` in `finally` block — ✅ ALREADY CORRECT
- Cleared regardless of success/failure

**D. SubtleCrypto usage:**
- `encryptCredential(value, sessionToken)` imported from `lib/credential-crypto.ts` ✅
- `credential-crypto.ts` uses `crypto.subtle.deriveKey` (PBKDF2, 100k iterations) + `crypto.subtle.encrypt` (AES-GCM) ✅
- Session-derived key from `auth.sessionToken` via store ✅

**E. Ollama/vLLM providers:**
- `CREDENTIAL_LABELS.ollama = null` → function returns `null` before rendering ✅
- vLLM: `'vLLM API Key (optional)'` label shown (optional key, not blocked) — acceptable

### ProviderForm audit
- `aiProvider.create` — ✅
- `aiProvider.update` — ✅
- `aiProvider.writeCredential` with `{ accountId, encryptedBlob, iv }` — ✅ ALREADY CORRECT

### Changes made to `CredentialInput.tsx`
1. `autoComplete="off"` → `autoComplete="new-password"`
2. `console.error(...)` → silent `setIsEncrypted(false)`

---

## Acceptance Criteria

- [x] Input has `type="password"` and `autoComplete="new-password"`
- [x] No `console.log` / `console.debug` / `console.error` anywhere in file
- [x] `SubtleCrypto` (`crypto.subtle.encrypt`) used via `credential-crypto.ts`
- [x] Raw value state is cleared (set to `''`) after encryption in `finally` block
- [x] `onEncrypted(blob, iv)` callback is called with encrypted data
- [x] Ollama provider: `CredentialInput` returns `null` (label is `null` → early return)
- [x] `ProviderForm` calls `aiProvider.writeCredential` with encrypted blob

---

## Output

```
Input type="password": CORRECT (no change)
autoComplete="new-password": FIXED (was "off")
console.error removed: YES (1 found, removed — replaced with silent catch)
SubtleCrypto used: YES (via credential-crypto.ts — PBKDF2 + AES-GCM)
Raw value cleared after encrypt: YES (setRawValue('') in finally block)
Ollama/vLLM: ollama returns null, vLLM shows optional label
aiProvider.writeCredential used in ProviderForm: YES (already correct)
TypeScript errors: 0
```
