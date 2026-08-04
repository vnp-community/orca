# TASK-BE-013.2: Instrument `connectRelayWebSocket()` (relay-websocket mode, BL-AWS-01)

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-013](../solutions/SOL-BE-TRACE-013-agent-ws.md) §2.2
**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-013.1
**Status:** ✅ Done (2026-08-04) — wrapped `attempt()` closure in `connectRelayWebSocket()` with `Tracers.agentWsHandshakeFlow`; span created fresh per `attempt()` call (incl. reconnect via `setTimeout(attempt, delayMs)`), `step('tcpConnected')` on open, `fail()` on all 3 error branches (tcpConnect timeout, ws error, handshake reject), `ok()` with platform/nodeVersion/agentVersion. No drift from doc — reconnect/backoff logic (`calcBackoffDelay`, `_relayWsReconnectAttempt`, `_relayWsReconnectTimer`) untouched, `relayCallTracer` untouched. typecheck:node clean for this file.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "DevServerRelayBridge.connectRelayWebSocket"
```

Symbol đã tồn tại (MODIFY case) — private method này chứa logic reconnect/backoff quan trọng. Chạy:

```
gitnexus_impact({ target: "DevServerRelayBridge.connectRelayWebSocket", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa — xác nhận không đổi logic `calcBackoffDelay`/`_relayWsReconnectAttempt`/`_relayWsReconnectTimer`. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Bọc `attempt()` closure bên trong `connectRelayWebSocket()` (`src/main/dev-server/dev-server-relay-bridge.ts`, Backend là WS CLIENT chủ động connect ra Agent) bằng `Tracers.agentWsHandshakeFlow`. Mỗi lần gọi `attempt()` — kể cả reconnect qua `setTimeout(attempt, delayMs)` sau khi `close` event — phải tạo **1 span mới hoàn toàn độc lập**, không tái dùng `id` cũ, vì đây là điểm rẽ nhánh quan trọng theo CR-TRACE-000 §5 rule 3. Tracer `relayCallTracer` (`relay:agentCall`) đã tồn tại cho các RPC call SAU KHI đã connect — giữ nguyên, không đổi.

## File: `src/main/dev-server/dev-server-relay-bridge.ts` [MODIFY]

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
import { Tracers } from '../../shared/trace/tracers'
// relayCallTracer (relay:agentCall) đã import sẵn — giữ nguyên, không đổi

private connectRelayWebSocket(
  rawUrl: string,
  opts: { testOnly?: boolean }
): Promise<RelayHandshakeInfo> {
  const url = new URL(rawUrl)
  const token = url.searchParams.get('token') ?? ''
  url.searchParams.delete('token')
  const cleanUrl = url.toString()
  const orcaVersion = getPlatform().app.getVersion()
  this._relayWsActive = !opts.testOnly

  return new Promise<RelayHandshakeInfo>((resolve, reject) => {
    let initialResolved = false

    const attempt = () => {
      if (!this._relayWsActive) return

      // [NEW] mỗi lần attempt() (kể cả reconnect qua setTimeout(attempt, delayMs))
      // là 1 span mới — điểm rẽ nhánh quan trọng theo CR-TRACE-000 mục 5 rule 3.
      const span = Tracers.agentWsHandshakeFlow.start({ devServerId: this.config.id })

      const ws = new WebSocket(cleanUrl, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      ;(ws as unknown as { binaryType: string }).binaryType = 'nodebuffer'

      const connectionTimeout = setTimeout(() => {
        ws.terminate()
        const timeoutMsg =
          `relay-websocket: TCP connection timed out after 10s to ${cleanUrl}. ` +
          `Verify the agent is running and the address is reachable.`
        span.fail(timeoutMsg, { phase: 'tcpConnect', devServerId: this.config.id })
        if (!initialResolved) {
          reject(new Error(timeoutMsg))
        } else {
          console.warn(`[RelayBridge] ${timeoutMsg} Retry in 15s.`)
        }
      }, 10_000)

      ws.on('error', (err: Error) => {
        clearTimeout(connectionTimeout)
        span.fail(err, { phase: 'tcpConnect', devServerId: this.config.id })
        if (!initialResolved) {
          reject(new Error(
            `relay-websocket: WebSocket error connecting to ${cleanUrl}: ${err.message}`
          ))
        } else {
          console.warn(`[RelayBridge] relay-ws error: ${err.message}`)
        }
      })

      ws.on('open', () => {
        clearTimeout(connectionTimeout)
        span.step('tcpConnected', { devServerId: this.config.id })

        runOrcaInitiatorHandshake(ws, orcaVersion)
          .then((info) => {
            span.ok({ platform: info.platform, nodeVersion: info.nodeVersion, agentVersion: info.agentVersion })

            const transport = createWebSocketTransport(ws)
            this.session = new SshChannelMultiplexer(transport, {
              connectionLostMessage: 'Connection lost, reconnecting...'
            })

            if (opts.testOnly) {
              void this.disconnect()
            } else {
              ws.on('close', () => {
                if (this.session) {
                  console.log('[RelayBridge] relay-ws disconnected — clearing session')
                  this.session = null
                  this.onSessionDropped()
                }
                if (this._relayWsActive) {
                  const delayMs = calcBackoffDelay(this._relayWsReconnectAttempt++)
                  console.log(`[RelayBridge] relay-ws will reconnect in ${Math.round(delayMs / 1000)}s (attempt ${this._relayWsReconnectAttempt})...`)
                  // attempt() gọi lại → tạo span agentWsHandshakeFlow MỚI cho lần thử này
                  this._relayWsReconnectTimer = setTimeout(attempt, delayMs)
                }
              })
            }

            if (!initialResolved) {
              initialResolved = true
              resolve({
                platform: (info.platform as NodeJS.Platform) ?? 'linux',
                arch: info.arch,
                nodeVersion: info.nodeVersion,
                relayVersion: info.agentVersion,
              })
            } else {
              this._relayWsReconnectAttempt = 0
            }
          })
          .catch((err: Error) => {
            span.fail(err, { phase: 'handshake', devServerId: this.config.id })
            // ...existing retry/reject logic không đổi...
          })
      })
    }

    attempt()
  })
}
```

**Lưu ý quan trọng:** `connectionTimeout`/`ws.on('error')` đều có thể fire SAU `ws.on('open')` đã set `span` mới (do reconnect) — code trên dùng `span` trong closure của `attempt()`, mỗi lần gọi lại `attempt()` tạo `span` mới hoàn toàn độc lập nên không có race giữa các lần thử.

**Ràng buộc bắt buộc:**
- Không đổi logic reconnect/backoff (`calcBackoffDelay`, `_relayWsReconnectAttempt`, `_relayWsReconnectTimer`) — chỉ thêm tracer calls.
- Không đưa giá trị `token` (Authorization header) vào bất kỳ field nào của `TraceFields`.
- `relayCallTracer` (`relay:agentCall`) giữ nguyên, không sửa.

## Verification

```bash
pnpm run typecheck:node
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.agentWsHandshakeFlow` bọc đúng `connectRelayWebSocket()` — mỗi lần `attempt()` (kể cả reconnect) là 1 span mới, không tái dùng `id` cũ
- [ ] `span.step('tcpConnected')` được gọi khi `ws.on('open')` fire, trước khi `runOrcaInitiatorHandshake()` được gọi
- [ ] `span.fail()` được gọi ở cả 3 nhánh lỗi: timeout (`phase:'tcpConnect'`), `ws.on('error')` (`phase:'tcpConnect'`), handshake reject (`phase:'handshake'`)
- [ ] `span.ok()` chứa `platform`, `nodeVersion`, `agentVersion` khi handshake thành công
- [ ] Không có giá trị `token` nào trong `TraceFields` của `agentWsHandshakeFlow`
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
