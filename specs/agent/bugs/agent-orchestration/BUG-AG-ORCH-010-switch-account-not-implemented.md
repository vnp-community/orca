# BUG-AG-ORCH-010: BL-AG-04 Switch Account chưa implement — không có rate-limit detection + provider switch

## Mức độ: 🔴 HIGH

## Tóm tắt

HLD (BL-AG-04):
```
[Dev Server → PTY output → JSON-RPC event: agent.output → Main]
    Main parse: rate-limit pattern match
    emit: agent:rateLimited { pattern, resetAt }
→ [Renderer] alert: "Claude Code bị rate limited. Reset lúc HH:MM"
→ [Main Process — AgentManager.switchAccount()]
    UPDATE orca_sessions SET status='stopped'
    conn.call('agent.kill', { ptyId })
    AIProviderResolver.resolve() với account mới
    conn.call('agent.spawn', { newEnv })
```

Thực tế:
1. **agent.output stream không có receiver** (BUG-AG-ORCH-006) → rate-limit pattern match không thể xảy ra
2. `agent:rateLimited` event **không tồn tại** trong codebase
3. `AgentManager.switchAccount()` **không tồn tại**
4. `AIProviderResolver` interface định nghĩa trong `ProfileAwareAgentSpawner.ts:21-26` nhưng không có `switchAccount` flow

**Code rate-limit tracking tồn tại nhưng ở layer khác:**
- `src/main/rate-limits/claude-pty.ts` — track Claude PTY rate limits cho **local** terminals
- `src/main/rate-limits/claude-fetcher.ts` — fetch usage từ Anthropic API dashboard

**Hai layer này không liên kết với agent orchestration remote PTY flow.**

## Ảnh hưởng

1. Khi Claude remote bị rate limited → không có alert, user không biết
2. Không có tự động switch account khi rate limit hit
3. Agent sẽ stuck ở rate-limited state mà không có recovery

## Fix đề xuất

Kết nối rate-limit detection vào `AgentHookParser` (sau khi implement BUG-AG-ORCH-007):
```typescript
// Thêm vào AgentHookParser
const RATE_LIMIT_PATTERNS = [
  /rate limited\.?\s+please try again/i,
  /you've reached your (daily|weekly|monthly) limit/i,
  /Claude usage is rate limited/i,
]

parse(ptyId: string, data: string) {
  for (const pattern of RATE_LIMIT_PATTERNS) {
    if (pattern.test(data)) {
      this.emit('agent:rateLimited', { ptyId, resetAt: estimateResetTime() })
      return
    }
  }
}
```

## Liên quan đến luồng

- **BL-AG-04**: Switch Account — không implement
- Phụ thuộc fix BUG-AG-ORCH-005 (AgentManager), BUG-AG-ORCH-006 (output stream), BUG-AG-ORCH-007 (parser)

---

## ⏸ Fix Status: DEFERRED

**Reason:** Switch account requires OAuth/token refresh from Orca Server. Deferred to Phase 3 backlog.
