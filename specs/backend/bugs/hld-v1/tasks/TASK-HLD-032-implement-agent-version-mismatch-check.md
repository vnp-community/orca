# TASK-HLD-032: Implement `isAgentVersionBelowMinimum()` + version-mismatch check trong `runOrcaReceiverHandshake`

**Priority:** 🟡 MEDIUM — `AGENT_MIN_VERSION` hiện là hằng số chết, Agent quá cũ có thể handshake thành công rồi fail mù mờ sâu trong RPC dispatch thay vì bị chặn rõ ràng ở handshake
**Effort:** ~40 phút
**Status:** ✅ DONE — 2026-08-09 (thêm `isAgentVersionBelowMinimum()` cuối `agent-wire-protocol.ts` đúng logic semver tối giản major.minor.patch. Chèn check vào `runOrcaReceiverHandshake()` (`ws-handshake.ts`) ngay sau khối validate token, trước "Send handshake-ok" — dùng đúng `AgentErrorCode.HandshakeFailed` cho error frame + `ws.close(1008, ...)` cùng convention với auth-fail phía trên. Cập nhật import đúng: `AGENT_MIN_VERSION`, `isAgentVersionBelowMinimum`. **Không** đụng vào `runOrcaInitiatorHandshake` — xác nhận qua `grep -rln isAgentVersionBelowMinimum backend/src/main/dev-server/*.ts` chỉ trả về `ws-handshake.ts`, đúng yêu cầu phạm vi (relay-websocket không sửa). `tsc --noEmit` sạch hoàn toàn cho cả 2 file. Cả 3 verification bash của task đều khớp. ⚠️ Chưa viết test case (bảng semver 5 case + integration handshake) — effort budget.)
**Bug refs:** BUG-BE-HLD-019
**Solution ref:** [SOLUTION-agent-ws-protocol-exact.md](../solutions/SOLUTION-agent-ws-protocol-exact.md) — Phần B (B.1, B.2, B.3)
**Depends on:** None (độc lập với TASK-HLD-031 — task đó chỉ sửa doc, task này sửa code)

---

## Mục tiêu

`AGENT_MIN_VERSION` được khai báo tại `agent-wire-protocol.ts:31` nhưng không được tham chiếu ở bất kỳ đâu khác trong `backend/src` — một Agent quá cũ hiện có thể handshake thành công và chỉ fail mù mờ sau này khi gọi RPC method không tương thích. Task này biến hằng số chết thành logic thật:

1. Thêm hàm so sánh semver tối giản `isAgentVersionBelowMinimum(version, minVersion)`.
2. Gọi check này trong `runOrcaReceiverHandshake()` (`ws-handshake.ts`), ngay sau khi validate token thành công, trước khi gửi `handshake-ok` — đóng kết nối bằng mã WS chuẩn `1008` (Policy Violation, cùng convention với auth-fail) nếu Agent quá cũ.

**Chỉ áp dụng cho `direct-websocket`** (`runOrcaReceiverHandshake`, Orca là WS server nhận kết nối từ Agent) — **KHÔNG sửa `runOrcaInitiatorHandshake`** (`relay-websocket`, Orca là client chủ động kết nối vào WS server do Agent tự host). Ở chiều `relay-websocket`, Orca không "sở hữu" kết nối theo cách có thể ép agent nâng cấp bằng cách đóng nó — nếu muốn cảnh báo người dùng ở chiều đó, cần làm ở tầng khác (UI banner dựa trên `WsHandshakeInfo.agentVersion`), đây là quyết định business riêng, không thuộc scope task này.

## File cần sửa/tạo

```
backend/src/shared/agent-wire-protocol.ts       (thêm hàm isAgentVersionBelowMinimum, cuối file)
backend/src/main/dev-server/ws-handshake.ts     (chèn version check trong runOrcaReceiverHandshake, sửa import)
```

## Thay đổi cụ thể

### 1. Thêm hàm so sánh semver — `backend/src/shared/agent-wire-protocol.ts` (thêm cuối file, sau `generateAgentToken`, sau dòng 92)

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

### 2. Gọi check trong `runOrcaReceiverHandshake` — `backend/src/main/dev-server/ws-handshake.ts` (chèn sau khối validate token thành công, trước dòng "Send handshake-ok", vùng dòng 174-204)

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

### 3. Cập nhật import — `backend/src/main/dev-server/ws-handshake.ts` (dòng 19-24)

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

### 4. KHÔNG sửa (ghi chú, không phải code cần viết trong task này)

`runOrcaInitiatorHandshake` (relay-websocket) — nếu sau này cần cảnh báo (không chặn) agent cũ ở chiều này, đó là task/quyết định business riêng:

```typescript
// TODO(BUG-BE-HLD-019, business decision cần xác nhận): nếu muốn cảnh báo (không
// chặn) agent cũ ở chiều relay-websocket, thêm kiểm tra không-chặn tại nơi gọi
// runOrcaInitiatorHandshake() (dev-server-relay-bridge.ts) dựa trên
// info.agentVersion, KHÔNG sửa ws-handshake.ts — Orca không phải bên có quyền
// đóng kết nối trong mode này.
```

## Verification

```bash
# 1. Hàm mới đã export đúng chữ ký
grep -n "export function isAgentVersionBelowMinimum" backend/src/shared/agent-wire-protocol.ts

# 2. ws-handshake.ts đã gọi check + import đúng
grep -n "isAgentVersionBelowMinimum\|AGENT_MIN_VERSION" backend/src/main/dev-server/ws-handshake.ts
# Expected: >= 2 dòng (import + lời gọi trong runOrcaReceiverHandshake)

# 3. Xác nhận KHÔNG có version check nào bị thêm vào runOrcaInitiatorHandshake
grep -n "isAgentVersionBelowMinimum" backend/src/main/dev-server/*.ts
# Expected: chỉ xuất hiện trong ws-handshake.ts, không trong file nào implement
# runOrcaInitiatorHandshake / dev-server-relay-bridge.ts

# 4. Type-check
pnpm --filter backend tsc --noEmit

# 5. Unit test cho isAgentVersionBelowMinimum (bảng test case):
#    isAgentVersionBelowMinimum('1.2.3', '1.3.0')  === true
#    isAgentVersionBelowMinimum('1.3.0', '1.3.0')  === false
#    isAgentVersionBelowMinimum('2.0.0', '1.3.0')  === false
#    isAgentVersionBelowMinimum('', '1.3.0')       === true   (malformed → treated as 0.0.0)
#    isAgentVersionBelowMinimum('1.3', '1.3.0')    === false  (missing patch → 0, khớp)

# 6. Integration test handshake:
#    - Agent gửi agentVersion thấp hơn AGENT_MIN_VERSION → kết nối bị đóng với code 1008,
#      message chứa "below the minimum supported version"
#    - Agent gửi agentVersion >= AGENT_MIN_VERSION → handshake-ok bình thường
#    - Agent KHÔNG gửi agentVersion (chuỗi rỗng/undefined) → không bị chặn (fail open cho
#      backward-compat với agent cũ chưa gửi field này ở params, nếu đó là hành vi mong đợi
#      — xác nhận lại với TDD nếu có yêu cầu khác)
pnpm --filter backend test -- ws-handshake
```
