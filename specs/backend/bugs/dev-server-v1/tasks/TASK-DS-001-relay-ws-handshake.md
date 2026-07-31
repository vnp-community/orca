# TASK-DS-001 — Fix relay-ws Handshake Token Validation

**Solution:** [SOL-DS-001](../solutions/SOL-DS-001-relay-ws-handshake-fix.md)  
**Bug:** [BUG-DS-001](../BUG-DS-001-relay-ws-handshake-token.md)  
**File:** `deploy/dev/agent/agent.js`  
**Estimated:** 15 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Bỏ re-validate `agentToken` trong handshake frame của relay-ws mode. Token đã được validate tại connection layer (`wss.on('connection')`). Orca không gửi `agentToken` trong handshake params → so sánh luôn fail → connection bị reject.

---

## Context

Đọc trước:
- `deploy/dev/agent/agent.js` — hàm `handleSession()`, nhánh `rpc.method === 'agent.handshake'`

---

## Thay Đổi Cần Thực Hiện

**File:** `deploy/dev/agent/agent.js`

Trong hàm `handleSession()`, tìm đoạn xử lý `agent.handshake` trong nhánh `if (rpc.method)`:

**TÌM** đoạn code này (relay-ws receiver path):
```javascript
if (rpc.method === 'agent.handshake') {
  // relay-ws mode: Orca sends us handshake → validate token + reply
  const incoming = rpc.params?.agentToken;
  if (incoming !== relayToken) {
    log.warn('Handshake rejected (bad token from Orca)');
    ws.close(1008, 'Unauthorized');
    return;
  }
  const okResp = {
```

**THAY BẰNG:**
```javascript
if (rpc.method === 'agent.handshake') {
  // relay-ws mode: Orca sends us handshake → reply OK.
  // Token đã được validate tại wss.on('connection') qua Authorization header
  // hoặc ?token= query param. Orca (initiator) KHÔNG gửi agentToken trong
  // handshake params — chỉ gửi orcaVersion. Không re-validate ở đây.
  const okResp = {
```

> [!IMPORTANT]
> Chỉ xóa 5 dòng token validation (từ `const incoming` đến `return;` + dấu `}`). Giữ nguyên phần tạo `okResp` và gọi `ws.send(...)`.

---

## Verify

```bash
# 1. Deploy agent mới lên dev server:
bash deploy/dev/scripts/connect-agent.sh --deploy

# 2. Start agent relay-ws:
bash deploy/dev/scripts/connect-agent.sh --mode relay-ws --start

# 3. Kiểm tra log — KHÔNG được thấy "Handshake rejected":
bash deploy/dev/scripts/connect-agent.sh --logs
# Expected: "Handshake OK [relay-ws]"

# 4. Trong Orca UI: Test Connection với relay-ws URL → phải "Connected" ✅
```

---

## Definition of Done

- [x] Đoạn `const incoming = rpc.params?.agentToken` đã bị xóa
- [x] Đoạn `if (incoming !== relayToken) { ... }` đã bị xóa
- [x] Comment mới giải thích lý do không validate
- [x] sessionId thêm random suffix để unique hơn
- [x] agent.js syntax check OK (`node --check`)
