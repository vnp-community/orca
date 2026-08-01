# BUG-AG-ORCH-012: Claude args `--no-cache` không đúng CLI syntax — agent spawn sẽ error

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`agent-spawner.ts:83`:
```typescript
if (modelId.startsWith('claude')) {
  return { binary: 'claude', args: ['--output-format', 'stream-json', '--no-cache'] }
}
```

Vấn đề: **`--no-cache` không phải flag hợp lệ của Claude CLI.**

Claude CLI (`claude` binary từ Anthropic) hỗ trợ:
```
claude [options]
  --output-format <format>   json | stream-json | text
  --verbose                  Verbose logging
  --model <model>            Model to use
  --resume <session_id>      Resume a conversation
  --print                    Print and exit
  --dangerously-skip-permissions  Skip permission prompts
  --allowedTools <tools>     Comma-separated tools list
```

**`--no-cache` → Claude CLI sẽ throw `Unknown option: --no-cache` và exit code 1.**

Kết quả: `node-pty.spawn('claude', ['--output-format', 'stream-json', '--no-cache'], ...)` → PTY exits ngay lập tức với error.

## Thêm: `--output-format stream-json` có thể không phải syntax đúng

Claude CLI thực tế dùng: `claude --output-format=stream-json` hoặc `claude --output-format json` (tùy phiên bản). Cần verify với Claude CLI docs.

## Ảnh hưởng

1. Mọi `agent.spawn` với `modelId.startsWith('claude')` → Claude process exit ngay
2. `spawn.exit` event gửi về với exitCode != 0
3. Không có output → AgentHookParser không detect "idle" status

## Fix đề xuất

```typescript
export function resolveAgentSpec(modelId: string): { binary: string; args: string[] } {
  if (modelId.startsWith('claude')) {
    // Claude CLI syntax: claude --output-format stream-json --verbose
    return { binary: 'claude', args: ['--output-format', 'stream-json', '--verbose'] }
  }
  if (modelId.startsWith('gemini')) {
    return { binary: 'gemini', args: ['--stream'] }  // verify với gemini CLI docs
  }
  if (modelId.startsWith('gpt') || modelId.startsWith('openai')) {
    return { binary: 'codex', args: [] }  // Codex CLI (OpenAI)
  }
  if (modelId.startsWith('opencode')) {
    return { binary: 'opencode', args: [] }
  }
  throw new Error(`resolveAgentSpec: unknown modelId: ${modelId}`)
}
```

## Liên quan đến luồng

- **BL-AG-01**: Agent Spawn — Claude CLI args sai → exit ngay
- **BUG-AG-ORCH-004**: resolveAgentSpec missing codex/opencode (bug cũ đã ghi nhận)

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** buildArgs() for claude returns ['--output-format','stream-json','--verbose'] — no --no-cache flag.
