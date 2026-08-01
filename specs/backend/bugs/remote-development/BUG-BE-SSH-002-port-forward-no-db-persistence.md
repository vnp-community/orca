# BUG-BE-SSH-002: `PortForwardManager` không persist `port_forwards` vào SQLite — mất data khi restart

**Status:** ✅ FIXED — 2026-08-01  
**Task:** BUG-BE-SSH-002  
**Note:** migration 0012: orca_port_forwards table added for restart-safe persistence  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-SSH-04) mô tả:
```
[PortForwardManager.onNewPort()]
    ├─ Tạo SSH local forward: localPort → remote:3000
    ├─ INSERT port_forwards { localPort, remotePort, pid, hostId }  ← SQLite
    ├─ Generate local URL: http://localhost:<localPort>
    └─ emit: portForward:created { localUrl, remotePort }
```

Nhưng `SshPortForwardManager` thực tế chỉ dùng **in-memory Map**:
```typescript
// ssh-port-forward.ts:19
private forwards = new Map<string, StartedPortForward>()
```

Grep toàn bộ `src/main/ssh/`:
```
INSERT.*port_forward   → No results
orca_port_forward      → No results
port_forwards.*db      → No results
```

## File liên quan

- [`src/main/ssh/ssh-port-forward.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/ssh/ssh-port-forward.ts) — Line 19: chỉ có in-memory Map

## Ảnh hưởng

1. Khi Orca restart → tất cả port forward mappings bị mất.
2. Không thể khôi phục active port forwards sau restart.
3. Admin không thể xem lịch sử port forward (chỉ thấy active).
4. Nếu relay reconnect → port forward ID có thể conflict (counter reset).

## Điểm đánh giá tích cực

Implementation thực tế có:
- `addForward()`, `updateForward()`, `removeForward()`, `removeAllForwards()`
- `PortForwardEntry` type với đủ fields
- Callback `onForwardClosed` khi forward disconnect

Chỉ thiếu persistence layer (SQLite).

## Liên quan đến luồng

- **BL-SSH-04**: Auto Port Forwarding — `INSERT port_forwards` missing.
