# BUG-AG-ORCH-002: `agent.kill` dùng SIGTERM thay vì SIGKILL cho force-kill path

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AG-02) mô tả force-kill path:
```
[A1] Timeout 10s → hiển thị "Force Kill?" dialog
    ├─ conn.call('agent.kill', { ptyId, signal: 'SIGKILL' })
    │     → Dev Server: ptyHandle.kill('SIGKILL')
```

Nhưng `handleAgentKill` trong `agent-spawner.ts` **hardcode SIGTERM** thay vì đọc signal từ params:

```typescript
// agent-spawner.ts:215
entry.pty.kill('SIGTERM')  // ← luôn SIGTERM, bỏ qua params.signal
```

## File liên quan

- [`src/relay/agent-spawner.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/relay/agent-spawner.ts) — Lines 195-220

## Code sai

```typescript
export async function handleAgentKill(
  id:     string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log:    AgentLogger,
): Promise<object> {
  const ptyId = typeof params.ptyId === 'string' ? params.ptyId : ''
  // ...
  entry.pty.kill('SIGTERM')  // ← BUG: bỏ qua params.signal = 'SIGKILL'
  // ...
}
```

## Ảnh hưởng

1. Khi Main Process gửi `{ signal: 'SIGKILL' }` để force kill một agent bị kẹt, agent nhận SIGTERM thay vì SIGKILL → có thể bắt được SIGTERM và không thực sự chết.
2. Force kill mất đi tính "force" — mục đích của force kill là đảm bảo process bị terminate kể cả khi process xử lý SIGTERM.
3. Claude Code, Codex có thể có SIGTERM handler làm cleanup trước khi exit → delay kill thêm vài giây.

## Cách fix đề xuất

```typescript
export async function handleAgentKill(
  id: string | number | null,
  params: Record<string, unknown>,
  _config: AgentConfig,
  log: AgentLogger,
): Promise<object> {
  const ptyId  = typeof params.ptyId   === 'string' ? params.ptyId   : ''
  // Validate signal — chỉ cho phép SIGTERM và SIGKILL
  const signal = (params.signal === 'SIGKILL' ? 'SIGKILL' : 'SIGTERM') as 'SIGTERM' | 'SIGKILL'
  // ...
  entry.pty.kill(signal)  // ← Dùng signal từ params
  // ...
}
```

## Liên quan đến luồng

- **BL-AG-02**: [A1] Force Kill path — signal sai.
- **BL-AG-04**: Switch Account — gọi BL-AG-02 internally.

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** handleAgentKill reads params.signal, validates 'SIGTERM'|'SIGKILL'. Calls entry.pty.kill(signal).
