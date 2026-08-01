# BUG-AG-ORCH-003: `buildAgentEnv` dùng `'placeholder-key'` hardcode — API key không bao giờ được load từ credential store

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AG-01) mô tả:
```
├─ AIProviderResolver.resolve(userId, projectId, devServerId)
│     → { provider, agentCommand, apiKeyEnvVar }
│ (apiKey được đọc trực tiếp từ file .enc trên Dev Server khi spawn)
```

Nhưng `buildAgentEnv` trong `agent-spawner.ts` hardcode API key là `'placeholder-key'`:

```typescript
// agent-spawner.ts:147
const env = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)
```

Agent Claude/Gemini sẽ không bao giờ được cấp API key thực sự → **mọi AI call đều fail với unauthorized**.

## File liên quan

- [`src/relay/agent-spawner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-spawner.ts) — Lines 143-155

## Code sai

```typescript
// Line 147 — ❌ BUG: 'placeholder-key' không phải API key thực
const env = await buildAgentEnv(req.accountId, 'placeholder-key', req.cwd ?? config.workDir)
```

## Ảnh hưởng

1. **CRITICAL**: AI Agent (claude, gemini) sẽ fail authentication với AI provider.
2. PTY spawn thành công nhưng ngay khi agent gọi API → nhận 401 Unauthorized.
3. `agent-credential-store.ts` đã implement `ai.provider.readCredential` → nhưng chưa được gọi trong spawn path.

## Code đúng theo HLD

Phải gọi `agent-credential-store` để đọc API key trước khi spawn:

```typescript
// Sửa handleAgentSpawn:
const { handleReadCredential } = await import('./agent-credential-store')
const credResult = await handleReadCredential(null, { 
  accountId: req.accountId 
}, config, log)
// Extract API key từ credential result
const apiKey = credResult.result?.apiKey ?? ''

const env = await buildAgentEnv(req.accountId, apiKey, req.cwd ?? config.workDir)
```

## Liên quan đến luồng

- **BL-AG-01**: Build agent env — `apiKeyEnvVar` không được resolve.
- **AI Credential File**: `~/.orca/ai-providers/<accountId>.enc` trên Dev Server — đã có `agent-credential-store.ts` nhưng chưa được gọi trong spawn path.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** buildAgentEnv now takes resolvedApiKey param. Injects into ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY. Fallback reads Layer1 blob from credStore.
