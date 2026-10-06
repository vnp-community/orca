# AG-REQ-TASK-033-12: Handshake có `protocolVersion`, `buildVersion`, `features`

**From Solution:** [AG-REQ-SOL-033-capability-report-handshake-and-ai-complete](../solutions/AG-REQ-SOL-033-capability-report-handshake-and-ai-complete.md) mục 2.3 và 3.5
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/agent-protocol-features.ts` (mới), `agent/src/relay/agent-protocol-features.test.ts` (mới), `agent/src/relay/agent-session-handshake.ts` (sửa), `agent/src/relay/__tests__/agent-session.test.ts` (sửa)
**Depends on:** [04](./AG-REQ-TASK-033-04-wire-readonly-and-workspace-into-handlers.md), [08](./AG-REQ-TASK-033-08-wire-result-block-and-changes-into-handlers.md), [10](./AG-REQ-TASK-033-10-capabilities-rpc-dispatch.md), [11](./AG-REQ-TASK-033-11-ai-complete-usage-maxtokens-error-data.md), và `agent-build-version.ts` của [09](./AG-REQ-TASK-033-09-capability-report-core.md)
**Status:** [ ] TODO

## Context

CR-033 mục 2.6: handshake thêm `protocolVersion` (số nguyên; `1` khi vắng, `2` là CR này), `buildVersion` (bằng `AGENT_VERSION`) và `features` (mảng chuỗi tĩnh). Không đổi `agentVersion: '5.0.0'` (`ResumeAgentSession` so bằng nhau và `MinAgentVersion` dùng nó; đã đọc `agentwsserver/server.go:201`, `isBelowMinimumVersion(params.AgentVersion, s.Cfg.MinAgentVersion)`) và không đổi mảng `capabilities` (cổng `ptyReady` ở server đòi `pty` và `pty.stream`, xem chú thích `STATIC_CAPABILITIES_FALLBACK`). `features` được tách khỏi `capabilities` để các kiểm tra hiện có không bị ảnh hưởng.

Đã đọc `agent-session-handshake.ts`: `sendHandshake` dựng `rpc.params = { agentVersion: '5.0.0', platform, arch, nodeVersion, capabilities, ...(token), devServerId, tools }` rồi `ws.send(encodeDataFrame(...))`; việc xây `capabilities` có cuộc đua 5 giây, còn `features` là tĩnh nên không thêm độ trễ. Cả ba chế độ (`agent-connection-direct.ts`, `-relay.ts`, `-stdio.ts`) tạo phiên qua `createSession` nên đi qua cùng hàm này. Phía Go: `agentwsserver.inboundHandshakeParams` và `devserveragent.HandshakeInfo` dùng `json.Unmarshal` thường, trường lạ vô hại (chưa thấy `DisallowUnknownFields`); bản sao `usecase.HandshakeInfo` bỏ `Capabilities` và sẽ cần `Features`, `ProtocolVersion`, `BuildVersion` (việc của backend).

Nguyên tắc: KHÔNG quảng cáo tính năng chưa có. Vì vậy task này phụ thuộc 04, 08, 10, 11 và có test đối chiếu từng tên `features` với mã thật.

## Việc cần làm

1. Tạo `agent-protocol-features.ts`:
   ```ts
   export const AGENT_PROTOCOL_VERSION = 2
   export const AGENT_FEATURES = [
     'agent.execPrompt', 'agent.execPrompt.readonly', 'agent.execPrompt.workspaceKind',
     'agent.execPrompt.changes', 'agent.execPrompt.resultBlock',
     'agent.capabilities', 'ai.complete', 'ai.complete.usage'
   ] as const
   export type AgentFeature = (typeof AGENT_FEATURES)[number]
   ```
   Thêm chú thích ngắn: `ai.complete.usage` hứa đồng thời `usage`/`provider`/`latencyMs`, `maxTokens` và `error.data` (một tên cho ba thay đổi theo CR); `agent.execPrompt.readonly` hứa cả `applied` echo (task 04); `agent.execPrompt.resultBlock` hứa `parsed`; `agent.execPrompt.changes` hứa `changes`.
2. Nếu task 09 đã khai báo `AGENT_PROTOCOL_VERSION` tối thiểu ở cùng file thì chỉ bổ sung `AGENT_FEATURES` tại đây; đừng khai báo hai lần.
3. Trong `sendHandshake` thêm vào `params` (không bỏ và không đổi trường nào có sẵn): `protocolVersion: AGENT_PROTOCOL_VERSION`, `buildVersion: AGENT_BUILD_VERSION`, `features: [...AGENT_FEATURES]`. Không đụng `agentVersion: '5.0.0'`, `capabilities`, `tools`.
4. Cập nhật chú thích đầu `sendHandshake`: một dòng nêu vì sao `features` tách khỏi `capabilities` và vì sao `agentVersion` không đổi.
5. Cập nhật tài liệu: thêm vào `specs/agent/tdd/v5/04-handshake-session.md`? KHÔNG sửa TDD trong task này (TDD do người điều phối quản); ghi vào "Rủi ro" việc cần cập nhật TDD sau khi triển khai.

## Kiểm thử

`agent-protocol-features.test.ts`:
- `has no duplicate feature names and all match ^[a-z][A-Za-z0-9.]*$`.
- `protocolVersion is 2`.
- `every feature has a corresponding implementation` (khẳng định bằng kiểm cấu trúc, không chạy: `agent.capabilities` có `case` trong `dispatchMiscRpc` bằng cách gọi nó với `MockWs` và khẳng định không trả `MethodNotFound`; `agent.execPrompt` tương tự qua `dispatchAgentExecRpc`; `ai.complete` qua `dispatchAiRpc` với prompt rỗng cho `InvalidParams` chứ không `null`; ba tên con của `execPrompt` kiểm bằng cách gọi `parseExecPromptOptions` với tham số tương ứng và khẳng định chấp nhận). Mục đích: không quảng cáo thứ chưa làm.

Thêm vào `__tests__/agent-session.test.ts` (đã có `waitForHandshake`, `createTestSession`, `MOCK_CAPS`, nhóm kiểm tham số handshake `handshake params include ...`):
- `handshake params include protocolVersion 2`.
- `handshake params include buildVersion as a non-empty string` (khi chạy test là `0.0.0-dev` vì `__AGENT_VERSION__` không được define).
- `handshake params include features equal to AGENT_FEATURES`.
- `handshake still sends agentVersion 5.0.0 and the unchanged capabilities array` (hồi quy: `capabilities` bằng `MOCK_CAPS`).
- `handshake still omits agentToken when empty` (hồi quy đã có, giữ).
- `a server that ignores unknown handshake params still completes the handshake` (không cần test Go ở đây; ghi chú trỏ test phía Go).

Lệnh (trong `/opt/repos/orca/agent`): `pnpm exec vitest run src/relay/agent-protocol-features.test.ts src/relay/__tests__/agent-session.test.ts`; rồi `pnpm test`.

Phía backend (ghi để phối hợp): `go test ./services/infra-fleet-service/internal/adapter/agentwsserver/... ./services/infra-fleet-service/internal/adapter/devserveragent/...` chạy ở `/opt/repos/orca/backend-go` với một handshake mẫu có ba trường mới, khẳng định bắt tay thành công kể cả khi `inboundHandshakeParams` chưa khai báo trường mới.

## Tiêu chí hoàn thành

- [ ] Handshake có `protocolVersion: 2`, `buildVersion`, `features`; các trường cũ không đổi.
- [ ] Không có tên trong `features` mà mã tương ứng chưa tồn tại (test đối chiếu).
- [ ] Tất cả test handshake hiện có xanh không sửa.
- [ ] Client Go cũ vẫn bắt tay thành công (xác nhận phía backend, ghi vào README tasks).
- [ ] Không đổi `STATIC_CAPABILITIES_FALLBACK`.

## Rủi ro và lưu ý

- Chưa kiểm chứng cách các trường mới tới Go ở relay-websocket và relay-ssh: Go `runInitiatorHandshake` (`session.go:267`) gửi request rồi đọc phản hồi, trong khi code `agent/` chỉ có phía agent khởi xướng gửi `agent.handshake`. Chạy thử từng chế độ trên dev server thật; nếu một chế độ không mang được `features`, hồ sơ chế độ đó rơi về `handshake_only` dù agent mới, và backend cần một đường lấy `features` (ví dụ gọi `agent.capabilities` luôn, vì `agent.capabilities` trả `agent.protocolVersion`).
- `features` là cam kết: một khi quảng cáo, backend dựa vào để chọn đường chạy (CR-REQ-008 `agent_readonly`). Gỡ một feature cần tăng `protocolVersion`.
- Backend cũ gọi agent mới: bỏ qua trường mới. Backend mới gọi agent cũ: không có `features`, coi `protocolVersion` là 1 và dùng đường degradation (README solutions, bảng hành vi).
- TDD `specs/agent/tdd/v5/04-handshake-session.md` (mục 1 và 7) cần được cập nhật bởi người điều phối sau khi triển khai; task này không sửa TDD.

## Không làm trong task này

- Không đổi `agentVersion: '5.0.0'` và không đổi mảng `capabilities` hay `STATIC_CAPABILITIES_FALLBACK`.
- Không sửa TDD (`specs/agent/tdd/v5/04-handshake-session.md`): người điều phối cập nhật sau.
- Không thêm trường nào khác vào handshake (ví dụ tài nguyên máy: đã có `agent.capabilities`).
