# BUG-AG-ORCH-004: `resolveAgentSpec` chỉ hỗ trợ Claude và Gemini — thiếu Codex và OpenCode

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AG-01) liệt kê AI Agent Process hỗ trợ:
```
claude / codex / opencode / gemini
```

Nhưng `resolveAgentSpec` trong `agent-spawner.ts` chỉ handle `claude.*` và `gemini.*`:

```typescript
export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) {
    return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
  }
  if (modelId.startsWith('gemini')) {
    return { binary: 'gemini', args: ['--stream'] }
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}
```

**Codex** và **OpenCode** không có case nào → throw error.

## File liên quan

- [`src/relay/agent-spawner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-spawner.ts) — Lines 81-89

## Code sai

```typescript
// Lines 81-89
export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) { return { binary: 'claude', ... } }
  if (modelId.startsWith('gemini')) { return { binary: 'gemini', ... } }
  // ← MISSING: codex, opencode
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}
```

## Ảnh hưởng

1. Người dùng chọn Codex hoặc OpenCode agent → spawn fail với `"resolveAgentSpec: unknown modelId: codex-XXX"`.
2. HLD feature `BL-AG-03 Resume` cho Codex: `["codex", "--session-file", sessionFilePath]` không thể thực hiện.
3. `BL-AG-04 Switch provider` sang Codex/OpenCode → cũng fail.

## Cách fix đề xuất

```typescript
export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) {
    return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
  }
  if (modelId.startsWith('gemini')) {
    return { binary: 'gemini', args: ['--stream'] }
  }
  if (modelId.startsWith('codex') || modelId === 'codex') {
    return { binary: 'codex', args: ['--full-auto'] }
  }
  if (modelId.startsWith('opencode') || modelId === 'opencode') {
    return { binary: 'opencode', args: [] }
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}
```

## Liên quan đến luồng

- **BL-AG-01**: Khởi động AI Agent — Codex, OpenCode bị thiếu.
- **BL-AG-03**: Resume — Codex resume path (`--session-file`) không có.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** resolveAgentSpec: Added codex (gpt- prefix), opencode, ollama (localInference). Prefix map handles claude/codex/gemini/opencode/ollama variants.
