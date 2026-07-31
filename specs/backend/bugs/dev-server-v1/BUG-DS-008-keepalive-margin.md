# BUG-DS-008 — Keepalive Interval 8s vs Server Timeout 20s — Margin Mỏng

**ID:** BUG-DS-008  
**Mức độ:** 🟡 Low  
**Module:** `agent.js` — keepalive / `SshChannelMultiplexer` — timeout  
**Phát hiện:** 2026-07-26  
**Status:** 🔴 Open

---

## Mô Tả

Agent gửi keepalive frame mỗi 8s. `SshChannelMultiplexer` trên server timeout connection nếu không nhận ACK progress trong 20s (TIMEOUT_MS). Margin 12s có thể không đủ trên high-latency connections (VPN, cloud dev server, congested network).

---

## Số Liệu

```
Agent keepalive interval:      8s  (deploy/dev/agent/agent.js startKeepalive)
Server TIMEOUT_MS:            20s  (src/main/ssh/relay-protocol.ts hoặc ssh-channel-multiplexer.ts)
Margin:                       12s
```

**Kịch bản timeout**:
```
T=0:  Agent gửi keepalive frame (ACK=N)
T=8:  Agent gửi keepalive frame (ACK=N+1)
T=8+latency: Server nhận frame, ACK progress OK
       → TIMEOUT_MS timer reset
...
T=16: Agent gửi keepalive
T=16+latency: Server nhận
      Nếu latency > 4s tại T=16 → server chưa nhận trong 20s từ T=0
      → TIMEOUT_MS expires → connection closed
```

Với LAN (<1ms latency): không có vấn đề.  
Với VPN/cloud (50-200ms latency): edge case, rất hiếm gặp.  
Với network congestion (>4s latency): có thể timeout.

---

## Root Cause

Keepalive interval được set trong user's edit:
```javascript
// agent.js
function startKeepalive(ws, ms = 8000) { ... }
```

Server timeout defined in:
```typescript
// relay-protocol.ts hoặc ssh-channel-multiplexer.ts
const TIMEOUT_MS = 20_000  // 20s
```

---

## Hậu Quả

- Trên LAN (172.20.2.31 ↔ 172.20.2.39): **không ảnh hưởng**
- Trên cloud dev server qua internet: tiềm ẩn disconnect ngẫu nhiên
- Connection drop sẽ trigger exit(2) → systemd restart → fresh token → reconnect

Hậu quả nhẹ vì có systemd restart. Nhưng mỗi lần disconnect = ~10-30s downtime.

---

## Fix

**Phương án A — Giảm keepalive interval xuống 5s**:

```javascript
function startKeepalive(ws, ms = 5000) { ... }
// Margin: 20 - 5 = 15s — tốt hơn
```

**Phương án B — Tăng server TIMEOUT_MS**:

```typescript
// relay-protocol.ts:
export const TIMEOUT_MS = 60_000  // 60s thay vì 20s
// Keepalive 8s → 7 frames trong 60s → rất an toàn
```

**Phương án C (recommended)** — Kết hợp:
- Agent keepalive: 5s
- Server TIMEOUT_MS: 60s  
- Margin: 55s — hoạt động tốt ngay cả trên high-latency connections

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `deploy/dev/agent/agent.js` | `startKeepalive(ws, ms=8000)` |
| `src/main/ssh/relay-protocol.ts` | `TIMEOUT_MS` hoặc equivalent |
| `src/main/ssh/ssh-channel-multiplexer.ts` | ACK timeout logic |
