# CR-AG-04: Session Handshake — Version + Capabilities Update

**CR:** CR-AG-04
**TDD:** [TDD-AG-04](../../tdd/v5/04-handshake-session.md)
**Ngày:** 2026-07-30
**Độ phức tạp:** Low — 2 dòng thay đổi

---

## 1. Phân tích Code Hiện Tại

### Code đã có ✅ — [`src/relay/agent-session.ts`](../../../../../src/relay/agent-session.ts)

Handshake hoàn toàn đầy đủ. Chỉ cần update 2 giá trị:

```typescript
// Line 60-64 hiện tại:
agentVersion:  '2.1.0',
capabilities:  ['fs', 'git', 'preflight'] as const,
```

---

## 2. Solution — Exact Diff

```diff
// src/relay/agent-session.ts — sendHandshake() function

     params: {
-      agentVersion:  '2.1.0',
+      agentVersion:  '5.0.0',
       platform:      process.platform,
       arch:          process.arch,
       nodeVersion:   process.version,
-      capabilities:  ['fs', 'git', 'preflight'] as const,
+      capabilities:  ['fs', 'git', 'preflight', 'ai.providers', 'worktrees'] as const,
       ...(config.agentToken ? { agentToken: config.agentToken } : {}),
       devServerId:   config.devServerId,
       tools:         tools.map(t => t.name),
     },
```

---

## 3. Tests

Cập nhật existing test snapshot (nếu có) để match `agentVersion: '5.0.0'`.

---

## 4. Implementation Checklist

- [ ] `src/relay/agent-session.ts` line 60 — `agentVersion: '2.1.0'` → `'5.0.0'`
- [ ] `src/relay/agent-session.ts` line 64 — thêm `'ai.providers'`, `'worktrees'` vào capabilities array
