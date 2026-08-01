# TASK-TRM-003: Fix Agent WS port 6769 → 6768 trong DevServerRelayBridge

**Priority:** 🟠 HIGH — Agent không connect được nếu dùng default  
**Effort:** ~5 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-TM-004  
**Solution ref:** [SOLUTION-TRM-BE-exact.md](../solutions/SOLUTION-TRM-BE-exact.md)

---

## Mục tiêu

Sửa default port cho Agent WebSocket URL từ `6769` sang `6768` để align với TDD-11 (Orca HTTP server chạy trên port 6768, cả Browser và Agent đều kết nối vào cùng một server).

## File cần sửa

```
src/main/dev-server/dev-server-relay-bridge.ts
```

## Thay đổi cụ thể

### Lines 250–256 — Sửa default port:

```diff
       const orcaWsUrl =
         process.env['ORCA_AGENT_WS_URL'] ??
         (() => {
           const host = process.env['ORCA_ADVERTISED_HOST'] ?? 'localhost'
-          const port = process.env['ORCA_HTTP_PORT'] ?? '6769'
+          const port = process.env['ORCA_HTTP_PORT'] ?? '6768'
           return `ws://${host}:${port}${AGENT_WS_PATH}`
         })()
```

### agent-ws-server.ts — Sửa comment (lines 5–9):

```diff
 // Architecture:
-//   Browser  → ws://:6768/        (existing OrcaRuntimeRpcServer — unchanged)
-//   Agent    → ws://:6769/agent   (NEW — this file handles /agent path on HTTP server)
+//   Browser  → ws://:6768/        (OrcaRuntimeRpcServer — unchanged)
+//   Agent    → ws://:6768/agent   (NEW — same HTTP server, path /agent)
```

## Lý do

TDD-11 (web-server-mode.md) quy định Orca HTTP server chạy trên một port duy nhất, phân biệt Browser vs Agent qua path (`/` vs `/agent`). Comment cũ ghi 6769 cho Agent gây confusion và default fallback sai.

## Verification

```bash
# Verify không còn hard-coded 6769 (chỉ có trong env var reference):
grep -n "6769" src/main/dev-server/dev-server-relay-bridge.ts
grep -n "6769" src/main/dev-server/agent-ws-server.ts

# Expected: no results (6769 only allowed in env vars or comments explaining the change)

pnpm tsc --noEmit
```
