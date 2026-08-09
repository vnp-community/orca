# SOLUTION: BUG-BE-HLD-019 — Agent WebSocket Protocol: keepalive/close-code/version-mismatch — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế
**Files nguồn đã đọc:**
`backend/src/main/ssh/relay-protocol.ts`,
`backend/src/shared/agent-wire-protocol.ts`,
`backend/src/main/dev-server/ws-handshake.ts`,
`backend/src/main/dev-server/agent-ws-server.ts`,
`docs/features/F29-agent-websocket-protocol.md`,
`specs/backend/tdd/v5/05-ssh-relay.md`, `specs/backend/tdd/v5/04-rpc-server.md`.

---

## Kết luận trước khi vá

Đây là bug **chủ yếu do tài liệu sai lệch với code thật**, không phải lỗi runtime — code thật (5s/20s keepalive, close code 1005/1008) đã hoạt động ổn định và nhất quán ở 2 nơi (`relay-protocol.ts` và `agent-wire-protocol.ts` dùng CÙNG giá trị 5_000/20_000), nên **không sửa số liệu code, chỉ sửa doc cho khớp**. Riêng phần `AGENT_MIN_VERSION` là hằng số chết thật sự — đây là gap cần code thật, không phải doc.

> Lưu ý phụ: `specs/backend/tdd/v5/04-rpc-server.md:22` ghi `KEEPALIVE_INTERVAL_MS = 10_000` (10s) — khác cả doc F29 (30s) lẫn code thật (5s). TDD này không thuộc phạm vi bug ticket (chỉ trỏ tới F29), nêu ở đây để lưu ý riêng — không sửa trong solution này.

---

## Phần A — Sửa tài liệu `docs/features/F29-agent-websocket-protocol.md`

### A.1 — Close code sai (dòng 91, 95)

**File:** `docs/features/F29-agent-websocket-protocol.md`
**Lines:** 91, 95

Text sai hiện tại:
```markdown
// Validate khi kết nối
SHA-256(received) === storedHash ? OK : close(4001)
```
```markdown
- UI: `AgentTokenPanel` — hiển thị token 1 lần, copy-to-clipboard, regenerate
- Close codes: 4001=Unauthorized, 4002=HandshakeTimeout, 4003=VersionMismatch
```

Fix — thay bằng mã đóng WS chuẩn thật đang dùng (`ws-handshake.ts:200`: `1008` cho auth fail; `agent-ws-server.ts:156`: `1008` cho slot expired; timeout dùng mã mặc định `1005` vì `ws.close()` không truyền code — `ws-handshake.ts:141,158,166`):

```markdown
// Validate khi kết nối
SHA-256(received) === storedHash ? OK : close(1008, 'Authentication failed...')
```
```markdown
- UI: `AgentTokenPanel` — hiển thị token 1 lần, copy-to-clipboard, regenerate
- Close codes: `1008` (Policy Violation) = token sai/slot hết hạn, `1005` (No Status
  Received, mặc định của `ws.close()` không tham số) = handshake timeout. Không có
  mã tùy biến 4000-4999 — xem mục A.3 nếu cần version-mismatch dùng mã riêng.
```

### A.2 — Keepalive timing sai (dòng 122, 156, 157)

**File:** `docs/features/F29-agent-websocket-protocol.md`
**Lines:** 122, 156-157

Text sai hiện tại:
```markdown
- [x] KeepAlive 0x09 gửi mỗi 30s, timeout 90s
```
```markdown
| KeepAlive interval | 30s |
| KeepAlive timeout | 90s (3 missed) |
```

Fix — khớp `KEEPALIVE_SEND_MS = 5_000` / `TIMEOUT_MS = 20_000` (`relay-protocol.ts:24-25`, lặp lại y hệt ở `AGENT_KEEPALIVE_INTERVAL_MS` / `AGENT_TIMEOUT_MS` trong `agent-wire-protocol.ts:21-22`) — không có logic "3 lần miss", timeout kích hoạt ngay khi quá 20s không nhận **bất kỳ** frame nào (Regular hoặc KeepAlive), không riêng KeepAlive:

```markdown
- [x] KeepAlive 0x09 gửi mỗi 5s nếu không có frame Regular nào được gửi; ngắt kết
  nối nếu không nhận được frame nào (Regular hoặc KeepAlive) trong 20s liên tục
```
```markdown
| KeepAlive interval | 5s |
| KeepAlive timeout | 20s (không nhận bất kỳ frame nào — không phải "3 lần miss") |
```

Đồng thời làm rõ sự tồn tại của một cơ chế `ws.ping()` **riêng biệt**, không liên quan tới khung KeepAlive 0x09 ở tầng ứng dụng — đây là nguồn gốc gây nhầm lẫn khi viết doc ban đầu. Thêm đoạn chú thích mới ngay sau bảng Metrics (dòng 159, cuối file):

```markdown
---

## Ghi chú: hai cơ chế keepalive độc lập

Có **hai** lớp keepalive riêng biệt, không nên gộp chung:

1. **Application-level KeepAlive (0x09)** — khung 13-byte header với `TYPE=0x09`,
   gửi mỗi 5s nếu không có frame Regular nào vừa gửi; bên nhận coi mất kết nối nếu
   không nhận **bất kỳ** frame nào (Regular hoặc KeepAlive) trong 20s. Định nghĩa ở
   `src/main/ssh/relay-protocol.ts` (`KEEPALIVE_SEND_MS`, `TIMEOUT_MS`) và lặp lại
   giá trị ở `src/shared/agent-wire-protocol.ts` (`AGENT_KEEPALIVE_INTERVAL_MS`,
   `AGENT_TIMEOUT_MS`). Đây là cơ chế chính bảo vệ giao thức JSON-RPC framed.
2. **Transport-level WS ping** — `ws.ping()` gọi mỗi 30s trong
   `src/main/dev-server/agent-ws-server.ts:124-126`, chỉ nhằm giữ kết nối sống qua
   reverse proxy/load balancer (ALB, Cloudflare) hay đóng idle socket sau một
   khoảng lặng nhất định — không phải cơ chế phát hiện mất kết nối ở tầng ứng
   dụng và không có logic timeout tương ứng ở phía Orca.
```

### A.3 — `AGENT_MIN_VERSION` là hằng số chết (bổ sung mới, sau mục "Agent Token Management")

**File:** `docs/features/F29-agent-websocket-protocol.md`
**Vị trí:** chèn section mới sau dòng 96 (trước "### Language-agnostic SDK examples")

```markdown
---

### Version Compatibility Check

`AGENT_MIN_VERSION` (`src/shared/agent-wire-protocol.ts`) là version Agent tối
thiểu Orca chấp nhận. Được kiểm tra trong `runOrcaReceiverHandshake()`
(`src/main/dev-server/ws-handshake.ts`) ngay sau khi validate token, trước khi
trả `handshake-ok`:

```typescript
if (semverLt(agentVersion, AGENT_MIN_VERSION)) {
  ws.close(1008, `Agent version ${agentVersion} is below minimum ${AGENT_MIN_VERSION}`)
}
```

- Áp dụng cho cả `direct-websocket` (Agent gửi `agentVersion` trong
  `agent.handshake` params) và `relay-websocket` (Orca đọc `agentVersion` từ
  handshake result của Agent).
- Không dùng close code tùy biến (4000-4999) cho version mismatch — dùng `1008`
  (Policy Violation), cùng mã đã dùng cho auth failure, kèm message phân biệt rõ
  lý do trong `reason` string (client phân loại theo message, không theo code).
```

---

## Phần B — Implement version-mismatch check thật (`ws-handshake.ts`)

`AGENT_MIN_VERSION` khai báo tại `agent-wire-protocol.ts:31` nhưng không được tham chiếu ở đâu khác trong `backend/src` — biến hằng số chết này thành logic thật, chỉ áp dụng ở **receiver handshake** (`direct-websocket`, Orca là WS server, có toàn quyền đóng kết nối agent không tương thích ngay khi handshake). Không sửa `runOrcaInitiatorHandshake` (`relay-websocket`) vì ở chiều đó Orca là client kết nối vào WS server của Agent — Orca không phải bên "đóng" agent, chỉ nên log cảnh báo và để người dùng quyết định kết nối tiếp hay không (agent tự vận hành, không phải do Orca cấp phát).

### B.1 — Thêm hàm so sánh semver tối giản

**File:** `backend/src/shared/agent-wire-protocol.ts`
**Lines:** thêm sau dòng 92 (cuối file, sau `generateAgentToken`)

```typescript
// backend/src/shared/agent-wire-protocol.ts — thêm cuối file:

/**
 * Minimal semver "less than" comparison (major.minor.patch only — no
 * pre-release/build metadata support, agent versions don't use them).
 * Returns true if `version` is strictly older than `minVersion`.
 * Non-numeric or malformed segments are treated as 0 (fail open toward
 * "too old" rather than silently accepting garbage as compatible).
 */
export function isAgentVersionBelowMinimum(version: string, minVersion: string): boolean {
  const parse = (v: string): [number, number, number] => {
    const [maj, min, pat] = v.trim().split('.')
    return [Number(maj) || 0, Number(min) || 0, Number(pat) || 0]
  }
  const [aMaj, aMin, aPat] = parse(version)
  const [bMaj, bMin, bPat] = parse(minVersion)
  if (aMaj !== bMaj) return aMaj < bMaj
  if (aMin !== bMin) return aMin < bMin
  return aPat < bPat
}
```

### B.2 — Gọi check trong `runOrcaReceiverHandshake`

**File:** `backend/src/main/dev-server/ws-handshake.ts`
**Lines:** 174-204 (chèn thêm bước version-check giữa validate token thành công và gửi `handshake-ok`)

Code hiện tại (rút gọn, dòng 174-217):
```typescript
      clearTimeout(timer)
      const requestId = (msg as { id?: number }).id ?? 1
      const params = (msg as { params?: AgentHandshakeParams }).params

      // Validate auth token
      const agentToken = params?.agentToken ?? ''
      if (!validateToken(agentToken)) {
        // ... close(1008, 'Authentication failed...')
        return
      }

      // Send handshake-ok
      const sessionId = `sess-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
      // ...
```

Fix — chèn version check ngay sau khối validate token, trước khi build `sessionId`/gửi ok:

```typescript
// backend/src/main/dev-server/ws-handshake.ts — chèn sau khối validate token
// (ngay trước dòng "// Send handshake-ok"), trong runOrcaReceiverHandshake():

      // FIX BUG-BE-HLD-019: reject agents older than AGENT_MIN_VERSION before
      // wiring the multiplexer — an incompatible agent talking the wire
      // protocol can otherwise fail confusingly deep inside RPC dispatch
      // instead of at a clear, actionable handshake boundary.
      const agentVersion = params?.agentVersion ?? ''
      if (agentVersion && isAgentVersionBelowMinimum(agentVersion, AGENT_MIN_VERSION)) {
        outSeq++
        const versionErrFrame = encodeJsonRpcFrame(
          {
            jsonrpc: '2.0',
            id: requestId,
            error: {
              code: AgentErrorCode.HandshakeFailed,
              message:
                `Agent version ${agentVersion} is below the minimum supported ` +
                `version ${AGENT_MIN_VERSION}. Please update the Orca agent.`,
            },
          },
          outSeq,
          0
        )
        ws.send(versionErrFrame)
        // Why: reuse 1008 (Policy Violation) — same convention as the token-auth
        // rejection above — rather than inventing an unused custom code in the
        // 4000-4999 range. Clients distinguish the reason via the error message
        // sent above, not the close code.
        ws.close(1008, `Agent version ${agentVersion} below minimum ${AGENT_MIN_VERSION}`)
        reject(new Error(
          `Agent version ${agentVersion} is below minimum ${AGENT_MIN_VERSION}`
        ))
        return
      }

      // Send handshake-ok
      const sessionId = `sess-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
```

Cập nhật import đầu file (`ws-handshake.ts:19-24`):
```typescript
// backend/src/main/dev-server/ws-handshake.ts — sửa import dòng 19-24:
import type { AgentHandshakeParams } from '../../shared/agent-wire-protocol'
import {
  AGENT_HANDSHAKE_METHOD,
  AGENT_MIN_VERSION,
  AGENT_TIMEOUT_MS,
  AgentErrorCode,
  isAgentVersionBelowMinimum,
} from '../../shared/agent-wire-protocol'
```

### B.3 — Vì sao KHÔNG thêm version check vào `runOrcaInitiatorHandshake` (relay-websocket)

Trong `relay-websocket`, Orca chủ động kết nối tới WS server do Agent tự host — Orca không "sở hữu" kết nối theo cách có thể ép agent nâng cấp bằng cách đóng nó. Nếu muốn cảnh báo người dùng, làm ở tầng khác (ví dụ hiển thị banner trong UI Dev Server dựa trên `agentVersion` trả về trong `WsHandshakeInfo`), KHÔNG đóng kết nối ngầm ở `ws-handshake.ts` — quyết định business này cần xác nhận riêng, không phải phần bắt buộc của bug fix này.

```typescript
// TODO(BUG-BE-HLD-019, business decision cần xác nhận): nếu muốn cảnh báo (không
// chặn) agent cũ ở chiều relay-websocket, thêm kiểm tra không-chặn tại nơi gọi
// runOrcaInitiatorHandshake() (dev-server-relay-bridge.ts) dựa trên
// info.agentVersion, KHÔNG sửa ws-handshake.ts — Orca không phải bên có quyền
// đóng kết nối trong mode này.
```

---

## Tóm tắt thay đổi

| Hạng mục | File | Lines | Loại thay đổi |
|---|---|---|---|
| Close code doc | `docs/features/F29-agent-websocket-protocol.md` | 91, 95 | Sửa text: 4001-4003 → 1008/1005 |
| Keepalive timing doc | `docs/features/F29-agent-websocket-protocol.md` | 122, 156-157 | Sửa text: 30s/90s(3 missed) → 5s/20s |
| Ghi chú 2 lớp keepalive | `docs/features/F29-agent-websocket-protocol.md` | cuối file (sau 159) | Thêm section mới |
| Version check doc | `docs/features/F29-agent-websocket-protocol.md` | sau dòng 96 | Thêm section mới |
| `isAgentVersionBelowMinimum()` | `backend/src/shared/agent-wire-protocol.ts` | cuối file (sau 92) | Hàm mới |
| Version-mismatch check thật | `backend/src/main/dev-server/ws-handshake.ts` | chèn trước dòng 206 (trong `runOrcaReceiverHandshake`) | Logic mới, dùng close code `1008` có sẵn |
| Import bổ sung | `backend/src/main/dev-server/ws-handshake.ts` | 19-24 | Thêm `AGENT_MIN_VERSION`, `isAgentVersionBelowMinimum` |

**Không đổi (giữ nguyên vì đã đúng và nhất quán):** `KEEPALIVE_SEND_MS=5_000`, `TIMEOUT_MS=20_000` trong `relay-protocol.ts` và `agent-wire-protocol.ts`; close code `1008`/`1005` trong `ws-handshake.ts`/`agent-ws-server.ts`; `ws.ping()` 30s trong `agent-ws-server.ts:124-126`.
