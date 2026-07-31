# TASK-DS-011 — Sửa Keepalive Interval: 8s → 5s Trong agent.js

**Solution:** [SOL-DS-005 §3](../solutions/SOL-DS-005-daemon-hardening.md)  
**Bug:** [BUG-DS-008](../BUG-DS-008-keepalive-margin.md)  
**File:** `deploy/dev/agent/agent.js`  
**TDD Ref:** TDD-05 §6 — `KEEPALIVE_SEND_MS = 5_000` trong `relay-protocol.ts`  
**Phụ thuộc:** Không  
**Estimated:** 10 phút  
**Status:** ✅ DONE — 2026-07-27 (implemented in TASK-DS-001/002 batch)

---

## Mục Tiêu

Giảm keepalive interval từ 8s xuống 5s để align với `KEEPALIVE_SEND_MS = 5_000` của server (`relay-protocol.ts`). Tăng margin từ 12s lên 15s (so với `TIMEOUT_MS = 20_000`).

---

## Context

Đọc trước:
- `deploy/dev/agent/agent.js` — hàm `startKeepalive(ws, ms)` hoặc `setInterval` gửi PING
- `src/main/ssh/relay-protocol.ts` dòng 24-25 — `KEEPALIVE_SEND_MS = 5_000`, `TIMEOUT_MS = 20_000`

---

## Thay Đổi Cần Thực Hiện

**File:** `deploy/dev/agent/agent.js`

Tìm hàm `startKeepalive` (hoặc setInterval PING):

**TÌM:**
```javascript
function startKeepalive(ws, ms = 8000) {
```

**THAY BẰNG:**
```javascript
// Align với relay-protocol.ts: KEEPALIVE_SEND_MS = 5_000, TIMEOUT_MS = 20_000.
// Interval 5s → margin 15s (20 - 5 = 15s) trước khi server timeout.
function startKeepalive(ws, ms = 5000) {
```

Nếu không có hàm `startKeepalive` mà dùng `setInterval` trực tiếp:

**TÌM:**
```javascript
setInterval(() => {
  // ... gửi PING
}, 8000)
```

**THAY BẰNG:**
```javascript
setInterval(() => {
  // ... gửi PING
}, 5000)  // align với server KEEPALIVE_SEND_MS = 5_000
```

---

## Verify

```bash
# Deploy + restart agent
bash deploy/dev/scripts/connect-agent.sh --deploy

# Quan sát PING frequency trong logs:
bash deploy/dev/scripts/connect-agent.sh --logs | grep -E "PING|keepalive"
# Expected: PING xuất hiện mỗi ~5 giây (không phải ~8 giây)

# Đo thực tế:
ssh ubuntu@172.20.2.31 "tail -f ~/orca-agent/logs/agent-direct.log | grep PING" &
sleep 20 && kill %1
# Đếm số PING trong 20s: phải ~4 lần (5s interval) thay vì ~2-3 lần (8s interval)
```

---

## Definition of Done

- [x] `startKeepalive(ws, ms = 5000)` — default từ 8000 → 5000
- [x] Comment giải thích alignment với `KEEPALIVE_SEND_MS` từ `relay-protocol.ts`
- [x] agent.js syntax check OK (`node --check`)
- [x] Margin mới: 20s - 5s = 15s (tốt hơn margin 12s trước)
