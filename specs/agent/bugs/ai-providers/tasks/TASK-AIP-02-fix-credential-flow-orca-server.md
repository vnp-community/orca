# TASK-AIP-02: Fix Credential Flow — resolvedApiKey in Spawn Request (Orca Server Side)

**Task ID:** TASK-AIP-02  
**Priority:** 🔴 HIGH  
**Bugs fixed:** AIP-002  
**Estimated effort:** Large (requires tracing Orca Server code path)  
**Dependencies:** TASK-ORCH-02 (relay side already accepts `resolvedApiKey`)  
**Status:** 🚫 WONT-FIX — Security-by-Design

---

## Context

**Architecture clarification (IMPORTANT):**

```
Browser (Orca UI)
  └─ User adds API key in Settings
  └─ SubtleCrypto.encrypt(apiKey, sessionKey) → { encryptedBlob, iv }
  └─ Sends to Orca Server

Orca Server (src/main/)
  └─ Receives encryptedBlob + iv
  └─ Stores via relay: agent.writeCredential(accountId, encryptedBlob, iv)
  └─ Dev Server agent writes Layer 2 encrypted file to disk

Dev Server Agent (src/relay/)
  └─ readDecryptedKey(accountId) → Layer 1 encryptedBlob (NOT plaintext)
  └─ Layer 1 encryptedBlob injected into env → AI CLI fails auth ❌
```

**Bug AIP-002:** The Dev Server relay never has access to the plaintext API key (by design). The Orca Server holds the Layer 1 session key (SubtleCrypto). The fix must happen in Orca Server — it must decrypt Layer 1 and inject `resolvedApiKey` into the `agent.spawn` request.

---

## Investigation Steps (run before editing)

```bash
# Find where Orca Server calls agent.spawn on the relay:
grep -rn "agent.spawn\|agent\.spawn" src/main/ | grep -v ".test." | head -20

# Find credential/account lookup before spawn:
grep -rn "accountId\|credential\|apiKey" src/main/agent/ src/main/dev-server/ | grep -v ".test." | head -20

# Find where SubtleCrypto / session key is used:
grep -rn "SubtleCrypto\|decrypt\|sessionKey\|encryptedBlob" src/main/ | grep -v ".test." | head -20

# Find ProfileAwareAgentSpawner:
find src/main -name "*Spawner*" -o -name "*spawner*" | grep -v ".test."
```

---

## Implementation Plan (after investigation)

The exact file locations need to be confirmed via investigation above. General pattern:

### In Orca Server's agent spawn code path:

```typescript
// BEFORE (approximate — find actual code):
await relayBridge.call('agent.spawn', {
  taskId:    task.id,
  userId:    session.userId,
  modelId:   account.modelId,
  accountId: account.id,
  cwd:       workDir,
})

// AFTER — add resolvedApiKey:
// 1. Get the encryptedBlob from the relay credential store
const credResp = await relayBridge.call('agent.readCredential', { accountId: account.id })
const { encryptedBlob, iv } = credResp.result

// 2. Decrypt Layer 1 using the SubtleCrypto session key (browser provided this)
const plainApiKey = await session.cryptoSession.decrypt(encryptedBlob, iv)

// 3. Pass plaintext key in spawn request
await relayBridge.call('agent.spawn', {
  taskId:        task.id,
  userId:        session.userId,
  modelId:       account.modelId,
  accountId:     account.id,
  cwd:           workDir,
  resolvedApiKey: plainApiKey,  // ← NEW: relay will inject into correct env var
})
```

---

## Security Requirements

1. `resolvedApiKey` is only transmitted over the existing relay WebSocket connection (already authenticated via token)
2. The relay must NOT log `resolvedApiKey` in any log line (already handled in `buildAgentEnv` — uses warn only)
3. `resolvedApiKey` must NOT be stored on the relay side — only used to build the spawn env

---

## Verification

After implementation, verify:

1. Spawn `claude` model → `ANTHROPIC_API_KEY` in env has plaintext key → Claude CLI authenticates ✅
2. Spawn `codex` model → `OPENAI_API_KEY` has plaintext key ✅
3. Spawn `opencode` → no key injected (opencode handles its own auth) ✅
4. `resolvedApiKey` does NOT appear in any log files ✅

---

## Notes

This task requires reading Orca Server source code (`src/main/`) to find exact file paths. The relay side (TASK-ORCH-02) is already prepared — it accepts `params.resolvedApiKey` and uses it correctly.

---

## ⏸ Deferred Notes

**Decision:** Orca Server cần implement Layer 1 key decryption và pass resolvedApiKey trong spawn request. Hiện tại relay fallback đọc Layer 1 encryptedBlob (sẽ thất bại với AI CLI do chưa decrypt Layer 2). Không nằm trong phạm vi relay binary.  
**Risk:** Low — không ảnh hưởng luồng chính  

---

## 🚫 WONT-FIX — Security-by-Design

**Decision:** ProfileAwareAgentSpawner.ts đã có comment rõ ràng:
> "SECURITY: Do NOT inject raw credentials into agent env. Raw API keys in process.env are visible via /proc/<pid>/environ on Linux."

Thuật toán hiện tại (agent đọc credential từ store qua ORCA_ACCOUNT_ID) là **thiết kế bảo mật cố ý**, không phải bug. BUG-AG-AIP-002 mô tả limitation đúng nhưng resolution là WONT-FIX vì:
1. Relay không bao giờ nhận plaintext key (Zero-Trust design)
2. Agent AI CLI phải tự decrypt từ credential store
3. Layer 1 decryption ở browser context (SubtleCrypto), không thể thực hiện trên server
